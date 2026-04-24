package api

import (
	"database/sql"
	"errors"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"damask/server/internal/audit"
	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/service"

	"github.com/agnivade/levenshtein"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

var hexColorRegex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// TagResponse is the extended shape returned by all tag endpoints.
type TagResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	AssetCount int64   `json:"asset_count"`
	Color      *string `json:"color"`
	GroupName  *string `json:"group_name"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
}

func tagDTOToResponse(d *service.TagDTO) TagResponse {
	r := TagResponse{
		ID:         d.ID,
		Name:       d.Name,
		AssetCount: d.AssetCount,
		Color:      d.Color,
		GroupName:  d.GroupName,
		CreatedAt:  d.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if d.LastUsedAt != nil {
		s := d.LastUsedAt.UTC().Format("2006-01-02T15:04:05Z")
		r.LastUsedAt = &s
	}
	return r
}

// tagRowToResponse converts a ListTagsWithCount row (used by handlers not yet wired to the service).
func tagRowToResponse(row dbgen.ListTagsWithCountRow) TagResponse {
	r := TagResponse{
		ID:         row.ID,
		Name:       row.Name,
		AssetCount: row.AssetCount,
		Color:      row.Color,
		GroupName:  row.GroupName,
		CreatedAt:  row.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if row.LastUsedAt != nil {
		s := row.LastUsedAt.UTC().Format("2006-01-02T15:04:05Z")
		r.LastUsedAt = &s
	}
	return r
}

// handleListTags handles GET /api/v1/tags
//
// @Summary List tags
// @Tags Tags
// @Produce json
// @Security BearerAuth
// @Success 200 {array} TagResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/tags [get]
func (s *Server) handleListTags(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	dtos, err := s.tags.List(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		return Respond(c, err)
	}

	items := make([]TagResponse, len(dtos))
	for i, d := range dtos {
		items[i] = tagDTOToResponse(d)
	}
	return c.JSON(items)
}

// handleCreateTag handles POST /api/v1/tags
//
// @Summary Create a tag
// @Tags Tags
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createTagRequest true "Tag details"
// @Success 201 {object} TagResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 409 {object} ErrorResponse "Tag already exists"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/tags [post]
func (s *Server) handleCreateTag(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &createTagRequest{})
	if !ok {
		return nil
	}

	dto, err := s.tags.Create(c.RequestCtx(), claims.WorkspaceID, service.CreateTagParams{
		Name:      body.Name,
		Color:     body.Color,
		GroupName: body.GroupName,
	})
	if err != nil {
		return Respond(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(tagDTOToResponse(dto))
}

// handlePatchTag handles PATCH /api/v1/tags/:name
// Kept on s.db: the service Patch does not yet update color/group_name.
//
// @Summary Update a tag
// @Tags Tags
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param name path string true "Current tag name"
// @Param body body patchTagRequest true "Fields to update"
// @Success 200 {object} TagResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Tag not found"
// @Failure 409 {object} ErrorResponse "Target name already in use"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/tags/{name} [patch]
func (s *Server) handlePatchTag(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	tagName := strings.ToLower(c.Params("name"))

	body, ok := decodeAndValidate(c, &patchTagRequest{})
	if !ok {
		return nil
	}

	existing, err := s.db.GetTagByWorkspaceAndName(c.RequestCtx(), dbgen.GetTagByWorkspaceAndNameParams{
		WorkspaceID: claims.WorkspaceID,
		Name:        tagName,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "tag not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load tag")
	}

	finalName := existing.Name
	if body.Name != nil && *body.Name != existing.Name {
		newName := *body.Name
		_, err := s.db.GetTagByWorkspaceAndName(c.RequestCtx(), dbgen.GetTagByWorkspaceAndNameParams{
			WorkspaceID: claims.WorkspaceID,
			Name:        newName,
		})
		if err == nil {
			return errRes(c, fiber.StatusConflict, "a tag with this name already exists")
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusInternalServerError, "could not check tag name")
		}
		if err := s.db.UpdateTagName(c.RequestCtx(), dbgen.UpdateTagNameParams{
			Name:        newName,
			WorkspaceID: claims.WorkspaceID,
			Name_2:      tagName,
		}); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not rename tag")
		}
		finalName = newName
	}

	newColor := existing.Color
	if body.Color != nil {
		newColor = body.Color
	}
	newGroup := existing.GroupName
	if body.GroupName != nil {
		newGroup = body.GroupName
	}
	if body.Color != nil || body.GroupName != nil {
		if err := s.db.UpdateTagMetadata(c.RequestCtx(), dbgen.UpdateTagMetadataParams{
			Color:       newColor,
			GroupName:   newGroup,
			WorkspaceID: claims.WorkspaceID,
			Name:        finalName,
		}); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not update tag")
		}
	}

	rows, err := s.db.ListTagsWithCount(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load tag")
	}
	for _, row := range rows {
		if row.Name == finalName {
			return c.JSON(tagRowToResponse(row))
		}
	}
	return errRes(c, fiber.StatusInternalServerError, "could not reload tag")
}

// handleBulkDeleteTags handles DELETE /api/v1/tags
// Kept on s.db: returns removed_from_assets count not covered by service.Delete.
//
// @Summary Bulk delete tags
// @Tags Tags
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body bulkDeleteTagsRequest true "Tag names to delete"
// @Success 200 {object} map[string]int
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/tags [delete]
func (s *Server) handleBulkDeleteTags(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &bulkDeleteTagsRequest{})
	if !ok {
		return nil
	}

	tx, err := s.sqlDB.BeginTx(c.RequestCtx(), nil)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not begin transaction")
	}
	defer tx.Rollback() //nolint:errcheck
	qtx := s.db.WithTx(tx)

	deleted := 0
	removedFromAssets := int64(0)

	for _, name := range body.Names {
		tag, err := qtx.GetTagByWorkspaceAndName(c.RequestCtx(), dbgen.GetTagByWorkspaceAndNameParams{
			WorkspaceID: claims.WorkspaceID,
			Name:        name,
		})
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not look up tag")
		}

		var count int64
		if err := tx.QueryRowContext(c.RequestCtx(), `SELECT COUNT(*) FROM asset_tags WHERE tag_id = ?`, tag.ID).Scan(&count); err == nil {
			removedFromAssets += count
		}

		if err := qtx.DeleteTag(c.RequestCtx(), dbgen.DeleteTagParams{
			WorkspaceID: claims.WorkspaceID,
			Name:        name,
		}); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not delete tag")
		}
		deleted++
	}

	if err := tx.Commit(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not commit transaction")
	}

	return c.JSON(fiber.Map{
		"deleted":             deleted,
		"removed_from_assets": removedFromAssets,
	})
}

// handleMergeTags handles POST /api/v1/tags/merge
// Kept on s.db: complex transaction with reassign + delete not yet in service.
//
// @Summary Merge tags
// @Tags Tags
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body mergeTagsRequest true "Source tag names and target tag name"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/tags/merge [post]
func (s *Server) handleMergeTags(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &mergeTagsRequest{})
	if !ok {
		return nil
	}

	tx, err := s.sqlDB.BeginTx(c.RequestCtx(), nil)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not begin transaction")
	}
	defer tx.Rollback() //nolint:errcheck
	qtx := s.db.WithTx(tx)

	target, err := qtx.GetOrCreateTag(c.RequestCtx(), dbgen.GetOrCreateTagParams{
		ID:          uuid.NewString(),
		WorkspaceID: claims.WorkspaceID,
		Name:        body.Target,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not ensure target tag")
	}

	mergedAssets := int64(0)
	for _, src := range body.Sources {
		srcTag, err := qtx.GetTagByWorkspaceAndName(c.RequestCtx(), dbgen.GetTagByWorkspaceAndNameParams{
			WorkspaceID: claims.WorkspaceID,
			Name:        src,
		})
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not load source tag")
		}

		var count int64
		_ = tx.QueryRowContext(c.RequestCtx(), `SELECT COUNT(*) FROM asset_tags WHERE tag_id = ?`, srcTag.ID).Scan(&count)
		mergedAssets += count

		if _, err = tx.ExecContext(c.RequestCtx(),
			`INSERT OR IGNORE INTO asset_tags (asset_id, tag_id)
			 SELECT asset_id, ? FROM asset_tags WHERE tag_id = ?`,
			target.ID, srcTag.ID,
		); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not reassign asset tags")
		}

		if err := qtx.DeleteTag(c.RequestCtx(), dbgen.DeleteTagParams{
			WorkspaceID: claims.WorkspaceID,
			Name:        src,
		}); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not delete source tag")
		}
	}

	if err := tx.Commit(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not commit merge")
	}

	allRows, err := s.db.ListTagsWithCount(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not reload tag")
	}
	var targetResp TagResponse
	for _, row := range allRows {
		if row.ID == target.ID {
			targetResp = tagRowToResponse(row)
			break
		}
	}

	return c.JSON(fiber.Map{
		"merged_assets": mergedAssets,
		"target":        targetResp,
	})
}

// handleTagDuplicateSuggestions handles GET /api/v1/tags/suggestions/duplicates
// Kept on s.db: needs AssetCount from ListTagsWithCount, not yet fully in service.
//
// @Summary Get duplicate tag suggestions
// @Tags Tags
// @Produce json
// @Security BearerAuth
// @Success 200 {array} map[string]interface{}
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/tags/suggestions/duplicates [get]
func (s *Server) handleTagDuplicateSuggestions(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	rows, err := s.db.ListTagsWithCount(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list tags")
	}

	active := make([]dbgen.ListTagsWithCountRow, 0, len(rows))
	for _, r := range rows {
		if r.AssetCount > 0 {
			active = append(active, r)
		}
	}

	type pair struct {
		A     string  `json:"a"`
		B     string  `json:"b"`
		Score float64 `json:"score"`
	}
	var pairs []pair

	for i := 0; i < len(active) && len(pairs) < 20; i++ {
		for j := i + 1; j < len(active) && len(pairs) < 20; j++ {
			a := strings.ToLower(active[i].Name)
			b := strings.ToLower(active[j].Name)
			maxLen := math.Max(float64(utf8.RuneCountInString(a)), float64(utf8.RuneCountInString(b)))
			if maxLen == 0 {
				continue
			}
			dist := levenshtein.ComputeDistance(a, b)
			score := float64(dist) / maxLen
			if score < 0.3 {
				pairs = append(pairs, pair{
					A:     active[i].Name,
					B:     active[j].Name,
					Score: math.Round(score*100) / 100,
				})
			}
		}
	}

	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Score < pairs[j].Score })

	if pairs == nil {
		pairs = []pair{}
	}
	return c.JSON(pairs)
}

// handleGetAssetTags handles GET /api/v1/assets/:id/tags
//
// @Summary Get tags for an asset
// @Tags Assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {array} string
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/tags [get]
func (s *Server) handleGetAssetTags(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	if _, err := s.assets.Get(c.RequestCtx(), claims.WorkspaceID, assetID); err != nil {
		return Respond(c, err)
	}

	tagDTOs, err := s.tags.ListForAsset(c.RequestCtx(), assetID)
	if err != nil {
		return Respond(c, err)
	}

	names := make([]string, len(tagDTOs))
	for i, t := range tagDTOs {
		names[i] = t.Name
	}
	return c.JSON(names)
}

// handleAddTagToAsset handles POST /api/v1/assets/:id/tags
//
// @Summary Add tag to an asset
// @Tags Assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param body body AddTagRequest true "Tag name"
// @Success 201 {object} map[string]string
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/assets/{id}/tags [post]
func (s *Server) handleAddTagToAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	body, ok := decodeAndValidate(c, &AddTagRequest{})
	if !ok {
		return nil
	}

	if _, err := s.assets.Get(c.RequestCtx(), claims.WorkspaceID, assetID); err != nil {
		return Respond(c, err)
	}

	tag, err := s.tags.AddToAsset(c.RequestCtx(), claims.WorkspaceID, assetID, body.Name)
	if err != nil {
		return Respond(c, err)
	}

	// Touch last_used_at (fire-and-forget).
	_ = s.db.TouchTagLastUsed(c.RequestCtx(), dbgen.TouchTagLastUsedParams{
		WorkspaceID: claims.WorkspaceID,
		Name:        tag.Name,
	})

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

// handleRemoveTagFromAsset handles DELETE /api/v1/assets/:id/tags/:name
//
// @Summary Remove tag from an asset
// @Tags Assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param name path string true "Tag name"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/tags/{name} [delete]
func (s *Server) handleRemoveTagFromAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	tagName := strings.ToLower(c.Params("name"))

	if _, err := s.assets.Get(c.RequestCtx(), claims.WorkspaceID, assetID); err != nil {
		return Respond(c, err)
	}

	if err := s.tags.RemoveFromAsset(c.RequestCtx(), claims.WorkspaceID, assetID, tagName); err != nil {
		return Respond(c, err)
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
