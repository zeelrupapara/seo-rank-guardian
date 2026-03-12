package v1

import (
	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	apperrors "github.com/zeelrupapara/seo-rank-guardian/pkg/errors"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
)

// DashboardStats godoc
// @Summary Get dashboard stats
// @Description Get aggregated dashboard statistics for the authenticated user
// @Tags dashboard
// @Produce json
// @Security BearerAuth
// @Success 200 {object} DashboardStatsResponse
// @Failure 401 {object} ErrorResponse
// @Router /dashboard/stats [get]
func (h *HttpServer) DashboardStats(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	// Get all job IDs for this user
	var jobIDs []uint
	h.DB.Model(&model.Job{}).Where("user_id = ?", userID).Pluck("id", &jobIDs)

	if len(jobIDs) == 0 {
		return httputil.SuccessResponse(c, fiber.StatusOK, DashboardStatsData{}, "Dashboard stats retrieved successfully")
	}

	// Get latest completed run per job
	var latestRunIDs []uint
	h.DB.Raw(`SELECT DISTINCT ON (job_id) id FROM srg_job_runs
		WHERE job_id IN ? AND status IN ('completed','partial')
		ORDER BY job_id, created_at DESC`, jobIDs).Scan(&latestRunIDs)

	if len(latestRunIDs) == 0 {
		return httputil.SuccessResponse(c, fiber.StatusOK, DashboardStatsData{}, "Dashboard stats retrieved successfully")
	}

	var stats DashboardStatsData

	// total_results: count of search results across latest runs
	h.DB.Model(&model.SearchResult{}).Where("run_id IN ?", latestRunIDs).Count(&stats.TotalResults)

	// avg_rank: average position where is_target = true
	h.DB.Model(&model.SearchResult{}).
		Where("run_id IN ? AND is_target = ?", latestRunIDs, true).
		Select("COALESCE(AVG(position), 0)").Scan(&stats.AvgRank)

	// keywords_at_risk: target keywords with position > 10
	h.DB.Model(&model.SearchResult{}).
		Where("run_id IN ? AND is_target = ? AND position > 10", latestRunIDs, true).
		Count(&stats.KeywordsAtRisk)

	// Also count keywords that dropped >= 3 positions
	var droppedCount int64
	h.DB.Model(&model.RankDiff{}).
		Where("run_id IN ? AND delta <= -3", latestRunIDs).
		Count(&droppedCount)

	// Merge: keywords at risk = position > 10 OR dropped >= 3
	// Use the larger count as a simple heuristic (they may overlap)
	if droppedCount > stats.KeywordsAtRisk {
		stats.KeywordsAtRisk = droppedCount
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, stats, "Dashboard stats retrieved successfully")
}
