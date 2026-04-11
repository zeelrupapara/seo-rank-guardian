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
	Name         string            `json:"name" validate:"required,min=1,max=255"`
	Domain       string            `json:"domain" validate:"required,min=1,max=255"`
	IsActive     *bool             `json:"is_active"`
	ScheduleTime string            `json:"schedule_time" validate:"omitempty,max=100"`
	Competitors  []string          `json:"competitors"`
	Keywords     []string          `json:"keywords" validate:"required,min=1"`
	Regions      []model.JobRegion `json:"regions" validate:"required,min=1"`
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

	// Normalize and validate domain
	req.Domain = model.NormalizeDomain(req.Domain)
	if err := model.ValidateDomain(req.Domain); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), fmt.Sprintf("Invalid domain: %s", err.Error()))
	}

	// Normalize, validate, and deduplicate competitors
	seen := make(map[string]bool)
	var competitors []string
	for _, comp := range req.Competitors {
		comp = model.NormalizeDomain(comp)
		if err := model.ValidateDomain(comp); err != nil {
			return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), fmt.Sprintf("Invalid competitor domain %q: %s", comp, err.Error()))
		}
		if comp == req.Domain {
			return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Competitor domain cannot be the same as the target domain")
		}
		if !seen[comp] {
			seen[comp] = true
			competitors = append(competitors, comp)
		}
	}
	req.Competitors = competitors

	// Deduplicate keywords
	keywords := deduplicateKeywords(req.Keywords)

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
	if err := job.SetRegions(req.Regions); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to set regions")
	}

	// Build keyword associations
	for _, kw := range keywords {
		job.Keywords = append(job.Keywords, model.JobKeyword{Keyword: kw})
	}

	job.CreatedBy = userID
	job.UpdatedBy = userID

	if err := h.DB.Create(&job).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to create job")
	}

	if h.Scheduler != nil {
		h.Scheduler.AddJob(job)
	}

	var actingUser model.User
	h.DB.Select("username").First(&actingUser, userID)
	h.writeAudit(c, userID, actingUser.Username, "job.create", "job", fmtID(job.ID), map[string]any{"job_name": job.Name, "domain": job.Domain})
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

	query := h.DB.Model(&model.Job{}).Where("user_id = ?", userID)

	// Search filter (ILIKE on name or domain)
	if search := c.Query("search"); search != "" {
		like := "%" + search + "%"
		query = query.Where("(name ILIKE ? OR domain ILIKE ?)", like, like)
	}

	// Status filter
	if status := c.Query("status"); status == "active" {
		query = query.Where("is_active = ?", true)
	} else if status == "inactive" {
		query = query.Where("is_active = ?", false)
	}

	var total int64
	query.Count(&total)

	var jobs []model.Job
	if err := query.Preload("Keywords").Order("created_at DESC").
		Offset(offset).Limit(pageSize).Find(&jobs).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to list jobs")
	}

	if len(jobs) == 0 {
		return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
			"items": []JobListItem{},
			"total": total,
			"page":  page,
			"limit": pageSize,
		}, "Jobs retrieved successfully")
	}

	// Collect job IDs
	jobIDs := make([]uint, len(jobs))
	for i, j := range jobs {
		jobIDs[i] = j.ID
	}

	// Batch-load last run per job
	type lastRunRow struct {
		JobID       uint
		RunID       uint
		Status      string
		CompletedAt *int64
		CreatedAt   int64
	}
	var lastRuns []lastRunRow
	h.DB.Raw(`SELECT DISTINCT ON (job_id) job_id, id AS run_id, status, completed_at, created_at
		FROM srg_job_runs WHERE job_id IN ? ORDER BY job_id, created_at DESC`, jobIDs).Scan(&lastRuns)

	lastRunMap := make(map[uint]*JobRunBrief)
	for _, lr := range lastRuns {
		lastRunMap[lr.JobID] = &JobRunBrief{
			ID:          lr.RunID,
			Status:      lr.Status,
			CompletedAt: lr.CompletedAt,
			CreatedAt:   lr.CreatedAt,
		}
	}

	// Batch-load latest health score per job from reports
	type healthRow struct {
		JobID       uint
		HealthScore *int
	}
	var healthRows []healthRow
	h.DB.Raw(`SELECT DISTINCT ON (job_id) job_id, (result->>'health_score')::int AS health_score
		FROM srg_reports WHERE job_id IN ? AND status = 'generated' ORDER BY job_id, created_at DESC`, jobIDs).Scan(&healthRows)

	healthMap := make(map[uint]*int)
	for _, hr := range healthRows {
		healthMap[hr.JobID] = hr.HealthScore
	}

	// Build enriched items
	items := make([]JobListItem, len(jobs))
	for i, j := range jobs {
		compFavicons := make([]DomainInfo, 0)
		for _, comp := range j.GetCompetitors() {
			compFavicons = append(compFavicons, DomainInfo{Domain: comp, FaviconURL: model.FaviconURL(comp)})
		}
		items[i] = JobListItem{
			Job:                j,
			KeywordCount:       len(j.Keywords),
			RegionCount:        len(j.GetRegions()),
			LastRun:            lastRunMap[j.ID],
			HealthScore:        healthMap[j.ID],
			FaviconURL:         model.FaviconURL(j.Domain),
			CompetitorFavicons: compFavicons,
		}
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"items": items,
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
	if err := h.DB.Preload("Keywords").Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrJobNotFound.Error(), "Job not found")
	}

	// Get summary stats
	var totalRuns int64
	h.DB.Model(&model.JobRun{}).Where("job_id = ?", jobID).Count(&totalRuns)

	var lastRun model.JobRun
	h.DB.Where("job_id = ?", jobID).Order("created_at DESC").First(&lastRun)

	compFavicons := make([]DomainInfo, 0)
	for _, comp := range job.GetCompetitors() {
		compFavicons = append(compFavicons, DomainInfo{Domain: comp, FaviconURL: model.FaviconURL(comp)})
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"job":                 job,
		"total_runs":          totalRuns,
		"last_run":            lastRun,
		"favicon_url":         model.FaviconURL(job.Domain),
		"competitor_favicons": compFavicons,
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
	if err := h.DB.Preload("Keywords").Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrJobNotFound.Error(), "Job not found")
	}

	var req JobRequest
	if err := c.BodyParser(&req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid request body")
	}

	if err := h.Validate.Struct(req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Validation failed")
	}

	// Normalize and validate domain
	req.Domain = model.NormalizeDomain(req.Domain)
	if err := model.ValidateDomain(req.Domain); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), fmt.Sprintf("Invalid domain: %s", err.Error()))
	}

	// Normalize, validate, and deduplicate competitors
	seen := make(map[string]bool)
	var competitors []string
	for _, comp := range req.Competitors {
		comp = model.NormalizeDomain(comp)
		if err := model.ValidateDomain(comp); err != nil {
			return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), fmt.Sprintf("Invalid competitor domain %q: %s", comp, err.Error()))
		}
		if comp == req.Domain {
			return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Competitor domain cannot be the same as the target domain")
		}
		if !seen[comp] {
			seen[comp] = true
			competitors = append(competitors, comp)
		}
	}
	req.Competitors = competitors

	job.Name = req.Name
	job.Domain = req.Domain
	job.ScheduleTime = req.ScheduleTime
	if req.IsActive != nil {
		job.IsActive = *req.IsActive
	}

	if err := job.SetCompetitors(req.Competitors); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to set competitors")
	}
	if err := job.SetRegions(req.Regions); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to set regions")
	}

	// Keyword lifecycle management
	newKeywords := deduplicateKeywords(req.Keywords)

	// Build maps of existing and new keywords
	existingMap := make(map[string]model.JobKeyword)
	for _, kw := range job.Keywords {
		existingMap[kw.Keyword] = kw
	}
	newMap := make(map[string]bool)
	for _, kw := range newKeywords {
		newMap[kw] = true
	}

	// Use a transaction for keyword lifecycle
	err = h.DB.Transaction(func(tx *gorm.DB) error {
		// Delete removed keywords and cascade their data
		for keyword, jk := range existingMap {
			if !newMap[keyword] {
				// Cascade delete related data
				tx.Where("job_id = ? AND keyword = ?", jobID, keyword).Delete(&model.RankDiff{})
				tx.Where("job_id = ? AND keyword = ?", jobID, keyword).Delete(&model.SearchResult{})
				tx.Where("job_id = ? AND keyword = ?", jobID, keyword).Delete(&model.SearchPair{})
				tx.Delete(&model.JobKeyword{}, jk.ID)
			}
		}

		// Add new keywords
		for _, keyword := range newKeywords {
			if _, exists := existingMap[keyword]; !exists {
				jk := model.JobKeyword{JobID: uint(jobID), Keyword: keyword}
				if err := tx.Create(&jk).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to update keywords")
	}

	job.UpdatedBy = userID

	// Clear Keywords before Save to prevent GORM from re-saving the old association
	job.Keywords = nil
	if err := h.DB.Omit("Keywords").Save(&job).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to update job")
	}

	// Reload keywords for response
	h.DB.Where("job_id = ?", jobID).Find(&job.Keywords)

	if h.Scheduler != nil {
		h.Scheduler.AddJob(job)
	}

	var actingUser model.User
	h.DB.Select("username").First(&actingUser, userID)
	h.writeAudit(c, userID, actingUser.Username, "job.update", "job", fmtID64(jobID), map[string]any{"job_name": job.Name})
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

	// Fetch job name before deletion for audit log
	var jobToDelete model.Job
	h.DB.Select("name").Where("id = ? AND user_id = ?", jobID, userID).First(&jobToDelete)

	result := h.DB.Where("id = ? AND user_id = ?", jobID, userID).Delete(&model.Job{})
	if result.Error != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to delete job")
	}
	if result.RowsAffected == 0 {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrJobNotFound.Error(), "Job not found")
	}

	if h.Scheduler != nil {
		h.Scheduler.RemoveJob(uint(jobID))
	}

	var actingUser model.User
	h.DB.Select("username").First(&actingUser, userID)
	h.writeAudit(c, userID, actingUser.Username, "job.delete", "job", fmtID64(jobID), map[string]any{"job_name": jobToDelete.Name})
	return httputil.SuccessResponse(c, fiber.StatusOK, nil, "Job deleted successfully")
}

// TriggerScrapeForJob is the shared logic called by both TriggerScrape (HTTP)
// and the scheduler. Returns "run_in_progress" error string if a run is active.
func (h *HttpServer) TriggerScrapeForJob(job model.Job, triggeredBy string, userID uint) (*model.JobRun, error) {
	regions := job.GetRegions()
	totalPairs := len(job.Keywords) * len(regions)

	now := time.Now().UnixNano()
	run := model.JobRun{
		JobID:       job.ID,
		Status:      "running",
		TotalPairs:  totalPairs,
		TriggeredBy: triggeredBy,
		StartedAt:   &now,
	}
	run.CreatedBy = userID

	// Use a transaction to atomically check for in-progress runs and create new one
	err := h.DB.Transaction(func(tx *gorm.DB) error {
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
		return nil, err
	}

	// Emit run_started event
	runStartedEvt := model.RunEvent{
		Type:      model.EventRunStarted,
		RunID:     run.ID,
		JobID:     job.ID,
		Timestamp: time.Now().UnixNano(),
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
	for _, kw := range job.Keywords {
		for _, region := range regions {
			searchQuery := fmt.Sprintf("%s in %s", kw.Keyword, region.State)

			pair := model.SearchPair{
				RunID:       run.ID,
				JobID:       job.ID,
				KeywordID:   kw.ID,
				Keyword:     kw.Keyword,
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
				"keyword":      kw.Keyword,
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
		run.TotalPairs = createdPairs
	}

	return &run, nil
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
	if err := h.DB.Preload("Keywords").Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrJobNotFound.Error(), "Job not found")
	}

	if !job.IsActive {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrJobNotActive.Error(), "Job is not active")
	}

	run, err := h.TriggerScrapeForJob(job, "manual", userID)
	if err != nil {
		if err.Error() == "run_in_progress" {
			return httputil.ErrorResponse(c, fiber.StatusConflict, apperrors.ErrRunInProgress.Error(), "A run is already in progress")
		}
		h.Log.Errorf("Failed to create run for job %d: %v", job.ID, err)
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to create run")
	}

	var actingUser model.User
	h.DB.Select("username").First(&actingUser, userID)
	h.writeAudit(c, userID, actingUser.Username, "job.trigger_scan", "job", fmtID64(jobID), map[string]any{"job_name": job.Name, "triggered_by": "manual"})
	return httputil.SuccessResponse(c, fiber.StatusCreated, fiber.Map{
		"run":         run,
		"total_pairs": run.TotalPairs,
	}, "Scrape triggered successfully")
}

// JobStats godoc
// @Summary Get job stats
// @Description Get computed stats for a job (health score, top 3 rankings, visibility index)
// @Tags jobs
// @Produce json
// @Security BearerAuth
// @Param jobId path int true "Job ID"
// @Success 200 {object} JobStatsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /jobs/{jobId}/stats [get]
func (h *HttpServer) JobStats(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	jobID, err := strconv.ParseUint(c.Params("jobId"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid job ID")
	}

	var job model.Job
	if err := h.DB.Preload("Keywords").Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrJobNotFound.Error(), "Job not found")
	}

	compDomains := job.GetCompetitors()
	compInfos := make([]DomainInfo, len(compDomains))
	for i, comp := range compDomains {
		compInfos[i] = DomainInfo{Domain: comp, FaviconURL: model.FaviconURL(comp)}
	}
	stats := JobStatsData{
		Competitors: compInfos,
	}

	regions := job.GetRegions()
	stats.TotalKeywords = len(job.Keywords) * len(regions)

	// Find latest completed run
	var latestRun model.JobRun
	if err := h.DB.Where("job_id = ? AND status IN ?", jobID, []string{"completed", "partial"}).
		Order("created_at DESC").First(&latestRun).Error; err != nil {
		// No completed runs yet
		return httputil.SuccessResponse(c, fiber.StatusOK, stats, "Job stats retrieved successfully")
	}
	stats.RunID = &latestRun.ID

	// Health score from latest report
	var healthScore *int
	h.DB.Raw(`SELECT (result->>'health_score')::int FROM srg_reports WHERE job_id = ? AND run_id = ? AND status = 'generated' ORDER BY created_at DESC LIMIT 1`,
		jobID, latestRun.ID).Scan(&healthScore)
	stats.HealthScore = healthScore

	// Top 3 rankings: count distinct keyword|state where is_target=true AND position<=3
	var top3Current int64
	h.DB.Model(&model.SearchResult{}).
		Where("run_id = ? AND is_target = ? AND position <= 3", latestRun.ID, true).
		Count(&top3Current)
	stats.Top3Rankings = top3Current

	// Top 3 change vs previous run
	var prevRun model.JobRun
	if err := h.DB.Where("job_id = ? AND status IN ? AND id < ?", jobID, []string{"completed", "partial"}, latestRun.ID).
		Order("created_at DESC").First(&prevRun).Error; err == nil {
		var top3Prev int64
		h.DB.Model(&model.SearchResult{}).
			Where("run_id = ? AND is_target = ? AND position <= 3", prevRun.ID, true).
			Count(&top3Prev)
		stats.Top3Change = top3Current - top3Prev
	}

	// Visibility index: (keywords in top 10 / total keyword pairs) * 100
	if stats.TotalKeywords > 0 {
		var inTop10 int64
		h.DB.Model(&model.SearchResult{}).
			Where("run_id = ? AND is_target = ? AND position <= 10", latestRun.ID, true).
			Count(&inTop10)
		stats.VisibilityIndex = float64(inTop10) / float64(stats.TotalKeywords) * 100
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, stats, "Job stats retrieved successfully")
}

// deduplicateKeywords returns a deduplicated list of keywords preserving order.
func deduplicateKeywords(keywords []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, kw := range keywords {
		if kw != "" && !seen[kw] {
			seen[kw] = true
			result = append(result, kw)
		}
	}
	return result
}
