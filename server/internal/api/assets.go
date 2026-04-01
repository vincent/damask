package api

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
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

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type assetResponse struct {
	ID               string    `json:"id"`
	WorkspaceID      string    `json:"workspace_id"`
	ProjectID        *string   `json:"project_id"`
	OriginalFilename string    `json:"original_filename"`
	MimeType         string    `json:"mime_type"`
	Size             int64     `json:"size"`
	Width            *int64    `json:"width"`
	Height           *int64    `json:"height"`
	ThumbnailKey     *string   `json:"thumbnail_key"`
	Metadata         *string   `json:"metadata"`
	Tags             []string  `json:"tags"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type assetListResponse struct {
	Assets     []assetResponse `json:"assets"`
	NextCursor *string         `json:"next_cursor"`
}

func assetToResponse(a dbgen.Asset, tags []string) assetResponse {
	if tags == nil {
		tags = []string{}
	}
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
		Tags:             tags,
		CreatedAt:        a.CreatedAt,
		UpdatedAt:        a.UpdatedAt,
	}
}

func (s *Server) handleUploadAsset(c fiber.Ctx) error {
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
	var width, height *int64
	if strings.HasPrefix(mimeType, "image/") {
		f3, err := fh.Open()
		if err == nil {
			cfg, _, decErr := image.DecodeConfig(f3)
			f3.Close()
			if decErr == nil {
				w, h := int64(cfg.Width), int64(cfg.Height)
				width, height = &w, &h
			}
		}
	}

	asset, err := s.db.CreateAsset(c.RequestCtx(), dbgen.CreateAssetParams{
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
		payload, _ := json.Marshal(thumbnailJobPayload{
			AssetID:     asset.ID,
			WorkspaceID: claims.WorkspaceID,
			StorageKey:  storageKey,
		})
		if _, err := s.queue.Enqueue(context.Background(), claims.WorkspaceID, queue.JobTypeThumbnail, string(payload)); err != nil {
			log.Printf("thumbnail: enqueue %s: %v", asset.ID, err)
		}
	}

	return c.Status(fiber.StatusCreated).JSON(assetToResponse(asset, nil))
}

func (s *Server) handleListAssets(c fiber.Ctx) error {
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

	// Tag filter — AND logic across multiple tags
	if tagsParam := c.Query("tags"); tagsParam != "" {
		tagNames := strings.Split(tagsParam, ",")
		for i, t := range tagNames {
			tagNames[i] = strings.TrimSpace(strings.ToLower(t))
		}
		return s.handleListAssetsByTags(c, claims.WorkspaceID, tagNames, limit)
	}

	// Folder filter
	if folderID := c.Query("folder_id"); folderID != "" {
		isRoot := folderID == "root"
		projectID := c.Query("project_id")
		return s.handleListAssetsInFolder(c, claims.WorkspaceID, folderID, isRoot, projectID, limit)
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
			params.CursorID = &id
		}
	}

	assets, err := s.db.ListAssets(c.RequestCtx(), params)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list assets")
	}

	return c.JSON(buildAssetListResponse(assets, limit))
}

func (s *Server) handleListAssetsByTags(c fiber.Ctx, workspaceID string, tagNames []string, limit int64) error {
	// Build placeholders for IN clause — tag names must come BEFORE the HAVING count arg
	placeholders := make([]string, len(tagNames))
	args := []interface{}{workspaceID, workspaceID}
	for i, name := range tagNames {
		placeholders[i] = "?"
		args = append(args, name)
	}
	args = append(args, int64(len(tagNames)))

	// Optional cursor
	var cursorClause string
	if cursor := c.Query("cursor"); cursor != "" {
		at, id, err := decodeCursor(cursor)
		if err == nil {
			cursorClause = "AND (a.created_at < ? OR (a.created_at = ? AND a.id < ?))"
			args = append(args, at.UTC().Format("2006-01-02 15:04:05"), at.UTC().Format("2006-01-02 15:04:05"), id)
		}
	}

	args = append(args, limit)

	query := fmt.Sprintf(`
		SELECT a.id, a.workspace_id, a.project_id, a.original_filename, a.storage_key,
		       a.mime_type, a.size, a.width, a.height, a.thumbnail_key, a.metadata,
		       a.created_at, a.updated_at
		FROM assets a
		WHERE a.workspace_id = ?
		  AND a.id IN (
		    SELECT at.asset_id FROM asset_tags at
		    JOIN tags t ON t.id = at.tag_id
		    WHERE t.workspace_id = ? AND t.name IN (%s)
		    GROUP BY at.asset_id HAVING COUNT(DISTINCT t.id) = ?
		  )
		  %s
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT ?
	`, strings.Join(placeholders, ","), cursorClause)

	rows, err := s.sqlDB.QueryContext(c.RequestCtx(), query, args...)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list assets")
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
		return errRes(c, fiber.StatusInternalServerError, "query failed")
	}

	return c.JSON(buildAssetListResponse(assets, limit))
}

func (s *Server) handleSearchAssets(c fiber.Ctx, workspaceID, q string, limit int64) error {
	rows, err := s.sqlDB.QueryContext(c.RequestCtx(), `
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
		items[i] = assetToResponse(a, nil)
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

func (s *Server) handleGetAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	asset, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	tagRows, err := s.db.GetTagsForAsset(c.RequestCtx(), id)
	if err != nil {
		tagRows = nil
	}
	tagNames := make([]string, len(tagRows))
	for i, t := range tagRows {
		tagNames[i] = t.Name
	}

	return c.JSON(assetToResponse(asset, tagNames))
}

func (s *Server) handleGetAssetFile(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	asset, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
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

func (s *Server) handleGetAssetThumb(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	asset, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	if asset.ThumbnailKey == nil {
		return errRes(c, fiber.StatusNotFound, "thumbnail not ready")
	}

	rc, err := s.storage.Get(*asset.ThumbnailKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "thumbnail not found")
	}

	c.Set("Content-Type", "image/jpeg")
	return c.SendStream(rc)
}

func (s *Server) handleDeleteAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	asset, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
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
	if asset.ThumbnailKey != nil {
		_ = s.storage.Delete(*asset.ThumbnailKey)
	}

	if err := s.db.DeleteAsset(c.RequestCtx(), dbgen.DeleteAssetParams{
		ID:          asset.ID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete asset")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// handleBulkTag adds a tag to multiple assets at once.
func (s *Server) handleBulkTag(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	var body struct {
		AssetIDs []string `json:"asset_ids"`
		TagName  string   `json:"tag_name"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	body.TagName = strings.TrimSpace(strings.ToLower(body.TagName))
	if body.TagName == "" || len(body.AssetIDs) == 0 {
		return errRes(c, fiber.StatusBadRequest, "asset_ids and tag_name are required")
	}

	tag, err := s.db.GetOrCreateTag(c.RequestCtx(), dbgen.GetOrCreateTagParams{
		ID:          uuid.NewString(),
		WorkspaceID: claims.WorkspaceID,
		Name:        body.TagName,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not get or create tag")
	}

	for _, assetID := range body.AssetIDs {
		// Verify ownership
		if _, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
			ID: assetID, WorkspaceID: claims.WorkspaceID,
		}); err != nil {
			continue // skip assets not in this workspace
		}
		_ = s.db.AddTagToAsset(c.RequestCtx(), dbgen.AddTagToAssetParams{
			AssetID: assetID,
			TagID:   tag.ID,
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// handleBulkProject assigns (or unassigns) a project to multiple assets.
func (s *Server) handleBulkProject(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	var body struct {
		AssetIDs  []string `json:"asset_ids"`
		ProjectID *string  `json:"project_id"` // null = unassign
	}
	if err := c.Bind().Body(&body); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	if len(body.AssetIDs) == 0 {
		return errRes(c, fiber.StatusBadRequest, "asset_ids is required")
	}

	// If project_id provided, verify it belongs to workspace
	if body.ProjectID != nil {
		if _, err := s.db.GetProjectByID(c.RequestCtx(), dbgen.GetProjectByIDParams{
			ID:          *body.ProjectID,
			WorkspaceID: claims.WorkspaceID,
		}); err != nil {
			return errRes(c, fiber.StatusNotFound, "project not found")
		}
	}

	var projectID *string
	if body.ProjectID != nil {
		projectID = body.ProjectID
	}

	for _, assetID := range body.AssetIDs {
		if _, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
			ID: assetID, WorkspaceID: claims.WorkspaceID,
		}); err != nil {
			continue
		}
		_ = s.db.UpdateAssetProject(c.RequestCtx(), dbgen.UpdateAssetProjectParams{
			ProjectID:   projectID,
			ID:          assetID,
			WorkspaceID: claims.WorkspaceID,
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) handleListAssetsInFolder(c fiber.Ctx, workspaceID, folderID string, isRoot bool, projectID string, limit int64) error {
	var args []interface{}
	var whereClause string

	if isRoot {
		if projectID == "" {
			return errRes(c, fiber.StatusBadRequest, "project_id is required when using folder_id=root")
		}
		whereClause = "a.workspace_id = ? AND a.folder_id IS NULL AND a.project_id = ?"
		args = []interface{}{workspaceID, projectID}
	} else {
		whereClause = "a.workspace_id = ? AND a.folder_id = ?"
		args = []interface{}{workspaceID, folderID}
	}

	var cursorClause string
	if cursor := c.Query("cursor"); cursor != "" {
		at, id, err := decodeCursor(cursor)
		if err == nil {
			cursorClause = "AND (a.created_at < ? OR (a.created_at = ? AND a.id < ?))"
			args = append(args, at.UTC().Format("2006-01-02 15:04:05"), at.UTC().Format("2006-01-02 15:04:05"), id)
		}
	}
	args = append(args, limit)

	query := fmt.Sprintf(`
		SELECT a.id, a.workspace_id, a.project_id, a.original_filename, a.storage_key,
		       a.mime_type, a.size, a.width, a.height, a.thumbnail_key, a.metadata,
		       a.created_at, a.updated_at
		FROM assets a
		WHERE %s
		  %s
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT ?
	`, whereClause, cursorClause)

	rows, err := s.sqlDB.QueryContext(c.RequestCtx(), query, args...)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list assets")
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
		return errRes(c, fiber.StatusInternalServerError, "query failed")
	}

	return c.JSON(buildAssetListResponse(assets, limit))
}

func (s *Server) handleUpdateAssetFolder(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	// Verify asset exists in workspace
	if _, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	var body struct {
		FolderID *string `json:"folder_id"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}

	var folderID *string
	if body.FolderID != nil && *body.FolderID != "" {
		// Verify folder belongs to workspace
		if _, err := s.db.GetFolderByID(c.RequestCtx(), dbgen.GetFolderByIDParams{
			ID:          *body.FolderID,
			WorkspaceID: claims.WorkspaceID,
		}); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errRes(c, fiber.StatusNotFound, "folder not found")
			}
			return errRes(c, fiber.StatusInternalServerError, "could not load folder")
		}
		folderID = body.FolderID
	}

	if err := s.db.UpdateAssetFolder(c.RequestCtx(), dbgen.UpdateAssetFolderParams{
		FolderID:    folderID,
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not update asset folder")
	}

	// Reload asset to return updated version
	updated, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not reload asset")
	}

	return c.JSON(assetToResponse(updated, nil))
}

// handleBulkDelete deletes multiple assets and their storage files.
func (s *Server) handleBulkDelete(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	var body struct {
		AssetIDs []string `json:"asset_ids"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	if len(body.AssetIDs) == 0 {
		return errRes(c, fiber.StatusBadRequest, "asset_ids is required")
	}

	for _, assetID := range body.AssetIDs {
		asset, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
			ID: assetID, WorkspaceID: claims.WorkspaceID,
		})
		if err != nil {
			continue
		}
		_ = s.storage.Delete(asset.StorageKey)
		if asset.ThumbnailKey != nil {
			_ = s.storage.Delete(*asset.ThumbnailKey)
		}
		_ = s.db.DeleteAsset(c.RequestCtx(), dbgen.DeleteAssetParams{
			ID:          asset.ID,
			WorkspaceID: claims.WorkspaceID,
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// fiber:context-methods migrated
