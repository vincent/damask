package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/repository"
	"damask/server/internal/storage"
)

type assetService struct {
	assets repository.AssetRepository
	tags   repository.TagRepository
	fields repository.FieldRepository
	stor   storage.Storage
	audit  audit.Writer
}

// NewAssetService returns an AssetService backed by the given repository.
func NewAssetService(assets repository.AssetRepository, tags repository.TagRepository, fields repository.FieldRepository, stor storage.Storage, aw audit.Writer) AssetService {
	return &assetService{assets: assets, tags: tags, fields: fields, stor: stor, audit: aw}
}

func (s *assetService) Get(ctx context.Context, workspaceID, assetID string) (*AssetDTO, error) {
	asset, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err
	}
	return toAssetDTO(asset), nil
}

func (s *assetService) List(ctx context.Context, params ListAssetsParams) ([]*AssetDTO, error) {
	// For taken_at sort, look up the exif field definition ID.
	exifFieldID := ""
	if params.SortField == "taken_at" {
		fd, err := s.fields.GetByKey(ctx, params.WorkspaceID, "_exif_taken_at")
		if err == nil {
			exifFieldID = fd.ID
		}
		// If the field doesn't exist, proceed with empty exifFieldID (join yields no rows).
	}

	rows, err := s.assets.List(ctx, repository.ListAssetsParams{
		WorkspaceID:  params.WorkspaceID,
		ProjectID:    params.ProjectID,
		FolderID:     params.FolderID,
		FolderIsRoot: params.FolderIsRoot,
		CollectionID: params.CollectionID,
		TagNames:     params.TagNames,
		SearchQuery:  params.SearchQuery,
		MimePrefix:   params.MimePrefix,
		SortField:    params.SortField,
		SortDesc:     params.SortDesc,
		CursorField:  params.CursorField,
		CursorValue:  params.CursorValue,
		CursorID:     params.CursorID,
		Limit:        params.Limit,
		ExifFieldID:  exifFieldID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*AssetDTO, len(rows))
	for i, r := range rows {
		out[i] = toAssetDTO(r)
	}
	return out, nil
}

func (s *assetService) Move(ctx context.Context, workspaceID, assetID string, p MoveAssetParams) (*AssetDTO, error) {
	before, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err
	}
	updated, err := s.assets.Update(ctx, repository.UpdateAssetParams{
		ID:          assetID,
		WorkspaceID: workspaceID,
		FolderID:    p.FolderID,
		ProjectID:   p.ProjectID,
	})
	if err != nil {
		return nil, err
	}
	dto := toAssetDTO(updated)
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetMoved,
		Payload: audit.AssetMovedPayload{
			V:               1,
			BeforeProjectID: before.ProjectID,
			AfterProjectID:  dto.ProjectID,
			BeforeFolderID:  before.FolderID,
			AfterFolderID:   dto.FolderID,
		},
	})
	return dto, nil
}

func (s *assetService) Rename(ctx context.Context, workspaceID, assetID, newStem string) (*AssetDTO, error) {
	newStem = strings.TrimSpace(newStem)
	if newStem == "" {
		return nil, fmt.Errorf("name cannot be empty: %w", apperr.ErrInvalidInput)
	}
	existing, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err
	}
	ext := filepath.Ext(existing.OriginalFilename)
	stem := strings.TrimSuffix(newStem, ext)
	newName := stem + ext
	if newName == existing.OriginalFilename {
		return toAssetDTO(existing), nil
	}
	updated, err := s.assets.Update(ctx, repository.UpdateAssetParams{
		ID:               assetID,
		WorkspaceID:      workspaceID,
		OriginalFilename: &newName,
	})
	if err != nil {
		return nil, err
	}
	dto := toAssetDTO(updated)
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetRenamed,
		Payload:     audit.AssetRenamedPayload{V: 1, Before: existing.OriginalFilename, After: dto.OriginalFilename},
	})
	return dto, nil
}

func (s *assetService) CountByIDs(ctx context.Context, workspaceID string, ids []string) (int64, error) {
	return s.assets.CountByIDs(ctx, workspaceID, ids)
}

func (s *assetService) RefreshFTS(ctx context.Context, assetID string) error {
	return s.assets.RefreshFTS(ctx, assetID)
}

func (s *assetService) ListByFields(ctx context.Context, params ListAssetsByFieldsParams) ([]*AssetDTO, error) {
	repoFilters := make([]repository.FieldFilter, len(params.FieldFilters))
	for i, f := range params.FieldFilters {
		repoFilters[i] = repository.FieldFilter{Key: f.Key, Operator: f.Operator, Value: f.Value}
	}
	rows, err := s.assets.ListByFields(ctx, repository.ListAssetsByFieldsParams{
		WorkspaceID:  params.WorkspaceID,
		FieldFilters: repoFilters,
		CursorAt:     params.CursorAt,
		CursorID:     params.CursorID,
		Limit:        params.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*AssetDTO, len(rows))
	for i, r := range rows {
		out[i] = toAssetDTO(r)
	}
	return out, nil
}

func (s *assetService) Delete(ctx context.Context, workspaceID, assetID string) error {
	isCover, err := s.assets.IsProjectCover(ctx, workspaceID, assetID)
	if err != nil {
		return err
	}
	if isCover {
		return fmt.Errorf("asset is used as a project cover: %w", apperr.ErrConflict)
	}
	isIcon, err := s.assets.IsWorkspaceIcon(ctx, workspaceID, assetID)
	if err != nil {
		return err
	}
	if isIcon {
		return fmt.Errorf("asset is used as the workspace icon: %w", apperr.ErrConflict)
	}
	return s.assets.SoftDelete(ctx, workspaceID, assetID)
}

func (s *assetService) HardDelete(ctx context.Context, workspaceID, assetID string) error {
	isCover, err := s.assets.IsProjectCover(ctx, workspaceID, assetID)
	if err != nil {
		return err
	}
	if isCover {
		return fmt.Errorf("asset is used as a project cover: %w", apperr.ErrConflict)
	}
	isIcon, err := s.assets.IsWorkspaceIcon(ctx, workspaceID, assetID)
	if err != nil {
		return err
	}
	if isIcon {
		return fmt.Errorf("asset is used as the workspace icon: %w", apperr.ErrConflict)
	}
	keys, err := s.assets.CollectStorageKeys(ctx, workspaceID, assetID)
	if err != nil {
		return err
	}
	if err := s.assets.HardDelete(ctx, workspaceID, assetID); err != nil {
		return err
	}
	s.deleteStorageKeys(keys)
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetDeleted,
		Payload:     audit.AssetDeletedPayload{V: 1},
	})
	return nil
}

func (s *assetService) BulkHardDelete(ctx context.Context, workspaceID string, assetIDs []string) error {
	type pending struct {
		keys repository.AssetStorageKeys
		id   string
	}
	var todo []pending
	for _, id := range assetIDs {
		keys, err := s.assets.CollectStorageKeys(ctx, workspaceID, id)
		if err != nil {
			continue // skip assets not in this workspace
		}
		todo = append(todo, pending{keys: keys, id: id})
	}
	for _, p := range todo {
		if err := s.assets.HardDelete(ctx, workspaceID, p.id); err != nil {
			return err
		}
		s.deleteStorageKeys(p.keys)
	}
	return nil
}

func (s *assetService) deleteStorageKeys(keys repository.AssetStorageKeys) {
	_ = s.stor.Delete(keys.AssetKey)
	if keys.ThumbKey != nil {
		_ = s.stor.Delete(*keys.ThumbKey)
	}
	for _, vk := range keys.VersionKeys {
		_ = s.stor.Delete(vk.StorageKey)
		if vk.ThumbnailKey != nil {
			_ = s.stor.Delete(*vk.ThumbnailKey)
		}
		for _, k := range vk.VariantKeys {
			_ = s.stor.Delete(k)
		}
	}
}

func (s *assetService) BulkTag(ctx context.Context, workspaceID, tagName string, assetIDs []string) error {
	tag, err := s.tags.Upsert(ctx, workspaceID, tagName)
	if err != nil {
		return err
	}
	for _, assetID := range assetIDs {
		if _, err := s.assets.GetByID(ctx, workspaceID, assetID); err != nil {
			continue // skip assets not in this workspace
		}
		_ = s.tags.AddToAsset(ctx, assetID, tag.ID)
	}
	return nil
}

func (s *assetService) BulkMoveProject(ctx context.Context, workspaceID string, assetIDs []string, projectID *string) error {
	for _, assetID := range assetIDs {
		if _, err := s.assets.GetByID(ctx, workspaceID, assetID); err != nil {
			continue // skip assets not in this workspace
		}
		if err := s.assets.SetProject(ctx, workspaceID, assetID, projectID); err != nil {
			return err
		}
	}
	return nil
}

func (s *assetService) GetComments(ctx context.Context, workspaceID, assetID string) ([]AssetCommentDTO, error) {
	if _, err := s.assets.GetByID(ctx, workspaceID, assetID); err != nil {
		return nil, err
	}
	rows, err := s.assets.ListComments(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]AssetCommentDTO, len(rows))
	for i, c := range rows {
		out[i] = AssetCommentDTO{
			ID:          c.ID,
			AssetID:     c.AssetID,
			ShareID:     c.ShareID,
			AuthorName:  c.AuthorName,
			AuthorEmail: c.AuthorEmail,
			Body:        c.Body,
			CreatedAt:   c.CreatedAt,
		}
	}
	return out, nil
}

func (s *assetService) CountVersionsByAsset(ctx context.Context, assetID string) (int64, error) {
	return s.assets.CountVersionsByAsset(ctx, assetID)
}

func (s *assetService) CountVariantsByCurrentVersion(ctx context.Context, assetID string) (int64, error) {
	return s.assets.CountVariantsByCurrentVersion(ctx, assetID)
}

func (s *assetService) IsRebuildingVariants(ctx context.Context, versionID string) (bool, error) {
	return s.assets.IsRebuildingVariants(ctx, versionID)
}

func (s *assetService) BatchVersionCounts(ctx context.Context, assetIDs []string) (map[string]int64, error) {
	return s.assets.BatchVersionCounts(ctx, assetIDs)
}

func (s *assetService) BatchVariantCounts(ctx context.Context, assetIDs []string) (map[string]int64, error) {
	return s.assets.BatchVariantCounts(ctx, assetIDs)
}

func (s *assetService) WriteAssetDownloadedAsync(workspaceID, assetID, userID string) {
	s.audit.WriteAssetAsync(audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      &userID,
		ActorType:   audit.ActorTypeUser,
		EventType:   audit.EventAssetDownloaded,
		Payload:     audit.AssetDownloadedPayload{V: 1, Via: "direct"},
	})
}

func toAssetDTO(a repository.Asset) *AssetDTO {
	return &AssetDTO{
		ID:               a.ID,
		WorkspaceID:      a.WorkspaceID,
		ProjectID:        a.ProjectID,
		FolderID:         a.FolderID,
		OriginalFilename: a.OriginalFilename,
		StorageKey:       a.StorageKey,
		MimeType:         a.MimeType,
		Size:             a.Size,
		Width:            a.Width,
		Height:           a.Height,
		ThumbnailKey:     a.ThumbnailKey,
		Metadata:         a.Metadata,
		CurrentVersionID: a.CurrentVersionID,
		CreatedAt:        a.CreatedAt,
		UpdatedAt:        a.UpdatedAt,
	}
}
