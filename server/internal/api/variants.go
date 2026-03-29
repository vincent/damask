package api

import (
	"bytes"
	"container/list"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"badam-dam/server/internal/auth"
	dbgen "badam-dam/server/internal/db/gen"
	"badam-dam/server/internal/queue"
	"badam-dam/server/internal/transform"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// ---- Response types ----

type variantResponse struct {
	ID              string         `json:"id"`
	AssetID         string         `json:"asset_id"`
	Type            string         `json:"type"`
	TransformParams sql.NullString `json:"transform_params"`
	Size            sql.NullInt64  `json:"size"`
	StorageKey      string         `json:"storage_key"`
	DownloadURL     string         `json:"download_url"`
	CreatedAt       time.Time      `json:"created_at"`
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
		queue.JobTypeResize:         true,
		queue.JobTypeWatermark:      true,
		queue.JobTypeConvert:        true,
		queue.JobTypeCrop:           true,
		queue.JobTypeVideoThumbnail: true,
		queue.JobTypeVideoTranscode: true,
		queue.JobTypeBgRemove:       true,
	}
	if !validTypes[body.Type] {
		return errRes(c, fiber.StatusBadRequest, "invalid variant type")
	}

	if (body.Type == queue.JobTypeVideoThumbnail || body.Type == queue.JobTypeVideoTranscode) &&
		!strings.HasPrefix(asset.MimeType, "video/") {
		return errRes(c, fiber.StatusBadRequest, "video transforms require a video asset")
	}
	if (body.Type == queue.JobTypeResize || body.Type == queue.JobTypeConvert || body.Type == queue.JobTypeCrop || body.Type == queue.JobTypeWatermark) &&
		!strings.HasPrefix(asset.MimeType, "image/") {
		return errRes(c, fiber.StatusBadRequest, "image transforms require an image asset")
	}
	if body.Type == queue.JobTypeBgRemove {
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
	c.Set("Content-Type", extToMime(ext))
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

// ---- Job handlers ----

// RegisterJobHandlers wires transform job handlers into the queue.
func (s *Server) RegisterJobHandlers() {
	s.queue.Register(queue.JobTypeThumbnail, s.jobThumbnail)
	s.queue.Register(queue.JobTypeResize, s.jobImageTransform)
	s.queue.Register(queue.JobTypeConvert, s.jobImageTransform)
	s.queue.Register(queue.JobTypeCrop, s.jobImageTransform)
	s.queue.Register(queue.JobTypeWatermark, s.jobImageTransform)
	s.queue.Register(queue.JobTypeVideoThumbnail, s.jobVideoThumbnail)
	s.queue.Register(queue.JobTypeVideoTranscode, s.jobVideoTranscode)
	s.queue.Register(queue.JobTypeBgRemove, s.jobBgRemove)
}

type variantJobPayload struct {
	AssetID     string          `json:"asset_id"`
	WorkspaceID string          `json:"workspace_id"`
	StorageKey  string          `json:"storage_key"`
	MimeType    string          `json:"mime_type"`
	Type        string          `json:"type"`
	Params      json.RawMessage `json:"params"`
}

type thumbnailJobPayload struct {
	AssetID     string `json:"asset_id"`
	WorkspaceID string `json:"workspace_id"`
	StorageKey  string `json:"storage_key"`
}

func (s *Server) jobThumbnail(ctx context.Context, job dbgen.Job) error {
	var p thumbnailJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	rc, err := s.storage.Get(p.StorageKey)
	if err != nil {
		return fmt.Errorf("get file: %w", err)
	}
	defer rc.Close()

	data, _, err := transform.Resize(rc, transform.ResizeParams{
		Width:   400,
		Height:  400,
		Fit:     "contain",
		Quality: 85,
		Format:  "jpeg",
	})
	if err != nil {
		return fmt.Errorf("resize: %w", err)
	}

	thumbKey := fmt.Sprintf("%s/%s/thumb.jpg", p.WorkspaceID, p.AssetID)
	if err := s.storage.Put(thumbKey, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("store thumb: %w", err)
	}

	return s.db.UpdateAssetThumbnail(ctx, dbgen.UpdateAssetThumbnailParams{
		ThumbnailKey: sql.NullString{String: thumbKey, Valid: true},
		ID:           p.AssetID,
	})
}

func (s *Server) jobImageTransform(ctx context.Context, job dbgen.Job) error {
	var p variantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	rc, err := s.storage.Get(p.StorageKey)
	if err != nil {
		return fmt.Errorf("get file: %w", err)
	}
	defer rc.Close()

	var data []byte
	var contentType string

	switch job.Type {
	case queue.JobTypeResize:
		var params transform.ResizeParams
		if err := json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse resize params: %w", err)
		}
		data, contentType, err = transform.Resize(rc, params)
	case queue.JobTypeConvert:
		var params transform.ConvertParams
		if err := json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse convert params: %w", err)
		}
		data, contentType, err = transform.Convert(rc, params)
	case queue.JobTypeCrop:
		var params transform.CropParams
		if err := json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse crop params: %w", err)
		}
		data, contentType, err = transform.Crop(rc, params)
	case queue.JobTypeWatermark:
		var params transform.WatermarkParams
		if err := json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse watermark params: %w", err)
		}
		data, contentType, err = transform.Watermark(rc, params)
	default:
		return fmt.Errorf("unknown image job type: %s", job.Type)
	}
	if err != nil {
		return fmt.Errorf("transform: %w", err)
	}

	ext := mimeToExt(contentType)
	variantID := uuid.NewString()
	storageKey := fmt.Sprintf("%s/%s/variants/%s%s", p.WorkspaceID, p.AssetID, variantID, ext)

	if err := s.storage.Put(storageKey, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	paramsJSON := string(p.Params)
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              variantID,
		AssetID:         p.AssetID,
		WorkspaceID:     p.WorkspaceID,
		Type:            job.Type,
		StorageKey:      storageKey,
		TransformParams: sql.NullString{String: paramsJSON, Valid: true},
		Size:            sql.NullInt64{Int64: int64(len(data)), Valid: true},
	})
	return err
}

func (s *Server) jobVideoThumbnail(ctx context.Context, job dbgen.Job) error {
	if !transform.FFmpegAvailable() {
		return fmt.Errorf("ffmpeg not found in PATH")
	}

	var p variantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	var params transform.VideoThumbnailParams
	if len(p.Params) > 0 {
		_ = json.Unmarshal(p.Params, &params)
	}

	srcExt := filepath.Ext(p.StorageKey)
	tmpPath, cleanup, err := s.writeToTempFile(ctx, p.StorageKey, srcExt)
	if err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	defer cleanup()

	data, err := transform.ExtractVideoThumbnail(ctx, tmpPath, params)
	if err != nil {
		return fmt.Errorf("extract thumbnail: %w", err)
	}

	variantID := uuid.NewString()
	storageKey := fmt.Sprintf("%s/%s/variants/%s.jpg", p.WorkspaceID, p.AssetID, variantID)
	if err := s.storage.Put(storageKey, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	// Set as asset thumbnail if none yet.
	asset, _ := s.db.GetAssetByID(ctx, dbgen.GetAssetByIDParams{ID: p.AssetID, WorkspaceID: p.WorkspaceID})
	if !asset.ThumbnailKey.Valid {
		_ = s.db.UpdateAssetThumbnail(ctx, dbgen.UpdateAssetThumbnailParams{
			ThumbnailKey: sql.NullString{String: storageKey, Valid: true},
			ID:           p.AssetID,
		})
	}

	paramsJSON, _ := json.Marshal(params)
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              variantID,
		AssetID:         p.AssetID,
		WorkspaceID:     p.WorkspaceID,
		Type:            queue.JobTypeVideoThumbnail,
		StorageKey:      storageKey,
		TransformParams: sql.NullString{String: string(paramsJSON), Valid: true},
		Size:            sql.NullInt64{Int64: int64(len(data)), Valid: true},
	})
	return err
}

func (s *Server) jobVideoTranscode(ctx context.Context, job dbgen.Job) error {
	if !transform.FFmpegAvailable() {
		return fmt.Errorf("ffmpeg not found in PATH")
	}

	var p variantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	var params transform.TranscodeParams
	if len(p.Params) > 0 {
		if err := json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse transcode params: %w", err)
		}
	}
	if params.Format == "" {
		params.Format = "mp4"
	}

	srcExt := filepath.Ext(p.StorageKey)
	srcPath, cleanSrc, err := s.writeToTempFile(ctx, p.StorageKey, srcExt)
	if err != nil {
		return fmt.Errorf("write src temp: %w", err)
	}
	defer cleanSrc()

	ext := transform.TranscodeExtension(params.Format)
	dstPath := srcPath + "_out" + ext
	defer removeFile(dstPath)

	if err := transform.TranscodeVideo(ctx, srcPath, dstPath, params); err != nil {
		return fmt.Errorf("transcode: %w", err)
	}

	dstData, err := readFile(dstPath)
	if err != nil {
		return fmt.Errorf("read output: %w", err)
	}

	variantID := uuid.NewString()
	storageKey := fmt.Sprintf("%s/%s/variants/%s%s", p.WorkspaceID, p.AssetID, variantID, ext)
	if err := s.storage.Put(storageKey, bytes.NewReader(dstData)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	paramsJSON, _ := json.Marshal(params)
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              variantID,
		AssetID:         p.AssetID,
		WorkspaceID:     p.WorkspaceID,
		Type:            queue.JobTypeVideoTranscode,
		StorageKey:      storageKey,
		TransformParams: sql.NullString{String: string(paramsJSON), Valid: true},
		Size:            sql.NullInt64{Int64: int64(len(dstData)), Valid: true},
	})
	return err
}

func (s *Server) jobBgRemove(ctx context.Context, job dbgen.Job) error {
	var p variantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	rc, err := s.storage.Get(p.StorageKey)
	if err != nil {
		return fmt.Errorf("get file: %w", err)
	}
	defer rc.Close()

	imgData, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	result, err := transform.RemoveBackground(ctx, imgData, s.removeBgAPIKey)
	if err != nil {
		return fmt.Errorf("remove background: %w", err)
	}

	variantID := uuid.NewString()
	storageKey := fmt.Sprintf("%s/%s/variants/%s.png", p.WorkspaceID, p.AssetID, variantID)
	if err := s.storage.Put(storageKey, bytes.NewReader(result)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:          variantID,
		AssetID:     p.AssetID,
		WorkspaceID: p.WorkspaceID,
		Type:        queue.JobTypeBgRemove,
		StorageKey:  storageKey,
		Size:        sql.NullInt64{Int64: int64(len(result)), Valid: true},
	})
	return err
}

// ---- OS helpers ----

func (s *Server) writeToTempFile(ctx context.Context, storageKey, ext string) (string, func(), error) {
	rc, err := s.storage.Get(storageKey)
	if err != nil {
		return "", nil, err
	}
	defer rc.Close()

	f, err := os.CreateTemp("", "badam-*"+ext)
	if err != nil {
		return "", nil, err
	}
	if _, err := io.Copy(f, rc); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, err
	}
	f.Close()
	return f.Name(), func() { os.Remove(f.Name()) }, nil
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func removeFile(path string) {
	_ = os.Remove(path)
}

// ---- Mime helpers ----

func extToMime(ext string) string {
	switch ext {
	case ".png":
		return "image/png"
	case ".tiff", ".tif":
		return "image/tiff"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	default:
		return "image/jpeg"
	}
}

func mimeToExt(ct string) string {
	switch ct {
	case "image/png":
		return ".png"
	case "image/tiff":
		return ".tiff"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	default:
		return ".jpg"
	}
}

// ---- LRU preview cache ----

type previewCacheEntry struct {
	key         string
	data        []byte
	contentType string
}

type lruPreviewCache struct {
	mu      sync.Mutex
	items   map[string]*list.Element
	order   *list.List
	maxSize int
}

func newLRUPreviewCache(maxSize int) *lruPreviewCache {
	return &lruPreviewCache{
		items:   make(map[string]*list.Element),
		order:   list.New(),
		maxSize: maxSize,
	}
}

func (c *lruPreviewCache) Get(key string) ([]byte, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, ""
	}
	c.order.MoveToFront(el)
	entry := el.Value.(*previewCacheEntry)
	return entry.data, entry.contentType
}

func (c *lruPreviewCache) Set(key string, data []byte, contentType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.order.MoveToFront(el)
		el.Value.(*previewCacheEntry).data = data
		return
	}
	entry := &previewCacheEntry{key: key, data: data, contentType: contentType}
	el := c.order.PushFront(entry)
	c.items[key] = el

	for c.order.Len() > c.maxSize {
		back := c.order.Back()
		if back == nil {
			break
		}
		c.order.Remove(back)
		delete(c.items, back.Value.(*previewCacheEntry).key)
	}
}

// fiber:context-methods migrated
