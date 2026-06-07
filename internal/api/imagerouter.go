package api

import (
	"damask/server/internal/auth"

	"github.com/gofiber/fiber/v3"
)

// ImageRouterModel describes a single image router model available for variants.
type ImageRouterModel struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Provider      string  `json:"provider"`
	PricePerImage float64 `json:"price_per_image"`
}

// ImageRouterModelsResponse is returned by the list image router models endpoint.
type ImageRouterModelsResponse struct {
	Models               []ImageRouterModel `json:"models"`
	Configured           bool               `json:"configured"`
	DefaultModel         string             `json:"default_model"`
	DefaultBgRemoveModel string             `json:"default_bg_remove_model"`
}

// handleListImageRouterModels handles GET /api/v1/imagerouter/models
//
// @Summary List available image router models
// @Tags ImageRouter
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ImageRouterModelsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/imagerouter/models [get].
func (s *Server) handleListImageRouterModels(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	models, status, err := s.workspace.ListImageRouterModels(c.Context(), claims.WorkspaceID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	out := make([]ImageRouterModel, len(models))
	for i, m := range models {
		out[i] = ImageRouterModel{
			ID:            m.ID,
			Name:          m.Name,
			Provider:      m.Provider,
			PricePerImage: m.PricePerImage,
		}
	}
	return c.JSON(ImageRouterModelsResponse{
		Models:               out,
		Configured:           status.KeySet,
		DefaultModel:         s.cfg.ImageRouter.DefaultModel,
		DefaultBgRemoveModel: s.cfg.ImageRouter.DefaultBgRemoveModel,
	})
}
