package api

import (
	"archive/zip"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"strconv"
	"strings"
	"time"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ShareInfoResponse is returned by GET /shared/:id/access.
type ShareInfoResponse struct {
	Label       string `json:"label"`
	HasPassword bool   `json:"has_password"`
}

// ShareAccessResponse is returned by POST /shared/:id/access on success.
type ShareAccessResponse struct {
	Token string `json:"token"`
}

// CommentResponse is the JSON shape for a share comment.
type CommentResponse struct {
	ID          string  `json:"id"`
	ShareID     string  `json:"share_id"`
	AssetID     string  `json:"asset_id"`
	AuthorName  string  `json:"author_name"`
	AuthorEmail *string `json:"author_email"`
	Body        string  `json:"body"`
	CreatedAt   string  `json:"created_at"`
}

func commentToResponse(c dbgen.ShareComment) CommentResponse {
	return CommentResponse{
		ID:          c.ID,
		ShareID:     c.ShareID,
		AssetID:     c.AssetID,
		AuthorName:  c.AuthorName,
		AuthorEmail: c.AuthorEmail,
		Body:        c.Body,
		CreatedAt:   c.CreatedAt,
	}
}

// isShareExpired returns true if the share's expires_at is in the past.
func isShareExpired(share dbgen.Share) bool {
	if share.ExpiresAt == nil {
		return false
	}
	t, err := time.Parse("2006-01-02T15:04:05Z", *share.ExpiresAt)
	if err != nil {
		t, err = time.Parse("2006-01-02 15:04:05", *share.ExpiresAt)
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

	if share.RevokedAt != nil {
		return dbgen.Share{}, fiber.NewError(fiber.StatusNotFound, "share not found")
	}

	if isShareExpired(share) {
		return dbgen.Share{}, fiber.NewError(fiber.StatusGone, "share has expired")
	}

	return share, nil
}

// ── S-4 ──────────────────────────────────────────────────────────────────────

// handleShareInfo returns public metadata about a share.
//
// @Summary Get share info (public)
// @Description Returns the share's label and whether it requires a password. Use this as the first call when a visitor lands on a share link — if <code>has_password</code> is true, show a password gate before calling <code>POST /shared/:id/access</code>. Returns 404 if the share does not exist or has been revoked, and 410 if it has expired.
// @Tags Public Shares
// @Produce json
// @Param id path string true "Share ID"
// @Success 200 {object} ShareInfoResponse
// @Failure 404 {object} ErrorResponse "Share not found"
// @Failure 410 {object} ErrorResponse "Share has expired"
// @Router /shared/{id}/access [get]
// GET /shared/:id/access — unauthenticated.
// Returns share metadata so the gate page can decide whether to show a password form.
func (s *Server) handleShareInfo(c fiber.Ctx) error {
	id := c.Params("id")
	share, err := s.loadActiveShare(c, id)
	if err != nil {
		return err
	}
	return c.JSON(ShareInfoResponse{
		Label:       share.Label,
		HasPassword: share.PasswordHash != nil,
	})
}

// handleShareAccess authenticates access to a share and returns a session token.
//
// @Summary Authenticate share access (public)
// @Description Validates the share (checks password if one is set) and issues a 24-hour share session token. Include this token as <code>Authorization: Bearer &lt;token&gt;</code> in all subsequent requests to <code>/shared/:id/*</code> endpoints. Also increments the share's view count.
// @Tags Public Shares
// @Accept json
// @Produce json
// @Param id path string true "Share ID"
// @Param body body ShareAccessRequest false "Password (if required)"
// @Success 200 {object} ShareAccessResponse
// @Failure 401 {object} ErrorResponse "Password required or incorrect"
// @Failure 404 {object} ErrorResponse "Share not found"
// @Failure 410 {object} ErrorResponse "Share has expired"
// @Router /shared/{id}/access [post]
// POST /shared/:id/access — unauthenticated.
// Validates the share, checks password if required, issues a share session token.
func (s *Server) handleShareAccess(c fiber.Ctx) error {
	id := c.Params("id")

	share, err := s.loadActiveShare(c, id)
	if err != nil {
		return err
	}

	// Parse optional password (errors ignored — body is optional)
	body := &ShareAccessRequest{}
	err = c.Bind().Body(body)
	if err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}

	// Check password
	if share.PasswordHash != nil {
		if body.Password == "" {
			return errRes(c, fiber.StatusUnauthorized, "password required")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(*share.PasswordHash), []byte(body.Password)); err != nil {
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

	return c.JSON(ShareAccessResponse{Token: token})
}

// ── S-5 ──────────────────────────────────────────────────────────────────────

// handleShareListAssets returns the assets accessible via this share.
//
// @Summary List share assets (public)
// @Description Returns all assets accessible through the share, along with share metadata (<code>allow_comments</code>, <code>allow_download</code>, etc.). The asset set depends on the share's target type: a single asset for <code>asset</code> shares, all project assets for <code>project</code> shares. Requires a valid share session token.
// @Tags Public Shares
// @Produce json
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Success 200 {object} map[string]interface{} "share metadata and assets array"
// @Failure 401 {object} ErrorResponse "Not authenticated (share token required)"
// @Failure 404 {object} ErrorResponse "Share not found"
// @Failure 410 {object} ErrorResponse "Share has expired or been revoked"
// @Router /shared/{id}/assets [get]
// GET /shared/:id/assets
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
		colAssets, err := s.db.ListCollectionAssets(c.RequestCtx(), sc.TargetID)
		if err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not list collection assets")
		}
		assets = colAssets
	}

	items := make([]AssetResponse, len(assets))
	for i, a := range assets {
		items[i] = assetToResponse(a, []string{})
	}

	share, err := s.db.GetShareByID(c.RequestCtx(), shareID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load share")
	}

	type shareView struct {
		ID            string  `json:"id"`
		Label         string  `json:"label"`
		AllowComments bool    `json:"allow_comments"`
		AllowDownload bool    `json:"allow_download"`
		ExpiresAt     *string `json:"expires_at"`
		HasPassword   bool    `json:"has_password"`
	}
	sv := shareView{
		ID:            share.ID,
		Label:         share.Label,
		AllowComments: share.AllowComments == 1,
		AllowDownload: share.AllowDownload == 1,
		HasPassword:   share.PasswordHash != nil,
		ExpiresAt:     share.ExpiresAt,
	}

	return c.JSON(fiber.Map{
		"share":  sv,
		"assets": items,
	})
}

// handleShareGetAsset returns a single asset within a share.
//
// @Summary Get share asset (public)
// @Description Returns a single asset accessible via the share. Returns 404 if the asset is not part of the share's target. Requires a valid share session token.
// @Tags Public Shares
// @Produce json
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Param aid path string true "Asset ID"
// @Success 200 {object} AssetResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found in this share"
// @Router /shared/{id}/assets/{aid} [get]
// GET /shared/:id/assets/:aid
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

// handleShareGetAssetFile streams the original file for a shared asset.
//
// @Summary Download shared asset file (public)
// @Description Streams the original file for an asset accessible via the share. Returns 403 if the share was created with <code>allow_download: false</code>.
// @Tags Public Shares
// @Produce application/octet-stream
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Param aid path string true "Asset ID"
// @Success 200 {file} binary
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Download not allowed for this share"
// @Failure 404 {object} ErrorResponse "Asset or file not found"
// @Router /shared/{id}/assets/{aid}/file [get]
// GET /shared/:id/assets/:aid/file
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
		SELECT a.mime_type, a.original_filename, v.storage_key, v.content_hash, v.size, v.created_at
		FROM assets a
		JOIN asset_versions v ON v.asset_id = a.id AND v.is_current = 1 AND v.deleted_at IS NULL
		WHERE a.id = ?`, assetID)
	var mimeType, filename, storageKey, contentHash, versionCreatedAt string
	var versionSize int64
	if err := row.Scan(&mimeType, &filename, &storageKey, &contentHash, &versionSize, &versionCreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	lastMod := parseVersionTime(versionCreatedAt)
	if setCacheHeaders(c, contentHash, lastMod, false) {
		return nil
	}

	rc, err := s.storage.Get(storageKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "file not found")
	}

	c.Set("Content-Type", mimeType)
	c.Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filename))
	if versionSize > 0 {
		c.Set("Content-Length", strconv.FormatInt(versionSize, 10))
	}
	return c.SendStream(rc)
}

// handleShareGetAssetThumb serves the thumbnail for a shared asset.
//
// @Summary Get shared asset thumbnail (public)
// @Description Streams the JPEG thumbnail for an asset accessible via the share. Thumbnails are always accessible regardless of the <code>allow_download</code> setting, as they are needed for review purposes. Returns 404 if the thumbnail has not yet been generated.
// @Tags Public Shares
// @Produce image/jpeg
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Param aid path string true "Asset ID"
// @Success 200 {file} binary
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found or thumbnail not ready"
// @Router /shared/{id}/assets/{aid}/thumb [get]
// GET /shared/:id/assets/:aid/thumb
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
		SELECT thumbnail_key, updated_at FROM assets WHERE id = ?`, assetID)
	var thumbKey *string
	var updatedAt time.Time
	if err := row.Scan(&thumbKey, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	if thumbKey == nil {
		return errRes(c, fiber.StatusNotFound, "thumbnail not ready")
	}

	thumbETag := assetID + "_" + strconv.FormatInt(updatedAt.Unix(), 10)
	if setCacheHeaders(c, thumbETag, updatedAt, false) {
		return nil
	}

	rc, err := s.storage.Get(*thumbKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "thumbnail not found")
	}

	c.Set("Content-Type", "image/jpeg")
	return c.SendStream(rc)
}

// ── S-6 ──────────────────────────────────────────────────────────────────────

// handleShareCreateComment posts a comment on a shared asset.
//
// @Summary Post a comment (public)
// @Description Posts a comment on an asset within the share. The share must have <code>allow_comments: true</code>. Commenters identify themselves by name; an optional email can be provided. Returns 403 if comments are disabled for this share.
// @Tags Public Shares
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Param body body CreateCommentRequest true "Comment content and author info"
// @Success 201 {object} CommentResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Comments not allowed for this share"
// @Failure 404 {object} ErrorResponse "Asset not found in this share"
// @Router /shared/{id}/comments [post]
// POST /shared/:id/comments
func (s *Server) handleShareCreateComment(c fiber.Ctx) error {
	sc := auth.GetShareClaims(c)
	shareID := c.Params("id")

	if !sc.AllowComments {
		return errRes(c, fiber.StatusForbidden, "comments not allowed for this share")
	}

	if err := s.reCheckShare(c, shareID); err != nil {
		return err
	}

	body, ok := decodeAndValidate(c, &CreateCommentRequest{})
	if !ok {
		return nil
	}

	// Confirm asset belongs to the share
	if err := s.assertAssetInShare(c, sc, body.AssetID); err != nil {
		return err
	}

	var authorEmail *string
	if body.AuthorEmail != nil && *body.AuthorEmail != "" {
		authorEmail = body.AuthorEmail
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

	if share, err := s.db.GetShareByID(c.RequestCtx(), shareID); err == nil {
		if owner, err := s.db.GetUserByID(c.RequestCtx(), share.CreatedBy); err == nil {
			if err := s.mailer.SendCommentPosted(c.RequestCtx(), owner.Email, body.AuthorName, share.Label, body.Body); err != nil {
				slog.ErrorContext(c.RequestCtx(), "failed to send comment posted mail", "error", err)
			}
		}
	}

	return c.Status(fiber.StatusCreated).JSON(commentToResponse(comment))
}

// handleShareListComments returns all comments on a share, grouped by asset.
//
// @Summary List share comments (public)
// @Description Returns all comments on the share, grouped by <code>asset_id</code>. Useful for rendering a review dashboard showing all feedback at once.
// @Tags Public Shares
// @Produce json
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Success 200 {array} map[string]interface{} "Array of {asset_id, comments[]} groups"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Share not found"
// @Router /shared/{id}/comments [get]
// GET /shared/:id/comments — all comments for this share, grouped by asset_id
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
	grouped := make(map[string][]CommentResponse)
	order := []string{}
	for _, cm := range comments {
		if _, exists := grouped[cm.AssetID]; !exists {
			order = append(order, cm.AssetID)
			grouped[cm.AssetID] = []CommentResponse{}
		}
		grouped[cm.AssetID] = append(grouped[cm.AssetID], commentToResponse(cm))
	}

	type group struct {
		AssetID  string            `json:"asset_id"`
		Comments []CommentResponse `json:"comments"`
	}
	result := make([]group, len(order))
	for i, aid := range order {
		result[i] = group{AssetID: aid, Comments: grouped[aid]}
	}

	return c.JSON(result)
}

// handleShareListAssetComments returns comments for one asset within a share.
//
// @Summary List asset comments (public)
// @Description Returns all comments posted on a specific asset within the share.
// @Tags Public Shares
// @Produce json
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Param aid path string true "Asset ID"
// @Success 200 {array} CommentResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found in this share"
// @Router /shared/{id}/assets/{aid}/comments [get]
// GET /shared/:id/assets/:aid/comments — comments for a specific asset
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

	comments, err := s.db.ListCommentsByShareAndAsset(c.RequestCtx(), dbgen.ListCommentsByShareAndAssetParams{
		ShareID: shareID,
		AssetID: assetID,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list comments")
	}

	items := make([]CommentResponse, len(comments))
	for i, cm := range comments {
		items[i] = commentToResponse(cm)
	}
	return c.JSON(items)
}

// ── S-7 ──────────────────────────────────────────────────────────────────────

// handleOwnerListComments returns all comments on a share for the workspace owner.
//
// @Summary List comments on a share (owner)
// @Description Returns all comments posted through the share, visible to workspace members for moderation. The authenticated user must be a member of the workspace that owns the share.
// @Tags Shares
// @Produce json
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Success 200 {array} CommentResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Share not found"
// @Router /api/v1/shares/{id}/comments [get]
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

	items := make([]CommentResponse, len(comments))
	for i, cm := range comments {
		items[i] = commentToResponse(cm)
	}
	return c.JSON(items)
}

// handleOwnerDeleteComment deletes a public comment (moderation).
//
// @Summary Delete a comment (owner)
// @Description Permanently deletes a comment posted on the share. Only workspace members can use this endpoint. Use it to moderate inappropriate or spam comments.
// @Tags Shares
// @Produce json
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Param cid path string true "Comment ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Share not found"
// @Router /api/v1/shares/{id}/comments/{cid} [delete]
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

// ── WS-5: anonymous ZIP export for shared collections ────────────────────────

// @Summary Download shared assets as ZIP (public)
// @Description Streams a ZIP archive of all assets accessible via the share. Works for <code>asset</code>, <code>project</code>, and <code>collection</code> target types. Requires the share to have <code>allow_download: true</code> and a valid share session token. The ZIP is scoped strictly to the share's target — it cannot expose assets outside it.
// @Tags Public Shares
// @Produce application/zip
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Success 200 {file} binary "ZIP archive"
// @Failure 401 {object} ErrorResponse "Not authenticated (share token required)"
// @Failure 403 {object} ErrorResponse "Download not allowed for this share"
// @Failure 404 {object} ErrorResponse "Share not found or expired"
// @Router /shared/{id}/export [get]
// GET /shared/:id/export — requires a valid share session token and allow_download.
func (s *Server) handleShareExport(c fiber.Ctx) error {
	sc := auth.GetShareClaims(c)
	shareID := c.Params("id")

	if !sc.AllowDownload {
		return errRes(c, fiber.StatusForbidden, "download not allowed for this share")
	}

	if err := s.reCheckShare(c, shareID); err != nil {
		return err
	}

	// Collect asset storage keys scoped strictly to this share's target.
	type entry struct {
		name       string
		storageKey string
	}
	var entries []entry
	usedNames := map[string]int{}

	var query string
	var arg interface{}
	switch sc.TargetType {
	case "asset":
		query = `SELECT a.original_filename, v.storage_key
			FROM assets a
			JOIN asset_versions v ON v.asset_id = a.id AND v.is_current = 1 AND v.deleted_at IS NULL
			WHERE a.id = ?`
		arg = sc.TargetID
	case "project":
		query = `SELECT a.original_filename, v.storage_key
			FROM assets a
			JOIN asset_versions v ON v.asset_id = a.id AND v.is_current = 1 AND v.deleted_at IS NULL
			WHERE a.project_id = ?`
		arg = sc.TargetID
	case "collection":
		query = `SELECT a.original_filename, v.storage_key
			FROM assets a
			JOIN asset_versions v ON v.asset_id = a.id AND v.is_current = 1 AND v.deleted_at IS NULL
			JOIN collection_assets ca ON ca.asset_id = a.id
			WHERE ca.collection_id = ?
			ORDER BY ca.position ASC, ca.added_at ASC`
		arg = sc.TargetID
	default:
		return errRes(c, fiber.StatusBadRequest, "unsupported share target type")
	}

	rows, err := s.sqlDB.QueryContext(c.RequestCtx(), query, arg)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list assets")
	}
	defer rows.Close()
	for rows.Next() {
		var origName, storageKey string
		if err := rows.Scan(&origName, &storageKey); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "scan failed")
		}
		base := origName
		usedNames[base]++
		name := base
		if usedNames[base] > 1 {
			ext := ""
			stem := base
			if dot := strings.LastIndex(base, "."); dot >= 0 {
				stem = base[:dot]
				ext = base[dot:]
			}
			name = fmt.Sprintf("%s_%d%s", stem, usedNames[base], ext)
		}
		entries = append(entries, entry{name: name, storageKey: storageKey})
	}

	share, err := s.db.GetShareByID(c.RequestCtx(), shareID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load share")
	}
	filename := sanitiseFilename(share.Label)
	if filename == "" {
		filename = "collection-export"
	}

	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename + ".zip"}))

	pr, pw := io.Pipe()
	go func() {
		zw := zip.NewWriter(pw)
		var missing []string
		for _, e := range entries {
			rc, err := s.storage.Get(e.storageKey)
			if err != nil {
				missing = append(missing, e.name)
				continue
			}
			fw, err := zw.Create(e.name)
			if err != nil {
				_ = rc.Close()
				missing = append(missing, e.name)
				continue
			}
			if _, err := io.Copy(fw, rc); err != nil {
				slog.Warn("share zip copy error", "name", e.name, "err", err)
			}
			_ = rc.Close()
		}
		if len(missing) > 0 {
			if fw, err := zw.Create("_missing_files.txt"); err == nil {
				for _, n := range missing {
					_, _ = fmt.Fprintln(fw, n)
				}
			}
		}
		if err := zw.Close(); err != nil {
			_ = pw.CloseWithError(err)
		} else {
			_ = pw.Close()
		}
	}()

	return c.SendStream(pr)
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
	if share.RevokedAt != nil {
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
		row := s.sqlDB.QueryRowContext(c.RequestCtx(),
			`SELECT COUNT(1) FROM collection_assets WHERE collection_id = ? AND asset_id = ?`, sc.TargetID, assetID)
		var count int
		if err := row.Scan(&count); err != nil || count == 0 {
			return fiber.NewError(fiber.StatusNotFound, "asset not found in this share")
		}
	}
	return nil
}
