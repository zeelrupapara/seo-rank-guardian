package v1

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	apperrors "github.com/zeelrupapara/seo-rank-guardian/pkg/errors"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
	"gorm.io/gorm"
)

type JobRequest struct {
	Name         string             `json:"name" validate:"required,min=1,max=255"`
	Domain       string             `json:"domain" validate:"required,min=1,max=255"`
	IsActive     *bool              `json:"is_active"`
	ScheduleTime string             `json:"schedule_time" validate:"omitempty,max=10"`
	Competitors  []string           `json:"competitors"`
	Keywords     []string           `json:"keywords" validate:"required,min=1"`
	Regions      []model.JobRegion  `json:"regions" validate:"required,min=1"`
}

// CreateJob godoc
// @Summary Create a new job
// @Description Create a new SEO tracking job configuration
// @Tags jobs
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body JobRequest true "Job configuration"
// @Success 201 {object} JobResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /jobs [post]
func (h *HttpServer) CreateJob(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	var req JobRequest
	if err := c.BodyParser(&req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid request body")
	}

	if err := h.Validate.Struct(req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Validation failed")
	}

	job := model.Job{
		UserID:       userID,
		Name:         req.Name,
		Domain:       req.Domain,
		IsActive:     true,
		ScheduleTime: req.ScheduleTime,
	}

	if req.IsActive != nil {
		job.IsActive = *req.IsActive
	}

	if err := job.SetCompetitors(req.Competitors); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to set competitors")
	}
	if err := job.SetKeywords(req.Keywords); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to set keywords")
	}
	if err := job.SetRegions(req.Regions); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to set regions")
	}

	job.CreatedBy = userID
	job.UpdatedBy = userID

	if err := h.DB.Create(&job).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to create job")
	}

	return httputil.SuccessResponse(c, fiber.StatusCreated, job, "Job created successfully")
}

// ListJobs godoc
// @Summary List all jobs
// @Description List all SEO tracking jobs for the authenticated user
// @Tags jobs
// @Produce json
// @Security BearerAuth
// @Success 200 {object} JobListResponse
// @Failure 401 {object} ErrorResponse
// @Router /jobs [get]
func (h *HttpServer) ListJobs(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	page, pageSize := parsePagination(c)
	offset := (page - 1) * pageSize

	var total int64
	h.DB.Model(&model.Job{}).Where("user_id = ?", userID).Count(&total)

	var jobs []model.Job
	if err := h.DB.Where("user_id = ?", userID).Order("created_at DESC").
		Offset(offset).Limit(pageSize).Find(&jobs).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to list jobs")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"items": jobs,
		"total": total,
		"page":  page,
		"limit": pageSize,
	}, "Jobs retrieved successfully")
}

// GetJob godoc
// @Summary Get a job
// @Description Get a specific job configuration with summary stats
// @Tags jobs
// @Produce json
// @Security BearerAuth
// @Param jobId path int true "Job ID"
// @Success 200 {object} JobDetailResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /jobs/{jobId} [get]
func (h *HttpServer) GetJob(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	jobID, err := strconv.ParseUint(c.Params("jobId"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid job ID")
	}

	var job model.Job
	if err := h.DB.Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrJobNotFound.Error(), "Job not found")
	}

	// Get summary stats
	var totalRuns int64
	h.DB.Model(&model.JobRun{}).Where("job_id = ?", jobID).Count(&totalRuns)

	var lastRun model.JobRun
	h.DB.Where("job_id = ?", jobID).Order("created_at DESC").First(&lastRun)

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"job":        job,
		"total_runs": totalRuns,
		"last_run":   lastRun,
	}, "Job retrieved successfully")
}

// UpdateJob godoc
// @Summary Update a job
// @Description Full replace of a job configuration
// @Tags jobs
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param jobId path int true "Job ID"
// @Param body body JobRequest true "Job configuration"
// @Success 200 {object} JobResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /jobs/{jobId} [put]
func (h *HttpServer) UpdateJob(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	jobID, err := strconv.ParseUint(c.Params("jobId"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid job ID")
	}

	var job model.Job
	if err := h.DB.Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrJobNotFound.Error(), "Job not found")
	}

	var req JobRequest
	if err := c.BodyParser(&req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid request body")
	}

	if err := h.Validate.Struct(req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Validation failed")
	}

	job.Name = req.Name
	job.Domain = req.Domain
	job.ScheduleTime = req.ScheduleTime
	if req.IsActive != nil {
		job.IsActive = *req.IsActive
	}

	if err := job.SetCompetitors(req.Competitors); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to set competitors")
	}
	if err := job.SetKeywords(req.Keywords); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to set keywords")
	}
	if err := job.SetRegions(req.Regions); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to set regions")
	}

	job.UpdatedBy = userID

	if err := h.DB.Save(&job).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to update job")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, job, "Job updated successfully")
}

// DeleteJob godoc
// @Summary Delete a job
// @Description Soft delete a job configuration
// @Tags jobs
// @Produce json
// @Security BearerAuth
// @Param jobId path int true "Job ID"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /jobs/{jobId} [delete]
func (h *HttpServer) DeleteJob(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	jobID, err := strconv.ParseUint(c.Params("jobId"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid job ID")
	}

	result := h.DB.Where("id = ? AND user_id = ?", jobID, userID).Delete(&model.Job{})
	if result.Error != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to delete job")
	}
	if result.RowsAffected == 0 {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrJobNotFound.Error(), "Job not found")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, nil, "Job deleted successfully")
}

// TriggerScrape godoc
// @Summary Trigger a scrape
// @Description Manually trigger a scrape run for a job
// @Tags jobs
// @Produce json
// @Security BearerAuth
// @Param jobId path int true "Job ID"
// @Success 201 {object} ScrapeResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /jobs/{jobId}/scrape [post]
func (h *HttpServer) TriggerScrape(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	jobID, err := strconv.ParseUint(c.Params("jobId"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid job ID")
	}

	var job model.Job
	if err := h.DB.Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrJobNotFound.Error(), "Job not found")
	}

	if !job.IsActive {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrJobNotActive.Error(), "Job is not active")
	}

	keywords := job.GetKeywords()
	regions := job.GetRegions()
	totalPairs := len(keywords) * len(regions)

	now := time.Now()
	run := model.JobRun{
		JobID:       job.ID,
		Status:      "running",
		TotalPairs:  totalPairs,
		TriggeredBy: "manual",
		StartedAt:   &now,
	}
	run.CreatedBy = userID

	// Use a transaction to atomically check for in-progress runs and create new one
	err = h.DB.Transaction(func(tx *gorm.DB) error {
		var runningCount int64
		if err := tx.Model(&model.JobRun{}).
			Where("job_id = ? AND status IN ?", job.ID, []string{"pending", "running"}).
			Count(&runningCount).Error; err != nil {
			return err
		}
		if runningCount > 0 {
			return fmt.Errorf("run_in_progress")
		}
		return tx.Create(&run).Error
	})
	if err != nil {
		if err.Error() == "run_in_progress" {
			return httputil.ErrorResponse(c, fiber.StatusConflict, apperrors.ErrRunInProgress.Error(), "A run is already in progress")
		}
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to create run")
	}

	// Emit run_started event
	runStartedEvt := model.RunEvent{
		Type:  model.EventRunStarted,
		RunID: run.ID,
		JobID: job.ID,
		Payload: model.RunStatusEventPayload{
			Message:    "Run started",
			TotalPairs: totalPairs,
		},
	}
	if evtData, err := json.Marshal(runStartedEvt); err == nil {
		evtLog := model.RunEventLog{
			RunID:     run.ID,
			JobID:     job.ID,
			EventType: model.EventRunStarted,
			Data:      evtData,
		}
		h.DB.Create(&evtLog)
		_ = h.Nats.PublishRaw(fmt.Sprintf("srg.logs.%d.%d", job.ID, run.ID), evtData)

		// Publish to per-user WS subject
		wsEvent := model.Event{Type: model.EventLogs, Payload: runStartedEvt}
		if wsData, err := json.Marshal(wsEvent); err == nil {
			_ = h.Nats.PublishRaw(model.SubjectUserEvents(userID), wsData)
		}
	}

	// Create search pairs and publish to NATS — track actual created count
	createdPairs := 0
	for _, keyword := range keywords {
		for _, region := range regions {
			searchQuery := fmt.Sprintf("%s in %s", keyword, region.State)

			pair := model.SearchPair{
				RunID:       run.ID,
				JobID:       job.ID,
				Keyword:     keyword,
				State:       region.State,
				Country:     region.Country,
				SearchQuery: searchQuery,
				Status:      "pending",
			}
			pair.CreatedBy = userID

			if err := h.DB.Create(&pair).Error; err != nil {
				h.Log.Errorf("Failed to create search pair: %v", err)
				continue
			}

			// Publish to NATS
			msg := map[string]interface{}{
				"pair_id":      pair.ID,
				"run_id":       run.ID,
				"job_id":       job.ID,
				"search_query": searchQuery,
				"keyword":      keyword,
				"state":        region.State,
				"country":      region.Country,
				"domain":       job.Domain,
				"competitors":  job.GetCompetitors(),
			}
			msgData, err := json.Marshal(msg)
			if err != nil {
				h.Log.Errorf("Failed to marshal scrape message: %v", err)
				continue
			}

			if err := h.Nats.Publish("srg.jobs.scrape", msgData); err != nil {
				h.Log.Errorf("Failed to publish scrape job: %v", err)
			}
			createdPairs++
		}
	}

	// Update TotalPairs to reflect actually created pairs
	if createdPairs != totalPairs {
		h.DB.Model(&run).Update("total_pairs", createdPairs)
		totalPairs = createdPairs
	}

	return httputil.SuccessResponse(c, fiber.StatusCreated, fiber.Map{
		"run":         run,
		"total_pairs": totalPairs,
	}, "Scrape triggered successfully")
}
