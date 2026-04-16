package v1

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/model"
)

const (
	autoBlockActiveFmt   = "auto_block:active:%s"
	autoBlockViolFmt     = "auto_block:violations:%s"
	autoBlockL1CountFmt  = "auto_block:l1_count:%s"
	autoBlockL2CountFmt  = "auto_block:l2_count:%s"
	autoBlockPolicyCacheKey = "auto_block:policy"
	autoBlockPolicyCacheTTL = 5 * time.Minute
	autoBlockViolTTL        = 24 * time.Hour
	autoBlockCountTTL       = 90 * 24 * time.Hour // 90 days
)

type autoBlockStatus struct {
	Level        int   `json:"level"`
	BlockedUntil int64 `json:"blocked_until"` // unix timestamp
	IsPermanent  bool  `json:"is_permanent"`
}

// AutoIPBlockMiddleware checks whether the client IP is currently under an automatic
// block (triggered by accumulated rate-limit violations). Runs early — right after
// IPFilterMiddleware — so blocked IPs never reach auth or business logic.
func (h *HttpServer) AutoIPBlockMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Admin routes and auth routes are always exempt — prevents admin lockout
		// when admin and user share the same IP address.
		path := c.Path()
		if strings.HasPrefix(path, "/api/v1/admin") || strings.HasPrefix(path, "/api/v1/auth") {
			return c.Next()
		}

		ip := c.IP()
		if ip == "" {
			return c.Next()
		}

		ctx := context.Background()
		key := fmt.Sprintf(autoBlockActiveFmt, ip)

		var status autoBlockStatus
		if err := h.Cache.Get(ctx, key, &status); err != nil {
			// Key missing or Redis error → not blocked
			return c.Next()
		}

		// Key exists — IP is blocked
		if status.IsPermanent {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": "Your IP has been permanently blocked. Contact support to appeal.",
			})
		}
		blockedUntil := time.Unix(status.BlockedUntil, 0)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Your IP has been automatically blocked (level %d). Try again after %s.",
				status.Level, blockedUntil.UTC().Format(time.RFC3339)),
		})
	}
}

// triggerAutoBlock is called (in a goroutine) by RateLimitMiddleware when a 429 is
// returned. It decides whether to auto-block the IP based on the configured policy
// and escalation counters.
func (h *HttpServer) triggerAutoBlock(ip string) {
	ctx := context.Background()

	// 1. Load policy (fail-open: if no policy or disabled, do nothing)
	policy, err := h.loadIPBlockPolicy(ctx)
	if err != nil || policy == nil || !policy.IsEnabled {
		return
	}

	// 2. Increment violation counter for this IP (24h window)
	violKey := fmt.Sprintf(autoBlockViolFmt, ip)
	violCount, err := h.Cache.IncrWithExpiry(ctx, violKey, autoBlockViolTTL)
	if err != nil {
		h.Log.Warnf("auto_block: violation incr error for %s: %v", ip, err)
		return
	}

	// 3. Not enough violations yet
	if int(violCount) < policy.L1ViolationThreshold {
		return
	}

	// 4. Read escalation counters
	l1Key := fmt.Sprintf(autoBlockL1CountFmt, ip)
	l2Key := fmt.Sprintf(autoBlockL2CountFmt, ip)

	l1Count, _ := h.Cache.GetInt(ctx, l1Key)
	l2Count, _ := h.Cache.GetInt(ctx, l2Key)

	// 5. Determine block level and duration
	var blockLevel int
	var blockDuration time.Duration

	switch {
	case int(l2Count) >= policy.L3ThresholdBlocks:
		blockLevel = 3
		blockDuration = time.Duration(policy.L3BlockSeconds) * time.Second
	case int(l1Count) >= policy.L2ThresholdBlocks:
		blockLevel = 2
		blockDuration = time.Duration(policy.L2BlockSeconds) * time.Second
		// Escalating to L2: increment l2 counter, reset l1 counter
		if _, err := h.Cache.IncrWithExpiry(ctx, l2Key, autoBlockCountTTL); err != nil {
			h.Log.Warnf("auto_block: l2 incr error for %s: %v", ip, err)
		}
		if err := h.Cache.SetInt(ctx, l1Key, 0, autoBlockCountTTL); err != nil {
			h.Log.Warnf("auto_block: l1 reset error for %s: %v", ip, err)
		}
	default:
		blockLevel = 1
		blockDuration = time.Duration(policy.L1BlockSeconds) * time.Second
		// Increment l1 counter
		if _, err := h.Cache.IncrWithExpiry(ctx, l1Key, autoBlockCountTTL); err != nil {
			h.Log.Warnf("auto_block: l1 incr error for %s: %v", ip, err)
		}
	}

	// 6. Set the active-block key with expiry
	blockedUntil := time.Now().Add(blockDuration)
	status := autoBlockStatus{
		Level:        blockLevel,
		BlockedUntil: blockedUntil.Unix(),
	}
	activeKey := fmt.Sprintf(autoBlockActiveFmt, ip)
	if err := h.Cache.Set(ctx, activeKey, status, blockDuration); err != nil {
		h.Log.Warnf("auto_block: set active key error for %s: %v", ip, err)
		return
	}

	// 7. Reset violation counter so the next window starts fresh
	if err := h.Cache.Delete(ctx, violKey); err != nil {
		h.Log.Warnf("auto_block: violation reset error for %s: %v", ip, err)
	}

	// 8. Re-read counters for DB upsert (they may have been updated above)
	newL1Count, _ := h.Cache.GetInt(ctx, l1Key)
	newL2Count, _ := h.Cache.GetInt(ctx, l2Key)

	// 9. Persist to DB (best-effort, non-blocking — we're already in a goroutine)
	h.upsertAutoIPBlock(ip, blockLevel, blockedUntil, int(newL1Count), int(newL2Count))

	h.Log.Infof("auto_block: IP %s blocked at level %d until %s", ip, blockLevel, blockedUntil.UTC().Format(time.RFC3339))
}

// upsertAutoIPBlock inserts or updates the AutoIPBlock record for the given IP.
func (h *HttpServer) upsertAutoIPBlock(ip string, level int, blockedUntil time.Time, l1Count, l2Count int) {
	var record model.AutoIPBlock
	err := h.DB.Where("ip_address = ?", ip).First(&record).Error
	if err != nil {
		// New record
		record = model.AutoIPBlock{
			IPAddress:    ip,
			BlockLevel:   level,
			BlockedUntil: blockedUntil,
			L1Count:      l1Count,
			L2Count:      l2Count,
			IsActive:     true,
		}
		if err := h.DB.Create(&record).Error; err != nil {
			h.Log.Warnf("auto_block: db create error for %s: %v", ip, err)
		}
		return
	}

	// Update existing record
	record.BlockLevel = level
	record.BlockedUntil = blockedUntil
	record.L1Count = l1Count
	record.L2Count = l2Count
	record.IsActive = true
	record.UnblockedAt = nil
	record.UnblockedBy = 0
	if err := h.DB.Save(&record).Error; err != nil {
		h.Log.Warnf("auto_block: db update error for %s: %v", ip, err)
	}
}

// loadIPBlockPolicy returns the first enabled policy from Redis cache, falling back to DB.
func (h *HttpServer) loadIPBlockPolicy(ctx context.Context) (*model.IPBlockPolicy, error) {
	var policy model.IPBlockPolicy
	if err := h.Cache.Get(ctx, autoBlockPolicyCacheKey, &policy); err == nil {
		return &policy, nil
	}

	// Cache miss — query DB for the first enabled policy
	if err := h.DB.Where("is_enabled = ?", true).First(&policy).Error; err != nil {
		return nil, err // no enabled policy
	}

	// Repopulate cache
	if err := h.Cache.Set(ctx, autoBlockPolicyCacheKey, policy, autoBlockPolicyCacheTTL); err != nil {
		h.Log.Warnf("auto_block: policy cache set error: %v", err)
	}

	return &policy, nil
}

// bustIPBlockPolicyCache invalidates the cached policy so it reloads on next request.
func (h *HttpServer) bustIPBlockPolicyCache() {
	if err := h.Cache.Delete(context.Background(), autoBlockPolicyCacheKey); err != nil {
		h.Log.Warnf("auto_block: policy cache bust error: %v", err)
	}
}
