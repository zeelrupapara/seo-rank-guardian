package v1

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/model"
)

const (
	botDetectionCacheKey = "bot_detection_rules:active"
	botDetectionCacheTTL = 5 * time.Minute

	// cdpLeakHeader is injected by Chrome DevTools / headless automation frameworks.
	cdpLeakHeader = "X-DevTools-Emulate-Network-Conditions-Client-Id"
)

// BotDetectionMiddleware inspects request headers to identify automated scripts and
// headless browsers before any authentication or route handling occurs.
//
// Two-layer evaluation:
//  1. Static checks (always-on, no DB round-trip):
//     - Missing User-Agent
//     - Missing Accept header
//     - CDP leak header present (headless Chrome DevTools)
//     - HeadlessChrome substring in User-Agent
//
//  2. DB-backed rules (cached in Redis, 5 min TTL):
//     - user_agent:    case-insensitive substring match against User-Agent value
//     - absent_header: block if the named header is absent
//     - allow rules:   whitelist — allow beats block when both match the same request
//
// Fail-open: Redis/DB errors let the request through.
func (h *HttpServer) BotDetectionMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// --- Static checks ---
		ua := c.Get("User-Agent")
		if ua == "" {
			return botForbidden(c)
		}
		if c.Get("Accept") == "" {
			return botForbidden(c)
		}
		if c.Get(cdpLeakHeader) != "" {
			return botForbidden(c)
		}
		if strings.Contains(strings.ToLower(ua), "headlesschrome") {
			return botForbidden(c)
		}

		// --- DB-backed checks ---
		rules, err := h.loadBotDetectionRules()
		if err != nil || len(rules) == 0 {
			return c.Next()
		}

		isAllowed := false
		isBlocked := false

		for _, rule := range rules {
			if !matchesBotRule(rule, c) {
				continue
			}
			switch rule.Type {
			case "allow":
				isAllowed = true
			case "block":
				isBlocked = true
			}
		}

		if isAllowed {
			return c.Next()
		}
		if isBlocked {
			return botForbidden(c)
		}

		return c.Next()
	}
}

// matchesBotRule returns true if the request matches the given rule's criteria.
func matchesBotRule(rule model.BotDetectionRule, c *fiber.Ctx) bool {
	switch rule.MatchField {
	case "user_agent":
		ua := strings.ToLower(c.Get("User-Agent"))
		return strings.Contains(ua, strings.ToLower(rule.Pattern))
	case "absent_header":
		return c.Get(rule.Pattern) == ""
	}
	return false
}

// loadBotDetectionRules returns all enabled bot detection rules.
// Tries Redis first; falls back to DB on miss. Fail-open on any error.
func (h *HttpServer) loadBotDetectionRules() ([]model.BotDetectionRule, error) {
	ctx := context.Background()

	var rules []model.BotDetectionRule
	if err := h.Cache.Get(ctx, botDetectionCacheKey, &rules); err == nil {
		return rules, nil
	}

	if err := h.DB.Where("is_enabled = ?", true).Find(&rules).Error; err != nil {
		h.Log.Warnf("bot detection rules load failed: %v", err)
		return nil, err
	}

	if err := h.Cache.Set(ctx, botDetectionCacheKey, rules, botDetectionCacheTTL); err != nil {
		h.Log.Warnf("bot detection cache set failed: %v", err)
	}

	return rules, nil
}

// bustBotDetectionCache deletes the Redis key so the middleware reloads on next request.
func (h *HttpServer) bustBotDetectionCache() {
	if err := h.Cache.Delete(context.Background(), botDetectionCacheKey); err != nil {
		h.Log.Warnf("bot detection cache bust failed: %v", err)
	}
}

func botForbidden(c *fiber.Ctx) error {
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"success": false,
		"message": "Forbidden",
	})
}
