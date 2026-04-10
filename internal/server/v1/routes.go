package v1

import (
	"time"

	_ "github.com/zeelrupapara/seo-rank-guardian/docs"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/authz"
	swagger "github.com/swaggo/fiber-swagger"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/websocket/v2"
)

func (h *HttpServer) RegisterV1() {
	h.App.Get("/swagger/*", swagger.WrapHandler)

	api := h.App.Group("/api")
	v1 := api.Group("/v1")

	v1.Get("/health", h.Health)

	// Rate limit auth endpoints: 10 requests per minute per IP
	authLimiter := limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * time.Minute,
	})

	// Auth — public (no Protect/Authorize)
	auth := v1.Group("/auth")
	auth.Post("/register", authLimiter, h.Register)
	auth.Post("/login", authLimiter, h.Login)
	auth.Post("/refresh", authLimiter, h.RefreshToken)
	auth.Delete("/logout", h.Middleware.Protect(), h.Logout)
	auth.Get("/google", h.GoogleLogin)
	auth.Get("/google/callback", h.GoogleCallback)

	// Profile — resource: profile
	users := v1.Group("/users", h.Middleware.Protect())
	users.Get("/me", h.Middleware.Authorize(authz.ResourceProfile, authz.ActionRead), h.GetProfile)
	users.Put("/me", h.Middleware.Authorize(authz.ResourceProfile, authz.ActionWrite), h.UpdateProfile)
	users.Put("/me/password", h.Middleware.Authorize(authz.ResourceProfile, authz.ActionWrite), h.ChangePassword)
	users.Post("/me/avatar", h.Middleware.Authorize(authz.ResourceProfile, authz.ActionWrite), h.UploadAvatar)
	users.Delete("/me/avatar", h.Middleware.Authorize(authz.ResourceProfile, authz.ActionWrite), h.RemoveAvatar)

	// Dashboard — resource: dashboard
	dashboard := v1.Group("/dashboard", h.Middleware.Protect())
	dashboard.Get("/stats", h.Middleware.Authorize(authz.ResourceDashboard, authz.ActionRead), h.DashboardStats)

	// Jobs — resource: jobs, runs, reports
	jobs := v1.Group("/jobs", h.Middleware.Protect())
	jobs.Post("/", h.Middleware.Authorize(authz.ResourceJobs, authz.ActionWrite), h.CreateJob)
	jobs.Get("/", h.Middleware.Authorize(authz.ResourceJobs, authz.ActionRead), h.ListJobs)
	jobs.Get("/:jobId", h.Middleware.Authorize(authz.ResourceJobs, authz.ActionRead), h.GetJob)
	jobs.Put("/:jobId", h.Middleware.Authorize(authz.ResourceJobs, authz.ActionWrite), h.UpdateJob)
	jobs.Delete("/:jobId", h.Middleware.Authorize(authz.ResourceJobs, authz.ActionDelete), h.DeleteJob)
	jobs.Post("/:jobId/scrape", h.Middleware.Authorize(authz.ResourceJobs, authz.ActionWrite), h.TriggerScrape)
	jobs.Get("/:jobId/stats", h.Middleware.Authorize(authz.ResourceJobs, authz.ActionRead), h.JobStats)
	jobs.Get("/:jobId/rankings", h.Middleware.Authorize(authz.ResourceRuns, authz.ActionRead), h.LatestRankings)
	jobs.Get("/:jobId/tracking-pairs", h.Middleware.Authorize(authz.ResourceRuns, authz.ActionRead), h.TrackingPairs)
	jobs.Get("/:jobId/trends", h.Middleware.Authorize(authz.ResourceRuns, authz.ActionRead), h.PositionTrends)
	jobs.Get("/:jobId/reports", h.Middleware.Authorize(authz.ResourceReports, authz.ActionRead), h.ListReports)
	jobs.Get("/:jobId/runs", h.Middleware.Authorize(authz.ResourceRuns, authz.ActionRead), h.ListRuns)
	jobs.Get("/:jobId/runs/:runId", h.Middleware.Authorize(authz.ResourceRuns, authz.ActionRead), h.GetRun)
	jobs.Get("/:jobId/runs/:runId/events", h.Middleware.Authorize(authz.ResourceRuns, authz.ActionRead), h.ListRunEvents)
	jobs.Get("/:jobId/runs/:runId/rankings", h.Middleware.Authorize(authz.ResourceRuns, authz.ActionRead), h.RunRankings)
	jobs.Get("/:jobId/runs/:runId/report", h.Middleware.Authorize(authz.ResourceReports, authz.ActionRead), h.GetRunReport)

	// Pair detail — resource: runs
	jobs.Get("/:jobId/pairs/:keyword/:state/summary", h.Middleware.Authorize(authz.ResourceRuns, authz.ActionRead), h.PairSummary)
	jobs.Get("/:jobId/pairs/:keyword/:state/scans", h.Middleware.Authorize(authz.ResourceRuns, authz.ActionRead), h.PairScanHistory)
	jobs.Get("/:jobId/pairs/:keyword/:state/competitors", h.Middleware.Authorize(authz.ResourceRuns, authz.ActionRead), h.PairCompetitors)

	// Admin — resource: users
	admin := v1.Group("/admin", h.Middleware.Protect())
	admin.Get("/stats", h.Middleware.Authorize(authz.ResourceUsers, authz.ActionRead), h.AdminGetStats)

	adminUsers := admin.Group("/users")
	adminUsers.Get("/", h.Middleware.Authorize(authz.ResourceUsers, authz.ActionRead), h.AdminListUsers)
	adminUsers.Get("/:userId", h.Middleware.Authorize(authz.ResourceUsers, authz.ActionRead), h.AdminGetUser)
	adminUsers.Post("/", h.Middleware.Authorize(authz.ResourceUsers, authz.ActionWrite), h.AdminCreateUser)
	adminUsers.Put("/:userId/role", h.Middleware.Authorize(authz.ResourceUsers, authz.ActionWrite), h.AdminUpdateUserRole)
	adminUsers.Put("/:userId/status", h.Middleware.Authorize(authz.ResourceUsers, authz.ActionWrite), h.AdminUpdateUserStatus)
	adminUsers.Delete("/:userId", h.Middleware.Authorize(authz.ResourceUsers, authz.ActionDelete), h.AdminDeleteUser)

	adminJobs := admin.Group("/jobs")
	adminJobs.Get("/", h.Middleware.Authorize(authz.ResourceUsers, authz.ActionRead), h.AdminListJobs)
	adminJobs.Get("/:jobId", h.Middleware.Authorize(authz.ResourceUsers, authz.ActionRead), h.AdminGetJob)

	// Policy management — resource: policies
	admin.Get("/policies", h.Middleware.Authorize(authz.ResourcePolicies, authz.ActionRead), h.AdminListPolicies)
	admin.Post("/policies", h.Middleware.Authorize(authz.ResourcePolicies, authz.ActionWrite), h.AdminAddPolicy)
	admin.Delete("/policies", h.Middleware.Authorize(authz.ResourcePolicies, authz.ActionDelete), h.AdminRemovePolicy)
	admin.Get("/resources", h.Middleware.Authorize(authz.ResourcePolicies, authz.ActionRead), h.AdminListResources)
	admin.Get("/roles", h.Middleware.Authorize(authz.ResourcePolicies, authz.ActionRead), h.AdminListRoles)

	// Static file serving for avatars
	h.App.Static("/uploads", "./uploads")

	// WebSocket endpoint (auth via query param)
	v1.Use("/ws", h.WebSocketUpgrade)
	v1.Get("/ws", websocket.New(h.ServeWS))
}
