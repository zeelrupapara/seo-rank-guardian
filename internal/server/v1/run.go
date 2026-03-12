package v1

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	apperrors "github.com/zeelrupapara/seo-rank-guardian/pkg/errors"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
)

// ListRuns godoc
// @Summary List runs for a job
// @Description List all scrape runs for a specific job
// @Tags runs
// @Produce json
// @Security BearerAuth
// @Param jobId path int true "Job ID"
// @Success 200 {object} RunListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /jobs/{jobId}/runs [get]
func (h *HttpServer) ListRuns(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	jobID, err := strconv.ParseUint(c.Params("jobId"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid job ID")
	}

	// Verify job ownership
	var job model.Job
	if err := h.DB.Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrJobNotFound.Error(), "Job not found")
	}

	page, pageSize := parsePagination(c)
	offset := (page - 1) * pageSize

	var total int64
	h.DB.Model(&model.JobRun{}).Where("job_id = ?", jobID).Count(&total)

	var runs []model.JobRun
	if err := h.DB.Where("job_id = ?", jobID).Order("created_at DESC").
		Offset(offset).Limit(pageSize).Find(&runs).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to list runs")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"items": runs,
		"total": total,
		"page":  page,
		"limit": pageSize,
	}, "Runs retrieved successfully")
}

// GetRun godoc
// @Summary Get run details
// @Description Get detailed run results including rankings, diffs, and AI report
// @Tags runs
// @Produce json
// @Security BearerAuth
// @Param jobId path int true "Job ID"
// @Param runId path int true "Run ID"
// @Success 200 {object} RunDetailResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /jobs/{jobId}/runs/{runId} [get]
func (h *HttpServer) GetRun(c *fiber.Ctx) error {
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

	// Get search pairs
	var pairs []model.SearchPair
	h.DB.Where("run_id = ?", runID).Find(&pairs)

	// Get search results
	var results []model.SearchResult
	h.DB.Where("run_id = ?", runID).Order("keyword, state, position").Find(&results)

	// Get rank diffs
	var diffs []model.RankDiff
	h.DB.Where("run_id = ?", runID).Find(&diffs)

	// Get report
	var report model.Report
	h.DB.Where("run_id = ?", runID).First(&report)

	// Compute metrics
	metrics := RunMetrics{
		CompletedPairs: run.CompletedPairs,
		FailedPairs:    run.FailedPairs,
		TotalPairs:     run.TotalPairs,
	}

	// Compute avg response time from search pairs that have both started_at and finished_at
	// Timestamps are int64 nanoseconds, convert to milliseconds
	var avgMs *float64
	h.DB.Model(&model.SearchPair{}).
		Where("run_id = ? AND started_at IS NOT NULL AND finished_at IS NOT NULL AND started_at > 0 AND finished_at > 0", runID).
		Select("AVG((finished_at - started_at)::float / 1000000)").
		Scan(&avgMs)
	metrics.AvgResponseTimeMs = avgMs

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"run":     run,
		"pairs":   pairs,
		"results": results,
		"diffs":   diffs,
		"report":  report,
		"metrics": metrics,
	}, "Run details retrieved successfully")
}
