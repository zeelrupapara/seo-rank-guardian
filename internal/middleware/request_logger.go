package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

func (m *Middleware) RequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		m.Log.Infow("HTTP request",
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"latency", time.Since(start).String(),
			"ip", c.IP(),
		)

		return err
	}
}
