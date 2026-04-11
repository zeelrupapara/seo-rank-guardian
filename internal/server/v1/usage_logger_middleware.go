package v1

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/model"
)

// UsageLoggerMiddleware records every API request into srg_request_logs after the
// response has been sent. The DB write happens in a goroutine so it never adds
// latency to the response.
//
// Skipped paths:
//   - /api/v1/health  — noise, not useful for analytics
//   - /swagger/       — documentation, not real API usage
//   - /uploads/       — static files
func (h *HttpServer) UsageLoggerMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		path := c.Path()
		if path == "/api/v1/health" ||
			strings.HasPrefix(path, "/swagger/") ||
			strings.HasPrefix(path, "/uploads/") {
			return err
		}

		latencyMs := time.Since(start).Milliseconds()
		routePattern := c.Route().Path
		statusCode := c.Response().StatusCode()
		userID, _ := c.Locals("userId").(uint)
		ip := c.IP()
		method := c.Method()

		go func() {
			entry := model.RequestLog{
				Method:       method,
				RoutePattern: routePattern,
				StatusCode:   statusCode,
				LatencyMs:    latencyMs,
				UserID:       userID,
				IPAddress:    ip,
			}
			if dbErr := h.DB.Create(&entry).Error; dbErr != nil {
				h.Log.Warnf("request log write failed: %v", dbErr)
			}
		}()

		return err
	}
}
