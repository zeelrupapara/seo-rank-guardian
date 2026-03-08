package v1

import (
	"github.com/gofiber/fiber/v2"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
)

// Health godoc
// @Summary Health check
// @Description Check if the service is running
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func (h *HttpServer) Health(c *fiber.Ctx) error {
	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{"status": "ok"}, "Service is healthy")
}
