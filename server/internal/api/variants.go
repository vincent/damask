package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"damask/server/internal/audit"
	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/jobs"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	"damask/server/internal/transform"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// ---- Response types ----

type VariantResponse struct {
	ID              string    `json:"id"`
	AssetVersionID  string    `json:"asset_version_id"`
	Type            string    `json:"type"`
	TransformParams *string   `json:"transform_params"`
	Size            *int64    `json:"size"`
	StorageKey      string    `json:"storage_key"`
	DownloadURL     string    `json:"download_url"`
	CreatedAt       time.Time `json:"created_at"`
}

type ListVariantsResponse struct {
	Variants   []VariantResponse `json:"variants"`
	Rebuilding bool              `json:"rebuilding"`
}

func variantToResponse(v dbgen.Variant) VariantResponse {
	return VariantResponse{
		ID:              v.ID,
		AssetVersionID:  v.AssetVersionID,
		Type:            v.Type,
		TransformParams: v.TransformParams,
		Size:            v.Size,
		StorageKey:      v.StorageKey,
		DownloadURL:     fmt.Sprintf("/api/v1/variants/%s/file", v.ID),
		CreatedAt:       v.CreatedAt,
	}
}

// isRebuildingVariants returns true when a rebuild_variants job for the given
// version is in pending or processing state.
func (s *Server) isRebuildingVariants(c fiber.Ctx, versionID string) bool {
	var count int64
	err := s.sqlDB.QueryRowContext(c.RequestCtx(),
		`SELECT COUNT(*) FROM jobs
		 WHERE type = 'rebuild_variants'
		   AND JSON_EXTRACT(payload, '$.new_version_id') = ?
		   AND status IN ('pending', 'processing')`,
		versionID,
	).Scan(&count)
	return err == nil && count > 0
}

// ---- Handlers ----

// handleListVariants handles GET /api/v1/assets/:id/variants
// Returns variants for the current version only, plus a rebuilding flag.
func (s *Server) handleListVariants(c fiber.Ctx) error {
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

	variants, err := s.db.ListVariantsByAssetCurrentVersion(c.RequestCtx(), assetID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list variants")
	}

	out := make([]VariantResponse, len(variants))
	for i, v := range variants {
		out[i] = variantToResponse(v)
	}

	// Determine if a rebuild job is in flight for the current version.
	rebuilding := false
	if asset.CurrentVersionID != nil {
		rebuilding = s.isRebuildingVariants(c, *asset.CurrentVersionID)
	}

	return c.JSON(ListVariantsResponse{
		Variants:   out,
		Rebuilding: rebuilding,
	})
}

// handleCreateVariant handles POST /api/v1/assets/:id/variants
// Creates a variant bound to the asset's current version.
func (s *Server) handleCreateVariant(c fiber.Ctx) error {
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

	// Require a current version.
	if asset.CurrentVersionID == nil {
		return errRes(c, fiber.StatusUnprocessableEntity, "asset has no current version")
	}

	// Block creation while a rebuild is in progress.
	if s.isRebuildingVariants(c, *asset.CurrentVersionID) {
		return errRes(c, fiber.StatusConflict, "variants are rebuilding — please wait for the rebuild to complete before creating new variants")
	}

	body, ok := decodeAndValidate(c, &createVariantRequest{})
	if !ok {
		return nil
	}

	validTypes := map[string]bool{
		queue.JobTypeImageResize:       true,
		queue.JobTypeImageWatermark:    true,
		queue.JobTypeImageConvert:      true,
		queue.JobTypeImageCrop:         true,
		queue.JobTypeVideoCaptureImage: true,
		queue.JobTypeVideoTranscode:    true,
		queue.JobTypeImageBgRemove:     true,
		queue.JobTypeImageSmartCrop:    true,
	}
	if !validTypes[body.Type] {
		return errRes(c, fiber.StatusBadRequest, "invalid variant type")
	}

	if (body.Type == queue.JobTypeVideoCaptureImage || body.Type == queue.JobTypeVideoTranscode) &&
		!strings.HasPrefix(asset.MimeType, "video/") {
		return errRes(c, fiber.StatusBadRequest, "video transforms require a video asset")
	}
	if (body.Type == queue.JobTypeImageResize || body.Type == queue.JobTypeImageConvert || body.Type == queue.JobTypeImageCrop || body.Type == queue.JobTypeImageWatermark || body.Type == queue.JobTypeImageSmartCrop) &&
		!strings.HasPrefix(asset.MimeType, "image/") {
		return errRes(c, fiber.StatusBadRequest, "image transforms require an image asset")
	}
	if body.Type == queue.JobTypeImageBgRemove {
		if s.cfg.RemoveBgAPIKey == "" {
			return errRes(c, fiber.StatusBadRequest, "background removal requires REMOVEBG_API_KEY to be configured")
		}
		if !strings.HasPrefix(asset.MimeType, "image/") {
			return errRes(c, fiber.StatusBadRequest, "background removal requires an image asset")
		}
	}

	// Load the current version to get its storage key and version num.
	currentVer, err := s.db.GetCurrentVersion(c.RequestCtx(), assetID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load current version")
	}

	params := json.RawMessage("{}")
	if len(body.Params) > 0 {
		params = body.Params
	}

	payload, _ := json.Marshal(jobs.VariantJobPayload{
		AssetID:        asset.ID,
		WorkspaceID:    asset.WorkspaceID,
		VersionID:      currentVer.ID,
		VersionNum:     currentVer.VersionNum,
		StorageKey:     currentVer.StorageKey,
		MimeType:       currentVer.MimeType,
		Type:           body.Type,
		Params:         params,
	})

	job, err := s.queue.Enqueue(c.RequestCtx(), claims.WorkspaceID, body.Type, string(payload))
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not enqueue job")
	}

	userID := claims.UserID
	s.audit.WriteAsset(c.RequestCtx(), audit.AssetEvent{
		WorkspaceID: claims.WorkspaceID,
		AssetID:     assetID,
		UserID:      &userID,
		ActorType:   audit.ActorTypeUser,
		EventType:   audit.EventAssetVariantCreated,
		Payload:     audit.AssetVariantCreatedPayload{V: 1, Type: body.Type},
	})

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"job_id":  job.ID,
		"status":  "pending",
		"message": "variant creation queued",
	})
}

// handleGetVariantFile handles GET /api/v1/assets/:id/variants/:vid/file
func (s *Server) handleGetVariantFile(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	variantID := c.Params("vid")

	if _, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	variant, err := s.db.GetVariantByID(c.RequestCtx(), dbgen.GetVariantByIDParams{
		ID:          variantID,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "variant not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load variant")
	}

	rc, err := s.storage.Get(variant.StorageKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "variant file not found")
	}

	if c.Get("Sec-Fetch-Dest") != "image" {
		userID := claims.UserID
		s.audit.WriteAssetAsync(audit.AssetEvent{
			WorkspaceID: claims.WorkspaceID,
			AssetID:     assetID,
			UserID:      &userID,
			ActorType:   audit.ActorTypeUser,
			EventType:   audit.EventAssetVariantDownloaded,
			Payload:     audit.AssetVariantDownloadedPayload{V: 1, VariantID: variantID, Type: variant.Type},
		})
	}

	ext := strings.ToLower(filepath.Ext(variant.StorageKey))
	c.Set("Content-Type", mime.TypeByExtension(ext))
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s_%s%s"`, assetID[:8], variantID[:8], ext))
	return c.SendStream(rc)
}

// handleDeleteVariant handles DELETE /api/v1/assets/:id/variants/:vid
// Guards against deleting variants that belong to non-current versions.
func (s *Server) handleDeleteVariant(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	variantID := c.Params("vid")

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

	variant, err := s.db.GetVariantByID(c.RequestCtx(), dbgen.GetVariantByIDParams{
		ID:          variantID,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "variant not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load variant")
	}

	// Guard: only allow deleting variants for the current version.
	// Variants on old versions are managed by the retention purge job.
	if asset.CurrentVersionID == nil || variant.AssetVersionID != *asset.CurrentVersionID {
		return errRes(c, fiber.StatusUnprocessableEntity,
			"this variant belongs to a previous version and will be cleaned up automatically")
	}

	_ = s.storage.Delete(variant.StorageKey)

	if err := s.db.DeleteVariant(c.RequestCtx(), dbgen.DeleteVariantParams{
		ID:          variantID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete variant")
	}

	userID := claims.UserID
	s.audit.WriteAsset(c.RequestCtx(), audit.AssetEvent{
		WorkspaceID: claims.WorkspaceID,
		AssetID:     assetID,
		UserID:      &userID,
		ActorType:   audit.ActorTypeUser,
		EventType:   audit.EventAssetVariantDeleted,
		Payload:     audit.AssetVariantDeletedPayload{V: 1, VariantID: variantID, Type: variant.Type},
	})

	return c.SendStatus(fiber.StatusNoContent)
}

// handleUploadManualVariant handles POST /api/v1/assets/:id/variants/upload
// Accepts a raw file upload and attaches it as a manual variant of type "manual"
// on the current version. Manual variants are NOT rebuilt on new version uploads.
func (s *Server) handleUploadManualVariant(c fiber.Ctx) error {
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

	if asset.CurrentVersionID == nil {
		return errRes(c, fiber.StatusUnprocessableEntity, "asset has no current version")
	}

	currentVer, err := s.db.GetCurrentVersion(c.RequestCtx(), assetID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load current version")
	}

	fh, err := c.FormFile("file")
	if err != nil {
		return errRes(c, fiber.StatusBadRequest, "file field is required")
	}

	f, err := fh.Open()
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not open uploaded file")
	}
	defer f.Close() //nolint:errcheck

	variantID := uuid.NewString()
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	// Use first 8 chars of variant ID as the params hash (manual = no transform params).
	storageKey := storage.VersionedVariantKey(
		asset.WorkspaceID, assetID, currentVer.VersionNum,
		"manual", variantID[:8], ext,
	)

	if err := s.storage.Put(storageKey, f); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not store file")
	}

	sz := fh.Size
	emptyParams := "{}"
	v, err := s.db.CreateVariant(c.RequestCtx(), dbgen.CreateVariantParams{
		ID:              variantID,
		WorkspaceID:     asset.WorkspaceID,
		AssetVersionID:  currentVer.ID,
		Type:            "manual",
		StorageKey:      storageKey,
		TransformParams: &emptyParams,
		Size:            &sz,
	})
	if err != nil {
		_ = s.storage.Delete(storageKey)
		return errRes(c, fiber.StatusInternalServerError, "could not create variant record")
	}

	userID := claims.UserID
	s.audit.WriteAsset(c.RequestCtx(), audit.AssetEvent{
		WorkspaceID: claims.WorkspaceID,
		AssetID:     assetID,
		UserID:      &userID,
		ActorType:   audit.ActorTypeUser,
		EventType:   audit.EventAssetVariantCreated,
		Payload:     audit.AssetVariantCreatedPayload{V: 1, Type: "manual"},
	})

	return c.Status(fiber.StatusCreated).JSON(variantToResponse(v))
}

// handlePreviewTransform runs a transform in-memory and returns a small image.
// GET /api/v1/assets/:id/preview?w=&h=&fit=&format=&q=
func (s *Server) handlePreviewTransform(c fiber.Ctx) error {
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

	if !strings.HasPrefix(asset.MimeType, "image/") {
		return errRes(c, fiber.StatusBadRequest, "preview only supported for images")
	}

	w, _ := strconv.Atoi(c.Query("w"))
	h, _ := strconv.Atoi(c.Query("h"))
	q, _ := strconv.Atoi(c.Query("q"))
	fit := c.Query("fit", "contain")
	format := c.Query("format", "jpeg")

	cacheKey := fmt.Sprintf("%s|w=%d|h=%d|fit=%s|format=%s|q=%d", assetID, w, h, fit, format, q)
	if cached, ct := s.previewCache.Get(cacheKey); cached != nil {
		c.Set("Content-Type", ct)
		c.Set("Cache-Control", "public, max-age=300")
		return c.Send(cached)
	}

	rc, err := s.storage.Get(asset.StorageKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "asset file not found")
	}
	defer rc.Close()

	data, ct, err := transform.Preview(rc, transform.PreviewParams{
		Width:   w,
		Height:  h,
		Fit:     fit,
		Quality: q,
		Format:  format,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "preview generation failed")
	}

	s.previewCache.Set(cacheKey, data, ct)
	c.Set("Content-Type", ct)
	c.Set("Cache-Control", "public, max-age=300")
	return c.Send(data)
}
