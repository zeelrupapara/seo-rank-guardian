package v1

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	apperrors "github.com/zeelrupapara/seo-rank-guardian/pkg/errors"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
)

// ListRunEvents godoc
// @Summary List run events
// @Description Get all persisted events for a run
// @Tags runs
// @Produce json
// @Security BearerAuth
// @Param jobId path int true "Job ID"
// @Param runId path int true "Run ID"
// @Success 200 {object} EventsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /jobs/{jobId}/runs/{runId}/events [get]
func (h *HttpServer) ListRunEvents(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	jobID, err := strconv.ParseUint(c.Params("jobId"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid job ID")
	}

	runID, err := strconv.ParseUint(c.Params("runId"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid run ID")
	}

	// Verify job ownership
	var job model.Job
	if err := h.DB.Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrJobNotFound.Error(), "Job not found")
	}

	var run model.JobRun
	if err := h.DB.Where("id = ? AND job_id = ?", runID, jobID).First(&run).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrRunNotFound.Error(), "Run not found")
	}

	page, pageSize := parsePagination(c)
	offset := (page - 1) * pageSize

	var total int64
	h.DB.Model(&model.RunEventLog{}).Where("run_id = ? AND job_id = ?", runID, jobID).Count(&total)

	var events []model.RunEventLog
	h.DB.Where("run_id = ? AND job_id = ?", runID, jobID).
		Order("created_at ASC").Offset(offset).Limit(pageSize).Find(&events)

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"items": events,
		"total": total,
		"page":  page,
		"limit": pageSize,
	}, "Events retrieved successfully")
}
