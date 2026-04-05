package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/jobs"
	"damask/server/internal/services"
	"damask/server/internal/versioning"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// --- Response types ---

type VersionCreatedByResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type VersionResponse struct {
	ID           string                   `json:"id"`
	VersionNum   int64                    `json:"version_num"`
	MimeType     string                   `json:"mime_type"`
	Size         int64                    `json:"size"`
	Width        *int64                   `json:"width"`
	Height       *int64                   `json:"height"`
	DurationSec  *float64                 `json:"duration_sec"`
	ThumbnailURL *string                  `json:"thumbnail_url"`
	Comment      *string                  `json:"comment"`
	CreatedBy    VersionCreatedByResponse `json:"created_by"`
	CreatedAt    string                   `json:"created_at"`
	IsCurrent    bool                     `json:"is_current"`
}

func (s *Server) buildVersionResponse(ctx context.Context, v dbgen.AssetVersion) VersionResponse {
	user, err := s.db.GetUserByID(ctx, v.CreatedBy)
	createdBy := VersionCreatedByResponse{ID: v.CreatedBy}
	if err == nil {
		createdBy.Name = user.Name
	}
	return buildVersionResponseWithCreator(v, createdBy)
}

// buildVersionResponseWithCreator builds a versionResponse using a pre-resolved creator.
// Use this in list paths to avoid issuing a GetUserByID query per row.
func buildVersionResponseWithCreator(v dbgen.AssetVersion, createdBy VersionCreatedByResponse) VersionResponse {
	var thumbURL *string
	if v.ThumbnailKey != nil {
		u := fmt.Sprintf("/api/v1/assets/%s/versions/%s/thumb", v.AssetID, v.ID)
		thumbURL = &u
	}

	return VersionResponse{
		ID:           v.ID,
		VersionNum:   v.VersionNum,
		MimeType:     v.MimeType,
		Size:         v.Size,
		Width:        v.Width,
		Height:       v.Height,
		DurationSec:  v.DurationSec,
		ThumbnailURL: thumbURL,
		Comment:      v.Comment,
		CreatedBy:    createdBy,
		CreatedAt:    v.CreatedAt,
		IsCurrent:    v.IsCurrent == 1,
	}
}

// setCurrentVersion promotes versionID to current within a transaction.
// It clears is_current on all other versions of assetID and updates
// assets.current_version_id. Must be called with s.sqlDB available.
func (s *Server) setCurrentVersion(ctx context.Context, assetID, versionID string) error {
	tx, err := s.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := s.db.WithTx(tx)

	// Clear all is_current flags for this asset.
	if err := qtx.ClearCurrentVersionFlags(ctx, assetID); err != nil {
		return err
	}

	// Set the target version as current.
	if err := qtx.SetCurrentVersionFlag(ctx, versionID); err != nil {
		return err
	}

	// Keep assets.current_version_id in sync.
	if err := qtx.UpdateAssetCurrentVersion(ctx, dbgen.UpdateAssetCurrentVersionParams{
		CurrentVersionID: &versionID,
		ID:               assetID,
	}); err != nil {
		return err
	}

	return tx.Commit()
}

// --- AV-1.3: Upload new version ---

// handleUploadAssetVersion handles POST /api/v1/assets/:id/versions
func (s *Server) handleUploadAssetVersion(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	asset, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	fh, err := c.FormFile("file")
	if err != nil {
		return errRes(c, fiber.StatusBadRequest, "file field is required")
	}

	comment := strings.TrimSpace(c.FormValue("comment"))
	if len(comment) > 500 {
		return errRes(c, fiber.StatusUnprocessableEntity, "comment must be 500 characters or fewer")
	}

	// Save to a temp file so we can hash + detect MIME + extract meta.
	tmpFile := filepath.Join(os.TempDir(), uuid.NewString()+"_"+fh.Filename)
	if err := c.SaveFile(fh, tmpFile); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "cannot save uploaded file")
	}
	defer os.Remove(tmpFile) //nolint:errcheck

	// Open the temp file once; hash it, then seek back to reuse for storage.Put.
	f, err := os.Open(tmpFile)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not open uploaded file")
	}
	defer f.Close() //nolint:errcheck

	hash, size, err := versioning.HashReader(f)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not hash file")
	}

	// Dedup: if identical bytes are already the current version, reject.
	existing, lookupErr := s.db.GetVersionByHash(c.RequestCtx(), dbgen.GetVersionByHashParams{
		AssetID:     assetID,
		ContentHash: hash,
	})
	if lookupErr == nil && existing.IsCurrent == 1 {
		return errRes(c, fiber.StatusConflict, "this file is identical to the current version")
	}

	// Determine next version_num via MAX to avoid counting deleted rows.
	var maxNum sql.NullInt64
	if err := s.sqlDB.QueryRowContext(c.RequestCtx(),
		`SELECT MAX(version_num) FROM asset_versions WHERE asset_id = ?`, assetID,
	).Scan(&maxNum); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not determine version number")
	}
	nextNum := maxNum.Int64 + 1

	// Detect MIME + dimensions (both use the temp file path, not the reader).
	mimeType, _ := services.DetectMimeType(tmpFile)
	if mimeType == "" {
		mimeType = fh.Header.Get("Content-Type")
	}

	meta, _ := services.ExtractMeta(c.RequestCtx(), tmpFile, mimeType)

	// Build storage key: workspace/asset/vN/filename.
	storageKey := fmt.Sprintf("%s/%s/v%d/%s", claims.WorkspaceID, assetID, nextNum, fh.Filename)

	// Only write to storage if this is truly a new file (not a hash-matched old version).
	if lookupErr != nil {
		// Seek back to start — f was already read for hashing above.
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not rewind file")
		}
		if err := s.storage.Put(storageKey, f); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not store file")
		}
	} else {
		// Reuse the existing storage key from the found version.
		storageKey = existing.StorageKey
	}

	// Persist version row (is_current=0; we promote it next).
	var commentPtr *string
	if comment != "" {
		commentPtr = &comment
	}

	newVersion, err := s.db.CreateAssetVersion(c.RequestCtx(), dbgen.CreateAssetVersionParams{
		ID:          uuid.NewString(),
		AssetID:     assetID,
		WorkspaceID: claims.WorkspaceID,
		VersionNum:  nextNum,
		StorageKey:  storageKey,
		ContentHash: hash,
		MimeType:    mimeType,
		Size:        size,
		Width:       meta.Width,
		Height:      meta.Height,
		DurationSec: meta.DurationSec,
		Comment:     commentPtr,
		CreatedBy:   claims.UserID,
		IsCurrent:   0,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create version")
	}

	// Atomically promote the new version to current.
	if err := s.setCurrentVersion(c.RequestCtx(), assetID, newVersion.ID); err != nil {
		log.Printf("set current version: %v", err)
		return errRes(c, fiber.StatusInternalServerError, "could not promote version")
	}
	newVersion.IsCurrent = 1

	// Also update the asset's top-level fields to stay in sync with the current version.
	if err := s.db.UpdateAssetThumbnail(c.RequestCtx(), dbgen.UpdateAssetThumbnailParams{
		ThumbnailKey: nil, // thumbnail will come from the job
		ID:           assetID,
	}); err != nil {
		log.Printf("clear asset thumbnail: %v", err)
	}

	// Enqueue thumbnail generation for the new version.
	s.enqueueVersionThumbnail(c.RequestCtx(), asset, newVersion)

	// Reload asset to return latest state.
	updatedAsset, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not reload asset")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"version": s.buildVersionResponse(c.RequestCtx(), newVersion),
		"asset":   assetToResponse(updatedAsset, nil),
	})
}

// --- AV-1.4: List versions ---

// handleListAssetVersions handles GET /api/v1/assets/:id/versions
func (s *Server) handleListAssetVersions(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	if _, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	versions, err := s.db.ListVersions(c.RequestCtx(), assetID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list versions")
	}

	// Batch-resolve creator names to avoid N+1 queries.
	userNames := make(map[string]string, len(versions))
	for _, v := range versions {
		if _, seen := userNames[v.CreatedBy]; !seen {
			userNames[v.CreatedBy] = "" // placeholder
			if u, err := s.db.GetUserByID(c.RequestCtx(), v.CreatedBy); err == nil {
				userNames[v.CreatedBy] = u.Name
			}
		}
	}

	resp := make([]VersionResponse, len(versions))
	for i, v := range versions {
		resp[i] = buildVersionResponseWithCreator(v, VersionCreatedByResponse{
			ID:   v.CreatedBy,
			Name: userNames[v.CreatedBy],
		})
	}
	return c.JSON(resp)
}

// --- AV-1.5: Restore (rollback) ---

// handleRestoreAssetVersion handles POST /api/v1/assets/:id/versions/:vid/restore
func (s *Server) handleRestoreAssetVersion(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	versionID := c.Params("vid")

	if _, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	target, err := s.db.GetVersionByID(c.RequestCtx(), dbgen.GetVersionByIDParams{
		ID:          versionID,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "version not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load version")
	}

	if target.AssetID != assetID {
		return errRes(c, fiber.StatusNotFound, "version not found")
	}
	if target.DeletedAt != nil {
		return errRes(c, fiber.StatusUnprocessableEntity, "cannot restore a deleted version")
	}
	if target.IsCurrent == 1 {
		return errRes(c, fiber.StatusConflict, "version is already current")
	}

	if err := s.setCurrentVersion(c.RequestCtx(), assetID, versionID); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not restore version")
	}

	// Sync the asset's thumbnail with the restored version's thumbnail.
	if err := s.db.UpdateAssetThumbnail(c.RequestCtx(), dbgen.UpdateAssetThumbnailParams{
		ThumbnailKey: target.ThumbnailKey,
		ID:           assetID,
	}); err != nil {
		log.Printf("restore: sync thumbnail: %v", err)
	}

	updatedAsset, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not reload asset")
	}

	target.IsCurrent = 1
	return c.JSON(fiber.Map{
		"version": s.buildVersionResponse(c.RequestCtx(), target),
		"asset":   assetToResponse(updatedAsset, nil),
	})
}

// --- AV-1.6: Soft-delete version ---

// handleDeleteAssetVersion handles DELETE /api/v1/assets/:id/versions/:vid
func (s *Server) handleDeleteAssetVersion(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	versionID := c.Params("vid")

	if _, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	target, err := s.db.GetVersionByID(c.RequestCtx(), dbgen.GetVersionByIDParams{
		ID:          versionID,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "version not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load version")
	}

	if target.AssetID != assetID {
		return errRes(c, fiber.StatusNotFound, "version not found")
	}
	// Safety: never delete the current version.
	if target.IsCurrent == 1 {
		return errRes(c, fiber.StatusUnprocessableEntity,
			"cannot delete the current version — restore another version first, then delete this one")
	}

	if err := s.db.SoftDeleteVersion(c.RequestCtx(), versionID); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete version")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// --- AV-1.7: Stream version file and thumbnail ---

// handleGetVersionFile handles GET /api/v1/assets/:id/versions/:vid/file
func (s *Server) handleGetVersionFile(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	versionID := c.Params("vid")

	asset, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	target, err := s.db.GetVersionByID(c.RequestCtx(), dbgen.GetVersionByIDParams{
		ID:          versionID,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "version not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load version")
	}
	if target.AssetID != assetID {
		return errRes(c, fiber.StatusNotFound, "version not found")
	}

	rc, err := s.storage.Get(target.StorageKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "file not found")
	}

	c.Set("Content-Type", target.MimeType)
	c.Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, asset.OriginalFilename))
	return c.SendStream(rc)
}

// handleGetVersionThumb handles GET /api/v1/assets/:id/versions/:vid/thumb
func (s *Server) handleGetVersionThumb(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	versionID := c.Params("vid")

	if _, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	target, err := s.db.GetVersionByID(c.RequestCtx(), dbgen.GetVersionByIDParams{
		ID:          versionID,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "version not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load version")
	}
	if target.AssetID != assetID {
		return errRes(c, fiber.StatusNotFound, "version not found")
	}
	if target.ThumbnailKey == nil {
		return errRes(c, fiber.StatusNotFound, "thumbnail not ready")
	}

	rc, err := s.storage.Get(*target.ThumbnailKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "thumbnail not found")
	}

	c.Set("Content-Type", "image/jpeg")
	return c.SendStream(rc)
}

// --- thumbnail job helper ---

// enqueueVersionThumbnail enqueues thumbnail generation for the given version.
// The job handler updates asset_versions.thumbnail_key (not assets.thumbnail_key)
// via a dedicated version thumbnail job type.
func (s *Server) enqueueVersionThumbnail(ctx context.Context, asset dbgen.Asset, version dbgen.AssetVersion) {
	payload := jobs.VersionThumbnailJobPayload{
		AssetID:     asset.ID,
		VersionID:   version.ID,
		WorkspaceID: asset.WorkspaceID,
		StorageKey:  version.StorageKey,
		MimeType:    version.MimeType,
	}
	if err := jobs.EnqueueVersionThumbnailJob(ctx, s.queue, asset.WorkspaceID, payload); err != nil {
		log.Printf("enqueue version thumbnail for %s/%s: %v", asset.ID, version.ID, err)
	}
}
