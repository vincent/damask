package api

import (
	"context"
	"errors"
	"strings"

	"damask/server/internal/auth"
	"damask/server/internal/imagerouter"

	"github.com/gofiber/fiber/v3"
)

type WorkspaceImageRouterStatusResponse struct {
	KeySet bool   `json:"key_set"`
	Source string `json:"source"`
}

type UpdateWorkspaceImageRouterKeyRequest struct {
	Key string `json:"key"`
}

func (r *UpdateWorkspaceImageRouterKeyRequest) Valid(_ context.Context) map[string]string {
	if strings.TrimSpace(r.Key) == "" {
		return map[string]string{"key": "required"}
	}
	return nil
}

// @Summary Get ImageRouter key status
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Success 200 {object} WorkspaceImageRouterStatusResponse
// @Router /api/v1/workspace/settings/imagerouter [get].
func (s *Server) handleGetWorkspaceImageRouterStatus(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	status, err := s.workspace.GetImageRouterKeyStatus(c.Context(), claims.WorkspaceID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(WorkspaceImageRouterStatusResponse{
		KeySet: status.KeySet,
		Source: string(status.Source),
	})
}

func (s *Server) handlePutWorkspaceImageRouterKey(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	body, ok := decodeAndValidate(c, &UpdateWorkspaceImageRouterKeyRequest{})
	if !ok {
		return nil
	}
	if err := s.workspace.SetImageRouterKey(c.Context(), claims.WorkspaceID, body.Key); err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) handleDeleteWorkspaceImageRouterKey(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if err := s.workspace.ClearImageRouterKey(c.Context(), claims.WorkspaceID); err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) handleTestWorkspaceImageRouterKey(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if err := s.workspace.TestImageRouterKey(c.Context(), claims.WorkspaceID); err != nil {
		if errors.Is(err, imagerouter.ErrInvalidKey) {
			return errRes(c, fiber.StatusUnprocessableEntity, err.Error())
		}
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
