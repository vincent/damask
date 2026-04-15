package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"damask/server/internal/audit"
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
	ID           string                    `json:"id"`
	VersionNum   int64                     `json:"version_num"`
	MimeType     string                    `json:"mime_type"`
	Size         int64                     `json:"size"`
	Width        *int64                    `json:"width"`
	Height       *int64                    `json:"height"`
	DurationSec  *float64                  `json:"duration_sec"`
	ThumbnailURL *string                   `json:"thumbnail_url"`
	Comment      *string                   `json:"comment"`
	CreatedBy    *VersionCreatedByResponse `json:"created_by"`
	CreatedAt    string                    `json:"created_at"`
	IsCurrent    bool                      `json:"is_current"`
	VariantCount int64                     `json:"variant_count"`
}

// VersionWithAssetResponse is returned by upload and restore endpoints.
type VersionWithAssetResponse struct {
	Version VersionResponse `json:"version"`
	Asset   AssetResponse   `json:"asset"`
}

func (s *Server) buildVersionResponse(ctx context.Context, v dbgen.AssetVersion) VersionResponse {
	var createdBy *VersionCreatedByResponse
	if v.CreatedBy != nil {
		user, err := s.db.GetUserByID(ctx, *v.CreatedBy)
		createdByResp := VersionCreatedByResponse{ID: *v.CreatedBy}
		if err == nil {
			createdByResp.Name = user.Name
		}
		createdBy = &createdByResp
	}
	return buildVersionResponseWithCreator(v, createdBy)
}

// buildVersionResponseWithCreator builds a VersionResponse using a pre-resolved creator.
// Use this in list paths to avoid issuing a GetUserByID query per row.
func buildVersionResponseWithCreator(v dbgen.AssetVersion, createdBy *VersionCreatedByResponse) VersionResponse {
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

// buildVersionResponseWithCount is like buildVersionResponseWithCreator but also
// carries the per-version variant count returned by ListVersionsWithVariantCount.
func buildVersionResponseWithCount(v dbgen.ListVersionsWithVariantCountRow, createdBy *VersionCreatedByResponse) VersionResponse {
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
		VariantCount: v.VariantCount,
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
	defer tx.Rollback()

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

// handleUploadAssetVersion uploads a new version of an asset.
//
// @Summary Upload a new version
// @Description Uploads a new file as the next version of an existing asset. The new version is immediately promoted to current. The previous current version is retained in history.<br><br> After upload, the server: <ul> <li>Enqueues a thumbnail generation job for the new version.</li> <li>Enqueues a variant rebuild job that recreates all variants from the previous current version.</li> <li>Clears the asset's cached thumbnail (pending the new job).</li> </ul> If the uploaded file is byte-for-byte identical to the current version (same content hash), the request is rejected with 409 to prevent no-op versions.<br><br> The response body contains both the new <code>version</code> object and the updated <code>asset</code> object.
// @Tags Versions
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param file formData file true "New version file"
// @Param comment formData string false "Optional change note (max 500 characters)"
// @Success 201 {object} VersionWithAssetResponse
// @Failure 400 {object} ErrorResponse "Missing file field"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Failure 409 {object} ErrorResponse "File is identical to the current version"
// @Failure 422 {object} ErrorResponse "Comment too long"
// @Router /api/v1/assets/{id}/versions [post]
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
	defer os.Remove(tmpFile)

	// Open the temp file once; hash it, then seek back to reuse for storage.Put.
	f, err := os.Open(tmpFile)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not open uploaded file")
	}
	defer f.Close()

	hash, size, err := versioning.HashReader(f)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not hash file")
	}

	// Capture the current version ID before we promote a new one — needed for rebuild job.
	var prevVersionID string
	if asset.CurrentVersionID != nil {
		prevVersionID = *asset.CurrentVersionID
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

	createdByPtr := &claims.UserID
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
		CreatedBy:   createdByPtr,
		IsCurrent:   0,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create version")
	}

	// Atomically promote the new version to current.
	if err := s.setCurrentVersion(c.RequestCtx(), assetID, newVersion.ID); err != nil {
		slog.Error("set current version", "error", err)
		return errRes(c, fiber.StatusInternalServerError, "could not promote version")
	}
	newVersion.IsCurrent = 1

	// Also update the asset's top-level fields to stay in sync with the current version.
	if err := s.db.UpdateAssetThumbnail(c.RequestCtx(), dbgen.UpdateAssetThumbnailParams{
		ThumbnailKey: nil, // thumbnail will come from the job
		ID:           assetID,
	}); err != nil {
		slog.Error("clear asset thumbnail", "error", err)
	}

	// Enqueue thumbnail generation for the new version.
	s.enqueueVersionThumbnail(c.RequestCtx(), asset, newVersion)

	// Enqueue variant rebuild: copy variant definitions from the previous version.
	// No-op if this is the first version (prevVersionID == "").
	if err := jobs.EnqueueRebuildVariantsJob(
		c.RequestCtx(), s.queue,
		claims.WorkspaceID, assetID, newVersion.ID, prevVersionID,
	); err != nil {
		slog.Error("enqueue rebuild variants", "asset_id", assetID, "version_id", newVersion.ID, "error", err)
	}

	// Reload asset to return latest state.
	updatedAsset, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not reload asset")
	}

	userID := claims.UserID
	commentStr := ""
	if newVersion.Comment != nil {
		commentStr = *newVersion.Comment
	}
	s.audit.WriteAsset(c.RequestCtx(), audit.AssetEvent{
		WorkspaceID: claims.WorkspaceID,
		AssetID:     assetID,
		UserID:      &userID,
		ActorType:   audit.ActorTypeUser,
		EventType:   audit.EventAssetVersionUploaded,
		Payload:     audit.AssetVersionUploadedPayload{V: 1, VersionNum: newVersion.VersionNum, Size: newVersion.Size, Comment: commentStr},
	})

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"version": s.buildVersionResponse(c.RequestCtx(), newVersion),
		"asset":   assetToResponse(updatedAsset, nil),
	})
}

// --- AV-1.4: List versions ---

// handleListAssetVersions returns all versions of an asset.
//
// @Summary List asset versions
// @Description Returns all versions of an asset in reverse chronological order. Each version includes its <code>version_num</code>, MIME type, dimensions, file size, optional comment, and the number of variants derived from it (<code>variant_count</code>). The current version has <code>is_current: true</code>.
// @Tags Versions
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {array} VersionResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/versions [get]
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

	versions, err := s.db.ListVersionsWithVariantCount(c.RequestCtx(), assetID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list versions")
	}

	// Batch-resolve creator names to avoid N+1 queries.
	userNames := make(map[string]string)
	for _, v := range versions {
		if v.CreatedBy != nil {
			if _, seen := userNames[*v.CreatedBy]; !seen {
				userNames[*v.CreatedBy] = ""
				if u, err := s.db.GetUserByID(c.RequestCtx(), *v.CreatedBy); err == nil {
					userNames[*v.CreatedBy] = u.Name
				}
			}
		}
	}

	resp := make([]VersionResponse, len(versions))
	for i, v := range versions {
		var createdBy *VersionCreatedByResponse
		if v.CreatedBy != nil {
			createdBy = &VersionCreatedByResponse{
				ID:   *v.CreatedBy,
				Name: userNames[*v.CreatedBy],
			}
		}
		resp[i] = buildVersionResponseWithCount(v, createdBy)
	}
	return c.JSON(resp)
}

// --- AV-1.5: Restore (rollback) ---

// handleRestoreAssetVersion restores an older version to current.
//
// @Summary Restore a version
// @Description Promotes a non-current version to be the current version of the asset. The asset's thumbnail is also updated to match the restored version. An audit event records the version numbers rolled back from and to.<br><br> Returns 409 if the requested version is already current. Returns 422 if the version has been soft-deleted. The response contains the updated <code>version</code> and <code>asset</code>.
// @Tags Versions
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param vid path string true "Version ID to restore"
// @Success 200 {object} VersionWithAssetResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset or version not found"
// @Failure 409 {object} ErrorResponse "Version is already current"
// @Failure 422 {object} ErrorResponse "Cannot restore a deleted version"
// @Router /api/v1/assets/{id}/versions/{vid}/restore [post]
// handleRestoreAssetVersion handles POST /api/v1/assets/:id/versions/:vid/restore
func (s *Server) handleRestoreAssetVersion(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	versionID := c.Params("vid")

	assetBeforeRestore, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
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
		slog.Error("restore: sync thumbnail", "error", err)
	}

	updatedAsset, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not reload asset")
	}

	// Record the version number we rolled back from (the previous current).
	var fromVersionNum int64
	if assetBeforeRestore.CurrentVersionID != nil {
		if prev, err := s.db.GetVersionByID(c.RequestCtx(), dbgen.GetVersionByIDParams{
			ID:          *assetBeforeRestore.CurrentVersionID,
			WorkspaceID: claims.WorkspaceID,
		}); err == nil {
			fromVersionNum = prev.VersionNum
		}
	}

	userID := claims.UserID
	s.audit.WriteAsset(c.RequestCtx(), audit.AssetEvent{
		WorkspaceID: claims.WorkspaceID,
		AssetID:     assetID,
		UserID:      &userID,
		ActorType:   audit.ActorTypeUser,
		EventType:   audit.EventAssetVersionRestored,
		Payload:     audit.AssetVersionRestoredPayload{V: 1, FromVersionNum: fromVersionNum, ToVersionNum: target.VersionNum},
	})

	target.IsCurrent = 1
	return c.JSON(fiber.Map{
		"version": s.buildVersionResponse(c.RequestCtx(), target),
		"asset":   assetToResponse(updatedAsset, nil),
	})
}

// --- AV-1.6: Soft-delete version ---

// handleDeleteAssetVersion soft-deletes a non-current version.
//
// @Summary Delete a version
// @Description Soft-deletes an asset version. The current version cannot be deleted — restore a different version first, then delete the old one. A version also cannot be deleted while it is in use as a project cover or workspace icon (returns 409).<br><br> Soft deletion marks the row as deleted; the storage file is not immediately removed (physical cleanup is handled by a background retention job).
// @Tags Versions
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param vid path string true "Version ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset or version not found"
// @Failure 409 {object} ErrorResponse "Version is in use as a cover/icon"
// @Failure 422 {object} ErrorResponse "Cannot delete the current version"
// @Router /api/v1/assets/{id}/versions/{vid} [delete]
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

	// Safety: block deletion if this version is in use as a project cover or workspace icon.
	if refs, err := s.db.IsVersionReferencedAsCover(c.RequestCtx(), dbgen.IsVersionReferencedAsCoverParams{
		CoverVersionID: &versionID,
		IconVersionID:  &versionID,
	}); err == nil && refs > 0 {
		return errRes(c, fiber.StatusConflict,
			"this version is in use as a project cover or workspace icon — update the cover first, then delete this version")
	}

	if err := s.db.SoftDeleteVersion(c.RequestCtx(), versionID); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete version")
	}

	userID := claims.UserID
	s.audit.WriteAsset(c.RequestCtx(), audit.AssetEvent{
		WorkspaceID: claims.WorkspaceID,
		AssetID:     assetID,
		UserID:      &userID,
		ActorType:   audit.ActorTypeUser,
		EventType:   audit.EventAssetVersionDeleted,
		Payload:     audit.AssetVersionDeletedPayload{V: 1, VersionNum: target.VersionNum},
	})

	return c.SendStatus(fiber.StatusNoContent)
}

// --- AV-1.7: Stream version file and thumbnail ---

// handleGetVersionFile streams the original file for a specific version.
//
// @Summary Get version file
// @Description Streams the raw file for the specified version. The response MIME type and <code>Content-Disposition</code> are set from the stored version metadata.
// @Tags Versions
// @Produce application/octet-stream
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param vid path string true "Version ID"
// @Success 200 {file} binary
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset, version, or file not found"
// @Router /api/v1/assets/{id}/versions/{vid}/file [get]
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

// handleGetVersionThumb serves the JPEG thumbnail for a specific version.
//
// @Summary Get version thumbnail
// @Description Returns the JPEG thumbnail for the given version. Returns 404 if the thumbnail has not yet been generated (the job runs asynchronously after upload — poll until the <code>thumbnail_url</code> field in the version response is non-null).
// @Tags Versions
// @Produce image/jpeg
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param vid path string true "Version ID"
// @Success 200 {file} binary
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset, version, or thumbnail not ready"
// @Router /api/v1/assets/{id}/versions/{vid}/thumb [get]
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
		slog.Error("enqueue version thumbnail", "asset_id", asset.ID, "version_id", version.ID, "error", err)
	}
}
