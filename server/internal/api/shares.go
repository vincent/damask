package api

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// shareResponse is the JSON shape for a share object returned to clients.
type shareResponse struct {
	ID            string  `json:"id"`
	WorkspaceID   string  `json:"workspace_id"`
	CreatedBy     string  `json:"created_by"`
	Label         string  `json:"label"`
	TargetType    string  `json:"target_type"`
	TargetID      string  `json:"target_id"`
	HasPassword   bool    `json:"has_password"`
	ExpiresAt     *string `json:"expires_at"`
	AllowComments bool    `json:"allow_comments"`
	AllowDownload bool    `json:"allow_download"`
	ViewCount     int64   `json:"view_count"`
	CreatedAt     string  `json:"created_at"`
	RevokedAt     *string `json:"revoked_at"`
	IsExpired     bool    `json:"is_expired"`
	PublicURL     string  `json:"public_url"`
}

func shareToResponse(s dbgen.Share, baseURL string) shareResponse {
	isExpired := false
	if s.ExpiresAt != nil {
		t, err := time.Parse("2006-01-02T15:04:05Z", *s.ExpiresAt)
		if err != nil {
			// try alternate format stored by SQLite datetime()
			t, err = time.Parse("2006-01-02 15:04:05", *s.ExpiresAt)
		}
		if err == nil && time.Now().After(t) {
			isExpired = true
		}
	}

	return shareResponse{
		ID:            s.ID,
		WorkspaceID:   s.WorkspaceID,
		CreatedBy:     s.CreatedBy,
		Label:         s.Label,
		TargetType:    s.TargetType,
		TargetID:      s.TargetID,
		HasPassword:   s.PasswordHash != nil,
		ExpiresAt:     s.ExpiresAt,
		AllowComments: s.AllowComments == 1,
		AllowDownload: s.AllowDownload == 1,
		ViewCount:     s.ViewCount,
		CreatedAt:     s.CreatedAt,
		RevokedAt:     s.RevokedAt,
		IsExpired:     isExpired,
		PublicURL:     baseURL + "/s/" + s.ID,
	}
}

// POST /api/v1/shares
func (s *Server) handleCreateShare(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &createShareRequest{})
	if !ok {
		return nil
	}

	// Validate target belongs to workspace
	if err := s.validateShareTarget(claims.WorkspaceID, body.TargetType, body.TargetID); err != nil {
		var te *shareTargetErr
		if errors.As(err, &te) {
			return errRes(c, te.status, te.msg)
		}
		return errRes(c, fiber.StatusInternalServerError, "could not validate target")
	}

	// Hash password if provided
	var passwordHash *string
	if body.Password != nil && *body.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*body.Password), bcryptCost)
		if err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not hash password")
		}
		h := string(hash)
		passwordHash = &h
	}

	// Compute expires_at
	var expiresAt *string
	if body.ExpiresInDays != nil && *body.ExpiresInDays > 0 {
		t := time.Now().UTC().Add(time.Duration(*body.ExpiresInDays) * 24 * time.Hour)
		s := t.Format("2006-01-02 15:04:05")
		expiresAt = &s
	}

	allowDownload := true
	if body.AllowDownload != nil {
		allowDownload = *body.AllowDownload
	}

	allowComments := int64(0)
	if body.AllowComments {
		allowComments = 1
	}
	allowDownloadInt := int64(0)
	if allowDownload {
		allowDownloadInt = 1
	}

	share, err := s.db.CreateShare(c.RequestCtx(), dbgen.CreateShareParams{
		ID:            uuid.NewString(),
		WorkspaceID:   claims.WorkspaceID,
		CreatedBy:     claims.UserID,
		Label:         body.Label,
		TargetType:    body.TargetType,
		TargetID:      body.TargetID,
		PasswordHash:  passwordHash,
		ExpiresAt:     expiresAt,
		AllowComments: allowComments,
		AllowDownload: allowDownloadInt,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create share")
	}

	return c.Status(fiber.StatusCreated).JSON(shareToResponse(share, s.baseUrl))
}

// shareTargetErr is a sentinel for target validation failures.
type shareTargetErr struct {
	status int
	msg    string
}

func (e *shareTargetErr) Error() string { return e.msg }

// validateShareTarget checks that the given target_id exists in the workspace.
// Returns a *shareTargetErr on validation failure, or a plain error on DB error.
func (s *Server) validateShareTarget(workspaceID, targetType, targetID string) error {
	bgCtx := context.Background()
	var notFound bool

	switch targetType {
	case "project":
		_, err := s.db.GetProjectByID(bgCtx, dbgen.GetProjectByIDParams{
			ID:          targetID,
			WorkspaceID: workspaceID,
		})
		notFound = errors.Is(err, sql.ErrNoRows)
		if err != nil && !notFound {
			return err
		}
	case "asset":
		_, err := s.db.GetAssetByID(bgCtx, dbgen.GetAssetByIDParams{
			ID:          targetID,
			WorkspaceID: workspaceID,
		})
		notFound = errors.Is(err, sql.ErrNoRows)
		if err != nil && !notFound {
			return err
		}
	case "collection":
		// Collections are not yet implemented; accept any non-empty id.
		return nil
	}

	if notFound {
		return &shareTargetErr{status: fiber.StatusNotFound, msg: "target not found"}
	}
	return nil
}

// GET /api/v1/shares
func (s *Server) handleListShares(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	shares, err := s.db.ListSharesByWorkspace(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list shares")
	}

	items := make([]shareResponse, len(shares))
	for i, sh := range shares {
		items[i] = shareToResponse(sh, s.baseUrl)
	}
	return c.JSON(items)
}

// GET /api/v1/shares/:id
func (s *Server) handleGetShare(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	share, err := s.db.GetShareByIDAndWorkspace(c.RequestCtx(), dbgen.GetShareByIDAndWorkspaceParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "share not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load share")
	}

	return c.JSON(shareToResponse(share, s.baseUrl))
}

// PUT /api/v1/shares/:id
func (s *Server) handleUpdateShare(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	// Verify exists and belongs to workspace
	existing, err := s.db.GetShareByIDAndWorkspace(c.RequestCtx(), dbgen.GetShareByIDAndWorkspaceParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "share not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load share")
	}

	body, ok := decodeAndValidate(c, &updateShareRequest{})
	if !ok {
		return nil
	}

	// Resolve label
	label := existing.Label
	if body.Label != nil {
		label = *body.Label
	}

	// Resolve password_hash
	passwordHash := existing.PasswordHash
	if body.ClearPassword != nil && *body.ClearPassword {
		passwordHash = nil
	} else if body.Password != nil {
		if *body.Password == "" {
			passwordHash = nil
		} else {
			hash, err := bcrypt.GenerateFromPassword([]byte(*body.Password), bcryptCost)
			if err != nil {
				return errRes(c, fiber.StatusInternalServerError, "could not hash password")
			}
			h := string(hash)
			passwordHash = &h
		}
	}

	// Resolve expires_at
	expiresAt := existing.ExpiresAt
	if body.ClearExpiry != nil && *body.ClearExpiry {
		expiresAt = nil
	} else if body.ExpiresAt != nil {
		expiresAt = body.ExpiresAt
	}

	// Resolve allow_comments / allow_download
	allowComments := existing.AllowComments
	if body.AllowComments != nil {
		if *body.AllowComments {
			allowComments = 1
		} else {
			allowComments = 0
		}
	}
	allowDownload := existing.AllowDownload
	if body.AllowDownload != nil {
		if *body.AllowDownload {
			allowDownload = 1
		} else {
			allowDownload = 0
		}
	}

	updated, err := s.db.UpdateShare(c.RequestCtx(), dbgen.UpdateShareParams{
		Label:         label,
		PasswordHash:  passwordHash,
		ExpiresAt:     expiresAt,
		AllowComments: allowComments,
		AllowDownload: allowDownload,
		ID:            id,
		WorkspaceID:   claims.WorkspaceID,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not update share")
	}

	return c.JSON(shareToResponse(updated, s.baseUrl))
}

// DELETE /api/v1/shares/:id  — soft delete via revoked_at
func (s *Server) handleRevokeShare(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	// Verify exists
	if _, err := s.db.GetShareByIDAndWorkspace(c.RequestCtx(), dbgen.GetShareByIDAndWorkspaceParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "share not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load share")
	}

	if err := s.db.RevokeShare(c.RequestCtx(), dbgen.RevokeShareParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not revoke share")
	}

	return c.SendStatus(fiber.StatusNoContent)
}
