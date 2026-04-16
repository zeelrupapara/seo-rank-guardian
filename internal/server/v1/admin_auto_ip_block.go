package v1

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	apperrors "github.com/zeelrupapara/seo-rank-guardian/pkg/errors"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
)

// AdminListAutoIPBlocks godoc
// @Summary List auto-blocked IPs
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param level query int false "Filter by block level (1, 2, or 3)"
// @Param active query string false "Filter by active status: true or false"
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Success 200 {object} PaginatedResponse
// @Router /admin/auto-ip-blocks [get]
func (h *HttpServer) AdminListAutoIPBlocks(c *fiber.Ctx) error {
	page, pageSize := parsePagination(c)
	offset := (page - 1) * pageSize

	query := h.DB.Model(&model.AutoIPBlock{})

	if lvl := c.Query("level"); lvl != "" {
		if l, err := strconv.Atoi(lvl); err == nil && l >= 1 && l <= 3 {
			query = query.Where("block_level = ?", l)
		}
	}
	if active := c.Query("active"); active != "" {
		if active == "true" {
			query = query.Where("is_active = ?", true)
		} else if active == "false" {
			query = query.Where("is_active = ?", false)
		}
	}

	var total int64
	query.Count(&total)

	var blocks []model.AutoIPBlock
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&blocks).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to list auto IP blocks")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"items": blocks,
		"total": total,
		"page":  page,
		"limit": pageSize,
	}, "Auto IP blocks retrieved")
}

// AdminUnblockIP godoc
// @Summary Manually unblock an auto-blocked IP
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path int true "AutoIPBlock record ID"
// @Success 200
// @Router /admin/auto-ip-blocks/{id} [delete]
func (h *HttpServer) AdminUnblockIP(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "invalid id", "Bad Request")
	}

	var block model.AutoIPBlock
	if err := h.DB.First(&block, id).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, "record not found", "Not Found")
	}

	adminID, _ := c.Locals("userId").(uint)
	now := time.Now()

	block.IsActive = false
	block.UnblockedAt = &now
	block.UnblockedBy = adminID

	if err := h.DB.Save(&block).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to unblock IP")
	}

	// Remove the active-block key from Redis so the IP can access immediately
	ctx := context.Background()
	activeKey := fmt.Sprintf(autoBlockActiveFmt, block.IPAddress)
	if err := h.Cache.Delete(ctx, activeKey); err != nil {
		h.Log.Warnf("admin_unblock: Redis delete error for %s: %v", block.IPAddress, err)
	}

	var adminUser model.User
	h.DB.Select("username").First(&adminUser, adminID)
	h.writeAudit(c, adminID, adminUser.Username, "admin.unblock_ip", "auto_ip_block", fmtID(block.ID), map[string]any{
		"ip_address":  block.IPAddress,
		"block_level": block.BlockLevel,
	})

	return httputil.SuccessResponse(c, fiber.StatusOK, block, "IP unblocked successfully")
}

// AdminResetIPCounts godoc
// @Summary Reset escalation counters for an auto-blocked IP
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path int true "AutoIPBlock record ID"
// @Success 200
// @Router /admin/auto-ip-blocks/{id}/reset [post]
func (h *HttpServer) AdminResetIPCounts(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "invalid id", "Bad Request")
	}

	var block model.AutoIPBlock
	if err := h.DB.First(&block, id).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, "record not found", "Not Found")
	}

	ctx := context.Background()

	// Delete all counters and violation keys for this IP
	l1Key := fmt.Sprintf(autoBlockL1CountFmt, block.IPAddress)
	l2Key := fmt.Sprintf(autoBlockL2CountFmt, block.IPAddress)
	violKey := fmt.Sprintf(autoBlockViolFmt, block.IPAddress)

	for _, key := range []string{l1Key, l2Key, violKey} {
		if err := h.Cache.Delete(ctx, key); err != nil {
			h.Log.Warnf("admin_reset_counts: Redis delete error for %s: %v", key, err)
		}
	}

	// Reset counts in DB
	adminID, _ := c.Locals("userId").(uint)
	block.L1Count = 0
	block.L2Count = 0
	block.UpdatedBy = adminID

	if err := h.DB.Save(&block).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to reset counts")
	}

	var adminUser model.User
	h.DB.Select("username").First(&adminUser, adminID)
	h.writeAudit(c, adminID, adminUser.Username, "admin.reset_ip_counts", "auto_ip_block", fmtID(block.ID), map[string]any{
		"ip_address": block.IPAddress,
	})

	return httputil.SuccessResponse(c, fiber.StatusOK, block, "IP escalation counts reset")
}
