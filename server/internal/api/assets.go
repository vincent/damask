package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"creativo-dam/server/internal/auth"
	dbgen "creativo-dam/server/internal/db/gen"

	"github.com/disintegration/imaging"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type assetResponse struct {
	ID               string         `json:"id"`
	WorkspaceID      string         `json:"workspace_id"`
	ProjectID        sql.NullString `json:"project_id"`
	OriginalFilename string         `json:"original_filename"`
	MimeType         string         `json:"mime_type"`
	Size             int64          `json:"size"`
	Width            sql.NullInt64  `json:"width"`
	Height           sql.NullInt64  `json:"height"`
	ThumbnailKey     sql.NullString `json:"thumbnail_key"`
	Metadata         sql.NullString `json:"metadata"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type assetListResponse struct {
	Assets     []assetResponse `json:"assets"`
	NextCursor *string         `json:"next_cursor"`
}

func assetToResponse(a dbgen.Asset) assetResponse {
	return assetResponse{
		ID:               a.ID,
		WorkspaceID:      a.WorkspaceID,
		ProjectID:        a.ProjectID,
		OriginalFilename: a.OriginalFilename,
		MimeType:         a.MimeType,
		Size:             a.Size,
		Width:            a.Width,
		Height:           a.Height,
		ThumbnailKey:     a.ThumbnailKey,
		Metadata:         a.Metadata,
		CreatedAt:        a.CreatedAt,
		UpdatedAt:        a.UpdatedAt,
	}
}

func (s *Server) handleUploadAsset(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)

	fh, err := c.FormFile("file")
	if err != nil {
		return errRes(c, fiber.StatusBadRequest, "file field is required")
	}

	// Detect MIME type from magic bytes
	f, err := fh.Open()
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not open uploaded file")
	}
	sniff := make([]byte, 512)
	n, _ := f.Read(sniff)
	f.Close()
	mimeType := http.DetectContentType(sniff[:n])
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	assetID := uuid.New().String()
	originalFilename := filepath.Base(fh.Filename)
	storageKey := fmt.Sprintf("%s/%s/%s", claims.WorkspaceID, assetID, originalFilename)

	// Write file to storage
	f2, err := fh.Open()
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not read uploaded file")
	}
	defer f2.Close()

	if err := s.storage.Put(storageKey, f2); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not store file")
	}

	// Extract image dimensions
	var width, height sql.NullInt64
	if strings.HasPrefix(mimeType, "image/") {
		f3, err := fh.Open()
		if err == nil {
			cfg, _, decErr := image.DecodeConfig(f3)
			f3.Close()
			if decErr == nil {
				width = sql.NullInt64{Int64: int64(cfg.Width), Valid: true}
				height = sql.NullInt64{Int64: int64(cfg.Height), Valid: true}
			}
		}
	}

	asset, err := s.db.CreateAsset(c.Context(), dbgen.CreateAssetParams{
		ID:               assetID,
		WorkspaceID:      claims.WorkspaceID,
		OriginalFilename: originalFilename,
		StorageKey:       storageKey,
		MimeType:         mimeType,
		Size:             fh.Size,
		Width:            width,
		Height:           height,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not save asset")
	}

	if strings.HasPrefix(mimeType, "image/") {
		go s.generateThumbnail(context.Background(), asset.ID, claims.WorkspaceID, storageKey)
	}

	return c.Status(fiber.StatusCreated).JSON(assetToResponse(asset))
}

func (s *Server) generateThumbnail(ctx context.Context, assetID, workspaceID, storageKey string) {
	rc, err := s.storage.Get(storageKey)
	if err != nil {
		log.Printf("thumbnail: get file %s: %v", assetID, err)
		return
	}
	defer rc.Close()

	src, err := imaging.Decode(rc)
	if err != nil {
		log.Printf("thumbnail: decode %s: %v", assetID, err)
		return
	}

	thumb := imaging.Fit(src, 400, 400, imaging.Lanczos)

	var buf bytes.Buffer
	if err := imaging.Encode(&buf, thumb, imaging.JPEG, imaging.JPEGQuality(85)); err != nil {
		log.Printf("thumbnail: encode %s: %v", assetID, err)
		return
	}

	thumbKey := fmt.Sprintf("%s/%s/thumb.jpg", workspaceID, assetID)
	if err := s.storage.Put(thumbKey, &buf); err != nil {
		log.Printf("thumbnail: store %s: %v", assetID, err)
		return
	}

	if err := s.db.UpdateAssetThumbnail(ctx, dbgen.UpdateAssetThumbnailParams{
		ThumbnailKey: sql.NullString{String: thumbKey, Valid: true},
		ID:           assetID,
	}); err != nil {
		log.Printf("thumbnail: update db %s: %v", assetID, err)
	}
}

func (s *Server) handleListAssets(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)

	limit := int64(50)
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.ParseInt(l, 10, 64); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	q := c.Query("q")
	if q != "" {
		return s.handleSearchAssets(c, claims.WorkspaceID, q, limit)
	}

	params := dbgen.ListAssetsParams{
		WorkspaceID: claims.WorkspaceID,
		Limit:       limit,
	}

	if pid := c.Query("project_id"); pid != "" {
		params.ProjectID = pid
	}
	if mime := c.Query("mime"); mime != "" {
		params.MimePrefix = mime + "%"
	}

	if cursor := c.Query("cursor"); cursor != "" {
		at, id, err := decodeCursor(cursor)
		if err == nil {
			params.CursorAt = at.UTC().Format("2006-01-02 15:04:05")
			params.CursorID = sql.NullString{String: id, Valid: true}
		}
	}

	assets, err := s.db.ListAssets(c.Context(), params)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list assets")
	}

	return c.JSON(buildAssetListResponse(assets, limit))
}

func (s *Server) handleSearchAssets(c *fiber.Ctx, workspaceID, q string, limit int64) error {
	rows, err := s.sqlDB.QueryContext(c.Context(), `
		SELECT a.id, a.workspace_id, a.project_id, a.original_filename, a.storage_key,
		       a.mime_type, a.size, a.width, a.height, a.thumbnail_key, a.metadata,
		       a.created_at, a.updated_at
		FROM assets a
		WHERE a.workspace_id = ?
		  AND a.rowid IN (SELECT rowid FROM assets_fts WHERE assets_fts MATCH ?)
		ORDER BY a.created_at DESC
		LIMIT ?
	`, workspaceID, q+"*", limit)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "search failed")
	}
	defer rows.Close()

	var assets []dbgen.Asset
	for rows.Next() {
		var a dbgen.Asset
		if err := rows.Scan(
			&a.ID, &a.WorkspaceID, &a.ProjectID, &a.OriginalFilename, &a.StorageKey,
			&a.MimeType, &a.Size, &a.Width, &a.Height, &a.ThumbnailKey, &a.Metadata,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "scan failed")
		}
		assets = append(assets, a)
	}
	if err := rows.Err(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "search failed")
	}

	return c.JSON(buildAssetListResponse(assets, limit))
}

func buildAssetListResponse(assets []dbgen.Asset, limit int64) assetListResponse {
	items := make([]assetResponse, len(assets))
	for i, a := range assets {
		items[i] = assetToResponse(a)
	}
	var nextCursor *string
	if int64(len(assets)) == limit && len(assets) > 0 {
		last := assets[len(assets)-1]
		encoded := encodeCursor(last.CreatedAt, last.ID)
		nextCursor = &encoded
	}
	return assetListResponse{Assets: items, NextCursor: nextCursor}
}

func encodeCursor(t time.Time, id string) string {
	raw := t.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(cursor string) (time.Time, string, error) {
	b, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", err
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", errors.New("invalid cursor")
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", err
	}
	return t, parts[1], nil
}

func (s *Server) handleGetAsset(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	asset, err := s.db.GetAssetByID(c.Context(), dbgen.GetAssetByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	return c.JSON(assetToResponse(asset))
}

func (s *Server) handleGetAssetFile(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	asset, err := s.db.GetAssetByID(c.Context(), dbgen.GetAssetByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	rc, err := s.storage.Get(asset.StorageKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "file not found")
	}

	c.Set("Content-Type", asset.MimeType)
	c.Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, asset.OriginalFilename))
	return c.SendStream(rc)
}

func (s *Server) handleGetAssetThumb(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	asset, err := s.db.GetAssetByID(c.Context(), dbgen.GetAssetByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	if !asset.ThumbnailKey.Valid {
		return errRes(c, fiber.StatusNotFound, "thumbnail not ready")
	}

	rc, err := s.storage.Get(asset.ThumbnailKey.String)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "thumbnail not found")
	}

	c.Set("Content-Type", "image/jpeg")
	return c.SendStream(rc)
}

func (s *Server) handleDeleteAsset(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	asset, err := s.db.GetAssetByID(c.Context(), dbgen.GetAssetByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	_ = s.storage.Delete(asset.StorageKey)
	if asset.ThumbnailKey.Valid {
		_ = s.storage.Delete(asset.ThumbnailKey.String)
	}

	if err := s.db.DeleteAsset(c.Context(), dbgen.DeleteAssetParams{
		ID:          asset.ID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete asset")
	}

	return c.SendStatus(fiber.StatusNoContent)
}
