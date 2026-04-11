package v1

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/model"
)

const (
	rateLimitCacheKey = "rate_limits:active"
	rateLimitCacheTTL = 5 * time.Minute
)

// RateLimitMiddleware checks the authenticated user's role against admin-configured
// rate limit rules. Must be placed after Protect() so userRole is available.
//
// Rules are evaluated per (role, endpoint) pair:
//   - endpoint = "*" matches all routes for that role
//   - endpoint = Fiber route pattern (e.g. "/api/v1/jobs/:jobId/scrape") matches exactly
//
// All matching rules must pass. Fail-open on Redis errors.
func (h *HttpServer) RateLimitMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("userRole").(string)
		if role == "" {
			return c.Next() // Protect() hasn't run (should not happen in auth groups)
		}

		rules, err := h.loadRateLimitRules()
		if err != nil || len(rules) == 0 {
			return c.Next()
		}

		routePath := c.Route().Path // e.g. "/api/v1/jobs/:jobId/scrape"
		ctx := context.Background()

		for _, rule := range rules {
			if !rule.IsEnabled {
				continue
			}
			if rule.Role != role {
				continue
			}
			if rule.Endpoint != "*" && rule.Endpoint != routePath {
				continue
			}

			// Rule matches — increment Redis counter for this window
			windowBucket := time.Now().Unix() / int64(rule.WindowSeconds)
			key := fmt.Sprintf("ratelimit:%d:%d", rule.ID, windowBucket)

			count, err := h.Cache.IncrWithExpiry(ctx, key, time.Duration(rule.WindowSeconds)*time.Second)
			if err != nil {
				h.Log.Warnf("rate limit counter error (rule %d): %v", rule.ID, err)
				continue // fail-open
			}

			remaining := rule.MaxRequests - int(count)
			if remaining < 0 {
				remaining = 0
			}
			c.Set("X-RateLimit-Limit", strconv.Itoa(rule.MaxRequests))
			c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if int(count) > rule.MaxRequests {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
					"success": false,
					"message": "Rate limit exceeded. Try again later.",
				})
			}
		}
		return c.Next()
	}
}

// loadRateLimitRules returns all enabled rate limit rules, using Redis as a cache.
func (h *HttpServer) loadRateLimitRules() ([]model.RateLimit, error) {
	ctx := context.Background()

	var rules []model.RateLimit
	if err := h.Cache.Get(ctx, rateLimitCacheKey, &rules); err == nil {
		return rules, nil
	}

	// Cache miss — load enabled rules from DB
	if err := h.DB.Where("is_enabled = ?", true).Find(&rules).Error; err != nil {
		h.Log.Warnf("rate limit rules load failed: %v", err)
		return nil, err
	}

	// Repopulate cache (non-fatal on error)
	if err := h.Cache.Set(ctx, rateLimitCacheKey, rules, rateLimitCacheTTL); err != nil {
		h.Log.Warnf("rate limit cache set failed: %v", err)
	}

	return rules, nil
}

// bustRateLimitCache invalidates the cached rule list so the middleware reloads on next request.
func (h *HttpServer) bustRateLimitCache() {
	if err := h.Cache.Delete(context.Background(), rateLimitCacheKey); err != nil {
		h.Log.Warnf("rate limit cache bust failed: %v", err)
	}
}
