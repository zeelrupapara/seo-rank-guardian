package v1

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	apperrors "github.com/zeelrupapara/seo-rank-guardian/pkg/errors"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
)

// --- Response types ---

type DomainPosition struct {
	Domain   string `json:"domain"`
	Position int    `json:"position"`
}

type RankingEntry struct {
	Keyword        string           `json:"keyword"`
	State          string           `json:"state"`
	TargetPosition int              `json:"target_position"`
	PrevPosition   int              `json:"prev_position"`
	Change         string           `json:"change"`
	TopDomains     []DomainPosition `json:"top_domains"`
	Competitors    []DomainPosition `json:"competitors"`
}

type TrendPoint struct {
	RunID    uint  `json:"run_id"`
	RunDate  int64 `json:"run_date"`
	Position int   `json:"position"`
}

// --- Handlers ---

// RunRankings godoc
// @Summary Get rankings for a specific run
// @Description Get search results grouped by keyword x state for a specific run
// @Tags runs
// @Produce json
// @Security BearerAuth
// @Param jobId path int true "Job ID"
// @Param runId path int true "Run ID"
// @Success 200 {object} RankingsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /jobs/{jobId}/runs/{runId}/rankings [get]
func (h *HttpServer) RunRankings(c *fiber.Ctx) error {
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

	rankings := h.buildRankings(uint(runID), uint(jobID))
	return httputil.SuccessResponse(c, fiber.StatusOK, rankings, "Rankings retrieved successfully")
}

// LatestRankings godoc
// @Summary Get latest rankings for a job
// @Description Get rankings from the most recent completed run with optional filters and pagination
// @Tags jobs
// @Produce json
// @Security BearerAuth
// @Param jobId path int true "Job ID"
// @Param search query string false "Filter keywords by search term"
// @Param region query string false "Filter by state/region"
// @Param page query int false "Page number (default 1)"
// @Param limit query int false "Items per page (default 20)"
// @Success 200 {object} RankingsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /jobs/{jobId}/rankings [get]
func (h *HttpServer) LatestRankings(c *fiber.Ctx) error {
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

	// Find most recent completed run
	var latestRun model.JobRun
	if err := h.DB.Where("job_id = ? AND status IN ?", jobID, []string{"completed", "partial"}).
		Order("created_at DESC").First(&latestRun).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrRunNotFound.Error(), "No completed runs found")
	}

	rankings := h.buildRankings(latestRun.ID, uint(jobID))

	// Apply filters
	search := c.Query("search")
	region := c.Query("region")
	if search != "" || region != "" {
		var filtered []RankingEntry
		for _, r := range rankings {
			if search != "" && !strings.Contains(strings.ToLower(r.Keyword), strings.ToLower(search)) {
				continue
			}
			if region != "" && r.State != region {
				continue
			}
			filtered = append(filtered, r)
		}
		rankings = filtered
	}

	// Pagination
	total := len(rankings)
	page, pageSize := parsePagination(c)
	offset := (page - 1) * pageSize
	end := offset + pageSize
	if offset > total {
		offset = total
	}
	if end > total {
		end = total
	}
	paged := rankings[offset:end]
	if paged == nil {
		paged = []RankingEntry{}
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"run_id":   latestRun.ID,
		"rankings": paged,
		"total":    total,
		"page":     page,
		"limit":    pageSize,
	}, "Latest rankings retrieved successfully")
}

// GetRunReport godoc
// @Summary Get AI report for a run
// @Description Get the AI-generated report for a specific run
// @Tags runs
// @Produce json
// @Security BearerAuth
// @Param jobId path int true "Job ID"
// @Param runId path int true "Run ID"
// @Success 200 {object} ReportResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /jobs/{jobId}/runs/{runId}/report [get]
func (h *HttpServer) GetRunReport(c *fiber.Ctx) error {
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

	var report model.Report
	if err := h.DB.Where("run_id = ? AND job_id = ?", runID, jobID).First(&report).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, "report not found", "No report found for this run")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, report, "Report retrieved successfully")
}

// PositionTrends godoc
// @Summary Get position trends over time
// @Description Track a domain's position across recent runs for a keyword x state
// @Tags jobs
// @Produce json
// @Security BearerAuth
// @Param jobId path int true "Job ID"
// @Param keyword query string true "Keyword to track"
// @Param state query string true "State/region to track"
// @Param domain query string false "Domain to track (defaults to job's target domain)"
// @Param limit query int false "Number of recent runs to include (default 10)"
// @Param range query string false "Time range: 7d, 30d, 90d, all"
// @Success 200 {object} TrendsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /jobs/{jobId}/trends [get]
func (h *HttpServer) PositionTrends(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	jobID, err := strconv.ParseUint(c.Params("jobId"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid job ID")
	}

	keyword := c.Query("keyword")
	state := c.Query("state")
	if keyword == "" || state == "" {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "keyword and state query params are required")
	}

	// Verify job ownership
	var job model.Job
	if err := h.DB.Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrJobNotFound.Error(), "Job not found")
	}

	domain := c.Query("domain", job.Domain)
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	// Build query for recent completed runs
	runsQuery := h.DB.Where("job_id = ? AND status IN ?", jobID, []string{"completed", "partial"})

	// Apply time range filter
	if rangeParam := c.Query("range"); rangeParam != "" && rangeParam != "all" {
		var duration time.Duration
		switch rangeParam {
		case "7d":
			duration = 7 * 24 * time.Hour
		case "30d":
			duration = 30 * 24 * time.Hour
		case "90d":
			duration = 90 * 24 * time.Hour
		}
		if duration > 0 {
			cutoff := time.Now().Add(-duration).UnixNano()
			runsQuery = runsQuery.Where("created_at >= ?", cutoff)
		}
	}

	var runs []model.JobRun
	runsQuery.Order("created_at DESC").Limit(limit).Find(&runs)

	var trends []TrendPoint
	for i := len(runs) - 1; i >= 0; i-- {
		run := runs[i]
		var result model.SearchResult
		// Use exact suffix match to avoid LIKE injection from user input
		cleanDomain := strings.TrimPrefix(strings.TrimPrefix(domain, "https://"), "http://")
		cleanDomain = strings.TrimPrefix(cleanDomain, "www.")
		err := h.DB.Where("run_id = ? AND keyword = ? AND state = ? AND REPLACE(domain, 'www.', '') = ?",
			run.ID, keyword, state, cleanDomain).
			First(&result).Error

		position := 0
		if err == nil {
			position = result.Position
		}

		runDate := run.CreatedAt
		if run.StartedAt != nil {
			runDate = *run.StartedAt
		}

		trends = append(trends, TrendPoint{
			RunID:    run.ID,
			RunDate:  runDate,
			Position: position,
		})
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, trends, "Trends retrieved successfully")
}

// ListReports godoc
// @Summary List reports for a job
// @Description Get a paginated list of AI reports for a specific job
// @Tags reports
// @Produce json
// @Security BearerAuth
// @Param jobId path int true "Job ID"
// @Param page query int false "Page number (default 1)"
// @Param limit query int false "Items per page (default 20)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /jobs/{jobId}/reports [get]
func (h *HttpServer) ListReports(c *fiber.Ctx) error {
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
	h.DB.Model(&model.Report{}).Where("job_id = ?", jobID).Count(&total)

	type reportRow struct {
		ID          uint
		RunID       uint
		Status      string
		HealthScore *int
		Provider    string
		CreatedAt   int64
	}
	var rows []reportRow
	h.DB.Raw(`SELECT id, run_id, status, (result->>'health_score')::int AS health_score, provider, created_at
		FROM srg_reports WHERE job_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`, jobID, pageSize, offset).Scan(&rows)

	items := make([]ReportListItem, len(rows))
	for i, r := range rows {
		items[i] = ReportListItem{
			ID:          r.ID,
			RunID:       r.RunID,
			Status:      r.Status,
			HealthScore: r.HealthScore,
			Provider:    r.Provider,
			CreatedAt:   r.CreatedAt,
		}
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"items": items,
		"total": total,
		"page":  page,
		"limit": pageSize,
	}, "Reports retrieved successfully")
}

// --- Pair Detail Handlers ---

// PairSummary godoc
// @Summary Get pair summary stats
// @Description Get summary statistics for a specific keyword x state pair
// @Tags pairs
// @Produce json
// @Security BearerAuth
// @Param jobId path int true "Job ID"
// @Param keyword path string true "Keyword (URL-encoded)"
// @Param state path string true "State/region (URL-encoded)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /jobs/{jobId}/pairs/{keyword}/{state}/summary [get]
func (h *HttpServer) PairSummary(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	jobID, err := strconv.ParseUint(c.Params("jobId"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid job ID")
	}

	keyword, _ := url.QueryUnescape(c.Params("keyword"))
	state, _ := url.QueryUnescape(c.Params("state"))
	if keyword == "" || state == "" {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "keyword and state are required")
	}

	var job model.Job
	if err := h.DB.Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrJobNotFound.Error(), "Job not found")
	}

	// Get latest completed run
	var latestRun model.JobRun
	if err := h.DB.Where("job_id = ? AND status IN ?", jobID, []string{"completed", "partial"}).
		Order("created_at DESC").First(&latestRun).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrRunNotFound.Error(), "No completed runs found")
	}

	// Get current target position
	var currentResult model.SearchResult
	currentPosition := 0
	if err := h.DB.Where("run_id = ? AND keyword = ? AND state = ? AND is_target = ?",
		latestRun.ID, keyword, state, true).First(&currentResult).Error; err == nil {
		currentPosition = currentResult.Position
	}

	// Get previous run target position for change
	change := 0
	var prevRun model.JobRun
	if err := h.DB.Where("job_id = ? AND status IN ? AND id < ?", jobID, []string{"completed", "partial"}, latestRun.ID).
		Order("created_at DESC").First(&prevRun).Error; err == nil {
		var prevResult model.SearchResult
		if err := h.DB.Where("run_id = ? AND keyword = ? AND state = ? AND is_target = ?",
			prevRun.ID, keyword, state, true).First(&prevResult).Error; err == nil {
			change = prevResult.Position - currentPosition // positive = improved
		}
	}

	// Aggregate stats across all runs
	type aggStats struct {
		AvgPosition  float64
		BestPosition int
		WorstPosition int
		TotalScans   int
	}
	var stats aggStats
	h.DB.Raw(`SELECT
		COALESCE(AVG(sr.position), 0) as avg_position,
		COALESCE(MIN(sr.position), 0) as best_position,
		COALESCE(MAX(sr.position), 0) as worst_position,
		COUNT(DISTINCT sr.run_id) as total_scans
		FROM srg_search_results sr
		JOIN srg_job_runs jr ON jr.id = sr.run_id
		WHERE sr.job_id = ? AND sr.keyword = ? AND sr.state = ? AND sr.is_target = true
		AND jr.status IN ('completed', 'partial')`, jobID, keyword, state).Scan(&stats)

	return httputil.SuccessResponse(c, fiber.StatusOK, PairSummaryData{
		Keyword:         keyword,
		State:           state,
		CurrentPosition: currentPosition,
		Change:          change,
		AvgPosition:     stats.AvgPosition,
		BestPosition:    stats.BestPosition,
		WorstPosition:   stats.WorstPosition,
		TotalScans:      stats.TotalScans,
		RunID:           latestRun.ID,
	}, "Pair summary retrieved successfully")
}

// PairScanHistory godoc
// @Summary Get pair scan history
// @Description Get paginated scan history for a keyword x state pair
// @Tags pairs
// @Produce json
// @Security BearerAuth
// @Param jobId path int true "Job ID"
// @Param keyword path string true "Keyword (URL-encoded)"
// @Param state path string true "State/region (URL-encoded)"
// @Param page query int false "Page number (default 1)"
// @Param limit query int false "Items per page (default 20)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /jobs/{jobId}/pairs/{keyword}/{state}/scans [get]
func (h *HttpServer) PairScanHistory(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	jobID, err := strconv.ParseUint(c.Params("jobId"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid job ID")
	}

	keyword, _ := url.QueryUnescape(c.Params("keyword"))
	state, _ := url.QueryUnescape(c.Params("state"))
	if keyword == "" || state == "" {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "keyword and state are required")
	}

	var job model.Job
	if err := h.DB.Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrJobNotFound.Error(), "Job not found")
	}

	page, pageSize := parsePagination(c)
	offset := (page - 1) * pageSize

	// Get completed runs with this pair
	type scanRow struct {
		RunID      uint
		PairStatus string
		StartedAt  *int64
		FinishedAt *int64
		RunCreatedAt int64
	}
	var total int64
	h.DB.Raw(`SELECT COUNT(*) FROM srg_search_pairs sp
		JOIN srg_job_runs jr ON jr.id = sp.run_id
		WHERE sp.job_id = ? AND sp.keyword = ? AND sp.state = ?
		AND jr.status IN ('completed', 'partial')`, jobID, keyword, state).Scan(&total)

	var rows []scanRow
	h.DB.Raw(`SELECT sp.run_id, sp.status as pair_status, sp.started_at, sp.finished_at, jr.created_at as run_created_at
		FROM srg_search_pairs sp
		JOIN srg_job_runs jr ON jr.id = sp.run_id
		WHERE sp.job_id = ? AND sp.keyword = ? AND sp.state = ?
		AND jr.status IN ('completed', 'partial')
		ORDER BY jr.created_at DESC
		LIMIT ? OFFSET ?`, jobID, keyword, state, pageSize, offset).Scan(&rows)

	// Get all run IDs for batch queries
	runIDs := make([]uint, len(rows))
	for i, r := range rows {
		runIDs[i] = r.RunID
	}

	// Batch get target positions
	type posRow struct {
		RunID    uint
		Position int
	}
	var positions []posRow
	if len(runIDs) > 0 {
		h.DB.Raw(`SELECT run_id, position FROM srg_search_results
			WHERE run_id IN ? AND keyword = ? AND state = ? AND is_target = true`,
			runIDs, keyword, state).Scan(&positions)
	}
	posMap := make(map[uint]int)
	for _, p := range positions {
		posMap[p.RunID] = p.Position
	}

	// Batch get result counts
	type countRow struct {
		RunID uint
		Cnt   int
	}
	var counts []countRow
	if len(runIDs) > 0 {
		h.DB.Raw(`SELECT run_id, COUNT(*) as cnt FROM srg_search_results
			WHERE run_id IN ? AND keyword = ? AND state = ?
			GROUP BY run_id`, runIDs, keyword, state).Scan(&counts)
	}
	countMap := make(map[uint]int)
	for _, c := range counts {
		countMap[c.RunID] = c.Cnt
	}

	// Check which runs have reports
	var reportRunIDs []uint
	if len(runIDs) > 0 {
		h.DB.Raw(`SELECT run_id FROM srg_reports WHERE run_id IN ? AND job_id = ? AND status = 'generated'`,
			runIDs, jobID).Scan(&reportRunIDs)
	}
	reportMap := make(map[uint]bool)
	for _, rid := range reportRunIDs {
		reportMap[rid] = true
	}

	// Build items
	items := make([]PairScanItem, len(rows))
	scanNumber := int(total) - offset
	for i, r := range rows {
		var durationNs int64
		if r.StartedAt != nil && r.FinishedAt != nil {
			durationNs = *r.FinishedAt - *r.StartedAt
		}

		// Compute change from previous scan
		change := 0
		if i < len(rows)-1 {
			prevPos := posMap[rows[i+1].RunID]
			curPos := posMap[r.RunID]
			if prevPos > 0 && curPos > 0 {
				change = prevPos - curPos
			}
		}

		items[i] = PairScanItem{
			RunID:       r.RunID,
			ScanNumber:  scanNumber,
			Position:    posMap[r.RunID],
			Change:      change,
			Status:      r.PairStatus,
			ResultCount: countMap[r.RunID],
			DurationNs:  durationNs,
			HasReport:   reportMap[r.RunID],
			CreatedAt:   r.RunCreatedAt,
		}
		scanNumber--
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"items": items,
		"total": total,
		"page":  page,
		"limit": pageSize,
	}, "Pair scan history retrieved successfully")
}

// PairCompetitors godoc
// @Summary Get pair competitor positions
// @Description Get competitor positions for a specific keyword x state pair
// @Tags pairs
// @Produce json
// @Security BearerAuth
// @Param jobId path int true "Job ID"
// @Param keyword path string true "Keyword (URL-encoded)"
// @Param state path string true "State/region (URL-encoded)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /jobs/{jobId}/pairs/{keyword}/{state}/competitors [get]
func (h *HttpServer) PairCompetitors(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	jobID, err := strconv.ParseUint(c.Params("jobId"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid job ID")
	}

	keyword, _ := url.QueryUnescape(c.Params("keyword"))
	state, _ := url.QueryUnescape(c.Params("state"))
	if keyword == "" || state == "" {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "keyword and state are required")
	}

	var job model.Job
	if err := h.DB.Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrJobNotFound.Error(), "Job not found")
	}

	// Get latest completed run
	var latestRun model.JobRun
	if err := h.DB.Where("job_id = ? AND status IN ?", jobID, []string{"completed", "partial"}).
		Order("created_at DESC").First(&latestRun).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrRunNotFound.Error(), "No completed runs found")
	}

	// Get target and competitor results from latest run
	var results []model.SearchResult
	h.DB.Where("run_id = ? AND keyword = ? AND state = ? AND (is_target = ? OR is_competitor = ?)",
		latestRun.ID, keyword, state, true, true).Find(&results)

	// Get previous run for change calculation
	var prevResults []model.SearchResult
	var prevRun model.JobRun
	if err := h.DB.Where("job_id = ? AND status IN ? AND id < ?", jobID, []string{"completed", "partial"}, latestRun.ID).
		Order("created_at DESC").First(&prevRun).Error; err == nil {
		h.DB.Where("run_id = ? AND keyword = ? AND state = ? AND (is_target = ? OR is_competitor = ?)",
			prevRun.ID, keyword, state, true, true).Find(&prevResults)
	}
	prevPosMap := make(map[string]int)
	for _, r := range prevResults {
		prevPosMap[r.Domain] = r.Position
	}

	// Get average positions across recent runs
	type avgRow struct {
		Domain      string
		AvgPosition float64
	}
	var avgRows []avgRow
	h.DB.Raw(`SELECT domain, AVG(position) as avg_position
		FROM srg_search_results
		WHERE job_id = ? AND keyword = ? AND state = ? AND (is_target = true OR is_competitor = true)
		AND run_id IN (SELECT id FROM srg_job_runs WHERE job_id = ? AND status IN ('completed', 'partial') ORDER BY created_at DESC LIMIT 10)
		GROUP BY domain`, jobID, keyword, state, jobID).Scan(&avgRows)
	avgMap := make(map[string]float64)
	for _, a := range avgRows {
		avgMap[a.Domain] = a.AvgPosition
	}

	// Build response
	data := make([]PairCompetitorData, 0, len(results))
	for _, r := range results {
		change := 0
		if prevPos, ok := prevPosMap[r.Domain]; ok {
			change = prevPos - r.Position
		}
		data = append(data, PairCompetitorData{
			Domain:      r.Domain,
			FaviconURL:  model.FaviconURL(r.Domain),
			IsTarget:    r.IsTarget,
			Position:    r.Position,
			AvgPosition: avgMap[r.Domain],
			Change:      change,
		})
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, data, "Pair competitors retrieved successfully")
}

// TrackingPairs godoc
// @Summary Get keyword x region tracking pairs
// @Description Returns all keyword x region pairs from job config with latest scan data if available
// @Tags jobs
// @Produce json
// @Security BearerAuth
// @Param jobId path int true "Job ID"
// @Param search query string false "Filter keywords by search term"
// @Param region query string false "Filter by state/region"
// @Param page query int false "Page number (default 1)"
// @Param limit query int false "Items per page (default 20)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /jobs/{jobId}/tracking-pairs [get]
func (h *HttpServer) TrackingPairs(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	jobID, err := strconv.ParseUint(c.Params("jobId"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid job ID")
	}

	// Load job with keywords
	var job model.Job
	if err := h.DB.Preload("Keywords").Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrJobNotFound.Error(), "Job not found")
	}

	regions := job.GetRegions()

	// Build all keyword x region pairs
	type pair struct {
		keyword string
		state   string
		country string
	}
	var allPairs []pair
	for _, kw := range job.Keywords {
		for _, r := range regions {
			allPairs = append(allPairs, pair{keyword: kw.Keyword, state: r.State, country: r.Country})
		}
	}

	// Apply filters
	search := c.Query("search")
	region := c.Query("region")
	if search != "" || region != "" {
		var filtered []pair
		for _, p := range allPairs {
			if search != "" && !strings.Contains(strings.ToLower(p.keyword), strings.ToLower(search)) {
				continue
			}
			if region != "" && p.state != region {
				continue
			}
			filtered = append(filtered, p)
		}
		allPairs = filtered
	}

	// Pagination
	total := len(allPairs)
	page, pageSize := parsePagination(c)
	offset := (page - 1) * pageSize
	end := offset + pageSize
	if offset > total {
		offset = total
	}
	if end > total {
		end = total
	}
	pagedPairs := allPairs[offset:end]

	// Try to find latest and previous completed runs
	var latestRun, prevRun model.JobRun
	hasLatest := h.DB.Where("job_id = ? AND status IN ?", jobID, []string{"completed", "partial"}).
		Order("created_at DESC").First(&latestRun).Error == nil

	hasPrev := false
	if hasLatest {
		hasPrev = h.DB.Where("job_id = ? AND status IN ? AND id < ?", jobID, []string{"completed", "partial"}, latestRun.ID).
			Order("created_at DESC").First(&prevRun).Error == nil
	}

	// Batch-fetch scan status and target positions for the paged pairs from latest run
	type posKey struct{ keyword, state string }
	latestPosMap := make(map[posKey]int)
	prevPosMap := make(map[posKey]int)

	type scannedInfo struct {
		status     string
		finishedAt *int64
	}
	scannedMap := make(map[posKey]scannedInfo)

	if hasLatest && len(pagedPairs) > 0 {
		// Build keyword/state lists for IN query
		keywords := make([]string, len(pagedPairs))
		states := make([]string, len(pagedPairs))
		for i, p := range pagedPairs {
			keywords[i] = p.keyword
			states[i] = p.state
		}

		// Query srg_search_pairs for scan status (determines has_data)
		type pairRow struct {
			Keyword    string
			State      string
			Status     string
			FinishedAt *int64
		}
		var pairRows []pairRow
		h.DB.Raw(`SELECT keyword, state, status, finished_at FROM srg_search_pairs
			WHERE run_id = ? AND keyword IN ? AND state IN ?`,
			latestRun.ID, keywords, states).Scan(&pairRows)

		for _, r := range pairRows {
			scannedMap[posKey{r.Keyword, r.State}] = scannedInfo{
				status:     r.Status,
				finishedAt: r.FinishedAt,
			}
		}

		// Query srg_search_results for target positions (optional enrichment)
		type resultRow struct {
			Keyword  string
			State    string
			Position int
		}
		var latestResults []resultRow
		h.DB.Raw(`SELECT keyword, state, position FROM srg_search_results
			WHERE run_id = ? AND is_target = true AND keyword IN ? AND state IN ?`,
			latestRun.ID, keywords, states).Scan(&latestResults)

		for _, r := range latestResults {
			latestPosMap[posKey{r.Keyword, r.State}] = r.Position
		}

		if hasPrev {
			var prevResults []resultRow
			h.DB.Raw(`SELECT keyword, state, position FROM srg_search_results
				WHERE run_id = ? AND is_target = true AND keyword IN ? AND state IN ?`,
				prevRun.ID, keywords, states).Scan(&prevResults)
			for _, r := range prevResults {
				prevPosMap[posKey{r.Keyword, r.State}] = r.Position
			}
		}
	}

	// Build response
	entries := make([]TrackingPairEntry, len(pagedPairs))
	for i, p := range pagedPairs {
		k := posKey{p.keyword, p.state}
		info := scannedMap[k]
		hasData := info.status == "completed"
		latestPos := latestPosMap[k]
		prevPos := prevPosMap[k]

		change := ""
		if hasData {
			if prevPos > 0 && latestPos > 0 {
				delta := prevPos - latestPos
				if delta > 0 {
					change = "improved"
				} else if delta < 0 {
					change = "dropped"
				} else {
					change = "stable"
				}
			} else {
				change = "new"
			}
		}

		entries[i] = TrackingPairEntry{
			Keyword:        p.keyword,
			State:          p.state,
			Country:        p.country,
			HasData:        hasData,
			LatestPosition: latestPos,
			PrevPosition:   prevPos,
			Change:         change,
			LastScannedAt:  info.finishedAt,
		}
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"pairs": entries,
		"total": total,
		"page":  page,
		"limit": pageSize,
	}, "Tracking pairs retrieved successfully")
}

// --- Helper ---

func (h *HttpServer) buildRankings(runID, _ uint) []RankingEntry {
	var results []model.SearchResult
	h.DB.Where("run_id = ?", runID).Order("keyword, state, position").Find(&results)

	var diffs []model.RankDiff
	h.DB.Where("run_id = ?", runID).Find(&diffs)

	// Build diff lookup: keyword|state|domain -> diff
	diffMap := make(map[string]model.RankDiff)
	for _, d := range diffs {
		key := d.Keyword + "|" + d.State + "|" + d.Domain
		diffMap[key] = d
	}

	// Group results by keyword x state
	type pairKey struct{ keyword, state string }
	groupOrder := []pairKey{}
	groups := make(map[pairKey][]model.SearchResult)
	for _, r := range results {
		pk := pairKey{r.Keyword, r.State}
		if _, exists := groups[pk]; !exists {
			groupOrder = append(groupOrder, pk)
		}
		groups[pk] = append(groups[pk], r)
	}

	var rankings []RankingEntry
	for _, pk := range groupOrder {
		groupResults := groups[pk]

		entry := RankingEntry{
			Keyword:    pk.keyword,
			State:      pk.state,
			TopDomains: []DomainPosition{},
			Competitors: []DomainPosition{},
		}

		for _, r := range groupResults {
			if r.IsTarget {
				entry.TargetPosition = r.Position
				diffKey := r.Keyword + "|" + r.State + "|" + r.Domain
				if d, ok := diffMap[diffKey]; ok {
					entry.PrevPosition = d.PrevPosition
					entry.Change = d.ChangeType
				} else {
					entry.Change = "stable"
				}
			}
			if r.IsCompetitor {
				entry.Competitors = append(entry.Competitors, DomainPosition{
					Domain:   r.Domain,
					Position: r.Position,
				})
			}
			if r.Position <= 3 {
				entry.TopDomains = append(entry.TopDomains, DomainPosition{
					Domain:   r.Domain,
					Position: r.Position,
				})
			}
		}

		rankings = append(rankings, entry)
	}

	return rankings
}
