package api

import (
	"database/sql"
	"errors"
	"strings"

	"creativo-dam/server/internal/auth"
	dbgen "creativo-dam/server/internal/db/gen"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type tagResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	AssetCount  int64  `json:"asset_count"`
}

func (s *Server) handleListTags(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)

	rows, err := s.db.ListTagsWithCount(c.Context(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list tags")
	}

	items := make([]tagResponse, len(rows))
	for i, row := range rows {
		items[i] = tagResponse{ID: row.ID, Name: row.Name, AssetCount: row.AssetCount}
	}
	return c.JSON(items)
}

func (s *Server) handleGetAssetTags(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	// Verify asset belongs to workspace
	if _, err := s.db.GetAssetByID(c.Context(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	tags, err := s.db.GetTagsForAsset(c.Context(), assetID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load tags")
	}

	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return c.JSON(names)
}

func (s *Server) handleAddTagToAsset(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	var body struct {
		Name string `json:"name"`
	}
	if err := c.BodyParser(&body); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	body.Name = strings.TrimSpace(strings.ToLower(body.Name))
	if body.Name == "" {
		return errRes(c, fiber.StatusBadRequest, "name is required")
	}

	// Verify asset belongs to workspace
	if _, err := s.db.GetAssetByID(c.Context(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	// Get or create tag
	tag, err := s.db.GetOrCreateTag(c.Context(), dbgen.GetOrCreateTagParams{
		ID:          uuid.NewString(),
		WorkspaceID: claims.WorkspaceID,
		Name:        body.Name,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create tag")
	}

	// Add to asset (idempotent)
	if err := s.db.AddTagToAsset(c.Context(), dbgen.AddTagToAssetParams{
		AssetID: assetID,
		TagID:   tag.ID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not add tag")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"name": tag.Name})
}

func (s *Server) handleRemoveTagFromAsset(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	tagName := strings.ToLower(c.Params("name"))

	// Verify asset belongs to workspace
	if _, err := s.db.GetAssetByID(c.Context(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	if err := s.db.RemoveTagFromAsset(c.Context(), dbgen.RemoveTagFromAssetParams{
		AssetID:     assetID,
		WorkspaceID: claims.WorkspaceID,
		Name:        tagName,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not remove tag")
	}

	return c.SendStatus(fiber.StatusNoContent)
}
