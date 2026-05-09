package api

import (
	"damask/server/internal/auth"
	"damask/server/internal/imagerouter"

	"github.com/gofiber/fiber/v3"
)

type ImageRouterModelsResponse struct {
	Models               []imagerouter.Model `json:"models"`
	Configured           bool                `json:"configured"`
	DefaultModel         string              `json:"default_model"`
	DefaultBgRemoveModel string              `json:"default_bg_remove_model"`
}

func (s *Server) handleListImageRouterModels(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	models, status, err := s.workspace.ListImageRouterModels(c.Context(), claims.WorkspaceID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(ImageRouterModelsResponse{
		Models:               models,
		Configured:           status.KeySet,
		DefaultModel:         s.cfg.ImageRouter.DefaultModel,
		DefaultBgRemoveModel: s.cfg.ImageRouter.DefaultBgRemoveModel,
	})
}
