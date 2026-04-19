package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// allAssetsInWorkspace returns true iff every ID in assetIDs belongs to workspaceID.
// Uses a single COUNT query with json_each to avoid N round-trips.
func (s *Server) allAssetsInWorkspace(ctx context.Context, workspaceID string, assetIDs []string) (bool, error) {
	if len(assetIDs) == 0 {
		return true, nil
	}
	idsJSON, err := json.Marshal(assetIDs)
	if err != nil {
		return false, err
	}
	row := s.sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM assets WHERE workspace_id = ? AND id IN (SELECT value FROM json_each(?))`,
		workspaceID, string(idsJSON),
	)
	var count int64
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count == int64(len(assetIDs)), nil
}

type CollectionResponse struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"created_by"`
	AssetCount  int64     `json:"asset_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func collectionRowToResponse(r dbgen.ListCollectionsRow) CollectionResponse {
	return CollectionResponse{
		ID:          r.ID,
		WorkspaceID: r.WorkspaceID,
		Name:        r.Name,
		Description: r.Description,
		CreatedBy:   r.CreatedBy,
		AssetCount:  r.AssetCount,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func collectionToResponse(c dbgen.Collection, assetCount int64) CollectionResponse {
	return CollectionResponse{
		ID:          c.ID,
		WorkspaceID: c.WorkspaceID,
		Name:        c.Name,
		Description: c.Description,
		CreatedBy:   c.CreatedBy,
		AssetCount:  assetCount,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

// @Summary Create a collection
// @Description Creates a named collection in the workspace. Supply <code>asset_ids</code> to pre-populate it (e.g. when saving a working stack). All asset IDs must belong to the authenticated workspace.
// @Tags Collections
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createCollectionRequest true "Collection details"
// @Success 201 {object} CollectionResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "One or more assets not in workspace"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/collections [post]
func (s *Server) handleCreateCollection(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &createCollectionRequest{})
	if !ok {
		return nil
	}

	// Verify all provided asset IDs belong to this workspace before inserting.
	if len(body.AssetIDs) > 0 {
		ok, err := s.allAssetsInWorkspace(c.RequestCtx(), claims.WorkspaceID, body.AssetIDs)
		if err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not verify assets")
		}
		if !ok {
			return errRes(c, fiber.StatusForbidden, "one or more assets not found in workspace")
		}
	}

	col, err := s.db.CreateCollection(c.RequestCtx(), dbgen.CreateCollectionParams{
		ID:          uuid.NewString(),
		WorkspaceID: claims.WorkspaceID,
		Name:        body.Name,
		Description: body.Description,
		CreatedBy:   claims.UserID,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create collection")
	}

	// If asset IDs were provided (e.g. "save stack as collection"), add them now.
	// INSERT OR IGNORE makes this idempotent; any other error is unexpected.
	added := int64(0)
	for _, aid := range body.AssetIDs {
		if err := s.db.AddCollectionAsset(c.RequestCtx(), dbgen.AddCollectionAssetParams{
			CollectionID:   col.ID,
			AssetID:        aid,
			CollectionID_2: col.ID,
		}); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not add asset to collection")
		}
		added++
	}

	return c.Status(fiber.StatusCreated).JSON(collectionToResponse(col, added))
}

// @Summary List collections
// @Description Returns all collections in the workspace with their asset counts.
// @Tags Collections
// @Produce json
// @Security BearerAuth
// @Success 200 {array} CollectionResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/collections [get]
func (s *Server) handleListCollections(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	rows, err := s.db.ListCollections(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list collections")
	}

	out := make([]CollectionResponse, len(rows))
	for i, r := range rows {
		out[i] = collectionRowToResponse(r)
	}
	return c.JSON(out)
}

// @Summary Get a collection
// @Description Returns collection metadata and its full asset list, ordered by position then insertion date.
// @Tags Collections
// @Produce json
// @Security BearerAuth
// @Param id path string true "Collection ID"
// @Success 200 {object} CollectionResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Collection not found"
// @Router /api/v1/collections/{id} [get]
func (s *Server) handleGetCollection(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	col, err := s.db.GetCollection(c.RequestCtx(), dbgen.GetCollectionParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "collection not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not get collection")
	}

	assets, err := s.db.ListCollectionAssets(c.RequestCtx(), id)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list collection assets")
	}

	type collectionDetailResponse struct {
		CollectionResponse
		Assets []AssetResponse `json:"assets"`
	}

	assetResponses := make([]AssetResponse, len(assets))
	for i, a := range assets {
		assetResponses[i] = assetToResponse(a, nil)
	}

	return c.JSON(collectionDetailResponse{
		CollectionResponse: collectionToResponse(col, int64(len(assets))),
		Assets:             assetResponses,
	})
}

// @Summary Update a collection
// @Description Updates the collection's name and description. Requires editor role.
// @Tags Collections
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Collection ID"
// @Param body body updateCollectionRequest true "Fields to update"
// @Success 200 {object} CollectionResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Insufficient role"
// @Failure 404 {object} ErrorResponse "Collection not found"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/collections/{id} [put]
func (s *Server) handleUpdateCollection(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	body, ok := decodeAndValidate(c, &updateCollectionRequest{})
	if !ok {
		return nil
	}

	col, err := s.db.UpdateCollection(c.RequestCtx(), dbgen.UpdateCollectionParams{
		Name:        body.Name,
		Description: body.Description,
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "collection not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not update collection")
	}

	count, err := s.db.CollectionBelongsToWorkspace(c.RequestCtx(), dbgen.CollectionBelongsToWorkspaceParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not get collection asset count")
	}
	return c.JSON(collectionToResponse(col, count))
}

// @Summary Delete a collection
// @Description Deletes the collection and removes all its asset memberships. The assets themselves are not deleted. Requires owner role.
// @Tags Collections
// @Produce json
// @Security BearerAuth
// @Param id path string true "Collection ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Insufficient role"
// @Failure 404 {object} ErrorResponse "Collection not found"
// @Router /api/v1/collections/{id} [delete]
func (s *Server) handleDeleteCollection(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	_, err := s.db.GetCollection(c.RequestCtx(), dbgen.GetCollectionParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "collection not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not get collection")
	}

	if err := s.db.DeleteCollection(c.RequestCtx(), dbgen.DeleteCollectionParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete collection")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Add asset to collection
// @Description Adds a single asset to the collection. Idempotent — adding an asset that is already in the collection succeeds silently. Requires editor role.
// @Tags Collections
// @Produce json
// @Security BearerAuth
// @Param id path string true "Collection ID"
// @Param aid path string true "Asset ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Insufficient role"
// @Failure 404 {object} ErrorResponse "Collection or asset not found"
// @Router /api/v1/collections/{id}/assets/{aid} [post]
func (s *Server) handleAddCollectionAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")
	assetID := c.Params("aid")

	// Verify collection belongs to workspace.
	count, err := s.db.CollectionBelongsToWorkspace(c.RequestCtx(), dbgen.CollectionBelongsToWorkspaceParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not verify collection")
	}
	if count == 0 {
		return errRes(c, fiber.StatusNotFound, "collection not found")
	}

	// Verify asset belongs to workspace.
	_, err = s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not get asset")
	}

	if err := s.db.AddCollectionAsset(c.RequestCtx(), dbgen.AddCollectionAssetParams{
		CollectionID:   id,
		AssetID:        assetID,
		CollectionID_2: id,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not add asset to collection")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Remove asset from collection
// @Description Removes a single asset from the collection. The asset itself is not deleted. Requires editor role.
// @Tags Collections
// @Produce json
// @Security BearerAuth
// @Param id path string true "Collection ID"
// @Param aid path string true "Asset ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Insufficient role"
// @Failure 404 {object} ErrorResponse "Collection not found"
// @Router /api/v1/collections/{id}/assets/{aid} [delete]
func (s *Server) handleRemoveCollectionAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")
	assetID := c.Params("aid")

	count, err := s.db.CollectionBelongsToWorkspace(c.RequestCtx(), dbgen.CollectionBelongsToWorkspaceParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not verify collection")
	}
	if count == 0 {
		return errRes(c, fiber.StatusNotFound, "collection not found")
	}

	if err := s.db.RemoveCollectionAsset(c.RequestCtx(), dbgen.RemoveCollectionAssetParams{
		CollectionID: id,
		AssetID:      assetID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not remove asset from collection")
	}

	return c.SendStatus(fiber.StatusNoContent)
}
