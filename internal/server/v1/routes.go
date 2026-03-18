package v1

import (
	"time"

	_ "github.com/zeelrupapara/seo-rank-guardian/docs"
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

	auth := v1.Group("/auth")
	auth.Post("/register", authLimiter, h.Register)
	auth.Post("/login", authLimiter, h.Login)
	auth.Post("/refresh", authLimiter, h.RefreshToken)
	auth.Delete("/logout", h.Middleware.Protect(), h.Logout)
	auth.Get("/google", h.GoogleLogin)
	auth.Get("/google/callback", h.GoogleCallback)

	users := v1.Group("/users", h.Middleware.Protect())
	users.Get("/me", h.GetProfile)
	users.Put("/me", h.UpdateProfile)
	users.Put("/me/password", h.ChangePassword)
	users.Post("/me/avatar", h.UploadAvatar)
	users.Delete("/me/avatar", h.RemoveAvatar)

	dashboard := v1.Group("/dashboard", h.Middleware.Protect())
	dashboard.Get("/stats", h.DashboardStats)

	jobs := v1.Group("/jobs", h.Middleware.Protect())
	jobs.Post("/", h.CreateJob)
	jobs.Get("/", h.ListJobs)
	jobs.Get("/:jobId", h.GetJob)
	jobs.Put("/:jobId", h.UpdateJob)
	jobs.Delete("/:jobId", h.DeleteJob)
	jobs.Post("/:jobId/scrape", h.TriggerScrape)
	jobs.Get("/:jobId/stats", h.JobStats)
	jobs.Get("/:jobId/rankings", h.LatestRankings)
	jobs.Get("/:jobId/tracking-pairs", h.TrackingPairs)
	jobs.Get("/:jobId/trends", h.PositionTrends)
	jobs.Get("/:jobId/reports", h.ListReports)
	jobs.Get("/:jobId/runs", h.ListRuns)
	jobs.Get("/:jobId/runs/:runId", h.GetRun)
	jobs.Get("/:jobId/runs/:runId/events", h.ListRunEvents)
	jobs.Get("/:jobId/runs/:runId/rankings", h.RunRankings)
	jobs.Get("/:jobId/runs/:runId/report", h.GetRunReport)

	// Pair detail endpoints
	jobs.Get("/:jobId/pairs/:keyword/:state/summary", h.PairSummary)
	jobs.Get("/:jobId/pairs/:keyword/:state/scans", h.PairScanHistory)
	jobs.Get("/:jobId/pairs/:keyword/:state/competitors", h.PairCompetitors)

	// Static file serving for avatars
	h.App.Static("/uploads", "./uploads")

	// WebSocket endpoint (auth via query param)
	v1.Use("/ws", h.WebSocketUpgrade)
	v1.Get("/ws", websocket.New(h.ServeWS))
}
