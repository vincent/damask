package api

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"damask/server/internal/audit"
	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/services"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type AssetResponse struct {
	ID                 string    `json:"id"`
	WorkspaceID        string    `json:"workspace_id"`
	ProjectID          *string   `json:"project_id"`
	FolderID           *string   `json:"folder_id"`
	OriginalFilename   string    `json:"original_filename"`
	MimeType           string    `json:"mime_type"`
	Size               int64     `json:"size"`
	Width              *int64    `json:"width"`
	Height             *int64    `json:"height"`
	ThumbnailKey       *string   `json:"thumbnail_key"`
	Metadata           *string   `json:"metadata"`
	Tags               []string  `json:"tags"`
	VersionCount       int64     `json:"version_count"`
	VariantCount       int64     `json:"variant_count"`
	VariantsRebuilding bool      `json:"variants_rebuilding"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type AssetListResponse struct {
	Assets     []AssetResponse `json:"assets"`
	NextCursor *string         `json:"next_cursor"`
}

func assetToResponse(a dbgen.Asset, tags []string) AssetResponse {
	return assetToResponseWithCount(a, tags, 0, 0, false)
}

func assetToResponseWithCount(a dbgen.Asset, tags []string, versionCount int64, variantCount int64, variantsRebuilding bool) AssetResponse {
	if tags == nil {
		tags = []string{}
	}
	return AssetResponse{
		ID:                 a.ID,
		WorkspaceID:        a.WorkspaceID,
		ProjectID:          a.ProjectID,
		FolderID:           a.FolderID,
		OriginalFilename:   a.OriginalFilename,
		MimeType:           a.MimeType,
		Size:               a.Size,
		Width:              a.Width,
		Height:             a.Height,
		ThumbnailKey:       a.ThumbnailKey,
		Metadata:           a.Metadata,
		Tags:               tags,
		VersionCount:       versionCount,
		VariantCount:       variantCount,
		VariantsRebuilding: variantsRebuilding,
		CreatedAt:          a.CreatedAt,
		UpdatedAt:          a.UpdatedAt,
	}
}

// handleUploadAsset uploads a file and creates a new asset.
//
// @Summary Upload an asset
// @Description Uploads a file as a new asset in the workspace. The request must be a multipart form with a <code>file</code> field. Optional form fields: <ul> <li><strong>project_id</strong> — assign the asset to a project on creation.</li> <li><strong>folder_id</strong> — assign the asset to a folder on creation.</li> </ul> On success a thumbnail generation job is enqueued automatically. An <code>asset_created</code> audit event is written and custom fields are inherited from the project if applicable.
// @Tags Assets
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "File to upload"
// @Param project_id formData string false "Project ID"
// @Param folder_id formData string false "Folder ID"
// @Success 201 {object} AssetResponse
// @Failure 400 {object} ErrorResponse "file field is required"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/assets [post]
func (s *Server) handleUploadAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	// Demo upload cap enforcement (DM-4.2) — no-op in non-demo builds
	if blocked, err := s.checkDemoUploadCap(c, claims); err != nil || blocked {
		return err
	}

	fh, err := c.FormFile("file")
	if err != nil {
		return errRes(c, fiber.StatusBadRequest, "file field is required")
	}

	tmpF, err := os.CreateTemp("", "damask-upload-*"+filepath.Ext(fh.Filename))
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "cannot create temp file")
	}
	tmpFile := tmpF.Name()
	_ = tmpF.Close()
	err = c.SaveFile(fh, tmpFile)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "cannot create temp file")
	}

	var uploadProjectID *string
	if pid := c.FormValue("project_id"); pid != "" {
		uploadProjectID = &pid
	}

	var uploadFolderID *string
	if fid := c.FormValue("folder_id"); fid != "" {
		uploadFolderID = &fid
	}

	asset, fErr := services.CreateAsset(c.RequestCtx(), s.db, s.sqlDB, s.storage, s.queue, claims.WorkspaceID, tmpFile, services.AssetOptions{
		ProjectID:     uploadProjectID,
		FolderID:      uploadFolderID,
		UserID:        claims.UserID,
		InheritFields: inheritProjectFields,
		OriginalName:  fh.Filename,
	})
	if fErr != nil {
		slog.Error("cannot create asset", "error", fErr)
		return errRes(c, fErr.Code, fErr.Message)
	}

	userID := claims.UserID
	s.audit.WriteAsset(c.RequestCtx(), audit.AssetEvent{
		WorkspaceID: claims.WorkspaceID,
		AssetID:     asset.ID,
		UserID:      &userID,
		ActorType:   audit.ActorTypeUser,
		EventType:   audit.EventAssetCreated,
		Payload:     audit.AssetCreatedPayload{V: 1, Filename: asset.OriginalFilename, Source: "upload"},
	})

	return c.Status(fiber.StatusCreated).JSON(assetToResponse(*asset, nil))
}

// handleListAssets lists assets in the workspace with filtering, sorting, and cursor pagination.
//
// @Summary List assets
// @Description Returns a paginated list of assets. The behaviour is controlled by query parameters: <ul> <li><strong>q</strong> — Full-text search across filenames and custom field text values.</li> <li><strong>tags</strong> — Comma-separated tag names. Returns only assets that have ALL listed tags (AND logic).</li> <li><strong>folder_id</strong> — Filter by folder. Use <code>root</code> to list assets with no folder in a project.</li> <li><strong>project_id</strong> — Filter by project (required when folder_id=root).</li> <li><strong>mime</strong> — Filter by MIME type prefix (e.g. <code>image/</code> or <code>video/mp4</code>).</li> <li><strong>sort</strong> — Sort order: <code>created_at_desc</code> (default), <code>created_at_asc</code>, <code>size_asc</code>, <code>size_desc</code>, <code>id_asc</code>, <code>id_desc</code>, <code>taken_at</code>, <code>taken_at_desc</code>.</li> <li><strong>limit</strong> — Page size, 1–100 (default 50).</li> <li><strong>cursor</strong> — Opaque cursor from the previous page's <code>next_cursor</code> field.</li> <li><strong>field[key]</strong> — Filter by custom field value, e.g. <code>field[status]=published</code>.</li> </ul>
// @Tags Assets
// @Produce json
// @Security BearerAuth
// @Param q query string false "Full-text search query"
// @Param tags query string false "Comma-separated tag names (AND filter)"
// @Param folder_id query string false "Folder ID (use 'root' for unfoldered assets in a project)"
// @Param project_id query string false "Project ID"
// @Param mime query string false "MIME type prefix filter"
// @Param sort query string false "Sort order"
// @Param limit query int false "Page size (1-100, default 50)"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} AssetListResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/assets [get]
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

	if sort == "taken_at" || sort == "taken_at_desc" {
		return s.handleListAssetsSortByTakenAt(c, claims.WorkspaceID, limit, sort == "taken_at_desc")
	}

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
	if cid := c.Query("collection_id"); cid != "" {
		whereClauses = append(whereClauses, "id IN (SELECT asset_id FROM collection_assets WHERE collection_id = ?)")
		args = append(args, cid)
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
		slog.Error("could not list assets", "error", err)
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

	versionCounts := s.batchVersionCounts(c.RequestCtx(), assets)
	variantCounts := s.batchVariantCounts(c.RequestCtx(), assets)
	return c.JSON(buildAssetListResponseWithCounts(assets, limit, sortField, versionCounts, variantCounts))
}

// handleListAssetsSortByTakenAt sorts assets by _exif_taken_at field value (ASC, NULLs last).
// desc=true reverses to DESC (still NULLs last).
func (s *Server) handleListAssetsSortByTakenAt(c fiber.Ctx, workspaceID string, limit int64, desc bool) error {
	// Look up the _exif_taken_at field definition ID.
	fd, err := s.db.GetFieldDefinitionByKey(c.RequestCtx(), dbgen.GetFieldDefinitionByKeyParams{
		WorkspaceID: workspaceID,
		Key:         "_exif_taken_at",
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return errRes(c, fiber.StatusInternalServerError, "could not query field definitions")
	}

	var whereExtra string
	var extraArgs []interface{}

	if pid := c.Query("project_id"); pid != "" {
		whereExtra += " AND a.project_id = ?"
		extraArgs = append(extraArgs, pid)
	}
	if mime := c.Query("mime"); mime != "" {
		whereExtra += " AND a.mime_type LIKE ?"
		extraArgs = append(extraArgs, mime+"%")
	}

	// Field ID for the LEFT JOIN (empty string if field doesn't exist yet — join yields no rows).
	fieldID := ""
	if err == nil {
		fieldID = fd.ID
	}

	orderDir := "ASC"
	if desc {
		orderDir = "DESC"
	}

	query := fmt.Sprintf(`
		SELECT a.id, a.workspace_id, a.project_id, a.folder_id, a.original_filename, a.storage_key,
		       a.mime_type, a.size, a.width, a.height, a.thumbnail_key, a.metadata,
		       a.created_at, a.updated_at
		FROM assets a
		LEFT JOIN asset_field_values afv ON afv.asset_id = a.id AND afv.field_id = ?
		WHERE a.workspace_id = ?%s
		ORDER BY afv.value_date %s NULLS LAST, a.created_at DESC, a.id DESC
		LIMIT ?
	`, whereExtra, orderDir)

	// Args order: fieldID (JOIN), workspaceID (WHERE), extra filters, limit.
	queryArgs := []interface{}{fieldID, workspaceID}
	queryArgs = append(queryArgs, extraArgs...)
	queryArgs = append(queryArgs, limit)

	rows, err := s.sqlDB.QueryContext(c.RequestCtx(), query, queryArgs...)
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
		return errRes(c, fiber.StatusInternalServerError, "could not list assets")
	}

	versionCounts := s.batchVersionCounts(c.RequestCtx(), assets)
	variantCounts := s.batchVariantCounts(c.RequestCtx(), assets)
	return c.JSON(buildAssetListResponseWithCounts(assets, limit, "created_at", versionCounts, variantCounts))
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
	variantCounts := s.batchVariantCounts(c.RequestCtx(), assets)
	return c.JSON(buildAssetListResponseWithCounts(assets, limit, "created_at", counts, variantCounts))
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
	variantCounts := s.batchVariantCounts(c.RequestCtx(), assets)
	return c.JSON(buildAssetListResponseWithCounts(assets, limit, "created_at", counts, variantCounts))
}

func buildAssetListResponseWithCounts(assets []dbgen.Asset, limit int64, sortField string, versionCounts map[string]int64, variantCounts map[string]int64) AssetListResponse {
	items := make([]AssetResponse, len(assets))
	for i, a := range assets {
		var vc, nVariants int64
		if versionCounts != nil {
			vc = versionCounts[a.ID]
		}
		if variantCounts != nil {
			nVariants = variantCounts[a.ID]
		}
		items[i] = assetToResponseWithCount(a, nil, vc, nVariants, false)
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
// query and returns a map of assetID → count.
func (s *Server) batchVersionCounts(ctx context.Context, assets []dbgen.Asset) map[string]int64 {
	counts := make(map[string]int64, len(assets))
	if len(assets) == 0 {
		return counts
	}
	placeholders := make([]string, len(assets))
	args := make([]any, len(assets))
	for i, a := range assets {
		placeholders[i] = "?"
		args[i] = a.ID
	}
	query := fmt.Sprintf(
		`SELECT asset_id, COUNT(*) 
		   FROM asset_versions 
		  WHERE deleted_at IS NULL 
		    AND asset_id IN (%s) 
		  GROUP BY asset_id`,
		strings.Join(placeholders, ","),
	)
	rows, err := s.sqlDB.QueryContext(ctx, query, args...)
	if err != nil {
		return counts
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n int64
		if err := rows.Scan(&id, &n); err == nil {
			counts[id] = n
		}
	}
	return counts
}

// batchVariantCounts fetches variant counts (on current version) for the given asset IDs.
func (s *Server) batchVariantCounts(ctx context.Context, assets []dbgen.Asset) map[string]int64 {
	counts := make(map[string]int64, len(assets))
	if len(assets) == 0 {
		return counts
	}
	placeholders := make([]string, len(assets))
	args := make([]any, len(assets))
	for i, a := range assets {
		placeholders[i] = "?"
		args[i] = a.ID
	}
	query := fmt.Sprintf(
		`SELECT av.asset_id, COUNT(v.id)
		   FROM asset_versions av
		   JOIN variants v ON v.asset_version_id = av.id
		  WHERE av.is_current = 1 
		    AND av.asset_id IN (%s)
		  GROUP BY av.asset_id`,
		strings.Join(placeholders, ","),
	)
	rows, err := s.sqlDB.QueryContext(ctx, query, args...)
	if err != nil {
		return counts
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n int64
		if err := rows.Scan(&id, &n); err == nil {
			counts[id] = n
		}
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

// handleGetComments returns all share comments on an asset.
//
// @Summary Get asset comments
// @Description Returns all public share comments that have been posted on this asset across all shares.
// @Tags Assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {array} CommentResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/comments [get]
func (s *Server) handleGetComments(c fiber.Ctx) error {
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

	comments, err := s.db.ListCommentsOnAsset(c.RequestCtx(), asset.ID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load comments")
	}

	return c.JSON(comments)
}

// handleGetAsset returns a single asset by ID.
//
// @Summary Get an asset
// @Description Returns the full asset record including tags, version count, and a flag indicating whether variants are currently being rebuilt.
// @Tags Assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {object} AssetResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id} [get]
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

	// Check if a variant rebuild job is in flight for the current version.
	variantsRebuilding := false
	if asset.CurrentVersionID != nil {
		var rebuildCount int64
		if err := s.sqlDB.QueryRowContext(c.RequestCtx(),
			`SELECT COUNT(*) FROM jobs
			 WHERE type = 'rebuild_variants'
			   AND JSON_EXTRACT(payload, '$.new_version_id') = ?
			   AND status IN ('pending', 'processing')`,
			*asset.CurrentVersionID,
		).Scan(&rebuildCount); err == nil {
			variantsRebuilding = rebuildCount > 0
		}
	}

	variantCount, _ := s.db.CountVariantsByVersion(c.RequestCtx(), func() string {
		if asset.CurrentVersionID != nil {
			return *asset.CurrentVersionID
		}
		return ""
	}())

	return c.JSON(assetToResponseWithCount(asset, tagNames, versionCount, variantCount, variantsRebuilding))
}

// handleGetAssetFile streams the current version of an asset file.
//
// @Summary Download asset file
// @Description Streams the raw file bytes of the asset's current version. The response Content-Type matches the asset's MIME type and Content-Disposition is set to <code>inline</code> with the original filename. An <code>asset_downloaded</code> audit event is recorded (browser image prefetch requests are excluded).
// @Tags Assets
// @Produce application/octet-stream
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {file} binary
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset or file not found"
// @Router /api/v1/assets/{id}/file [get]
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

	version, err := s.db.GetCurrentVersion(c.RequestCtx(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset file not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset version")
	}

	lastMod := parseVersionTime(version.CreatedAt)
	if setCacheHeaders(c, version.ContentHash, lastMod, false) {
		return nil
	}

	rc, err := s.storage.Get(version.StorageKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "file not found")
	}

	if c.Get("Sec-Fetch-Dest") != "image" {
		userID := claims.UserID
		s.audit.WriteAssetAsync(audit.AssetEvent{
			WorkspaceID: claims.WorkspaceID,
			AssetID:     asset.ID,
			UserID:      &userID,
			ActorType:   audit.ActorTypeUser,
			EventType:   audit.EventAssetDownloaded,
			Payload:     audit.AssetDownloadedPayload{V: 1, Via: "direct"},
		})
	}

	c.Set("Content-Type", asset.MimeType)
	c.Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": asset.OriginalFilename}))
	if version.Size > 0 {
		c.Set("Content-Length", strconv.FormatInt(version.Size, 10))
	}
	return c.SendStream(rc)
}

// handleGetAssetThumb serves the asset thumbnail as a JPEG image.
//
// @Summary Get asset thumbnail
// @Description Streams the JPEG thumbnail for the asset. Thumbnails are generated asynchronously after upload; returns 404 if generation has not yet completed.
// @Tags Assets
// @Produce image/jpeg
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {file} binary
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found or thumbnail not ready"
// @Router /api/v1/assets/{id}/thumb [get]
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

	// log.Println("thumbnail: serve", *asset.ThumbnailKey)

	thumbETag := asset.ID + "_" + strconv.FormatInt(asset.UpdatedAt.Unix(), 10)
	if setCacheHeaders(c, thumbETag, asset.UpdatedAt, false) {
		return nil
	}

	rc, err := s.storage.Get(*asset.ThumbnailKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "thumbnail not found")
	}

	c.Set("Content-Type", "image/jpeg")
	return c.SendStream(rc)
}

// handleDeleteAsset permanently deletes an asset and its stored files.
//
// @Summary Delete an asset
// @Description Permanently deletes the asset record, its storage file, and its thumbnail. All associated variants, versions, tags, and field values are also removed via cascade. This action cannot be undone.
// @Tags Assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id} [delete]
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

	// Block delete if asset is used as a project cover
	_, err = s.db.GetProjectByCoverAsset(c.RequestCtx(), dbgen.GetProjectByCoverAssetParams{
		CoverAssetID: &asset.ID,
		WorkspaceID:  claims.WorkspaceID,
	})
	if err == nil {
		return errRes(c, fiber.StatusConflict, "asset is used as a project cover")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return errRes(c, fiber.StatusInternalServerError, "could not check asset usage")
	}

	// Block delete if asset is used as the workspace icon
	_, err = s.db.GetWorkspaceByIconAsset(c.RequestCtx(), dbgen.GetWorkspaceByIconAssetParams{
		IconAssetID: &asset.ID,
		ID:          claims.WorkspaceID,
	})
	if err == nil {
		return errRes(c, fiber.StatusConflict, "asset is used as the workspace icon")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return errRes(c, fiber.StatusInternalServerError, "could not check asset usage")
	}

	// Collect all storage keys before deleting, then delete DB rows in a
	// transaction. Storage cleanup happens after commit so files are never
	// orphaned by a failed DB delete.
	tx, err := s.sqlDB.BeginTx(c.RequestCtx(), nil)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not begin transaction")
	}
	defer tx.Rollback()
	qtx := s.db.WithTx(tx)

	var storageKeys []string
	storageKeys = append(storageKeys, asset.StorageKey)
	if asset.ThumbnailKey != nil {
		storageKeys = append(storageKeys, *asset.ThumbnailKey)
	}

	versions, err := qtx.ListAllVersions(c.RequestCtx(), asset.ID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list asset versions")
	}
	for _, v := range versions {
		storageKeys = append(storageKeys, v.StorageKey)
		if v.ThumbnailKey != nil {
			storageKeys = append(storageKeys, *v.ThumbnailKey)
		}
		variants, err := qtx.ListVariantsByVersion(c.RequestCtx(), v.ID)
		if err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not list variants")
		}
		for _, variant := range variants {
			storageKeys = append(storageKeys, variant.StorageKey)
		}
	}

	if err := qtx.DeleteAsset(c.RequestCtx(), dbgen.DeleteAssetParams{
		ID:          asset.ID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete asset")
	}

	if err := tx.Commit(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not commit deletion")
	}

	for _, key := range storageKeys {
		_ = s.storage.Delete(key)
	}

	userID := claims.UserID
	s.audit.WriteAsset(c.RequestCtx(), audit.AssetEvent{
		WorkspaceID: claims.WorkspaceID,
		AssetID:     asset.ID,
		UserID:      &userID,
		ActorType:   audit.ActorTypeUser,
		EventType:   audit.EventAssetDeleted,
		Payload:     audit.AssetDeletedPayload{V: 1},
	})

	return c.SendStatus(fiber.StatusNoContent)
}

// handleBulkTag adds a tag to multiple assets at once.
//
// @Summary Bulk tag assets
// @Description Adds a single tag to all specified assets. If the tag does not exist in the workspace it is created automatically. Assets not found in the workspace are silently skipped.
// @Tags Assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body BulkTagRequest true "Asset IDs and tag name"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/assets/bulk/tag [post]
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

// handleBulkProject assigns or unassigns a project for multiple assets.
//
// @Summary Bulk assign assets to a project
// @Description Assigns all listed assets to the given project. Set <code>project_id</code> to null to remove assets from their current project. Assets not found in the workspace are silently skipped.
// @Tags Assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body BulkProjectRequest true "Asset IDs and optional project ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Project not found"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/assets/bulk/project [post]
func (s *Server) handleBulkProject(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &BulkProjectRequest{})
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
	variantCounts := s.batchVariantCounts(c.RequestCtx(), assets)
	return c.JSON(buildAssetListResponseWithCounts(assets, limit, "created_at", counts, variantCounts))
}

// handleUpdateAssetFolder moves an asset to a different folder or project.
//
// @Summary Move asset to a folder
// @Description Updates the asset's <code>folder_id</code> and/or <code>project_id</code>. Set either field to null to remove the assignment. The target folder and project must belong to the same workspace.
// @Tags Assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param body body UpdateAssetFolderRequest true "New folder and/or project assignment"
// @Success 200 {object} AssetResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/assets/{id} [patch]
func (s *Server) handleUpdateAssetFolder(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	body, ok := decodeAndValidate(c, &UpdateAssetFolderRequest{})
	if !ok {
		return nil
	}

	// Verify asset exists in workspace
	before, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
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

	userID := claims.UserID
	s.audit.WriteAsset(c.RequestCtx(), audit.AssetEvent{
		WorkspaceID: claims.WorkspaceID,
		AssetID:     id,
		UserID:      &userID,
		ActorType:   audit.ActorTypeUser,
		EventType:   audit.EventAssetMoved,
		Payload: audit.AssetMovedPayload{
			V:               1,
			BeforeProjectID: before.ProjectID,
			AfterProjectID:  updated.ProjectID,
			BeforeFolderID:  before.FolderID,
			AfterFolderID:   updated.FolderID,
		},
	})

	return c.JSON(assetToResponse(updated, nil))
}

// handleRenameAsset updates the display name of an asset.
//
// @Summary Rename an asset
// @Description Updates the asset's <code>original_filename</code>. The new name must include the correct file extension matching the asset's MIME type.
// @Tags Assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param body body RenameAssetRequest true "New filename"
// @Success 200 {object} AssetResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/assets/{id}/rename [put]
func (s *Server) handleRenameAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	body, ok := decodeAndValidate(c, &RenameAssetRequest{})
	if !ok {
		return nil
	}

	before, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	// Preserve the original file extension.
	// body.Name is the stem only; reconstruct the full filename.
	ext := filepath.Ext(before.OriginalFilename)
	stem := strings.TrimSuffix(body.Name, ext) // guard: strip ext if client sent it
	newName := stem + ext

	// No-op: return early if the name hasn't changed.
	if newName == before.OriginalFilename {
		return c.JSON(assetToResponse(before, nil))
	}

	if err := s.db.UpdateAssetName(c.RequestCtx(), dbgen.UpdateAssetNameParams{
		OriginalFilename: newName,
		ID:               id,
		WorkspaceID:      claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not rename asset")
	}

	// Refresh FTS index so search reflects the new name.
	s.refreshAssetFTS(c.RequestCtx(), id)

	updated, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not reload asset")
	}

	userID := claims.UserID
	s.audit.WriteAsset(c.RequestCtx(), audit.AssetEvent{
		WorkspaceID: claims.WorkspaceID,
		AssetID:     id,
		UserID:      &userID,
		ActorType:   audit.ActorTypeUser,
		EventType:   audit.EventAssetRenamed,
		Payload: audit.AssetRenamedPayload{
			V:      1,
			Before: before.OriginalFilename,
			After:  updated.OriginalFilename,
		},
	})

	return c.JSON(assetToResponse(updated, nil))
}

// handleBulkDelete permanently deletes multiple assets.
//
// @Summary Bulk delete assets
// @Description Permanently deletes all listed assets, their storage files, thumbnails, variants, and associated data. Assets not found in the workspace are silently skipped. This action cannot be undone.
// @Tags Assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body BulkDeleteRequest true "Asset IDs to delete"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/assets/bulk [delete]
func (s *Server) handleBulkDelete(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &BulkDeleteRequest{})
	if !ok {
		return nil
	}

	type pendingDelete struct {
		asset       dbgen.Asset
		storageKeys []string
	}
	var pending []pendingDelete

	for _, assetID := range body.AssetIDs {
		asset, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
			ID: assetID, WorkspaceID: claims.WorkspaceID,
		})
		if err != nil {
			continue
		}
		keys := []string{asset.StorageKey}
		if asset.ThumbnailKey != nil {
			keys = append(keys, *asset.ThumbnailKey)
		}
		versions, err := s.db.ListAllVersions(c.RequestCtx(), asset.ID)
		if err == nil {
			for _, v := range versions {
				keys = append(keys, v.StorageKey)
				if v.ThumbnailKey != nil {
					keys = append(keys, *v.ThumbnailKey)
				}
				variants, err := s.db.ListVariantsByVersion(c.RequestCtx(), v.ID)
				if err == nil {
					for _, variant := range variants {
						keys = append(keys, variant.StorageKey)
					}
				}
			}
		}
		pending = append(pending, pendingDelete{asset: asset, storageKeys: keys})
	}

	// Delete all DB rows in a single transaction before touching storage.
	tx, err := s.sqlDB.BeginTx(c.RequestCtx(), nil)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not begin transaction")
	}
	defer tx.Rollback()
	qtx := s.db.WithTx(tx)
	for _, pd := range pending {
		if err := qtx.DeleteAsset(c.RequestCtx(), dbgen.DeleteAssetParams{
			ID:          pd.asset.ID,
			WorkspaceID: claims.WorkspaceID,
		}); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not delete asset")
		}
	}
	if err := tx.Commit(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not commit deletion")
	}

	for _, pd := range pending {
		for _, key := range pd.storageKeys {
			_ = s.storage.Delete(key)
		}
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// fiber:context-methods migrated
