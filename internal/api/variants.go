package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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

type CreateVariantResponse struct {
	JobID   string `json:"job_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func variantToResponse(assetID string, v dbgen.Variant) VariantResponse {
	return VariantResponse{
		ID:              v.ID,
		AssetVersionID:  v.AssetVersionID,
		Type:            v.Type,
		TransformParams: v.TransformParams,
		Size:            v.Size,
		StorageKey:      v.StorageKey,
		DownloadURL:     fmt.Sprintf("/api/v1/assets/%s/variants/%s/file", assetID, v.ID),
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
	if err != nil {
		slog.Error("is_rebuilding_variants", "error", err)
	}
	return err == nil && count > 0
}

// ---- Handlers ----

// handleListVariants returns all variants for the asset's current version.
//
// @Summary List asset variants
// @Description Returns all generated variants for the asset's current version, plus a <code>rebuilding</code> flag that is true when a variant rebuild job is in progress (e.g. after a new version upload). Each variant includes a <code>download_url</code> for direct file access.
// @Tags Variants
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {object} ListVariantsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/variants [get]
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
		out[i] = variantToResponse(assetID, v)
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

// handleCreateVariant enqueues a transform job to produce a new variant.
//
// @Summary Create a variant
// @Description Enqueues a background job to generate a transformed variant of the asset's current version. Supported types and their required params: <ul> <li><strong>image_resize</strong> — <code>{"width": N, "height": N, "fit": "contain|cover|fill"}</code></li> <li><strong>image_convert</strong> — <code>{"format": "jpeg|png|webp|avif"}</code></li> <li><strong>image_crop</strong> — <code>{"x": N, "y": N, "width": N, "height": N}</code></li> <li><strong>image_watermark</strong> — <code>{"text": "...", "position": "..."}</code></li> <li><strong>image_smart_crop</strong> — <code>{"width": N, "height": N}</code> (AI-assisted)</li> <li><strong>image_bg_remove</strong> — requires <code>REMOVEBG_API_KEY</code> env var</li> <li><strong>video_transcode</strong> — <code>{"format": "mp4", "codec": "h264"}</code></li> <li><strong>video_capture_image</strong> — <code>{"time_sec": N}</code></li> </ul> Returns a job ID immediately; poll <code>GET /api/v1/assets/:id/variants</code> to check completion. Returns 409 if a variant rebuild is already in progress.
// @Tags Variants
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param body body CreateVariantRequest true "Variant type and transform params"
// @Success 202 {object} CreateVariantResponse
// @Failure 400 {object} ErrorResponse "Invalid variant type or wrong asset type"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Failure 409 {object} ErrorResponse "Variant rebuild already in progress"
// @Failure 422 {object} ErrorResponse "Asset has no current version"
// @Router /api/v1/assets/{id}/variants [post]
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

	body, ok := decodeAndValidate(c, &CreateVariantRequest{})
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
		AssetID:     asset.ID,
		WorkspaceID: asset.WorkspaceID,
		VersionID:   currentVer.ID,
		VersionNum:  currentVer.VersionNum,
		StorageKey:  currentVer.StorageKey,
		MimeType:    currentVer.MimeType,
		Type:        body.Type,
		Params:      params,
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

	return c.Status(fiber.StatusAccepted).JSON(CreateVariantResponse{
		JobID:   job.ID,
		Status:  "pending",
		Message: "variant creation queued",
	})
}

// handleGetVariantFile streams the variant file bytes.
//
// @Summary Download variant file
// @Description Streams the variant's stored file. Content-Type is derived from the file extension. An <code>asset_variant_downloaded</code> audit event is recorded (browser image prefetch excluded).
// @Tags Variants
// @Produce application/octet-stream
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param vid path string true "Variant ID"
// @Success 200 {file} binary
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset, variant, or file not found"
// @Router /api/v1/assets/{id}/variants/{vid}/file [get]
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

	if setCacheHeaders(c, variant.ID, variant.CreatedAt, true) {
		return nil
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
	if variant.Size != nil && *variant.Size > 0 {
		c.Set("Content-Length", strconv.FormatInt(*variant.Size, 10))
	}
	return c.SendStream(rc)
}

// handleDeleteVariant deletes a variant and its stored file.
//
// @Summary Delete a variant
// @Description Permanently removes the variant record and its stored file. Only variants attached to the <em>current</em> version can be deleted manually — variants on older versions are removed automatically by the retention purge job.
// @Tags Variants
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param vid path string true "Variant ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset or variant not found"
// @Failure 422 {object} ErrorResponse "Variant belongs to a previous version"
// @Router /api/v1/assets/{id}/variants/{vid} [delete]
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

	if err := s.db.DeleteVariant(c.RequestCtx(), dbgen.DeleteVariantParams{
		ID:          variantID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete variant")
	}

	_ = s.storage.Delete(variant.StorageKey)

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

// handleUploadManualVariant attaches an externally produced file as a manual variant.
//
// @Summary Upload a manual variant
// @Description Accepts a multipart file upload and stores it as a variant of type <code>manual</code> on the asset's current version. Unlike transform-generated variants, manual variants are <em>not</em> automatically rebuilt when a new version is uploaded — they persist across version changes. Use this to attach pre-processed or third-party exports (e.g. color-corrected TIFF, print-ready PDF).
// @Tags Variants
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param file formData file true "File to attach as a manual variant"
// @Success 201 {object} VariantResponse
// @Failure 400 {object} ErrorResponse "file field is required"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Failure 422 {object} ErrorResponse "Asset has no current version"
// @Router /api/v1/assets/{id}/variants/upload [post]
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
	defer f.Close()

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

	return c.Status(fiber.StatusCreated).JSON(variantToResponse(assetID, v))
}

// handlePreviewTransform applies an in-memory image transform and returns the result.
//
// @Summary Preview image transform
// @Description Applies a resize/format transform to the asset's current version in memory and returns the result directly (never stored). Responses are cached in an LRU cache (100 entries, 5-minute TTL) so repeated identical calls are cheap. <br>Query parameters: <ul> <li><strong>w</strong> — Target width in pixels</li> <li><strong>h</strong> — Target height in pixels</li> <li><strong>fit</strong> — Fit mode: <code>contain</code> (default), <code>cover</code>, <code>fill</code></li> <li><strong>format</strong> — Output format: <code>jpeg</code> (default), <code>png</code>, <code>webp</code></li> <li><strong>q</strong> — JPEG quality 1–100 (default: encoder default)</li> </ul> Only supported for image assets. Returns 400 for video, audio, or PDF assets.
// @Tags Variants
// @Produce image/jpeg
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param w query int false "Target width"
// @Param h query int false "Target height"
// @Param fit query string false "Fit mode (contain|cover|fill)"
// @Param format query string false "Output format (jpeg|png|webp)"
// @Param q query int false "JPEG quality (1-100)"
// @Success 200 {file} binary
// @Failure 400 {object} ErrorResponse "Preview only supported for images"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/preview [get]
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

	cacheKey := fmt.Sprintf("%s|%s|w=%d|h=%d|fit=%s|format=%s|q=%d", asset.ID, *asset.CurrentVersionID, w, h, fit, format, q)

	// Check conditional request before doing any work (ETag = cacheKey hash).
	if setCacheHeaders(c, cacheKey, asset.UpdatedAt, false) {
		return nil
	}

	if cached, ct := s.previewCache.Get(cacheKey); cached != nil {
		c.Set("Content-Type", ct)
		return c.Send(cached)
	}

	rc, err := s.storage.Get(asset.StorageKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "asset file not found")
	}
	defer rc.Close()

	data, ct, err := transform.ImagePreview(rc, transform.PreviewParams{
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
	return c.Send(data)
}
