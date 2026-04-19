package api

import (
	"encoding/json"

	"damask/server/internal/auth"
	"damask/server/internal/queue"

	"github.com/gofiber/fiber/v3"
)

type stackMergePayload struct {
	WorkspaceID    string   `json:"workspace_id"`
	CreatedBy      string   `json:"created_by"`
	AssetIDs       []string `json:"asset_ids"`
	OutputType     string   `json:"output_type"` // "gif" | "pdf"
	OutputFilename string   `json:"output_filename"`
	GifFrameMs     int      `json:"gif_frame_ms"`
}

// @Summary Merge stack assets into GIF or PDF
// @Description Enqueues an async job that merges the given assets into a single animated GIF or PDF contact sheet. All assets must belong to the authenticated workspace. Returns 202 immediately with a <code>job_id</code>; listen for event <code>stack_merge_done</code> to check completion. On success the job result contains the new asset ID.
// @Tags Stack
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body stackMergeRequest true "Asset IDs, output type and options"
// @Success 202 {object} map[string]string "job_id"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "One or more assets not in workspace"
// @Failure 422 {object} ValidationErrorResponse "Validation failed (e.g. output_type not gif/pdf)"
// @Router /api/v1/stack/merge [post]
// handleStackMerge enqueues a stack_merge job and returns 202 with the job ID.
func (s *Server) handleStackMerge(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &stackMergeRequest{})
	if !ok {
		return nil
	}

	// Verify all asset IDs belong to the workspace in one query.
	ok, err := s.allAssetsInWorkspace(c.RequestCtx(), claims.WorkspaceID, body.AssetIDs)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not verify assets")
	}
	if !ok {
		return errRes(c, fiber.StatusForbidden, "one or more assets not found in workspace")
	}

	payload := stackMergePayload{
		WorkspaceID:    claims.WorkspaceID,
		CreatedBy:      claims.UserID,
		AssetIDs:       body.AssetIDs,
		OutputType:     body.OutputType,
		OutputFilename: body.Filename,
		GifFrameMs:     body.GifFrameMs,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not encode job payload")
	}

	job, err := s.queue.Enqueue(c.RequestCtx(), claims.WorkspaceID, queue.JobTypeStackMerge, string(payloadJSON))
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not enqueue merge job")
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"job_id": job.ID})
}
