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

func tagToResponse(t dbgen.Tag, assetCount int64) TagResponse {
	r := TagResponse{
		ID:         t.ID,
		Name:       t.Name,
		AssetCount: assetCount,
		Color:      t.Color,
		GroupName:  t.GroupName,
		CreatedAt:  t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if t.LastUsedAt != nil {
		s := t.LastUsedAt.UTC().Format("2006-01-02T15:04:05Z")
		r.LastUsedAt = &s
	}
	return r
}

// handleListTags handles GET /api/v1/tags
func (s *Server) handleListTags(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	rows, err := s.db.ListTagsWithCount(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list tags")
	}

	items := make([]TagResponse, len(rows))
	for i, row := range rows {
		items[i] = tagRowToResponse(row)
	}
	return c.JSON(items)
}

// handleCreateTag handles POST /api/v1/tags
func (s *Server) handleCreateTag(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &createTagRequest{})
	if !ok {
		return nil
	}

	// Check for existing tag
	_, err := s.db.GetTagByWorkspaceAndName(c.RequestCtx(), dbgen.GetTagByWorkspaceAndNameParams{
		WorkspaceID: claims.WorkspaceID,
		Name:        body.Name,
	})
	if err == nil {
		return errRes(c, fiber.StatusConflict, "a tag with this name already exists")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return errRes(c, fiber.StatusInternalServerError, "could not check tag")
	}

	tag, err := s.db.CreateTag(c.RequestCtx(), dbgen.CreateTagParams{
		ID:          uuid.NewString(),
		WorkspaceID: claims.WorkspaceID,
		Name:        body.Name,
		Color:       body.Color,
		GroupName:   body.GroupName,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create tag")
	}

	return c.Status(fiber.StatusCreated).JSON(tagToResponse(tag, 0))
}

// handlePatchTag handles PATCH /api/v1/tags/:name
func (s *Server) handlePatchTag(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	tagName := strings.ToLower(c.Params("name"))

	body, ok := decodeAndValidate(c, &patchTagRequest{})
	if !ok {
		return nil
	}

	// Verify the tag exists
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

	// Rename if requested
	finalName := existing.Name
	if body.Name != nil && *body.Name != existing.Name {
		newName := *body.Name
		// Check conflict
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

	// Update color/group_name if provided
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

	// Return updated tag with count
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
	defer tx.Rollback()
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
		row := tx.QueryRowContext(c.RequestCtx(), `SELECT COUNT(*) FROM asset_tags WHERE tag_id = ?`, tag.ID)
		if err := row.Scan(&count); err == nil {
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
	defer tx.Rollback()
	qtx := s.db.WithTx(tx)

	// 1. Upsert target tag
	target, err := qtx.GetOrCreateTag(c.RequestCtx(), dbgen.GetOrCreateTagParams{
		ID:          uuid.NewString(),
		WorkspaceID: claims.WorkspaceID,
		Name:        body.Target,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not ensure target tag")
	}

	// 2. For each source, reassign asset_tags and delete
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
		row := tx.QueryRowContext(c.RequestCtx(), `SELECT COUNT(*) FROM asset_tags WHERE tag_id = ?`, srcTag.ID)
		_ = row.Scan(&count)
		mergedAssets += count

		_, err = tx.ExecContext(c.RequestCtx(),
			`INSERT OR IGNORE INTO asset_tags (asset_id, tag_id)
			 SELECT asset_id, ? FROM asset_tags WHERE tag_id = ?`,
			target.ID, srcTag.ID,
		)
		if err != nil {
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

	// Reload target with count
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
func (s *Server) handleTagDuplicateSuggestions(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	rows, err := s.db.ListTagsWithCount(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list tags")
	}

	// Only compare tags that have at least 1 asset
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
func (s *Server) handleGetAssetTags(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

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

// handleAddTagToAsset handles POST /api/v1/assets/:id/tags
func (s *Server) handleAddTagToAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	body, ok := decodeAndValidate(c, &AddTagRequest{})
	if !ok {
		return nil
	}

	if _, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          assetID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	tag, err := s.db.GetOrCreateTag(c.RequestCtx(), dbgen.GetOrCreateTagParams{
		ID:          uuid.NewString(),
		WorkspaceID: claims.WorkspaceID,
		Name:        body.Name,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create tag")
	}

	if err := s.db.AddTagToAsset(c.RequestCtx(), dbgen.AddTagToAssetParams{
		AssetID: assetID,
		TagID:   tag.ID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not add tag")
	}

	// Touch last_used_at
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
func (s *Server) handleRemoveTagFromAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	tagName := strings.ToLower(c.Params("name"))

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
