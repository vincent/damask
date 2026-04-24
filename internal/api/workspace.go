package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/services"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// WorkspaceResponse is the public representation of a workspace.
type WorkspaceResponse struct {
	ID                       string    `json:"id"`
	Name                     string    `json:"name"`
	VersionRetentionCount    int64     `json:"version_retention_count"`
	EventLogRetentionDays    int64     `json:"event_log_retention_days"`
	DownloadLogRetentionDays int64     `json:"download_log_retention_days"`
	IconAssetID              *string   `json:"icon_asset_id"`
	IconVersionID            *string   `json:"icon_version_id"`
	ExifKeep                 bool      `json:"exif_keep"`
	ExifKeepGps              bool      `json:"exif_keep_gps"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

func workspaceToResponse(w dbgen.Workspace) WorkspaceResponse {
	return WorkspaceResponse{
		ID:                       w.ID,
		Name:                     w.Name,
		VersionRetentionCount:    w.VersionRetentionCount,
		EventLogRetentionDays:    w.EventLogRetentionDays,
		DownloadLogRetentionDays: w.DownloadLogRetentionDays,
		IconAssetID:              w.IconAssetID,
		IconVersionID:            w.IconVersionID,
		ExifKeep:                 w.ExifKeep != 0,
		ExifKeepGps:              w.ExifKeepGps != 0,
		CreatedAt:                w.CreatedAt,
		UpdatedAt:                w.UpdatedAt,
	}
}

type WorkspaceMeResponse struct {
	Workspace       WorkspaceResponse `json:"workspace"`
	User            UserResponse      `json:"user"`
	Role            auth.Role         `json:"role"`
	TotalAssetCount int64             `json:"total_asset_count"`
}

// handleWorkspaceMe returns the current user, their active workspace, and their role.
//
// @Summary Get current workspace context
// @Description Returns the workspace embedded in the auth token, the authenticated user's profile, and their membership role in that workspace. Use this endpoint on app startup to hydrate the session.
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Success 200 {object} WorkspaceMeResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Workspace or user not found"
// @Router /api/v1/workspace/me [get]
func (s *Server) handleWorkspaceMe(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	workspace, err := s.db.GetWorkspaceByID(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "workspace not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load workspace")
	}

	user, err := s.db.GetUserByID(c.RequestCtx(), claims.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "user not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load user")
	}

	member, err := s.db.GetMember(c.RequestCtx(), dbgen.GetMemberParams{
		WorkspaceID: claims.WorkspaceID,
		UserID:      claims.UserID,
	})
	if err != nil {
		return errRes(c, fiber.StatusForbidden, "not a member of this workspace")
	}

	totalAssetCount, err := s.db.CountWorkspaceAssets(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		slog.ErrorContext(c.RequestCtx(), "could not count workspace assets", "error", err)
	}

	return c.JSON(WorkspaceMeResponse{
		Workspace:       workspaceToResponse(workspace),
		User:            userToResponse(user),
		Role:            auth.Role(member.Role),
		TotalAssetCount: totalAssetCount,
	})
}

// handleCreateWorkspace creates a new workspace for the authenticated user.
//
// @Summary Create a workspace
// @Description Creates a new workspace and makes the authenticated user its owner. The user may belong to multiple workspaces; use <code>POST /api/v1/workspace/switch</code> to activate a different workspace in subsequent requests.
// @Tags Workspace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateWorkspaceRequest true "Workspace details"
// @Success 201 {object} AuthResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/workspace [post]
func (s *Server) handleCreateWorkspace(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	user, err := s.db.GetUserByID(c.RequestCtx(), claims.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "user not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load user")
	}

	req, ok := decodeAndValidate(c, &CreateWorkspaceRequest{})
	if !ok {
		return nil
	}

	tx, err := s.sqlDB.BeginTx(c.RequestCtx(), nil)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not begin transaction")
	}
	defer tx.Rollback()

	qtx := s.db.WithTx(tx)

	workspace, err := services.CreateWorkspaceForUser(c.RequestCtx(), qtx, req.Name, user.ID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create workspace")
	}

	if err := tx.Commit(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not commit transaction")
	}

	wr := workspaceToResponse(*workspace)
	return c.Status(fiber.StatusCreated).JSON(AuthResponse{Workspace: &wr})
}

type WorkspaceWithRoleResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Role        auth.Role `json:"role"`
	MemberCount int64     `json:"member_count"`
	AssetCount  int64     `json:"asset_count"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
}

// handleListWorkspaces lists all workspaces the authenticated user belongs to.
//
// @Summary List workspaces
// @Description Returns every workspace the authenticated user is a member of, along with their role in each. Use this to populate a workspace-switcher UI.
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Success 200 {array} WorkspaceWithRoleResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/workspaces [get]
func (s *Server) handleListWorkspaces(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	rows, err := s.db.ListWorkspacesByUserID(c.RequestCtx(), claims.UserID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list workspaces")
	}

	result := make([]WorkspaceWithRoleResponse, len(rows))
	for i, r := range rows {
		memberCount, err := s.db.CountWorkspaceMembers(c.RequestCtx(), r.ID)
		if err != nil {
			slog.ErrorContext(c.RequestCtx(), "could not count members", "workspace_id", r.ID, "error", err)
		}
		assetCount, err := s.db.CountWorkspaceAssets(c.RequestCtx(), r.ID)
		if err != nil {
			slog.ErrorContext(c.RequestCtx(), "could not count assets", "workspace_id", r.ID, "error", err)
		}
		result[i] = WorkspaceWithRoleResponse{
			ID:          r.ID,
			Name:        r.Name,
			Role:        auth.Role(r.Role),
			MemberCount: memberCount,
			AssetCount:  assetCount,
			CreatedAt:   r.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   r.UpdatedAt.Format(time.RFC3339),
		}
	}
	return c.JSON(result)
}

type SwitchWorkspaceResponse struct {
	Token     string            `json:"token"`
	Workspace WorkspaceResponse `json:"workspace"`
	Role      auth.Role         `json:"role"`
}

// handleSwitchWorkspace issues a new JWT scoped to a different workspace.
//
// @Summary Switch active workspace
// @Description Issues a fresh JWT scoped to the requested workspace and updates the <code>auth_token</code> cookie accordingly. All subsequent authenticated requests will operate in the new workspace context. The user must be a member of the target workspace.
// @Tags Workspace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body SwitchWorkspaceRequest true "Target workspace ID"
// @Success 200 {object} SwitchWorkspaceResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Not a member of this workspace"
// @Router /api/v1/workspace/switch [post]
func (s *Server) handleSwitchWorkspace(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	req, ok := decodeAndValidate(c, &SwitchWorkspaceRequest{})
	if !ok {
		return nil
	}

	member, err := s.db.GetMember(c.RequestCtx(), dbgen.GetMemberParams{
		WorkspaceID: req.WorkspaceID,
		UserID:      claims.UserID,
	})
	if err != nil {
		return errRes(c, fiber.StatusForbidden, "not a member of this workspace")
	}

	workspace, err := s.db.GetWorkspaceByID(c.RequestCtx(), req.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusForbidden, "not a member of this workspace")
	}

	token, err := s.tokenMaker.CreateToken(claims.UserID, req.WorkspaceID, 7*24*time.Hour)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create token")
	}

	s.setAuthCookie(c, token)
	return c.JSON(SwitchWorkspaceResponse{
		Token:     token,
		Workspace: workspaceToResponse(workspace),
		Role:      auth.Role(member.Role),
	})
}

// handleUpdateWorkspaceSettings updates workspace-level configuration.
//
// @Summary Update workspace settings
// @Description Updates the workspace's version retention policy and EXIF metadata settings. <ul> <li><strong>version_retention_count</strong>: Maximum number of asset versions to keep per asset. 0 = keep all.</li> <li><strong>exif_keep</strong>: Whether EXIF metadata is preserved on uploaded images.</li> <li><strong>exif_keep_gps</strong>: Whether GPS coordinates are preserved within EXIF data (requires exif_keep=true).</li> </ul>
// @Tags Workspace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body UpdateWorkspaceSettingsRequest true "Settings to update"
// @Success 200 {object} WorkspaceResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/workspace/settings [put]
func (s *Server) handleUpdateWorkspaceSettings(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &UpdateWorkspaceSettingsRequest{})
	if !ok {
		return nil
	}

	tx, err := s.sqlDB.BeginTx(c.RequestCtx(), nil)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not begin transaction")
	}
	defer tx.Rollback()
	qtx := s.db.WithTx(tx)

	if err := qtx.UpdateWorkspaceVersionRetention(c.RequestCtx(), dbgen.UpdateWorkspaceVersionRetentionParams{
		VersionRetentionCount: body.VersionRetentionCount,
		ID:                    claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not update settings")
	}

	exifKeep := int64(0)
	if body.ExifKeep {
		exifKeep = 1
	}
	exifKeepGPS := int64(0)
	if body.ExifKeepGPS {
		exifKeepGPS = 1
	}
	if err := qtx.UpdateWorkspaceExifSettings(c.RequestCtx(), dbgen.UpdateWorkspaceExifSettingsParams{
		ExifKeep:    exifKeep,
		ExifKeepGps: exifKeepGPS,
		ID:          claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not update exif settings")
	}

	if err := tx.Commit(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not commit settings update")
	}

	workspace, err := s.db.GetWorkspaceByID(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not reload workspace")
	}
	return c.JSON(workspaceToResponse(workspace))
}

// triggerableJobs is the allowlist of job types that can be triggered via the
// generic POST /workspace/jobs/:type/trigger endpoint.
var triggerableJobs = map[string]string{
	"extract_exif": queue.JobTypeExtractExif,
}

// handleTriggerWorkspaceJob enqueues a workspace-wide background job.
//
// @Summary Trigger a workspace-wide background job
// @Description Enqueues a background job for the entire workspace. Currently supported job types: <ul> <li><strong>extract_exif</strong> — Backfills EXIF metadata (make, model, GPS, taken-at, etc.) for all image assets that do not yet have the <code>_exif_make</code> custom field populated.</li> </ul> Returns the number of assets enqueued for processing.
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Param type path string true "Job type (e.g. extract_exif)"
// @Success 202 {object} map[string]int "enqueued count"
// @Failure 400 {object} ErrorResponse "Unknown job type"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/workspace/jobs/{type}/trigger [post]
func (s *Server) handleTriggerWorkspaceJob(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	jobTypeKey := c.Params("type")

	jobType, ok := triggerableJobs[jobTypeKey]
	if !ok {
		return errRes(c, fiber.StatusBadRequest, "unknown job type")
	}

	switch jobType {
	case queue.JobTypeExtractExif:
		return s.triggerExtractExifBackfill(c, claims.WorkspaceID, claims.UserID)
	default:
		return errRes(c, fiber.StatusBadRequest, "unknown job type")
	}
}

func (s *Server) triggerExtractExifBackfill(c fiber.Ctx, workspaceID, userID string) error {
	// Find the _exif_make field definition to use as tombstone marker.
	// If it doesn't exist yet, every image asset is pending.
	var pendingIDs []string

	tombstoneDef, err := s.db.GetFieldDefinitionByKey(c.RequestCtx(), dbgen.GetFieldDefinitionByKeyParams{
		WorkspaceID: workspaceID,
		Key:         "_exif_make",
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return errRes(c, fiber.StatusInternalServerError, "could not query field definitions")
	}

	if errors.Is(err, sql.ErrNoRows) {
		// No field definitions yet — every image asset is pending.
		ids, qErr := s.db.ListImageAssetIDs(c.RequestCtx(), workspaceID)
		if qErr != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not list assets")
		}
		pendingIDs = ids
	} else {
		ids, qErr := s.db.ListAssetsMissingExifField(c.RequestCtx(), dbgen.ListAssetsMissingExifFieldParams{
			FieldID:     tombstoneDef.ID,
			WorkspaceID: workspaceID,
			Limit:       10000,
		})
		if qErr != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not list pending assets")
		}
		pendingIDs = ids
	}

	if len(pendingIDs) == 0 {
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"enqueued": 0})
	}

	for _, assetID := range pendingIDs {
		payload, _ := json.Marshal(map[string]string{
			"asset_id":     assetID,
			"workspace_id": workspaceID,
			"user_id":      userID,
		})
		if _, err := s.queue.Enqueue(c.RequestCtx(), workspaceID, queue.JobTypeExtractExif, string(payload)); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not enqueue jobs")
		}
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"enqueued": len(pendingIDs)})
}

// fiber:context-methods migrated

type MemberResponse struct {
	UserID   string    `json:"user_id"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

// handleListMembers lists all members of the active workspace.
//
// @Summary List workspace members
// @Description Returns all members of the workspace embedded in the auth token, including their names, emails, roles, and join dates.
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Success 200 {array} MemberResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/workspace/members [get]
func (s *Server) handleListMembers(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	rows, err := s.db.ListMembers(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list members")
	}
	result := make([]MemberResponse, len(rows))
	for i, row := range rows {
		result[i] = MemberResponse{
			UserID:   row.UserID,
			Name:     row.Name,
			Email:    row.Email,
			Role:     row.Role,
			JoinedAt: row.CreatedAt,
		}
	}
	return c.JSON(result)
}

// handleRemoveMember removes a member from the workspace.
//
// @Summary Remove a workspace member
// @Description Removes a user from the active workspace. Safeguards: <ul> <li>A user cannot remove themselves.</li> <li>The last owner of a workspace cannot be removed.</li> </ul>
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Param userId path string true "User ID to remove"
// @Success 204
// @Failure 400 {object} ErrorResponse "Cannot remove yourself or the last owner"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/workspace/members/{userId} [delete]
func (s *Server) handleRemoveMember(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	targetUserID := c.Params("userId")

	if targetUserID == claims.UserID {
		return errRes(c, fiber.StatusBadRequest, "cannot remove yourself")
	}

	// Prevent removing the last owner.
	members, err := s.db.ListMembers(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list members")
	}
	ownerCount := 0
	for _, m := range members {
		if m.Role == string(auth.Owner) {
			ownerCount++
		}
	}
	for _, m := range members {
		if m.UserID == targetUserID && m.Role == string(auth.Owner) && ownerCount <= 1 {
			return errRes(c, fiber.StatusBadRequest, "cannot remove the last owner")
		}
	}

	if err := s.db.DeleteMember(c.RequestCtx(), dbgen.DeleteMemberParams{
		WorkspaceID: claims.WorkspaceID,
		UserID:      targetUserID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not remove member")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// handleUpdateMemberRole changes a workspace member's role.
//
// @Summary Update a member's role
// @Description Changes the role (<code>owner</code>, <code>editor</code>, or <code>viewer</code>) of a workspace member. Safeguard: the last owner cannot demote themselves.
// @Tags Workspace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param userId path string true "User ID whose role to change"
// @Param body body UpdateMemberRoleRequest true "New role"
// @Success 204
// @Failure 400 {object} ErrorResponse "Cannot demote the last owner"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/workspace/members/{userId} [put]
func (s *Server) handleUpdateMemberRole(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	targetUserID := c.Params("userId")

	body, ok := decodeAndValidate(c, &UpdateMemberRoleRequest{})
	if !ok {
		return nil
	}

	// Prevent demoting self if last owner.
	if targetUserID == claims.UserID && body.Role != auth.Owner {
		members, err := s.db.ListMembers(c.RequestCtx(), claims.WorkspaceID)
		if err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not list members")
		}
		ownerCount := 0
		for _, m := range members {
			if m.Role == string(auth.Owner) {
				ownerCount++
			}
		}
		if ownerCount <= 1 {
			return errRes(c, fiber.StatusBadRequest, "cannot demote the last owner")
		}
	}

	if err := s.db.UpdateMemberRole(c.RequestCtx(), dbgen.UpdateMemberRoleParams{
		Role:        string(body.Role),
		WorkspaceID: claims.WorkspaceID,
		UserID:      targetUserID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not update member role")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// handleListInvites lists all pending (unredeemed) invites for the workspace.
//
// @Summary List pending invites
// @Description Returns all workspace invites that have not yet been accepted and have not expired. The <code>invite_token</code> field is omitted from list responses for security — it is only returned once, when the invite is created.
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Success 200 {array} InviteResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/workspace/invites [get]
func (s *Server) handleListInvites(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	invites, err := s.db.ListPendingInvites(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list invites")
	}
	result := make([]InviteResponse, len(invites))
	for i, inv := range invites {
		result[i] = InviteResponse{
			ID:        inv.ID,
			Email:     inv.Email,
			Role:      inv.Role,
			ExpiresAt: inv.ExpiresAt,
			CreatedAt: inv.CreatedAt,
		}
	}
	return c.JSON(result)
}

// handleDeleteInvite cancels a pending workspace invite.
//
// @Summary Delete an invite
// @Description Cancels a pending invite, preventing the recipient from accepting it. Has no effect on already-accepted invites (they are already marked as used).
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Param inviteId path string true "Invite ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/workspace/invites/{inviteId} [delete]
func (s *Server) handleDeleteInvite(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	inviteID := c.Params("inviteId")

	if err := s.db.DeleteInvite(c.RequestCtx(), dbgen.DeleteInviteParams{
		ID:          inviteID,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete invite")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type InviteResponse struct {
	ID          string    `json:"id"`
	InviteToken string    `json:"invite_token,omitempty"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
}

// handleCreateInvite generates a workspace invite for a new member.
//
// @Summary Create a workspace invite
// @Description Generates a time-limited invite token (7-day expiry) for the given email address. The <code>invite_token</code> in the response should be sent to the recipient; they redeem it via <code>POST /auth/invite/accept</code>. Only workspace owners may create invites.
// @Tags Workspace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateInviteRequest true "Invite details (email and role)"
// @Success 201 {object} InviteResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Insufficient role — owner required"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/workspace/invites [post]
func (s *Server) handleCreateInvite(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	req, ok := decodeAndValidate(c, &CreateInviteRequest{})
	if !ok {
		return nil
	}

	invite, err := s.db.CreateInvite(c.RequestCtx(), dbgen.CreateInviteParams{
		ID:          uuid.New().String(),
		WorkspaceID: claims.WorkspaceID,
		Email:       req.Email,
		Token:       uuid.New().String(),
		Role:        string(req.Role),
		InvitedBy:   claims.UserID,
		ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create invite")
	}

	if err = s.mailer.SendInvite(c.RequestCtx(), invite.Email, string(req.Role), invite.Token); err != nil {
		slog.ErrorContext(c.RequestCtx(), "failed to send invitation mail", "error", err)
	}

	return c.Status(fiber.StatusCreated).JSON(InviteResponse{
		ID:          invite.ID,
		InviteToken: invite.Token,
		Email:       invite.Email,
		Role:        invite.Role,
	})
}

// handleAcceptInvite redeems a workspace invite and creates the new user account.
//
// @Summary Accept a workspace invite
// @Description Redeems a one-time invite token: creates a new user account (name + password required) and adds them to the workspace at the role encoded in the invite. The invite token is marked as accepted and cannot be reused. Returns a JWT just like <code>POST /auth/register</code>.
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body AcceptInviteRequest true "Invite token, name, and password"
// @Success 201 {object} AuthResponse
// @Failure 404 {object} ErrorResponse "Invalid or expired invite token"
// @Failure 409 {object} ErrorResponse "Email already in use"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /auth/invite/accept [post]
func (s *Server) handleAcceptInvite(c fiber.Ctx) error {
	req, ok := decodeAndValidate(c, &AcceptInviteRequest{})
	if !ok {
		return nil
	}

	invite, err := s.db.GetInviteByToken(c.RequestCtx(), req.Token)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "invalid or expired invite token")
	}

	hash, err := bcryptHash(req.Password)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not hash password")
	}

	tx, err := s.sqlDB.BeginTx(c.RequestCtx(), nil)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not begin transaction")
	}
	defer tx.Rollback()

	qtx := s.db.WithTx(tx)

	userID := uuid.New().String()
	user, err := qtx.CreateUser(c.RequestCtx(), dbgen.CreateUserParams{
		ID:           userID,
		Email:        invite.Email,
		PasswordHash: hash,
		Name:         req.Name,
	})
	if err != nil {
		return errRes(c, fiber.StatusConflict, "email already in use")
	}

	if err := qtx.CreateMember(c.RequestCtx(), dbgen.CreateMemberParams{
		WorkspaceID: invite.WorkspaceID,
		UserID:      userID,
		Role:        invite.Role,
		InvitedBy:   &invite.InvitedBy,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not add workspace member")
	}

	if err := s.mailer.SendWelcome(c.RequestCtx(), invite.Email, req.Name, invite.WorkspaceID); err != nil {
		slog.ErrorContext(c.RequestCtx(), "failed to send welcome mail", "error", err)
	}

	if err := qtx.AcceptInvite(c.RequestCtx(), invite.ID); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not mark invite as accepted")
	}

	if err := tx.Commit(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not commit transaction")
	}

	if inviter, err := s.db.GetUserByID(c.RequestCtx(), invite.InvitedBy); err == nil {
		if err := s.mailer.SendInviteAccepted(c.RequestCtx(), inviter.Email, req.Name, invite.Email, invite.Role); err != nil {
			slog.ErrorContext(c.RequestCtx(), "failed to send invite accepted mail", "error", err)
		}
	}

	token, err := s.tokenMaker.CreateToken(userID, invite.WorkspaceID, 7*24*time.Hour)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create token")
	}

	s.setAuthCookie(c, token)
	return c.Status(fiber.StatusCreated).JSON(AuthResponse{Token: token, User: userToResponse(user)})
}
