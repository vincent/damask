package api

import (
	"strings"

	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

type FolderResponse struct {
	ID          string           `json:"id"`
	WorkspaceID string           `json:"workspace_id"`
	ProjectID   string           `json:"project_id"`
	ParentID    *string          `json:"parent_id"`
	Name        string           `json:"name"`
	Slug        *string          `json:"slug"`
	Position    int64            `json:"position"`
	AssetCount  int64            `json:"asset_count"`
	Children    []FolderResponse `json:"children"`
	CreatedAt   string           `json:"created_at"`
}

func folderDTOToResponse(d *service.FolderDTO) FolderResponse {
	return FolderResponse{
		ID:          d.ID,
		WorkspaceID: d.WorkspaceID,
		ProjectID:   d.ProjectID,
		ParentID:    d.ParentID,
		Name:        d.Name,
		Slug:        d.Slug,
		Position:    d.Position,
		AssetCount:  0,
		Children:    []FolderResponse{},
		CreatedAt:   d.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// handleCreateFolder creates a folder inside a project.
//
// @Summary Create a folder
// @Description Creates a new folder in the given project. Folders are at most 2 levels deep — a root folder can have child folders, but child folders cannot have their own children.<br> Use <code>parent_id</code> to create a sub-folder inside an existing root folder. An optional <code>position</code> integer controls ordering in the UI.
// @Tags Folders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Param body body CreateFolderRequest true "Folder details"
// @Success 201 {object} FolderResponse
// @Failure 400 {object} ErrorResponse "Parent folder belongs to a different project"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Project or parent folder not found"
// @Failure 409 {object} ErrorResponse "A folder with that name already exists here"
// @Failure 422 {object} ErrorResponse "Max folder depth exceeded"
// @Router /api/v1/projects/{id}/folders [post]
func (s *Server) handleCreateFolder(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	projectID := c.Params("id")

	// Verify project belongs to workspace via the projects service.
	if _, err := s.projects.Get(c.RequestCtx(), claims.WorkspaceID, projectID); err != nil {
		return Respond(c, err)
	}

	body, ok := decodeAndValidate(c, &CreateFolderRequest{})
	if !ok {
		return nil
	}

	dto, err := s.folders.Create(c.RequestCtx(), claims.WorkspaceID, projectID, service.CreateFolderParams{
		Name:     body.Name,
		ParentID: body.ParentID,
		Position: body.Position,
	})
	if err != nil {
		return Respond(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(folderDTOToResponse(dto))
}

// handleGetFolders returns the full folder tree for a project.
//
// @Summary Get folder tree
// @Description Returns a recursive tree of all folders in the project (up to 2 levels deep). Each folder includes its children in a <code>children</code> array and an <code>asset_count</code> showing how many assets are directly inside it (not counting descendants).
// @Tags Folders
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Success 200 {array} FolderResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Project not found"
// @Router /api/v1/projects/{id}/folders [get]
func (s *Server) handleGetFolders(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	projectID := c.Params("id")

	// Verify project belongs to workspace.
	if _, err := s.projects.Get(c.RequestCtx(), claims.WorkspaceID, projectID); err != nil {
		return Respond(c, err)
	}

	// Run the recursive CTE directly — the service/repo layer returns only a flat
	// list without asset counts, so the tree-building query stays here until
	// FolderRepository gains a ListTree method in a future step.
	rows, err := s.sqlDB.QueryContext(c.RequestCtx(), `
		WITH RECURSIVE tree AS (
			SELECT *, 0 AS depth FROM folders
			WHERE project_id = ? AND parent_id IS NULL AND workspace_id = ?
			UNION ALL
			SELECT f.*, t.depth + 1 FROM folders f
			JOIN tree t ON f.parent_id = t.id
			WHERE t.depth < 2
		)
		SELECT t.id, t.workspace_id, t.project_id, t.parent_id, t.name, t.slug, t.position, t.created_at, t.depth,
			(SELECT COUNT(*) FROM assets a WHERE a.folder_id = t.id AND a.workspace_id = ?) AS asset_count
		FROM tree t
		ORDER BY t.depth, t.position, t.name
	`, projectID, claims.WorkspaceID, claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load folders")
	}
	defer rows.Close()

	type flatRow struct {
		FolderResponse
		depth    int64
		id       string
		parentID *string
	}
	var flat []flatRow
	for rows.Next() {
		var f flatRow
		var depth int64
		var parentID *string
		var id string
		if err := rows.Scan(
			&id, &f.WorkspaceID, &f.ProjectID, &parentID,
			&f.Name, &f.Slug, &f.Position, &f.CreatedAt, &depth, &f.AssetCount,
		); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "scan failed")
		}
		f.id = id
		f.parentID = parentID
		f.depth = depth
		f.ID = id
		f.ParentID = parentID
		f.Children = []FolderResponse{}
		flat = append(flat, f)
	}
	if err := rows.Err(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "query failed")
	}

	rootMap := make(map[string]int)
	var roots []FolderResponse
	for _, row := range flat {
		if row.parentID == nil {
			rootMap[row.id] = len(roots)
			roots = append(roots, row.FolderResponse)
		}
	}
	for _, row := range flat {
		if row.parentID != nil {
			if idx, ok := rootMap[*row.parentID]; ok {
				roots[idx].Children = append(roots[idx].Children, row.FolderResponse)
			}
		}
	}

	if roots == nil {
		roots = []FolderResponse{}
	}
	return c.JSON(roots)
}

// handleUpdateFolder renames or repositions a folder.
//
// @Summary Update a folder
// @Description Updates the folder's name and/or position. All fields are optional. Renaming automatically regenerates the folder's URL slug.
// @Tags Folders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Folder ID"
// @Param body body UpdateFolderRequest true "Fields to update"
// @Success 200 {object} FolderResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Folder not found"
// @Failure 409 {object} ErrorResponse "A folder with that name already exists here"
// @Router /api/v1/folders/{id} [put]
func (s *Server) handleUpdateFolder(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	body, ok := decodeAndValidate(c, &UpdateFolderRequest{})
	if !ok {
		return nil
	}

	dto, err := s.folders.Update(c.RequestCtx(), claims.WorkspaceID, id, service.UpdateFolderParams{
		Name:     body.Name,
		Position: body.Position,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return errRes(c, fiber.StatusConflict, "a folder with that name already exists here")
		}
		return Respond(c, err)
	}

	return c.JSON(folderDTOToResponse(dto))
}

// handleDeleteFolder deletes a folder and its children.
//
// @Summary Delete a folder
// @Description Permanently deletes the folder and any child folders. Assets inside the deleted folder(s) are <em>not</em> deleted — their <code>folder_id</code> is set to null so they remain in the project.
// @Tags Folders
// @Produce json
// @Security BearerAuth
// @Param id path string true "Folder ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Folder not found"
// @Router /api/v1/folders/{id} [delete]
func (s *Server) handleDeleteFolder(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	if err := s.folders.Delete(c.RequestCtx(), claims.WorkspaceID, id); err != nil {
		return Respond(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// fiber:context-methods migrated
