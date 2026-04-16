package v1

import (
	"context"
	"net"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/model"
)

const (
	ipFilterCacheKey = "ip_filters:active"
	ipFilterCacheTTL = 5 * time.Minute
)

// IPFilterMiddleware checks the client IP against admin-defined allow/block rules before any
// authentication or route handling occurs. Rules are cached in Redis (5 min TTL).
//
// Evaluation logic:
//   - No rules → allow all (fail-open, backward-compatible)
//   - Only block rules → allow all except matched IPs
//   - Any allow rule present → allowlist mode: only matched IPs pass
//   - Block always beats allow when both match the same IP
func (h *HttpServer) IPFilterMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		rules, err := h.loadIPFilterRules()
		if err != nil || len(rules) == 0 {
			return c.Next()
		}

		clientIP := net.ParseIP(c.IP())
		if clientIP == nil {
			return c.Next()
		}

		hasAllowRules := false
		isAllowed := false
		isBlocked := false

		for _, rule := range rules {
			if rule.Type == "allow" {
				hasAllowRules = true
			}
			if ipRangeContains(rule.IPRange, clientIP) {
				if rule.Type == "block" {
					isBlocked = true
				}
				if rule.Type == "allow" {
					isAllowed = true
				}
			}
		}

		if isBlocked || (hasAllowRules && !isAllowed) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": "Forbidden",
			})
		}
		return c.Next()
	}
}

// loadIPFilterRules returns all enabled IP filter rules.
// Tries Redis first; falls back to DB on miss. Fail-open on any error.
func (h *HttpServer) loadIPFilterRules() ([]model.IPFilter, error) {
	ctx := context.Background()

	var rules []model.IPFilter
	if err := h.Cache.Get(ctx, ipFilterCacheKey, &rules); err == nil {
		return rules, nil
	}

	// Cache miss — load enabled rules from DB
	if err := h.DB.Where("is_enabled = ?", true).Find(&rules).Error; err != nil {
		h.Log.Warnf("ip filter rules load failed: %v", err)
		return nil, err
	}

	// Repopulate cache (non-fatal on error)
	if err := h.Cache.Set(ctx, ipFilterCacheKey, rules, ipFilterCacheTTL); err != nil {
		h.Log.Warnf("ip filter cache set failed: %v", err)
	}

	return rules, nil
}

// ipRangeContains returns true if ipRange (plain IP or CIDR) contains clientIP.
func ipRangeContains(ipRange string, clientIP net.IP) bool {
	if _, ipNet, err := net.ParseCIDR(ipRange); err == nil {
		return ipNet.Contains(clientIP)
	}
	if parsed := net.ParseIP(ipRange); parsed != nil {
		return parsed.Equal(clientIP)
	}
	return false
}
