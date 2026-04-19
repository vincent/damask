package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
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

type JobStatusResponse struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	Error       *string    `json:"error,omitempty"`
	Result      *string    `json:"result,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func jobToStatusResponse(j dbgen.Job) JobStatusResponse {
	return JobStatusResponse{
		ID:        j.ID,
		Type:      j.Type,
		Status:    j.Status,
		Error:     j.Error,
		Result:    j.Result,
		CreatedAt: j.CreatedAt,
		UpdatedAt: j.UpdatedAt,
	}
}

// @Summary Merge stack assets into GIF or PDF
// @Description Enqueues an async job that merges the given assets into a single animated GIF or PDF contact sheet. All assets must belong to the authenticated workspace. Returns 202 immediately with a <code>job_id</code>; poll <code>GET /api/v1/jobs/:id</code> to check completion. On success the job result contains the new asset ID.
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

// @Summary Get job status
// @Description Returns the current status of an async job (pending, processing, done, failed). When <code>status</code> is <code>done</code> the <code>result</code> field contains the output asset ID for <code>stack_merge</code> jobs.
// @Tags Stack
// @Produce json
// @Security BearerAuth
// @Param id path string true "Job ID"
// @Success 200 {object} JobStatusResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Job not found"
// @Router /api/v1/jobs/{id} [get]
// handleGetJob returns the status of a job by ID.
func (s *Server) handleGetJob(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	job, err := s.db.GetJobByID(c.RequestCtx(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "job not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not get job")
	}

	// Ensure job belongs to the caller's workspace.
	if job.WorkspaceID != claims.WorkspaceID {
		return errRes(c, fiber.StatusNotFound, "job not found")
	}

	return c.JSON(jobToStatusResponse(job))
}
