package api

import (
	"database/sql"
	"errors"
	"strings"

	"creativo-dam/server/internal/auth"
	dbgen "creativo-dam/server/internal/db/gen"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type folderResponse struct {
	ID          string           `json:"id"`
	WorkspaceID string           `json:"workspace_id"`
	ProjectID   string           `json:"project_id"`
	ParentID    sql.NullString   `json:"parent_id"`
	Name        string           `json:"name"`
	Position    int64            `json:"position"`
	AssetCount  int64            `json:"asset_count"`
	Children    []folderResponse `json:"children"`
	CreatedAt   string           `json:"created_at"`
}

func folderToResponse(f dbgen.Folder, assetCount int64) folderResponse {
	return folderResponse{
		ID:          f.ID,
		WorkspaceID: f.WorkspaceID,
		ProjectID:   f.ProjectID,
		ParentID:    f.ParentID,
		Name:        f.Name,
		Position:    f.Position,
		AssetCount:  assetCount,
		Children:    []folderResponse{},
		CreatedAt:   f.CreatedAt,
	}
}

func (s *Server) handleCreateFolder(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	projectID := c.Params("id")

	// Verify project belongs to workspace
	if _, err := s.db.GetProjectByID(c.RequestCtx(), dbgen.GetProjectByIDParams{
		ID:          projectID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "project not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load project")
	}

	var body struct {
		Name     string  `json:"name"`
		ParentID *string `json:"parent_id"`
		Position int64   `json:"position"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		return errRes(c, fiber.StatusBadRequest, "name is required")
	}

	var parentID sql.NullString
	if body.ParentID != nil && *body.ParentID != "" {
		// Verify parent exists in workspace
		parent, err := s.db.GetFolderByID(c.RequestCtx(), dbgen.GetFolderByIDParams{
			ID:          *body.ParentID,
			WorkspaceID: claims.WorkspaceID,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errRes(c, fiber.StatusNotFound, "parent folder not found")
			}
			return errRes(c, fiber.StatusInternalServerError, "could not load parent folder")
		}
		// Max depth is 2 levels — parent must be a root folder (no parent of its own)
		if parent.ParentID.Valid {
			return errRes(c, fiber.StatusUnprocessableEntity, "max folder depth is 2")
		}
		if parent.ProjectID != projectID {
			return errRes(c, fiber.StatusBadRequest, "parent folder belongs to a different project")
		}
		parentID = sql.NullString{String: *body.ParentID, Valid: true}
	}

	// SQLite does not enforce UNIQUE constraints when parent_id IS NULL,
	// so we check for duplicates at the application level for root folders.
	if !parentID.Valid {
		var existingCount int
		err := s.sqlDB.QueryRowContext(c.RequestCtx(),
			`SELECT COUNT(*) FROM folders WHERE project_id = ? AND parent_id IS NULL AND name = ? AND workspace_id = ?`,
			projectID, body.Name, claims.WorkspaceID,
		).Scan(&existingCount)
		if err == nil && existingCount > 0 {
			return errRes(c, fiber.StatusConflict, "a folder with that name already exists here")
		}
	}

	folder, err := s.db.CreateFolder(c.RequestCtx(), dbgen.CreateFolderParams{
		ID:          uuid.NewString(),
		WorkspaceID: claims.WorkspaceID,
		ProjectID:   projectID,
		ParentID:    parentID,
		Name:        body.Name,
		Position:    body.Position,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return errRes(c, fiber.StatusConflict, "a folder with that name already exists here")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not create folder")
	}

	return c.Status(fiber.StatusCreated).JSON(folderToResponse(folder, 0))
}

func (s *Server) handleGetFolders(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	projectID := c.Params("id")

	// Verify project belongs to workspace
	if _, err := s.db.GetProjectByID(c.RequestCtx(), dbgen.GetProjectByIDParams{
		ID:          projectID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "project not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load project")
	}

	rows, err := s.sqlDB.QueryContext(c.RequestCtx(), `
		WITH RECURSIVE tree AS (
			SELECT *, 0 AS depth FROM folders
			WHERE project_id = ? AND parent_id IS NULL AND workspace_id = ?
			UNION ALL
			SELECT f.*, t.depth + 1 FROM folders f
			JOIN tree t ON f.parent_id = t.id
			WHERE t.depth < 2
		)
		SELECT t.id, t.workspace_id, t.project_id, t.parent_id, t.name, t.position, t.created_at, t.depth,
			(SELECT COUNT(*) FROM assets a WHERE a.folder_id = t.id AND a.workspace_id = ?) AS asset_count
		FROM tree t
		ORDER BY t.depth, t.position, t.name
	`, projectID, claims.WorkspaceID, claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load folders")
	}
	defer rows.Close()

	type flatRow struct {
		folderResponse
		depth    int64
		id       string
		parentID sql.NullString
	}
	var flat []flatRow
	for rows.Next() {
		var f flatRow
		var depth int64
		var parentID sql.NullString
		var id string
		if err := rows.Scan(
			&id, &f.WorkspaceID, &f.ProjectID, &parentID,
			&f.Name, &f.Position, &f.CreatedAt, &depth, &f.AssetCount,
		); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "scan failed")
		}
		f.id = id
		f.parentID = parentID
		f.depth = depth
		f.folderResponse.ID = id
		f.folderResponse.ParentID = parentID
		f.folderResponse.Children = []folderResponse{}
		flat = append(flat, f)
	}
	if err := rows.Err(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "query failed")
	}

	// Build tree — two-pass approach (works correctly with slice mutation)
	// Pass 1: collect roots into slice + map their index
	rootMap := make(map[string]int) // folder id -> index in roots slice
	var roots []folderResponse
	for _, row := range flat {
		if !row.parentID.Valid {
			rootMap[row.id] = len(roots)
			roots = append(roots, row.folderResponse)
		}
	}
	// Pass 2: attach children to parent root entries
	for _, row := range flat {
		if row.parentID.Valid {
			if idx, ok := rootMap[row.parentID.String]; ok {
				roots[idx].Children = append(roots[idx].Children, row.folderResponse)
			}
		}
	}

	if roots == nil {
		roots = []folderResponse{}
	}
	return c.JSON(roots)
}

func (s *Server) handleUpdateFolder(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	if _, err := s.db.GetFolderByID(c.RequestCtx(), dbgen.GetFolderByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "folder not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load folder")
	}

	var body struct {
		Name     *string `json:"name"`
		Position *int64  `json:"position"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}

	if body.Name != nil {
		trimmed := strings.TrimSpace(*body.Name)
		if trimmed == "" {
			return errRes(c, fiber.StatusBadRequest, "name cannot be empty")
		}
		body.Name = &trimmed
	}

	folder, err := s.db.UpdateFolder(c.RequestCtx(), dbgen.UpdateFolderParams{
		Name:        sql.NullString{String: ptrStr(body.Name), Valid: body.Name != nil},
		Position:    sql.NullInt64{Int64: ptrInt64(body.Position), Valid: body.Position != nil},
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return errRes(c, fiber.StatusConflict, "a folder with that name already exists here")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not update folder")
	}

	return c.JSON(folderToResponse(folder, 0))
}

func (s *Server) handleDeleteFolder(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	if _, err := s.db.GetFolderByID(c.RequestCtx(), dbgen.GetFolderByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "folder not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load folder")
	}

	tx, err := s.sqlDB.BeginTx(c.RequestCtx(), nil)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not start transaction")
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := s.db.WithTx(tx)

	// Find and delete children first (using the transaction connection)
	children, err := qtx.GetFolderChildren(c.RequestCtx(), dbgen.GetFolderChildrenParams{
		ParentID:    sql.NullString{String: id, Valid: true},
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load children")
	}
	for _, child := range children {
		if err := qtx.NullifyFolderAssets(c.RequestCtx(), dbgen.NullifyFolderAssetsParams{
			FolderID:    sql.NullString{String: child.ID, Valid: true},
			WorkspaceID: claims.WorkspaceID,
		}); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not nullify child assets")
		}
		if err := qtx.DeleteFolder(c.RequestCtx(), dbgen.DeleteFolderParams{
			ID:          child.ID,
			WorkspaceID: claims.WorkspaceID,
		}); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not delete child folder")
		}
	}

	// Nullify assets in this folder
	if err := qtx.NullifyFolderAssets(c.RequestCtx(), dbgen.NullifyFolderAssetsParams{
		FolderID:    sql.NullString{String: id, Valid: true},
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not nullify assets")
	}

	if err := qtx.DeleteFolder(c.RequestCtx(), dbgen.DeleteFolderParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete folder")
	}

	if err := tx.Commit(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not commit transaction")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ptrInt64 safely dereferences a *int64.
func ptrInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

// fiber:context-methods migrated
