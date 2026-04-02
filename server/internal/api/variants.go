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

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/transform"

	"github.com/gofiber/fiber/v3"
)

// ---- Response types ----

type variantResponse struct {
	ID              string    `json:"id"`
	AssetID         string    `json:"asset_id"`
	Type            string    `json:"type"`
	TransformParams *string   `json:"transform_params"`
	Size            *int64    `json:"size"`
	StorageKey      string    `json:"storage_key"`
	DownloadURL     string    `json:"download_url"`
	CreatedAt       time.Time `json:"created_at"`
}

func (s *Server) variantToResponse(v dbgen.Variant) variantResponse {
	return variantResponse{
		ID:              v.ID,
		AssetID:         v.AssetID,
		Type:            v.Type,
		TransformParams: v.TransformParams,
		Size:            v.Size,
		StorageKey:      v.StorageKey,
		DownloadURL:     fmt.Sprintf("/api/v1/assets/%s/variants/%s/file", v.AssetID, v.ID),
		CreatedAt:       v.CreatedAt,
	}
}

// ---- Handlers ----

func (s *Server) handleListVariants(c fiber.Ctx) error {
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

	variants, err := s.db.ListVariants(c.RequestCtx(), dbgen.ListVariantsParams{
		AssetID:     assetID,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list variants")
	}

	out := make([]variantResponse, len(variants))
	for i, v := range variants {
		out[i] = s.variantToResponse(v)
	}
	return c.JSON(out)
}

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

	var body struct {
		Type   string          `json:"type"`
		Params json.RawMessage `json:"params"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}

	validTypes := map[string]bool{
		queue.JobTypeImageResize:    true,
		queue.JobTypeImageWatermark: true,
		queue.JobTypeImageConvert:   true,
		queue.JobTypeImageCrop:      true,
		queue.JobTypeVideoThumbnail: true,
		queue.JobTypeVideoTranscode: true,
		queue.JobTypeImageBgRemove:  true,
	}
	if !validTypes[body.Type] {
		return errRes(c, fiber.StatusBadRequest, "invalid variant type")
	}

	if (body.Type == queue.JobTypeVideoThumbnail || body.Type == queue.JobTypeVideoTranscode) &&
		!strings.HasPrefix(asset.MimeType, "video/") {
		return errRes(c, fiber.StatusBadRequest, "video transforms require a video asset")
	}
	if (body.Type == queue.JobTypeImageResize || body.Type == queue.JobTypeImageConvert || body.Type == queue.JobTypeImageCrop || body.Type == queue.JobTypeImageWatermark) &&
		!strings.HasPrefix(asset.MimeType, "image/") {
		return errRes(c, fiber.StatusBadRequest, "image transforms require an image asset")
	}
	if body.Type == queue.JobTypeImageBgRemove {
		if s.removeBgAPIKey == "" {
			return errRes(c, fiber.StatusBadRequest, "background removal requires REMOVEBG_API_KEY to be configured")
		}
		if !strings.HasPrefix(asset.MimeType, "image/") {
			return errRes(c, fiber.StatusBadRequest, "background removal requires an image asset")
		}
	}

	params := json.RawMessage("{}")
	if len(body.Params) > 0 {
		params = body.Params
	}

	payload, _ := json.Marshal(variantJobPayload{
		AssetID:     asset.ID,
		WorkspaceID: asset.WorkspaceID,
		StorageKey:  asset.StorageKey,
		MimeType:    asset.MimeType,
		Type:        body.Type,
		Params:      params,
	})

	job, err := s.queue.Enqueue(c.RequestCtx(), claims.WorkspaceID, body.Type, string(payload))
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not enqueue job")
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"job_id":  job.ID,
		"status":  "pending",
		"message": "variant creation queued",
	})
}

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

	ext := strings.ToLower(filepath.Ext(variant.StorageKey))
	c.Set("Content-Type", mime.TypeByExtension(ext))
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s_%s%s"`, assetID[:8], variantID[:8], ext))
	return c.SendStream(rc)
}

func (s *Server) handleDeleteVariant(c fiber.Ctx) error {
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

	_ = s.storage.Delete(variant.StorageKey)

	if err := s.db.DeleteVariant(c.RequestCtx(), dbgen.DeleteVariantParams{
		ID:          variantID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete variant")
	}

	return c.SendStatus(fiber.StatusNoContent)
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

// fiber:context-methods migrated
