package middleware

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
)

func (m *Middleware) Authorize(obj, act string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, ok := c.Locals("userRole").(string)
		if !ok || role == "" {
			return httputil.ErrorResponse(c, fiber.StatusForbidden, "no role found", "Forbidden")
		}

		allowed, err := m.Authz.Enforce(role, obj, act)
		if err != nil {
			m.Log.Errorf("Casbin enforce error: %v", err)
			return httputil.ErrorResponse(c, fiber.StatusInternalServerError, "authorization error", "Internal error")
		}

		if !allowed {
			return httputil.ErrorResponse(c, fiber.StatusForbidden, fmt.Sprintf("access denied for %s %s", act, obj), "Forbidden")
		}

		return c.Next()
	}
}
