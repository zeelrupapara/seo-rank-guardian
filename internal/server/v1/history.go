package v1

import (
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
	RunID    uint      `json:"run_id"`
	RunDate  time.Time `json:"run_date"`
	Position int       `json:"position"`
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
// @Description Get rankings from the most recent completed run
// @Tags jobs
// @Produce json
// @Security BearerAuth
// @Param jobId path int true "Job ID"
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
	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"run_id":   latestRun.ID,
		"rankings": rankings,
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

	// Get recent completed runs
	var runs []model.JobRun
	h.DB.Where("job_id = ? AND status IN ?", jobID, []string{"completed", "partial"}).
		Order("created_at DESC").Limit(limit).Find(&runs)

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

// --- Helper ---

func (h *HttpServer) buildRankings(runID, jobID uint) []RankingEntry {
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
