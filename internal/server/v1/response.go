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
