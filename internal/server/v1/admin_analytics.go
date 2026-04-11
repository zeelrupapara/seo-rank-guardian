package v1

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
)

const (
	analyticsCacheKey = "analytics:summary"
	analyticsCacheTTL = 5 * time.Minute
)

type analyticsData struct {
	RequestsOverTime    []requestsOverTimePoint `json:"requests_over_time"`
	TopUsers            []topUserRow            `json:"top_users"`
	SlowestEndpoints    []endpointLatencyRow    `json:"slowest_endpoints"`
	ErrorRateByEndpoint []endpointErrorRateRow  `json:"error_rate_by_endpoint"`
	RequestsPerHour     []hourlyCountRow        `json:"requests_per_hour"`
}

type requestsOverTimePoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

type topUserRow struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Count    int64  `json:"count"`
}

type endpointLatencyRow struct {
	Route        string  `json:"route"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	Count        int64   `json:"count"`
}

type endpointErrorRateRow struct {
	Route     string  `json:"route"`
	Total     int64   `json:"total"`
	Errors    int64   `json:"errors"`
	ErrorRate float64 `json:"error_rate"`
}

type hourlyCountRow struct {
	Hour  int   `json:"hour"`
	Count int64 `json:"count"`
}

// AdminGetAnalytics godoc
// @Summary Usage analytics summary
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Success 200
// @Router /admin/analytics [get]
func (h *HttpServer) AdminGetAnalytics(c *fiber.Ctx) error {
	ctx := context.Background()

	var data analyticsData
	if err := h.Cache.Get(ctx, analyticsCacheKey, &data); err == nil {
		return httputil.SuccessResponse(c, fiber.StatusOK, data, "Analytics retrieved")
	}

	data = h.buildAnalytics()

	if err := h.Cache.Set(ctx, analyticsCacheKey, data, analyticsCacheTTL); err != nil {
		h.Log.Warnf("analytics cache set failed: %v", err)
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, data, "Analytics retrieved")
}

func (h *HttpServer) buildAnalytics() analyticsData {
	return analyticsData{
		RequestsOverTime:    h.analyticsRequestsOverTime(),
		TopUsers:            h.analyticsTopUsers(),
		SlowestEndpoints:    h.analyticsSlowestEndpoints(),
		ErrorRateByEndpoint: h.analyticsErrorRateByEndpoint(),
		RequestsPerHour:     h.analyticsRequestsPerHour(),
	}
}

// analyticsRequestsOverTime returns daily request counts for the last 7 days.
func (h *HttpServer) analyticsRequestsOverTime() []requestsOverTimePoint {
	points := make([]requestsOverTimePoint, 7)
	now := time.Now()
	for i := 6; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		dayStart := day.Truncate(24 * time.Hour).UnixNano()
		dayEnd := day.Truncate(24*time.Hour).Add(24*time.Hour).UnixNano() - 1
		var count int64
		h.DB.Table("srg_request_logs").
			Where("created_at >= ? AND created_at <= ?", dayStart, dayEnd).
			Count(&count)
		points[6-i] = requestsOverTimePoint{
			Date:  day.Format("Jan 02"),
			Count: count,
		}
	}
	return points
}

// analyticsTopUsers returns the top 10 users by request count (excludes unauthenticated).
func (h *HttpServer) analyticsTopUsers() []topUserRow {
	type row struct {
		UserID   uint
		Username string
		Count    int64
	}
	var rows []row
	h.DB.Table("srg_request_logs").
		Select("srg_request_logs.user_id, srg_users.username, COUNT(*) as count").
		Joins("LEFT JOIN srg_users ON srg_users.id = srg_request_logs.user_id").
		Where("srg_request_logs.user_id != 0").
		Group("srg_request_logs.user_id, srg_users.username").
		Order("count DESC").
		Limit(10).
		Scan(&rows)

	out := make([]topUserRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, topUserRow{UserID: r.UserID, Username: r.Username, Count: r.Count})
	}
	return out
}

// analyticsSlowestEndpoints returns the 10 routes with the highest average latency.
func (h *HttpServer) analyticsSlowestEndpoints() []endpointLatencyRow {
	type row struct {
		Route        string
		AvgLatencyMs float64
		Count        int64
	}
	var rows []row
	h.DB.Table("srg_request_logs").
		Select("route_pattern as route, AVG(latency_ms) as avg_latency_ms, COUNT(*) as count").
		Group("route_pattern").
		Order("avg_latency_ms DESC").
		Limit(10).
		Scan(&rows)

	out := make([]endpointLatencyRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, endpointLatencyRow{Route: r.Route, AvgLatencyMs: r.AvgLatencyMs, Count: r.Count})
	}
	return out
}

// analyticsErrorRateByEndpoint returns error rates for routes with at least 5 requests.
func (h *HttpServer) analyticsErrorRateByEndpoint() []endpointErrorRateRow {
	type row struct {
		Route  string
		Total  int64
		Errors int64
	}
	var rows []row
	h.DB.Table("srg_request_logs").
		Select("route_pattern as route, COUNT(*) as total, SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) as errors").
		Group("route_pattern").
		Having("COUNT(*) >= 5").
		Order("errors DESC").
		Scan(&rows)

	out := make([]endpointErrorRateRow, 0, len(rows))
	for _, r := range rows {
		var rate float64
		if r.Total > 0 {
			rate = float64(r.Errors) / float64(r.Total) * 100
		}
		out = append(out, endpointErrorRateRow{
			Route:     r.Route,
			Total:     r.Total,
			Errors:    r.Errors,
			ErrorRate: rate,
		})
	}
	return out
}

// analyticsRequestsPerHour returns request counts bucketed by hour-of-day (0–23)
// across the last 7 days. Missing hours are zero-filled.
func (h *HttpServer) analyticsRequestsPerHour() []hourlyCountRow {
	sevenDaysAgo := time.Now().AddDate(0, 0, -7).UnixNano()

	type row struct {
		Hour  int
		Count int64
	}
	var rows []row
	h.DB.Table("srg_request_logs").
		Select("EXTRACT(HOUR FROM to_timestamp(created_at / 1000000000.0))::int AS hour, COUNT(*) as count").
		Where("created_at >= ?", sevenDaysAgo).
		Group("hour").
		Order("hour").
		Scan(&rows)

	// Build a lookup map and zero-fill all 24 hours
	byHour := make(map[int]int64, len(rows))
	for _, r := range rows {
		byHour[r.Hour] = r.Count
	}

	out := make([]hourlyCountRow, 24)
	for h := 0; h < 24; h++ {
		out[h] = hourlyCountRow{Hour: h, Count: byHour[h]}
	}
	return out
}
