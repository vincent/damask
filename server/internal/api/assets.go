package api

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/services"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type AssetResponse struct {
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
	VersionCount     int64     `json:"version_count"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type AssetListResponse struct {
	Assets     []AssetResponse `json:"assets"`
	NextCursor *string         `json:"next_cursor"`
}

func assetToResponse(a dbgen.Asset, tags []string) AssetResponse {
	return assetToResponseWithCount(a, tags, 0)
}

func assetToResponseWithCount(a dbgen.Asset, tags []string, versionCount int64) AssetResponse {
	if tags == nil {
		tags = []string{}
	}
	return AssetResponse{
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
		VersionCount:     versionCount,
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

	tmpFile := filepath.Join(os.TempDir(), fh.Filename)
	err = c.SaveFile(fh, tmpFile)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "cannot create temp file")
	}

	var uploadProjectID *string
	if pid := c.FormValue("project_id"); pid != "" {
		uploadProjectID = &pid
	}

	asset, fErr := services.CreateAsset(c.RequestCtx(), s.db, s.sqlDB, s.storage, s.queue, claims.WorkspaceID, tmpFile, services.AssetOptions{
		ProjectID:     uploadProjectID,
		UserID:        claims.UserID,
		InheritFields: inheritProjectFields,
	})
	if fErr != nil {
		return errRes(c, fErr.Code, fErr.Message)
	}

	return c.Status(fiber.StatusCreated).JSON(assetToResponse(*asset, nil))
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

	// Field value filter — look for any query param prefixed with "field["
	if hasFieldFilters(c) {
		return s.handleListAssetsByFields(c, claims.WorkspaceID, limit)
	}

	// Folder filter
	if folderID := c.Query("folder_id"); folderID != "" {
		isRoot := folderID == "root"
		projectID := c.Query("project_id")
		return s.handleListAssetsInFolder(c, claims.WorkspaceID, folderID, isRoot, projectID, limit)
	}

	sort := c.Query("sort")
	orderBy := "created_at DESC, id DESC"
	sortField := "created_at"
	switch sort {
	case "size_asc":
		orderBy = "size ASC, id DESC"
		sortField = "size"
	case "size_desc":
		orderBy = "size DESC, id DESC"
		sortField = "size"
	case "created_at_asc":
		orderBy = "created_at ASC, id ASC"
	case "created_at_desc":
		orderBy = "created_at DESC, id DESC"
	case "id_asc":
		orderBy = "id ASC"
		sortField = "id"
	case "id_desc":
		orderBy = "id DESC"
		sortField = "id"
	}

	var whereClauses []string
	var args []interface{}
	whereClauses = append(whereClauses, "workspace_id = ?")
	args = append(args, claims.WorkspaceID)

	if pid := c.Query("project_id"); pid != "" {
		whereClauses = append(whereClauses, "project_id = ?")
		args = append(args, pid)
	}
	if mime := c.Query("mime"); mime != "" {
		whereClauses = append(whereClauses, "mime_type LIKE ?")
		args = append(args, mime+"%")
	}

	if cursor := c.Query("cursor"); cursor != "" {
		cv, err := decodeCursor(cursor)
		if err == nil {
			switch cv.Field {
			case "size":
				if sort == "size_asc" {
					whereClauses = append(whereClauses, "(size > ? OR (size = ? AND id < ?))")
				} else {
					whereClauses = append(whereClauses, "(size < ? OR (size = ? AND id < ?))")
				}
				args = append(args, cv.Value, cv.Value, cv.ID)
			case "id":
				if sort == "id_asc" {
					whereClauses = append(whereClauses, "id > ?")
				} else {
					whereClauses = append(whereClauses, "id < ?")
				}
				args = append(args, cv.ID)
			default: // "created_at"
				if sort == "created_at_asc" {
					whereClauses = append(whereClauses, "(created_at > ? OR (created_at = ? AND id > ?))")
				} else {
					whereClauses = append(whereClauses, "(created_at < ? OR (created_at = ? AND id < ?))")
				}
				args = append(args, cv.Value, cv.Value, cv.ID)
			}
		}
	}

	args = append(args, limit)

	query := fmt.Sprintf(`
		SELECT id, workspace_id, project_id, folder_id, original_filename, storage_key,
		       mime_type, size, width, height, thumbnail_key, metadata, created_at, updated_at
		FROM assets
		WHERE %s
		ORDER BY %s
		LIMIT ?
	`, strings.Join(whereClauses, " AND "), orderBy)

	rows, err := s.sqlDB.QueryContext(c.RequestCtx(), query, args...)
	if err != nil {
		log.Println("could not list assets", err)
		return errRes(c, fiber.StatusInternalServerError, "could not list assets")
	}
	defer rows.Close()

	var assets []dbgen.Asset
	for rows.Next() {
		var a dbgen.Asset
		if err := rows.Scan(
			&a.ID, &a.WorkspaceID, &a.ProjectID, &a.FolderID, &a.OriginalFilename, &a.StorageKey,
			&a.MimeType, &a.Size, &a.Width, &a.Height, &a.ThumbnailKey, &a.Metadata,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "scan failed")
		}
		assets = append(assets, a)
	}
	if err := rows.Err(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list assets")
	}

	counts := s.batchVersionCounts(c.RequestCtx(), assets)
	return c.JSON(buildAssetListResponseWithCounts(assets, limit, sortField, counts))
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
		cv, err := decodeCursor(cursor)
		if err == nil {
			cursorClause = "AND (a.created_at < ? OR (a.created_at = ? AND a.id < ?))"
			args = append(args, cv.Value, cv.Value, cv.ID)
		}
	}

	args = append(args, limit)

	query := fmt.Sprintf(`
		SELECT a.id, a.workspace_id, a.project_id, a.folder_id, a.original_filename, a.storage_key,
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
			&a.ID, &a.WorkspaceID, &a.ProjectID, &a.FolderID, &a.OriginalFilename, &a.StorageKey,
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

	counts := s.batchVersionCounts(c.RequestCtx(), assets)
	return c.JSON(buildAssetListResponseWithCounts(assets, limit, "created_at", counts))
}

func (s *Server) handleSearchAssets(c fiber.Ctx, workspaceID, q string, limit int64) error {
	args := []interface{}{workspaceID, q + "*"}

	var cursorClause string
	if cursor := c.Query("cursor"); cursor != "" {
		cv, err := decodeCursor(cursor)
		if err == nil {
			cursorClause = "AND (a.created_at < ? OR (a.created_at = ? AND a.id < ?))"
			args = append(args, cv.Value, cv.Value, cv.ID)
		}
	}

	args = append(args, limit)

	rows, err := s.sqlDB.QueryContext(c.RequestCtx(), fmt.Sprintf(`
		SELECT a.id, a.workspace_id, a.project_id, a.folder_id, a.original_filename, a.storage_key,
		       a.mime_type, a.size, a.width, a.height, a.thumbnail_key, a.metadata,
		       a.created_at, a.updated_at
		FROM assets a
		WHERE a.workspace_id = ?
		  AND a.rowid IN (SELECT rowid FROM assets_fts WHERE assets_fts MATCH ?)
		  %s
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT ?
	`, cursorClause), args...)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "search failed")
	}
	defer rows.Close()

	var assets []dbgen.Asset
	for rows.Next() {
		var a dbgen.Asset
		if err := rows.Scan(
			&a.ID, &a.WorkspaceID, &a.ProjectID, &a.FolderID, &a.OriginalFilename, &a.StorageKey,
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

	counts := s.batchVersionCounts(c.RequestCtx(), assets)
	return c.JSON(buildAssetListResponseWithCounts(assets, limit, "created_at", counts))
}

func buildAssetListResponseWithCounts(assets []dbgen.Asset, limit int64, sortField string, counts map[string]int64) AssetListResponse {
	items := make([]AssetResponse, len(assets))
	for i, a := range assets {
		var vc int64
		if counts != nil {
			vc = counts[a.ID]
		}
		items[i] = assetToResponseWithCount(a, nil, vc)
	}
	var nextCursor *string
	if int64(len(assets)) == limit && len(assets) > 0 {
		last := assets[len(assets)-1]
		var cv cursorVal
		cv.ID = last.ID
		switch sortField {
		case "size":
			cv.Field = "size"
			cv.Value = fmt.Sprintf("%d", last.Size)
		case "id":
			cv.Field = "id"
			cv.Value = last.ID
		default:
			cv.Field = "created_at"
			cv.Value = last.CreatedAt.UTC().Format("2006-01-02 15:04:05")
		}
		encoded := encodeCursor(cv)
		nextCursor = &encoded
	}
	return AssetListResponse{Assets: items, NextCursor: nextCursor}
}

// batchVersionCounts fetches version counts for the given asset IDs in a single
// pass and returns a map of assetID → count.
func (s *Server) batchVersionCounts(ctx context.Context, assets []dbgen.Asset) map[string]int64 {
	counts := make(map[string]int64, len(assets))
	for _, a := range assets {
		counts[a.ID], _ = s.db.CountVersionsForAsset(ctx, a.ID)
	}
	return counts
}

type cursorVal struct {
	Field string // "created_at", "size", or "id"
	Value string // stringified sort-field value
	ID    string // asset UUID tiebreaker
}

func encodeCursor(v cursorVal) string {
	raw := v.Field + "|" + v.Value + "|" + v.ID
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(cursor string) (cursorVal, error) {
	b, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return cursorVal{}, err
	}
	parts := strings.SplitN(string(b), "|", 3)
	if len(parts) != 3 {
		// Legacy cursor format (created_at|id) — parse and upgrade transparently
		parts2 := strings.SplitN(string(b), "|", 2)
		if len(parts2) != 2 {
			return cursorVal{}, errors.New("invalid cursor")
		}
		// Validate the first part is a timestamp (SQLite format or RFC3339)
		_, errSQLite := time.Parse("2006-01-02 15:04:05", parts2[0])
		_, errRFC := time.Parse(time.RFC3339Nano, parts2[0])
		if errSQLite != nil && errRFC != nil {
			return cursorVal{}, errors.New("invalid cursor")
		}
		// Normalise to SQLite format so the WHERE clause comparison works
		val := parts2[0]
		if errSQLite != nil {
			t, _ := time.Parse(time.RFC3339Nano, val)
			val = t.UTC().Format("2006-01-02 15:04:05")
		}
		return cursorVal{Field: "created_at", Value: val, ID: parts2[1]}, nil
	}
	return cursorVal{Field: parts[0], Value: parts[1], ID: parts[2]}, nil
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

	versionCount, _ := s.db.CountVersionsForAsset(c.RequestCtx(), id)

	return c.JSON(assetToResponseWithCount(asset, tagNames, versionCount))
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
	body, ok := decodeAndValidate(c, &BulkTagRequest{})
	if !ok {
		return nil
	}
	claims := auth.GetClaims(c)
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

	body, ok := decodeAndValidate(c, &bulkProjectRequest{})
	if !ok {
		return nil
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
		cv, err := decodeCursor(cursor)
		if err == nil {
			cursorClause = "AND (a.created_at < ? OR (a.created_at = ? AND a.id < ?))"
			args = append(args, cv.Value, cv.Value, cv.ID)
		}
	}
	args = append(args, limit)

	query := fmt.Sprintf(`
		SELECT a.id, a.workspace_id, a.project_id, a.folder_id, a.original_filename, a.storage_key,
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
			&a.ID, &a.WorkspaceID, &a.ProjectID, &a.FolderID, &a.OriginalFilename, &a.StorageKey,
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

	counts := s.batchVersionCounts(c.RequestCtx(), assets)
	return c.JSON(buildAssetListResponseWithCounts(assets, limit, "created_at", counts))
}

func (s *Server) handleUpdateAssetFolder(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	body, ok := decodeAndValidate(c, &updateAssetFolderRequest{})
	if !ok {
		return nil
	}

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

	body, ok := decodeAndValidate(c, &bulkDeleteRequest{})
	if !ok {
		return nil
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
