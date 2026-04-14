package api

import (
	"database/sql"
	"errors"
	"time"

	"damask/server/internal/audit"
	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type ProjectResponse struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspace_id"`
	Name         string    `json:"name"`
	Description  *string   `json:"description"`
	Color        *string   `json:"color"`
	CoverAssetID *string   `json:"cover_asset_id"`
	AssetCount   int64     `json:"asset_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func projectToResponse(p dbgen.Project, assetCount int64) ProjectResponse {
	return ProjectResponse{
		ID:           p.ID,
		WorkspaceID:  p.WorkspaceID,
		Name:         p.Name,
		Description:  p.Description,
		Color:        p.Color,
		CoverAssetID: p.CoverAssetID,
		AssetCount:   assetCount,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

// handleCreateProject creates a new project in the active workspace.
//
// @Summary Create a project
// @Description Creates a new project container. Projects group assets and folders together. An optional color (hex) and description can be provided at creation time.
// @Tags Projects
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateProjectRequest true "Project details"
// @Success 201 {object} ProjectResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/projects [post]
func (s *Server) handleCreateProject(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &CreateProjectRequest{})
	if !ok {
		return nil
	}

	p, err := s.db.CreateProject(c.RequestCtx(), dbgen.CreateProjectParams{
		ID:          uuid.NewString(),
		WorkspaceID: claims.WorkspaceID,
		Name:        body.Name,
		Description: body.Description,
		Color:       body.Color,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create project")
	}

	userID := claims.UserID
	s.audit.WriteProject(c.RequestCtx(), audit.ProjectEvent{
		WorkspaceID: claims.WorkspaceID,
		ProjectID:   p.ID,
		UserID:      &userID,
		ActorType:   audit.ActorTypeUser,
		EventType:   audit.EventProjectCreated,
		Payload:     audit.ProjectCreatedPayload{V: 1, Name: p.Name},
	})

	return c.Status(fiber.StatusCreated).JSON(projectToResponse(p, 0))
}

// handleListProjects returns all projects in the active workspace.
//
// @Summary List projects
// @Description Returns all projects in the workspace, each with an <code>asset_count</code> field reflecting the number of assets currently assigned to it.
// @Tags Projects
// @Produce json
// @Security BearerAuth
// @Success 200 {array} ProjectResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/projects [get]
func (s *Server) handleListProjects(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	rows, err := s.db.ListProjectsWithCount(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list projects")
	}

	items := make([]ProjectResponse, len(rows))
	for i, row := range rows {
		items[i] = ProjectResponse{
			ID:           row.ID,
			WorkspaceID:  row.WorkspaceID,
			Name:         row.Name,
			Description:  row.Description,
			Color:        row.Color,
			CoverAssetID: row.CoverAssetID,
			AssetCount:   row.AssetCount,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
		}
	}

	return c.JSON(items)
}

// handleGetProject returns a single project by ID.
//
// @Summary Get a project
// @Description Returns the project with the given ID. Returns 404 if the project does not exist or belongs to a different workspace.
// @Tags Projects
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Success 200 {object} ProjectResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Project not found"
// @Router /api/v1/projects/{id} [get]
func (s *Server) handleGetProject(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	p, err := s.db.GetProjectByID(c.RequestCtx(), dbgen.GetProjectByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "project not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load project")
	}

	// Get asset count separately
	rows, err := s.db.ListProjectsWithCount(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load project")
	}
	var count int64
	for _, row := range rows {
		if row.ID == id {
			count = row.AssetCount
			break
		}
	}

	return c.JSON(projectToResponse(p, count))
}

// handleUpdateProject updates a project's metadata.
//
// @Summary Update a project
// @Description Updates the project's name, description, color, or cover asset. All fields are optional — only the fields present in the request body are updated; omitted fields retain their current values.
// @Tags Projects
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Param body body UpdateProjectRequest true "Fields to update"
// @Success 200 {object} ProjectResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Project not found"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/projects/{id} [put]
func (s *Server) handleUpdateProject(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	// Verify project exists and belongs to workspace
	before, err := s.db.GetProjectByID(c.RequestCtx(), dbgen.GetProjectByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "project not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load project")
	}

	body, ok := decodeAndValidate(c, &UpdateProjectRequest{})
	if !ok {
		return nil
	}

	if body.Color == nil {
		body.Color = before.Color
	}
	if body.CoverAssetID == nil {
		body.CoverAssetID = before.CoverAssetID
	}
	if body.Description == nil {
		body.Description = before.Description
	}
	if body.Name == nil {
		body.Name = &before.Name
	}

	p, err := s.db.UpdateProject(c.RequestCtx(), dbgen.UpdateProjectParams{
		Name:         body.Name,
		Description:  body.Description,
		Color:        body.Color,
		CoverAssetID: body.CoverAssetID,
		ID:           id,
		WorkspaceID:  claims.WorkspaceID,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not update project")
	}

	if p.Name != before.Name {
		userID := claims.UserID
		s.audit.WriteProject(c.RequestCtx(), audit.ProjectEvent{
			WorkspaceID: claims.WorkspaceID,
			ProjectID:   id,
			UserID:      &userID,
			ActorType:   audit.ActorTypeUser,
			EventType:   audit.EventProjectRenamed,
			Payload:     audit.ProjectRenamedPayload{V: 1, Before: before.Name, After: p.Name},
		})
	}

	return c.JSON(projectToResponse(p, 0))
}

// handleDeleteProject deletes a project.
//
// @Summary Delete a project
// @Description Permanently deletes the project. Assets that belonged to the project are <em>not</em> deleted — their <code>project_id</code> is set to null so they remain accessible in the unfiltered asset library.
// @Tags Projects
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Project not found"
// @Router /api/v1/projects/{id} [delete]
func (s *Server) handleDeleteProject(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	// Verify exists
	if _, err := s.db.GetProjectByID(c.RequestCtx(), dbgen.GetProjectByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "project not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load project")
	}

	tx, err := s.sqlDB.BeginTx(c.RequestCtx(), nil)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not start transaction")
	}
	defer tx.Rollback()

	qtx := s.db.WithTx(tx)

	if err := qtx.NullifyProjectAssets(c.RequestCtx(), dbgen.NullifyProjectAssetsParams{
		ProjectID:   &id,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not unlink assets")
	}

	if err := qtx.DeleteProject(c.RequestCtx(), dbgen.DeleteProjectParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete project")
	}

	if err := tx.Commit(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not commit transaction")
	}

	userID := claims.UserID
	s.audit.WriteProject(c.RequestCtx(), audit.ProjectEvent{
		WorkspaceID: claims.WorkspaceID,
		ProjectID:   id,
		UserID:      &userID,
		ActorType:   audit.ActorTypeUser,
		EventType:   audit.EventProjectDeleted,
		Payload:     audit.ProjectDeletedPayload{V: 1},
	})

	return c.SendStatus(fiber.StatusNoContent)
}

// fiber:context-methods migrated
