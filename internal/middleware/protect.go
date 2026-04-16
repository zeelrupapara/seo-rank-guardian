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

		// Check if an admin has force-revoked this session.
		// Fail-open on Redis errors: a temporary Redis outage should not log everyone out.
		// The 15-minute access token TTL is the ultimate backstop.
		revoked, err := m.OAuth2.IsSessionRevoked(claims.SessionID)
		if err != nil {
			m.Log.Warnf("Redis revocation check failed for session %s: %v", claims.SessionID, err)
		} else if revoked {
			return httputil.ErrorResponse(c, fiber.StatusUnauthorized, "session_revoked", "Session has been revoked")
		}

		c.Locals("userId", claims.UserID)
		c.Locals("userRole", claims.Role)
		c.Locals("sessionId", claims.SessionID)

		return c.Next()
	}
}
