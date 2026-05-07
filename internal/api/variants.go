package api

import (
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
	"damask/server/internal/events"
	"damask/server/internal/jobs"
	"damask/server/internal/queue"
	"damask/server/internal/service"
	"damask/server/internal/storage"
	"damask/server/internal/systemtags"
	apptelemetry "damask/server/internal/telemetry"
	"damask/server/internal/transform"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

// ---- Response types ----

type VariantResponse struct {
	ID                   string    `json:"id"`
	AssetVersionID       string    `json:"asset_version_id"`
	Type                 string    `json:"type"`
	TransformParams      *string   `json:"transform_params"`
	Size                 *int64    `json:"size"`
	StorageKey           string    `json:"storage_key"`
	DownloadURL          string    `json:"download_url"`
	ThumbnailURL         *string   `json:"thumbnail_url"`
	ThumbnailContentType string    `json:"thumbnail_content_type"`
	CreatedAt            time.Time `json:"created_at"`
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

type WatermarkAssetResponse struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	StorageKey   string  `json:"storage_key"`
	MimeType     string  `json:"mime_type"`
	ThumbnailURL *string `json:"thumbnail_url"`
	Scope        string  `json:"scope"`
}

func variantDTOToResponse(assetID string, v *service.VariantDTO) VariantResponse {
	var thumbURL *string
	if v.ThumbnailKey != nil {
		u := fmt.Sprintf("/api/v1/assets/%s/variants/%s/thumb", assetID, v.ID)
		thumbURL = &u
	}
	ct := v.ThumbnailContentType
	if ct == "" {
		ct = "image/jpeg"
	}
	return VariantResponse{
		ID:                   v.ID,
		AssetVersionID:       v.AssetVersionID,
		Type:                 v.Type,
		TransformParams:      v.TransformParams,
		Size:                 v.Size,
		StorageKey:           v.StorageKey,
		DownloadURL:          fmt.Sprintf("/api/v1/assets/%s/variants/%s/file", assetID, v.ID),
		ThumbnailURL:         thumbURL,
		ThumbnailContentType: ct,
		CreatedAt:            v.CreatedAt,
	}
}

func systemTagAssetToWatermarkResponse(v *service.AssetDTO, scope string) WatermarkAssetResponse {
	u := fmt.Sprintf("/api/v1/assets/%s/thumb", v.ID)
	return WatermarkAssetResponse{
		ID:           v.ID,
		Name:         v.OriginalFilename,
		StorageKey:   v.StorageKey,
		MimeType:     v.MimeType,
		ThumbnailURL: &u,
		Scope:        scope,
	}
}

// isRebuildingVariants returns true when a rebuild_variants job for the given
// version is in pending or processing state.
func (s *Server) isRebuildingVariants(c fiber.Ctx, versionID string) bool {
	rebuilding, err := s.assets.IsRebuildingVariants(c.Context(), versionID)
	if err != nil {
		slog.ErrorContext(c, "is_rebuilding_variants", "error", err)
	}
	return rebuilding
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

	asset, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	variants, err := s.variants.List(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	out := make([]VariantResponse, len(variants))
	for i, v := range variants {
		out[i] = variantDTOToResponse(assetID, v)
	}

	rebuilding := false
	if asset.CurrentVersionID != nil {
		rebuilding = s.isRebuildingVariants(c, *asset.CurrentVersionID)
	}

	return c.JSON(ListVariantsResponse{
		Variants:   out,
		Rebuilding: rebuilding,
	})
}

func (s *Server) handleResolveWatermarkAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	asset, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	wm, err := s.tags.ResolveSystemTag(c.Context(), claims.WorkspaceID, systemtags.Watermark, service.SystemTagScope{
		FolderID:  asset.FolderID,
		ProjectID: asset.ProjectID,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	if wm == nil {
		return errRes(c, fiber.StatusUnprocessableEntity, "no_watermark_asset")
	}
	scope := "workspace"
	if asset.FolderID != nil && wm.FolderID != nil && *asset.FolderID == *wm.FolderID {
		scope = "folder"
	} else if asset.ProjectID != nil && wm.ProjectID != nil && *asset.ProjectID == *wm.ProjectID {
		scope = "project"
	}
	return c.JSON(systemTagAssetToWatermarkResponse(wm, scope))
}

// handleCreateVariant enqueues a transform job to produce a new variant.
//
// @Summary Create a variant
// @Description Enqueues a background job to generate a transformed variant of the asset's current version. Supported types and their required params: <ul> <li><strong>image_resize</strong> — <code>{"width": N, "height": N, "fit": "contain|cover|fill"}</code></li> <li><strong>image_convert</strong> — <code>{"format": "jpeg|png|webp|avif"}</code></li> <li><strong>image_crop</strong> — <code>{"x": N, "y": N, "width": N, "height": N}</code></li> <li><strong>image_watermark</strong> — <code>{"opacity": 0.5}</code></li> <li><strong>image_smart_crop</strong> — <code>{"width": N, "height": N}</code> (AI-assisted)</li> <li><strong>image_bg_remove</strong> — <code>{"model": "bria/remove-background"}</code></li> <li><strong>image_with_prompt</strong> — <code>{"prompt": "...", "model": "black-forest-labs/FLUX.1-fill-dev"}</code></li> <li><strong>video_transcode</strong> — <code>{"format": "mp4", "codec": "h264"}</code></li> <li><strong>video_watermark</strong> — <code>{"opacity": 0.5, "format": "mp4"}</code></li> <li><strong>video_capture_image</strong> — <code>{"time_sec": N}</code></li> </ul> Returns a job ID immediately; poll <code>GET /api/v1/assets/:id/variants</code> to check completion. Returns 409 if a variant rebuild is already in progress.
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

	asset, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if asset.CurrentVersionID == nil {
		return errRes(c, fiber.StatusUnprocessableEntity, "asset has no current version")
	}

	if s.isRebuildingVariants(c, *asset.CurrentVersionID) {
		return errRes(c, fiber.StatusConflict, "variants are rebuilding — please wait for the rebuild to complete before creating new variants")
	}

	body, ok := decodeAndValidate(c, &CreateVariantRequest{})
	if !ok {
		return nil
	}

	if isDemoSession(c) && (body.Type == queue.JobTypeImageBgRemove || body.Type == queue.JobTypeImageWithPrompt) {
		return c.Status(fiber.StatusForbidden).JSON(demoRestrictedResponse)
	}

	currentVer, err := s.versions.GetCurrentByAsset(c.Context(), assetID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load current version")
	}

	prepared, err := s.variants.PrepareCreate(c.Context(), service.PrepareCreateVariantParams{
		WorkspaceID:           claims.WorkspaceID,
		AssetID:               assetID,
		Type:                  body.Type,
		Params:                body.Params,
		AssetMimeType:         asset.MimeType,
		ImageRouterConfigured: s.cfg.ImageRouter.IsConfigured(),
		DefaultImageModel:     s.cfg.ImageRouter.DefaultModel,
		DefaultBgRemoveModel:  s.cfg.ImageRouter.DefaultBgRemoveModel,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidVariantType) {
			return errRes(c, fiber.StatusBadRequest, "invalid variant type")
		}
		if errors.Is(err, service.ErrInvalidVariantReq) {
			return errRes(c, fiber.StatusBadRequest, err.Error())
		}
		return ErrorStatusResponse(c, err)
	}

	payload, _ := json.Marshal(jobs.VariantJobPayload{
		AssetID:     asset.ID,
		WorkspaceID: asset.WorkspaceID,
		VersionID:   currentVer.ID,
		VersionNum:  currentVer.VersionNum,
		StorageKey:  currentVer.StorageKey,
		MimeType:    currentVer.MimeType,
		Type:        prepared.Type,
		Params:      prepared.Params,
	})

	_, enqueueSpan := apptelemetry.StartSpan(c.Context(), "api.variants.enqueue_create",
		attribute.String("damask.workspace_id", claims.WorkspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.job.type", prepared.Type),
	)
	job, err := s.queue.Enqueue(c.Context(), claims.WorkspaceID, prepared.Type, string(payload))
	apptelemetry.EndSpan(enqueueSpan, err)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not enqueue job")
	}

	s.variants.WriteVariantQueued(c.Context(), claims.WorkspaceID, assetID, prepared.Type)

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

	if _, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	variant, err := s.variants.Get(c.Context(), claims.WorkspaceID, variantID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if setCacheHeaders(c, variant.ID, variant.CreatedAt, true) {
		return nil
	}

	rc, err := s.storage.Get(variant.StorageKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "variant file not found")
	}

	if !audit.IsBrowserPrefetch(c.Get("Sec-Fetch-Dest")) {
		s.variants.WriteVariantDownloadedAsync(claims.WorkspaceID, assetID, variantID, variant.Type)
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

	variant, err := s.variants.Get(c.Context(), claims.WorkspaceID, variantID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if err := s.variants.Delete(c.Context(), claims.WorkspaceID, assetID, variantID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	_ = s.storage.Delete(variant.StorageKey)
	if variant.ThumbnailKey != nil {
		_ = s.storage.Delete(*variant.ThumbnailKey)
	}

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

	asset, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if asset.CurrentVersionID == nil {
		return errRes(c, fiber.StatusUnprocessableEntity, "asset has no current version")
	}

	currentVer, err := s.versions.GetCurrentByAsset(c.Context(), assetID)
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
	storageKey := storage.VersionedVariantKey(
		asset.WorkspaceID, assetID, currentVer.VersionNum,
		"manual", variantID[:8], ext,
	)

	_, putSpan := apptelemetry.StartSpan(c.Context(), "api.variants.manual_storage_put",
		attribute.String("damask.workspace_id", claims.WorkspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.storage.key", storageKey),
		attribute.Int64("damask.upload.bytes", fh.Size),
	)
	err = s.storage.Put(storageKey, f)
	apptelemetry.EndSpan(putSpan, err)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not store file")
	}

	sz := fh.Size
	emptyParams := "{}"
	uploadedMimeType := mime.TypeByExtension(ext)
	if uploadedMimeType == "" {
		uploadedMimeType = "application/octet-stream"
	}

	v, err := s.variants.Create(c.Context(), service.CreateVariantParams{
		ID:              variantID,
		WorkspaceID:     asset.WorkspaceID,
		AssetID:         assetID,
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

	s.hub.Publish(c.Context(), asset.WorkspaceID, events.Event{
		Type:      "variant_ready",
		AssetID:   assetID,
		VariantID: variantID,
	})

	if thumbPayload, err := json.Marshal(jobs.VariantThumbnailJobPayload{
		VariantID:   variantID,
		WorkspaceID: asset.WorkspaceID,
		AssetID:     assetID,
		StorageKey:  storageKey,
		MimeType:    uploadedMimeType,
	}); err == nil {
		_, _ = s.queue.Enqueue(c.Context(), asset.WorkspaceID, queue.JobTypeVariantThumbnail, string(thumbPayload))
	}

	return c.Status(fiber.StatusCreated).JSON(variantDTOToResponse(assetID, v))
}

// handleGetVariantThumb handles GET /api/v1/assets/:id/variants/:vid/thumb
// Returns 202 while the thumbnail job is still processing, streams the file once ready.
func (s *Server) handleGetVariantThumb(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	variantID := c.Params("vid")

	variant, err := s.variants.Get(c.Context(), claims.WorkspaceID, variantID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if variant.ThumbnailKey == nil {
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"status": "processing"})
	}

	_, getSpan := apptelemetry.StartSpan(c.Context(), "api.variants.thumbnail_storage_get",
		attribute.String("damask.variant_id", variantID),
		attribute.String("damask.storage.key", *variant.ThumbnailKey),
	)
	rc, err := s.storage.Get(*variant.ThumbnailKey)
	apptelemetry.EndSpan(getSpan, err)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "thumbnail not found")
	}

	ct := variant.ThumbnailContentType
	if ct == "" {
		ct = "image/jpeg"
	}
	c.Set("Content-Type", ct)
	return c.SendStream(rc)
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

	asset, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
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

	_, getSpan := apptelemetry.StartSpan(c.Context(), "api.variants.preview_storage_get",
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.storage.key", asset.StorageKey),
	)
	rc, err := s.storage.Get(asset.StorageKey)
	apptelemetry.EndSpan(getSpan, err)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "asset file not found")
	}
	defer rc.Close()

	_, previewSpan := apptelemetry.StartSpan(c.Context(), "api.variants.preview_transform",
		attribute.String("damask.asset_id", assetID),
		attribute.Int("damask.preview.width", w),
		attribute.Int("damask.preview.height", h),
		attribute.String("damask.preview.fit", fit),
		attribute.String("damask.preview.format", format),
	)
	data, ct, err := s.trf.ImagePreview(rc, transform.PreviewParams{
		Width:   w,
		Height:  h,
		Fit:     fit,
		Quality: q,
		Format:  format,
	})
	previewSpan.SetAttributes(attribute.Int("damask.preview.bytes", len(data)))
	apptelemetry.EndSpan(previewSpan, err)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "preview generation failed")
	}

	s.previewCache.Set(cacheKey, data, ct)
	c.Set("Content-Type", ct)
	return c.Send(data)
}
