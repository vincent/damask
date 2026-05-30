package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/jobs"
	"damask/server/internal/media/ingest"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/storage"
	apptelemetry "damask/server/internal/telemetry"
	"damask/server/internal/transform"
	"damask/server/internal/versioning"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

const maxCommentLength = 500

// VersionDTO is the output of VersionService methods.
type VersionDTO struct {
	ID           string
	AssetID      string
	WorkspaceID  string
	VersionNum   int64
	StorageKey   string
	ContentHash  string
	MimeType     string
	Size         int64
	Width        *int64
	Height       *int64
	DurationSec  *float64
	ThumbnailKey *string
	Comment      *string
	CreatedBy    *string
	CreatedAt    time.Time
	IsCurrent    bool
	DeletedAt    *string
}

type versionService struct {
	versions   repository.VersionRepository
	assets     repository.AssetRepository
	storage    storage.Storage
	queue      queue.JobQueue
	media      *ingest.Registry
	audit      audit.Writer
	triggers   WorkflowTriggerPublisher
	invalidate StorageInvalidator
}

type VersionServiceDeps struct {
	Assets     repository.AssetRepository
	Storage    storage.Storage
	Queue      queue.JobQueue
	Media      *ingest.Registry
	Triggers   WorkflowTriggerPublisher
	Invalidate StorageInvalidator
}

// NewVersionService returns a VersionService.
func NewVersionService(
	versions repository.VersionRepository,
	aw audit.Writer,
	deps ...VersionServiceDeps,
) VersionService {
	cfg := VersionServiceDeps{}
	if len(deps) > 0 {
		cfg = deps[0]
	}
	return &versionService{
		versions:   versions,
		assets:     cfg.Assets,
		storage:    cfg.Storage,
		queue:      cfg.Queue,
		media:      cfg.Media,
		audit:      aw,
		triggers:   workflowTriggerPublisherOrNop(cfg.Triggers),
		invalidate: cfg.Invalidate,
	}
}

func (s *versionService) List(ctx context.Context, assetID string) ([]*VersionDTO, error) {
	rows, err := s.versions.ListByAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]*VersionDTO, len(rows))
	for i, r := range rows {
		out[i] = toVersionDTO(r)
	}
	return out, nil
}

func (s *versionService) Get(ctx context.Context, workspaceID, id string) (*VersionDTO, error) {
	v, err := s.versions.GetByIDForWorkspace(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	return toVersionDTO(v), nil
}

func (s *versionService) GetCurrentByAsset(ctx context.Context, assetID string) (*VersionDTO, error) {
	v, err := s.versions.GetCurrentByAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	return toVersionDTO(v), nil
}

func (s *versionService) GetFirstByAsset(ctx context.Context, assetID string) (*VersionDTO, error) {
	v, err := s.versions.GetFirstByAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	return toVersionDTO(v), nil
}

func (s *versionService) ListWithVariantCount(ctx context.Context, assetID string) ([]*VersionWithCountDTO, error) {
	rows, err := s.versions.ListWithVariantCount(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]*VersionWithCountDTO, len(rows))
	for i, r := range rows {
		out[i] = &VersionWithCountDTO{
			VersionDTO:   *toVersionDTO(r.AssetVersion),
			VariantCount: r.VariantCount,
		}
	}
	return out, nil
}

func (s *versionService) GetByHash(ctx context.Context, assetID, contentHash string) (*VersionDTO, error) {
	v, err := s.versions.GetByHash(ctx, assetID, contentHash)
	if err != nil {
		return nil, err
	}
	return toVersionDTO(v), nil
}

func (s *versionService) NextVersionNum(ctx context.Context, assetID string) (int64, error) {
	return s.versions.NextVersionNum(ctx, assetID)
}

func (s *versionService) Create(ctx context.Context, v *VersionDTO) (out *VersionDTO, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.versions.create",
		attribute.String("damask.workspace_id", v.WorkspaceID),
		attribute.String("damask.asset_id", v.AssetID),
		attribute.Int64("damask.version.number", v.VersionNum),
		attribute.Int64("damask.version.size", v.Size),
	)
	defer func() {
		if out != nil {
			span.SetAttributes(attribute.String("damask.version_id", out.ID))
		}
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"version create failed",
				"workspace_id",
				v.WorkspaceID,
				"asset_id",
				v.AssetID,
				"error",
				err,
			)
		}
	}()

	created, err := s.versions.Create(ctx, repository.AssetVersion{
		ID:          v.ID,
		AssetID:     v.AssetID,
		WorkspaceID: v.WorkspaceID,
		VersionNum:  v.VersionNum,
		StorageKey:  v.StorageKey,
		ContentHash: v.ContentHash,
		MimeType:    v.MimeType,
		Size:        v.Size,
		Width:       v.Width,
		Height:      v.Height,
		DurationSec: v.DurationSec,
		Comment:     v.Comment,
		CreatedBy:   v.CreatedBy,
	})
	if err != nil {
		return nil, err
	}
	return toVersionDTO(created), nil
}

func (s *versionService) UploadNewVersion(
	ctx context.Context,
	p UploadAssetVersionParams,
) (out *UploadAssetVersionResult, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.versions.upload_new",
		attribute.String("damask.workspace_id", p.WorkspaceID),
		attribute.String("damask.asset_id", p.AssetID),
		attribute.String("damask.filename", p.Filename),
	)
	defer func() {
		if out != nil && out.Version != nil {
			span.SetAttributes(attribute.String("damask.version_id", out.Version.ID))
		}
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"version upload failed",
				"workspace_id",
				p.WorkspaceID,
				"asset_id",
				p.AssetID,
				"filename",
				p.Filename,
				"error",
				err,
			)
		}
	}()

	if err := s.validateUploadNewVersionDeps(); err != nil {
		return nil, err
	}
	if p.WorkspaceID == "" || p.AssetID == "" || p.Filename == "" || p.UserID == "" || p.Reader == nil {
		return nil, fmt.Errorf(
			"workspace_id, asset_id, filename, user_id, and reader are required: %w",
			apperr.ErrInvalidInput,
		)
	}

	comment := strings.TrimSpace(p.Comment)
	if len(comment) > maxCommentLength {
		return nil, fmt.Errorf("comment must be 500 characters or fewer: %w", apperr.ErrInvalidInput)
	}

	asset, err := s.assets.GetByID(ctx, p.WorkspaceID, p.AssetID)
	if err != nil {
		return nil, err
	}

	tmpF, err := os.CreateTemp("", "damask-version-*"+filepath.Ext(p.Filename))
	if err != nil {
		return nil, fmt.Errorf("cannot create temp file: %w", err)
	}
	tmpPath := tmpF.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpF, p.Reader); err != nil {
		_ = tmpF.Close()
		return nil, fmt.Errorf("cannot write temp file: %w", err)
	}
	if err := tmpF.Close(); err != nil {
		return nil, fmt.Errorf("cannot close temp file: %w", err)
	}

	hashFile, err := os.Open(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("could not open uploaded file: %w", err)
	}
	hash, size, err := versioning.HashReader(hashFile)
	_ = hashFile.Close()
	if err != nil {
		return nil, fmt.Errorf("could not hash file: %w", err)
	}

	var prevVersionID string
	if asset.CurrentVersionID != nil {
		prevVersionID = *asset.CurrentVersionID
	}

	existing, hashErr := s.versions.GetByHash(ctx, p.AssetID, hash)
	if hashErr == nil && existing.IsCurrent {
		return nil, fmt.Errorf("this file is identical to the current version: %w", apperr.ErrConflict)
	}

	nextNum, err := s.versions.NextVersionNum(ctx, p.AssetID)
	if err != nil {
		return nil, fmt.Errorf("could not determine version number: %w", err)
	}

	mimeType, _ := transform.DetectMimeType(tmpPath)
	if mimeType == "" {
		mimeType = p.ContentType
	}
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(p.Filename))
	}

	meta := ingest.FileMeta{}
	if s.media != nil {
		if extracted, metaErr := s.media.ExtractMeta(ctx, tmpPath, mimeType); metaErr != nil {
			slog.WarnContext(
				ctx,
				"version metadata extraction failed",
				"asset_id",
				p.AssetID,
				"mime_type",
				mimeType,
				"error",
				metaErr,
			)
		} else {
			meta = extracted
		}
	}

	storageKey := fmt.Sprintf("%s/%s/v%d/%s", p.WorkspaceID, p.AssetID, nextNum, p.Filename)
	if hashErr != nil {
		storeFile, err := os.Open(tmpPath)
		if err != nil {
			return nil, fmt.Errorf("could not reopen uploaded file: %w", err)
		}
		err = s.storage.Put(storageKey, storeFile)
		_ = storeFile.Close()
		if err != nil {
			return nil, fmt.Errorf("could not store file: %w", err)
		}
	} else {
		storageKey = existing.StorageKey
	}

	var commentPtr *string
	if comment != "" {
		commentPtr = &comment
	}
	createdBy := p.UserID

	newVersion, err := s.Create(ctx, &VersionDTO{
		ID:          uuid.NewString(),
		AssetID:     p.AssetID,
		WorkspaceID: p.WorkspaceID,
		VersionNum:  nextNum,
		StorageKey:  storageKey,
		ContentHash: hash,
		MimeType:    mimeType,
		Size:        size,
		Width:       meta.Width,
		Height:      meta.Height,
		DurationSec: meta.DurationSec,
		Comment:     commentPtr,
		CreatedBy:   &createdBy,
	})
	if err != nil {
		return nil, fmt.Errorf("could not create version: %w", err)
	}

	if err := s.SetCurrent(ctx, p.AssetID, newVersion.ID); err != nil {
		return nil, fmt.Errorf("could not promote version: %w", err)
	}
	newVersion.IsCurrent = true

	if err := s.SetAssetThumbnail(ctx, p.AssetID, nil); err != nil {
		slog.ErrorContext(
			ctx,
			"clear asset thumbnail",
			"asset_id",
			p.AssetID,
			"version_id",
			newVersion.ID,
			"error",
			err,
		)
	}

	s.enqueueVersionThumbnail(ctx, asset, newVersion)

	if strings.HasPrefix(mimeType, "audio/") || strings.HasPrefix(mimeType, "video/") {
		payload, _ := json.Marshal(jobs.ExtractMediaTagsPayload{
			AssetID:     p.AssetID,
			WorkspaceID: p.WorkspaceID,
		})
		if _, err := s.queue.Enqueue(ctx, p.WorkspaceID, queue.JobTypeExtractMediaTags, string(payload)); err != nil {
			slog.ErrorContext(
				ctx,
				"enqueue extract_media_tags",
				"asset_id",
				p.AssetID,
				"version_id",
				newVersion.ID,
				"error",
				err,
			)
		}
	}

	// todo: remove me ?
	if err := jobs.EnqueueRebuildVariantsJob(
		ctx,
		s.queue,
		p.WorkspaceID,
		p.AssetID,
		newVersion.ID,
		prevVersionID,
	); err != nil {
		slog.ErrorContext(
			ctx,
			"enqueue rebuild variants",
			"asset_id",
			p.AssetID,
			"version_id",
			newVersion.ID,
			"error",
			err,
		)
	}

	updatedAsset, err := s.assets.GetByID(ctx, p.WorkspaceID, p.AssetID)
	if err != nil {
		return nil, fmt.Errorf("could not reload asset: %w", err)
	}

	s.WriteVersionUploaded(ctx, p.WorkspaceID, p.AssetID, newVersion, comment)
	publishWorkflowTriggerAsync(ctx, s.triggers, "trigger.version_uploaded", map[string]any{
		"asset_id":          updatedAsset.ID,
		"workspace_id":      updatedAsset.WorkspaceID,
		"project_id":        ptrStr(updatedAsset.ProjectID),
		"folder_id":         ptrStr(updatedAsset.FolderID),
		"mime_type":         newVersion.MimeType,
		"size":              newVersion.Size,
		"original_filename": updatedAsset.OriginalFilename,
		"filename":          updatedAsset.OriginalFilename,
		"version_id":        newVersion.ID,
		"version_num":       newVersion.VersionNum,
		"storage_key":       newVersion.StorageKey,
	})

	if s.invalidate != nil {
		s.invalidate.Invalidate(p.WorkspaceID)
	}
	return &UploadAssetVersionResult{
		Asset:   toAssetDTO(updatedAsset),
		Version: newVersion,
	}, nil
}

func (s *versionService) validateUploadNewVersionDeps() error {
	if s.assets == nil {
		return errors.New("version upload requires asset repository (misconfigured service)")
	}
	if s.storage == nil {
		return errors.New("version upload requires storage (misconfigured service)")
	}
	if s.queue == nil {
		return errors.New("version upload requires queue (misconfigured service)")
	}
	return nil
}

func (s *versionService) enqueueVersionThumbnail(ctx context.Context, asset repository.Asset, version *VersionDTO) {
	payload := jobs.VersionThumbnailJobPayload{
		AssetID:     asset.ID,
		VersionID:   version.ID,
		WorkspaceID: asset.WorkspaceID,
		StorageKey:  version.StorageKey,
		MimeType:    version.MimeType,
	}
	if err := jobs.EnqueueVersionThumbnailJob(ctx, s.queue, asset.WorkspaceID, payload); err != nil {
		slog.ErrorContext(
			ctx,
			"enqueue version thumbnail",
			"asset_id",
			asset.ID,
			"version_id",
			version.ID,
			"error",
			err,
		)
	}
}

func (s *versionService) SetCurrent(ctx context.Context, assetID, versionID string) (err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.versions.set_current",
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.version_id", versionID),
	)
	defer func() { apptelemetry.EndSpan(span, err) }()
	return s.versions.SetCurrent(ctx, assetID, versionID)
}

func (s *versionService) SetAssetThumbnail(ctx context.Context, assetID string, key *string) error {
	return s.versions.SetAssetThumbnail(ctx, assetID, key)
}

// Delete soft-deletes a non-current version that is not in use as a cover.
func (s *versionService) Delete(ctx context.Context, workspaceID, assetID, versionID string) (err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.versions.delete",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.version_id", versionID),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"version delete failed",
				"workspace_id",
				workspaceID,
				"asset_id",
				assetID,
				"version_id",
				versionID,
				"error",
				err,
			)
		}
	}()

	v, err := s.versions.GetByIDForWorkspace(ctx, workspaceID, versionID)
	if err != nil {
		return err
	}
	if v.AssetID != assetID {
		return fmt.Errorf("version %q: %w", versionID, apperr.ErrNotFound)
	}
	if v.IsCurrent {
		return fmt.Errorf("cannot delete the current version: %w", apperr.ErrInvalidInput)
	}
	isCover, err := s.versions.IsReferencedAsCover(ctx, versionID)
	if err != nil {
		return err
	}
	if isCover {
		return fmt.Errorf("version is in use as a project cover or workspace icon: %w", apperr.ErrConflict)
	}
	if err := s.versions.SoftDelete(ctx, versionID); err != nil {
		return err
	}
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetVersionDeleted,
		Payload:     audit.AssetVersionDeletedPayload{V: 1, VersionNum: v.VersionNum},
	})
	if s.invalidate != nil {
		s.invalidate.Invalidate(workspaceID)
	}
	return nil
}

// WriteVersionUploaded emits an asset_version_uploaded audit event.
// Called by handlers that orchestrate the multi-step upload flow.
func (s *versionService) WriteVersionUploaded(
	ctx context.Context,
	workspaceID, assetID string,
	v *VersionDTO,
	comment string,
) {
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetVersionUploaded,
		Payload:     audit.AssetVersionUploadedPayload{V: 1, VersionNum: v.VersionNum, Size: v.Size, Comment: comment},
	})
}

// WriteVersionRestored emits an asset_version_restored audit event.
// Called by handlers after SetCurrent succeeds.
func (s *versionService) WriteVersionRestored(
	ctx context.Context,
	workspaceID, assetID string,
	fromVersionNum, toVersionNum int64,
) {
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetVersionRestored,
		Payload: audit.AssetVersionRestoredPayload{
			V:              1,
			FromVersionNum: fromVersionNum,
			ToVersionNum:   toVersionNum,
		},
	})
}

func toVersionDTO(v repository.AssetVersion) *VersionDTO {
	return &VersionDTO{
		ID:           v.ID,
		AssetID:      v.AssetID,
		WorkspaceID:  v.WorkspaceID,
		VersionNum:   v.VersionNum,
		StorageKey:   v.StorageKey,
		ContentHash:  v.ContentHash,
		MimeType:     v.MimeType,
		Size:         v.Size,
		Width:        v.Width,
		Height:       v.Height,
		DurationSec:  v.DurationSec,
		ThumbnailKey: v.ThumbnailKey,
		Comment:      v.Comment,
		CreatedBy:    v.CreatedBy,
		CreatedAt:    v.CreatedAt,
		IsCurrent:    v.IsCurrent,
		DeletedAt:    v.DeletedAt,
	}
}
