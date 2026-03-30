package api

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// shareAccessResponse is returned by POST /s/:id/access on success.
type shareAccessResponse struct {
	Token string `json:"token"`
}

// commentResponse is the JSON shape for a share comment.
type commentResponse struct {
	ID          string  `json:"id"`
	ShareID     string  `json:"share_id"`
	AssetID     string  `json:"asset_id"`
	AuthorName  string  `json:"author_name"`
	AuthorEmail *string `json:"author_email"`
	Body        string  `json:"body"`
	CreatedAt   string  `json:"created_at"`
}

func commentToResponse(c dbgen.ShareComment) commentResponse {
	var email *string
	if c.AuthorEmail.Valid {
		email = &c.AuthorEmail.String
	}
	return commentResponse{
		ID:          c.ID,
		ShareID:     c.ShareID,
		AssetID:     c.AssetID,
		AuthorName:  c.AuthorName,
		AuthorEmail: email,
		Body:        c.Body,
		CreatedAt:   c.CreatedAt,
	}
}

// isShareExpired returns true if the share's expires_at is in the past.
func isShareExpired(share dbgen.Share) bool {
	if !share.ExpiresAt.Valid {
		return false
	}
	t, err := time.Parse("2006-01-02T15:04:05Z", share.ExpiresAt.String)
	if err != nil {
		t, err = time.Parse("2006-01-02 15:04:05", share.ExpiresAt.String)
	}
	return err == nil && time.Now().After(t)
}

// loadActiveShare loads a share by ID and validates it is not revoked or expired.
// Returns 404 if not found or revoked (preserves audit trail behaviour).
// Returns 410 Gone if expired.
// Uses fiber.NewError so callers receive a non-nil error on failure.
func (s *Server) loadActiveShare(c fiber.Ctx, id string) (dbgen.Share, error) {
	share, err := s.db.GetShareByID(c.RequestCtx(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dbgen.Share{}, fiber.NewError(fiber.StatusNotFound, "share not found")
		}
		return dbgen.Share{}, fiber.NewError(fiber.StatusInternalServerError, "could not load share")
	}

	if share.RevokedAt.Valid {
		return dbgen.Share{}, fiber.NewError(fiber.StatusNotFound, "share not found")
	}

	if isShareExpired(share) {
		return dbgen.Share{}, fiber.NewError(fiber.StatusGone, "share has expired")
	}

	return share, nil
}

// ── S-4 ──────────────────────────────────────────────────────────────────────

// POST /s/:id/access — unauthenticated.
// Validates the share, checks password if required, issues a share session token.
func (s *Server) handleShareAccess(c fiber.Ctx) error {
	id := c.Params("id")

	share, err := s.loadActiveShare(c, id)
	if err != nil {
		return err
	}

	// Parse optional password
	var body struct {
		Password string `json:"password"`
	}
	_ = c.Bind().Body(&body) // body is optional

	// Check password
	if share.PasswordHash.Valid {
		if body.Password == "" {
			return errRes(c, fiber.StatusUnauthorized, "password required")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(share.PasswordHash.String), []byte(body.Password)); err != nil {
			return errRes(c, fiber.StatusUnauthorized, "incorrect password")
		}
	}

	// Increment view count (best-effort — do not fail the request on error)
	_ = s.db.IncrementShareViewCount(c.RequestCtx(), share.ID)

	// Issue 24-hour share session token
	token, err := s.tokenMaker.CreateShareToken(
		share.ID,
		share.TargetType,
		share.TargetID,
		share.AllowComments == 1,
		share.AllowDownload == 1,
		24*time.Hour,
	)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not issue share token")
	}

	return c.JSON(shareAccessResponse{Token: token})
}

// ── S-5 ──────────────────────────────────────────────────────────────────────

// GET /s/:id/assets
// Lists assets belonging to the share's target. Requires valid share session token.
func (s *Server) handleShareListAssets(c fiber.Ctx) error {
	sc := auth.GetShareClaims(c)
	shareID := c.Params("id")

	// Re-check expiry/revocation on every request
	if err := s.reCheckShare(c, shareID); err != nil {
		return err
	}

	var assets []dbgen.Asset

	switch sc.TargetType {
	case "asset":
		// Single-asset share — return just that asset
		asset, err := s.sqlDB.QueryContext(c.RequestCtx(), `
			SELECT id, workspace_id, project_id, folder_id, original_filename, storage_key,
			       mime_type, size, width, height, thumbnail_key, metadata, created_at, updated_at
			FROM assets WHERE id = ?`, sc.TargetID)
		if err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not load asset")
		}
		defer asset.Close()
		for asset.Next() {
			var a dbgen.Asset
			if err := asset.Scan(
				&a.ID, &a.WorkspaceID, &a.ProjectID, &a.FolderID, &a.OriginalFilename, &a.StorageKey,
				&a.MimeType, &a.Size, &a.Width, &a.Height, &a.ThumbnailKey, &a.Metadata,
				&a.CreatedAt, &a.UpdatedAt,
			); err != nil {
				return errRes(c, fiber.StatusInternalServerError, "scan failed")
			}
			assets = append(assets, a)
		}

	case "project":
		rows, err := s.sqlDB.QueryContext(c.RequestCtx(), `
			SELECT id, workspace_id, project_id, folder_id, original_filename, storage_key,
			       mime_type, size, width, height, thumbnail_key, metadata, created_at, updated_at
			FROM assets WHERE project_id = ?
			ORDER BY created_at DESC, id DESC`, sc.TargetID)
		if err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not list assets")
		}
		defer rows.Close()
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

	case "collection":
		// Collections not yet implemented — return empty list
		assets = []dbgen.Asset{}
	}

	items := make([]assetResponse, len(assets))
	for i, a := range assets {
		items[i] = assetToResponse(a, []string{})
	}
	return c.JSON(items)
}

// GET /s/:id/assets/:aid
// Returns a single asset detail. Requires valid share session token.
func (s *Server) handleShareGetAsset(c fiber.Ctx) error {
	sc := auth.GetShareClaims(c)
	shareID := c.Params("id")
	assetID := c.Params("aid")

	// Re-check share liveness
	if err := s.reCheckShare(c, shareID); err != nil {
		return err
	}

	// Confirm asset is part of this share
	if err := s.assertAssetInShare(c, sc, assetID); err != nil {
		return err
	}

	row := s.sqlDB.QueryRowContext(c.RequestCtx(), `
		SELECT id, workspace_id, project_id, folder_id, original_filename, storage_key,
		       mime_type, size, width, height, thumbnail_key, metadata, created_at, updated_at
		FROM assets WHERE id = ?`, assetID)
	var a dbgen.Asset
	if err := row.Scan(
		&a.ID, &a.WorkspaceID, &a.ProjectID, &a.FolderID, &a.OriginalFilename, &a.StorageKey,
		&a.MimeType, &a.Size, &a.Width, &a.Height, &a.ThumbnailKey, &a.Metadata,
		&a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	return c.JSON(assetToResponse(a, []string{}))
}

// GET /s/:id/assets/:aid/file
// Streams the original file. Requires allow_download in share session token.
func (s *Server) handleShareGetAssetFile(c fiber.Ctx) error {
	sc := auth.GetShareClaims(c)
	shareID := c.Params("id")
	assetID := c.Params("aid")

	if !sc.AllowDownload {
		return errRes(c, fiber.StatusForbidden, "download not allowed for this share")
	}

	if err := s.reCheckShare(c, shareID); err != nil {
		return err
	}
	if err := s.assertAssetInShare(c, sc, assetID); err != nil {
		return err
	}

	row := s.sqlDB.QueryRowContext(c.RequestCtx(), `
		SELECT storage_key, mime_type, original_filename FROM assets WHERE id = ?`, assetID)
	var storageKey, mimeType, filename string
	if err := row.Scan(&storageKey, &mimeType, &filename); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	rc, err := s.storage.Get(storageKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "file not found")
	}

	c.Set("Content-Type", mimeType)
	c.Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filename))
	return c.SendStream(rc)
}

// GET /s/:id/assets/:aid/thumb
// Streams the thumbnail. Always allowed (thumbnails are required for review).
func (s *Server) handleShareGetAssetThumb(c fiber.Ctx) error {
	sc := auth.GetShareClaims(c)
	shareID := c.Params("id")
	assetID := c.Params("aid")

	if err := s.reCheckShare(c, shareID); err != nil {
		return err
	}
	if err := s.assertAssetInShare(c, sc, assetID); err != nil {
		return err
	}

	row := s.sqlDB.QueryRowContext(c.RequestCtx(), `
		SELECT thumbnail_key FROM assets WHERE id = ?`, assetID)
	var thumbKey sql.NullString
	if err := row.Scan(&thumbKey); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	if !thumbKey.Valid {
		return errRes(c, fiber.StatusNotFound, "thumbnail not ready")
	}

	rc, err := s.storage.Get(thumbKey.String)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "thumbnail not found")
	}

	c.Set("Content-Type", "image/jpeg")
	return c.SendStream(rc)
}

// ── S-6 ──────────────────────────────────────────────────────────────────────

// POST /s/:id/comments
func (s *Server) handleShareCreateComment(c fiber.Ctx) error {
	sc := auth.GetShareClaims(c)
	shareID := c.Params("id")

	if !sc.AllowComments {
		return errRes(c, fiber.StatusForbidden, "comments not allowed for this share")
	}

	if err := s.reCheckShare(c, shareID); err != nil {
		return err
	}

	var body struct {
		AssetID     string  `json:"asset_id"`
		AuthorName  string  `json:"author_name"`
		AuthorEmail *string `json:"author_email"`
		Body        string  `json:"body"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(body.AuthorName) == "" {
		return errRes(c, fiber.StatusBadRequest, "author_name is required")
	}
	if strings.TrimSpace(body.Body) == "" {
		return errRes(c, fiber.StatusBadRequest, "body is required")
	}
	if body.AssetID == "" {
		return errRes(c, fiber.StatusBadRequest, "asset_id is required")
	}

	// Confirm asset belongs to the share
	if err := s.assertAssetInShare(c, sc, body.AssetID); err != nil {
		return err
	}

	var authorEmail sql.NullString
	if body.AuthorEmail != nil && *body.AuthorEmail != "" {
		authorEmail = sql.NullString{String: *body.AuthorEmail, Valid: true}
	}

	comment, err := s.db.CreateComment(c.RequestCtx(), dbgen.CreateCommentParams{
		ID:          uuid.NewString(),
		ShareID:     shareID,
		AssetID:     body.AssetID,
		AuthorName:  body.AuthorName,
		AuthorEmail: authorEmail,
		Body:        body.Body,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create comment")
	}

	return c.Status(fiber.StatusCreated).JSON(commentToResponse(comment))
}

// GET /s/:id/comments — all comments for this share, grouped by asset_id
func (s *Server) handleShareListComments(c fiber.Ctx) error {
	shareID := c.Params("id")

	if err := s.reCheckShare(c, shareID); err != nil {
		return err
	}

	comments, err := s.db.ListCommentsByShare(c.RequestCtx(), shareID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list comments")
	}

	// Group by asset_id
	grouped := make(map[string][]commentResponse)
	order := []string{}
	for _, cm := range comments {
		if _, exists := grouped[cm.AssetID]; !exists {
			order = append(order, cm.AssetID)
			grouped[cm.AssetID] = []commentResponse{}
		}
		grouped[cm.AssetID] = append(grouped[cm.AssetID], commentToResponse(cm))
	}

	type group struct {
		AssetID  string            `json:"asset_id"`
		Comments []commentResponse `json:"comments"`
	}
	result := make([]group, len(order))
	for i, aid := range order {
		result[i] = group{AssetID: aid, Comments: grouped[aid]}
	}

	return c.JSON(result)
}

// GET /s/:id/assets/:aid/comments — comments for a specific asset
func (s *Server) handleShareListAssetComments(c fiber.Ctx) error {
	sc := auth.GetShareClaims(c)
	shareID := c.Params("id")
	assetID := c.Params("aid")

	if err := s.reCheckShare(c, shareID); err != nil {
		return err
	}
	if err := s.assertAssetInShare(c, sc, assetID); err != nil {
		return err
	}

	comments, err := s.db.ListCommentsByAsset(c.RequestCtx(), dbgen.ListCommentsByAssetParams{
		ShareID: shareID,
		AssetID: assetID,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list comments")
	}

	items := make([]commentResponse, len(comments))
	for i, cm := range comments {
		items[i] = commentToResponse(cm)
	}
	return c.JSON(items)
}

// ── S-7 ──────────────────────────────────────────────────────────────────────

// GET /api/v1/shares/:id/comments — owner view of all comments on a share
func (s *Server) handleOwnerListComments(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	shareID := c.Params("id")

	// Verify share belongs to the workspace
	if _, err := s.db.GetShareByIDAndWorkspace(c.RequestCtx(), dbgen.GetShareByIDAndWorkspaceParams{
		ID:          shareID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "share not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load share")
	}

	comments, err := s.db.ListCommentsByShare(c.RequestCtx(), shareID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list comments")
	}

	items := make([]commentResponse, len(comments))
	for i, cm := range comments {
		items[i] = commentToResponse(cm)
	}
	return c.JSON(items)
}

// DELETE /api/v1/shares/:id/comments/:cid — moderation: delete a comment
func (s *Server) handleOwnerDeleteComment(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	shareID := c.Params("id")
	commentID := c.Params("cid")

	// Verify share belongs to the workspace
	if _, err := s.db.GetShareByIDAndWorkspace(c.RequestCtx(), dbgen.GetShareByIDAndWorkspaceParams{
		ID:          shareID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "share not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load share")
	}

	if err := s.db.DeleteComment(c.RequestCtx(), dbgen.DeleteCommentParams{
		ID:      commentID,
		ShareID: shareID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete comment")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// reCheckShare re-validates that the share is still active (not revoked, not expired).
// Uses fiber.NewError so callers receive a non-nil error on failure.
func (s *Server) reCheckShare(c fiber.Ctx, shareID string) error {
	share, err := s.db.GetShareByID(c.RequestCtx(), shareID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fiber.NewError(fiber.StatusNotFound, "share not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, "could not load share")
	}
	if share.RevokedAt.Valid {
		return fiber.NewError(fiber.StatusGone, "share has been revoked")
	}
	if isShareExpired(share) {
		return fiber.NewError(fiber.StatusGone, "share has expired")
	}
	return nil
}

// assertAssetInShare verifies that the given assetID is accessible via the share's target.
// Uses fiber.NewError so callers receive a non-nil error on failure.
func (s *Server) assertAssetInShare(c fiber.Ctx, sc *auth.ShareClaims, assetID string) error {
	switch sc.TargetType {
	case "asset":
		if assetID != sc.TargetID {
			return fiber.NewError(fiber.StatusNotFound, "asset not found in this share")
		}
	case "project":
		row := s.sqlDB.QueryRowContext(c.RequestCtx(),
			`SELECT COUNT(1) FROM assets WHERE id = ? AND project_id = ?`, assetID, sc.TargetID)
		var count int
		if err := row.Scan(&count); err != nil || count == 0 {
			return fiber.NewError(fiber.StatusNotFound, "asset not found in this share")
		}
	case "collection":
		// Collections not yet implemented — reject all
		return fiber.NewError(fiber.StatusNotFound, "asset not found in this share")
	}
	return nil
}
