package api

import (
	"context"
	"damask/server/internal/imagerouter"
	"time"

	"github.com/gofiber/fiber/v3"
)

type ImageRouterModelsResponse struct {
	Models               []imagerouter.Model `json:"models"`
	Configured           bool                `json:"configured"`
	DefaultModel         string              `json:"default_model"`
	DefaultBgRemoveModel string              `json:"default_bg_remove_model"`
}

func (s *Server) handleListImageRouterModels(c fiber.Ctx) error {
	models := append([]imagerouter.Model(nil), imagerouter.HardcodedModels...)
	if s.cfg.ImageRouter.IsConfigured() {
		ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
		defer cancel()
		fetched, err := imagerouter.FetchModels(ctx, s.cfg.ImageRouter.APIKey)
		if err == nil {
			models = fetched
		}
	}

	return c.JSON(ImageRouterModelsResponse{
		Models:               models,
		Configured:           s.cfg.ImageRouter.IsConfigured(),
		DefaultModel:         s.cfg.ImageRouter.DefaultModel,
		DefaultBgRemoveModel: s.cfg.ImageRouter.DefaultBgRemoveModel,
	})
}
