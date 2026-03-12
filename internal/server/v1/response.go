package v1

import (
	"github.com/zeelrupapara/seo-rank-guardian/model"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/oauth2"
)

// --- Error Response ---

// ErrorResponse is the standard error response
type ErrorResponse struct {
	Success bool   `json:"success" example:"false"`
	Code    int    `json:"code" example:"400"`
	Error   string `json:"error" example:"bad request"`
	Message string `json:"message" example:"Something went wrong"`
}

// --- Auth Responses ---

type AuthData struct {
	User   model.User       `json:"user"`
	Tokens *oauth2.TokenPair `json:"tokens"`
}

// AuthResponse is returned by register, login, google callback
type AuthResponse struct {
	Success bool     `json:"success" example:"true"`
	Code    int      `json:"code" example:"200"`
	Data    AuthData `json:"data"`
	Message string   `json:"message" example:"Login successful"`
}

// TokenResponse is returned by refresh token
type TokenResponse struct {
	Success bool             `json:"success" example:"true"`
	Code    int              `json:"code" example:"200"`
	Data    *oauth2.TokenPair `json:"data"`
	Message string           `json:"message" example:"Token refreshed successfully"`
}

// MessageResponse is returned by endpoints with no data (logout, delete)
type MessageResponse struct {
	Success bool   `json:"success" example:"true"`
	Code    int    `json:"code" example:"200"`
	Message string `json:"message" example:"Operation successful"`
}

// --- User Responses ---

// UserResponse is returned by get/update profile
type UserResponse struct {
	Success bool       `json:"success" example:"true"`
	Code    int        `json:"code" example:"200"`
	Data    model.User `json:"data"`
	Message string     `json:"message" example:"Profile retrieved"`
}

// --- Health Response ---

type HealthData struct {
	Status string `json:"status" example:"ok"`
}

type HealthResponse struct {
	Success bool       `json:"success" example:"true"`
	Code    int        `json:"code" example:"200"`
	Data    HealthData `json:"data"`
	Message string     `json:"message" example:"Service is healthy"`
}

// --- Job Responses ---

// JobResponse is returned by create/update job
type JobResponse struct {
	Success bool      `json:"success" example:"true"`
	Code    int       `json:"code" example:"200"`
	Data    model.Job `json:"data"`
	Message string    `json:"message" example:"Job created successfully"`
}

// JobListResponse is returned by list jobs
type JobListResponse struct {
	Success bool        `json:"success" example:"true"`
	Code    int         `json:"code" example:"200"`
	Data    []model.Job `json:"data"`
	Message string      `json:"message" example:"Jobs retrieved successfully"`
}

type JobDetailData struct {
	Job       model.Job    `json:"job"`
	TotalRuns int64        `json:"total_runs" example:"5"`
	LastRun   model.JobRun `json:"last_run"`
}

// JobDetailResponse is returned by get job
type JobDetailResponse struct {
	Success bool          `json:"success" example:"true"`
	Code    int           `json:"code" example:"200"`
	Data    JobDetailData `json:"data"`
	Message string        `json:"message" example:"Job retrieved successfully"`
}

// --- Scrape Response ---

type ScrapeData struct {
	Run        model.JobRun `json:"run"`
	TotalPairs int          `json:"total_pairs" example:"4"`
}

type ScrapeResponse struct {
	Success bool       `json:"success" example:"true"`
	Code    int        `json:"code" example:"201"`
	Data    ScrapeData `json:"data"`
	Message string     `json:"message" example:"Scrape triggered successfully"`
}

// --- Run Responses ---

// RunListResponse is returned by list runs
type RunListResponse struct {
	Success bool           `json:"success" example:"true"`
	Code    int            `json:"code" example:"200"`
	Data    []model.JobRun `json:"data"`
	Message string         `json:"message" example:"Runs retrieved successfully"`
}

type RunDetailData struct {
	Run     model.JobRun        `json:"run"`
	Pairs   []model.SearchPair  `json:"pairs"`
	Results []model.SearchResult `json:"results"`
	Diffs   []model.RankDiff    `json:"diffs"`
	Report  model.Report        `json:"report"`
}

// RunDetailResponse is returned by get run
type RunDetailResponse struct {
	Success bool          `json:"success" example:"true"`
	Code    int           `json:"code" example:"200"`
	Data    RunDetailData `json:"data"`
	Message string        `json:"message" example:"Run details retrieved successfully"`
}

// --- Events Response ---

type EventsResponse struct {
	Success bool                `json:"success" example:"true"`
	Code    int                 `json:"code" example:"200"`
	Data    []model.RunEventLog `json:"data"`
	Message string              `json:"message" example:"Events retrieved successfully"`
}

// --- Rankings Response ---

type RankingsResponse struct {
	Success bool           `json:"success" example:"true"`
	Code    int            `json:"code" example:"200"`
	Data    []RankingEntry `json:"data"`
	Message string         `json:"message" example:"Rankings retrieved successfully"`
}

// --- Report Response ---

type ReportResponse struct {
	Success bool         `json:"success" example:"true"`
	Code    int          `json:"code" example:"200"`
	Data    model.Report `json:"data"`
	Message string       `json:"message" example:"Report retrieved successfully"`
}

// --- Trends Response ---

type TrendsResponse struct {
	Success bool         `json:"success" example:"true"`
	Code    int          `json:"code" example:"200"`
	Data    []TrendPoint `json:"data"`
	Message string       `json:"message" example:"Trends retrieved successfully"`
}

// --- Dashboard Response ---

type DashboardStatsData struct {
	TotalResults   int64   `json:"total_results"`
	AvgRank        float64 `json:"avg_rank"`
	KeywordsAtRisk int64   `json:"keywords_at_risk"`
}

type DashboardStatsResponse struct {
	Success bool               `json:"success" example:"true"`
	Code    int                `json:"code" example:"200"`
	Data    DashboardStatsData `json:"data"`
	Message string             `json:"message" example:"Dashboard stats retrieved successfully"`
}

// --- Enriched Job List Types ---

type DomainInfo struct {
	Domain     string `json:"domain"`
	FaviconURL string `json:"favicon_url"`
}

type JobRunBrief struct {
	ID          uint   `json:"id"`
	Status      string `json:"status"`
	CompletedAt *int64 `json:"completed_at"`
	CreatedAt   int64  `json:"created_at"`
}

type JobListItem struct {
	model.Job
	KeywordCount       int          `json:"keyword_count"`
	RegionCount        int          `json:"region_count"`
	LastRun            *JobRunBrief `json:"last_run"`
	HealthScore        *int         `json:"health_score"`
	FaviconURL         string       `json:"favicon_url"`
	CompetitorFavicons []DomainInfo `json:"competitor_favicons"`
}

// --- Job Stats Response ---

type JobStatsData struct {
	HealthScore     *int         `json:"health_score"`
	Top3Rankings    int64        `json:"top_3_rankings"`
	Top3Change      int64        `json:"top_3_change"`
	VisibilityIndex float64      `json:"visibility_index"`
	TotalKeywords   int          `json:"total_keywords"`
	Competitors     []DomainInfo `json:"competitors"`
	RunID           *uint        `json:"run_id"`
}

type JobStatsResponse struct {
	Success bool         `json:"success" example:"true"`
	Code    int          `json:"code" example:"200"`
	Data    JobStatsData `json:"data"`
	Message string       `json:"message" example:"Job stats retrieved successfully"`
}

// --- Report List Response ---

type ReportListItem struct {
	ID          uint   `json:"id"`
	RunID       uint   `json:"run_id"`
	Status      string `json:"status"`
	HealthScore *int   `json:"health_score"`
	Provider    string `json:"provider"`
	CreatedAt   int64  `json:"created_at"`
}

// --- Run Metrics ---

type RunMetrics struct {
	AvgResponseTimeMs *float64 `json:"avg_response_time_ms"`
	CompletedPairs    int      `json:"completed_pairs"`
	FailedPairs       int      `json:"failed_pairs"`
	TotalPairs        int      `json:"total_pairs"`
}

// --- Pair Detail Response Types ---

type PairSummaryData struct {
	Keyword         string  `json:"keyword"`
	State           string  `json:"state"`
	CurrentPosition int     `json:"current_position"`
	Change          int     `json:"change"`
	AvgPosition     float64 `json:"avg_position"`
	BestPosition    int     `json:"best_position"`
	WorstPosition   int     `json:"worst_position"`
	TotalScans      int     `json:"total_scans"`
	RunID           uint    `json:"run_id"`
}

type PairScanItem struct {
	RunID       uint   `json:"run_id"`
	ScanNumber  int    `json:"scan_number"`
	Position    int    `json:"position"`
	Change      int    `json:"change"`
	Status      string `json:"status"`
	ResultCount int    `json:"result_count"`
	DurationNs  int64  `json:"duration_ns"`
	HasReport   bool   `json:"has_report"`
	CreatedAt   int64  `json:"created_at"`
}

type PairCompetitorData struct {
	Domain      string  `json:"domain"`
	FaviconURL  string  `json:"favicon_url"`
	IsTarget    bool    `json:"is_target"`
	Position    int     `json:"current_position"`
	AvgPosition float64 `json:"avg_position"`
	Change      int     `json:"change"`
}
