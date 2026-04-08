package api

import (
	"database/sql"
	"errors"
	"strings"

	"damask/server/internal/audit"
	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type TagResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	AssetCount int64  `json:"asset_count"`
}

func (s *Server) handleListTags(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	rows, err := s.db.ListTagsWithCount(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list tags")
	}

	items := make([]TagResponse, len(rows))
	for i, row := range rows {
		items[i] = TagResponse{ID: row.ID, Name: row.Name, AssetCount: row.AssetCount}
	}
	return c.JSON(items)
}

func (s *Server) handleGetAssetTags(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	// Verify asset belongs to workspace
	if _, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	tags, err := s.db.GetTagsForAsset(c.RequestCtx(), assetID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load tags")
	}

	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return c.JSON(names)
}

func (s *Server) handleAddTagToAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	body, ok := decodeAndValidate(c, &AddTagRequest{})
	if !ok {
		return nil
	}

	// Verify asset belongs to workspace
	if _, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	// Get or create tag
	tag, err := s.db.GetOrCreateTag(c.RequestCtx(), dbgen.GetOrCreateTagParams{
		ID:          uuid.NewString(),
		WorkspaceID: claims.WorkspaceID,
		Name:        body.Name,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create tag")
	}

	// Add to asset (idempotent)
	if err := s.db.AddTagToAsset(c.RequestCtx(), dbgen.AddTagToAssetParams{
		AssetID: assetID,
		TagID:   tag.ID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not add tag")
	}

	userID := claims.UserID
	s.audit.WriteAsset(c.RequestCtx(), audit.AssetEvent{
		WorkspaceID: claims.WorkspaceID,
		AssetID:     assetID,
		UserID:      &userID,
		ActorType:   audit.ActorTypeUser,
		EventType:   audit.EventAssetTagged,
		Payload:     audit.AssetTaggedPayload{V: 1, Tag: tag.Name},
	})

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"name": tag.Name})
}

func (s *Server) handleRemoveTagFromAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	tagName := strings.ToLower(c.Params("name"))

	// Verify asset belongs to workspace
	if _, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	if err := s.db.RemoveTagFromAsset(c.RequestCtx(), dbgen.RemoveTagFromAssetParams{
		AssetID:     assetID,
		WorkspaceID: claims.WorkspaceID,
		Name:        tagName,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not remove tag")
	}

	userID := claims.UserID
	s.audit.WriteAsset(c.RequestCtx(), audit.AssetEvent{
		WorkspaceID: claims.WorkspaceID,
		AssetID:     assetID,
		UserID:      &userID,
		ActorType:   audit.ActorTypeUser,
		EventType:   audit.EventAssetUntagged,
		Payload:     audit.AssetUntaggedPayload{V: 1, Tag: tagName},
	})

	return c.SendStatus(fiber.StatusNoContent)
}

// fiber:context-methods migrated
