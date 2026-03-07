package middleware

import (
	"github.com/gofiber/fiber/v2"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
	"github.com/zeelrupapara/seo-rank-guardian/utils"
)

func (m *Middleware) Protect() fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := utils.GetTokenFromHeader(c.Get("Authorization"))
		if token == "" {
			return httputil.ErrorResponse(c, fiber.StatusUnauthorized, "missing token", "Unauthorized")
		}

		claims, err := m.OAuth2.ValidateAccessToken(token)
		if err != nil {
			return httputil.ErrorResponse(c, fiber.StatusUnauthorized, "invalid token", "Unauthorized")
		}

		c.Locals("userId", claims.UserID)
		c.Locals("userRole", claims.Role)

		return c.Next()
	}
}
