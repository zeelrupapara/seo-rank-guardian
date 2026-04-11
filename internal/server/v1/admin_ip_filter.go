package v1

import (
	"context"
	"net"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	apperrors "github.com/zeelrupapara/seo-rank-guardian/pkg/errors"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
)

// AdminListIPFilters godoc
// @Summary List IP filter rules
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param type query string false "Filter by type: allow or block"
// @Param enabled query string false "Filter by enabled status: true or false"
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Success 200 {object} PaginatedResponse
// @Router /admin/ip-filters [get]
func (h *HttpServer) AdminListIPFilters(c *fiber.Ctx) error {
	page, pageSize := parsePagination(c)
	offset := (page - 1) * pageSize

	query := h.DB.Model(&model.IPFilter{})

	if t := c.Query("type"); t == "allow" || t == "block" {
		query = query.Where("type = ?", t)
	}
	if en := c.Query("enabled"); en != "" {
		if en == "true" {
			query = query.Where("is_enabled = ?", true)
		} else if en == "false" {
			query = query.Where("is_enabled = ?", false)
		}
	}

	var total int64
	query.Count(&total)

	var rules []model.IPFilter
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&rules).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to list IP filters")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"items": rules,
		"total": total,
		"page":  page,
		"limit": pageSize,
	}, "IP filters retrieved")
}

// AdminCreateIPFilter godoc
// @Summary Create an IP filter rule
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 201
// @Router /admin/ip-filters [post]
func (h *HttpServer) AdminCreateIPFilter(c *fiber.Ctx) error {
	var req struct {
		IPRange     string `json:"ip_range"`
		Type        string `json:"type"`
		Description string `json:"description"`
	}
	if err := c.BodyParser(&req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "invalid body", "Bad Request")
	}

	if req.IPRange == "" {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "ip_range is required", "Bad Request")
	}
	// Validate: must be a plain IP or a valid CIDR
	if net.ParseIP(req.IPRange) == nil {
		if _, _, err := net.ParseCIDR(req.IPRange); err != nil {
			return httputil.ErrorResponse(c, fiber.StatusBadRequest, "invalid IP address or CIDR range", "Bad Request")
		}
	}
	if req.Type != "allow" && req.Type != "block" {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "type must be 'allow' or 'block'", "Bad Request")
	}

	adminID, _ := c.Locals("userId").(uint)

	rule := model.IPFilter{
		IPRange:     req.IPRange,
		Type:        req.Type,
		Description: req.Description,
		IsEnabled:   true,
	}
	rule.CreatedBy = adminID

	if err := h.DB.Create(&rule).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusConflict, "ip_range already exists", "Conflict")
	}

	h.bustIPFilterCache()

	var adminUser model.User
	h.DB.Select("username").First(&adminUser, adminID)
	h.writeAudit(c, adminID, adminUser.Username, "admin.create_ip_filter", "ip_filter", fmtID(rule.ID), map[string]any{
		"ip_range": rule.IPRange,
		"type":     rule.Type,
	})

	return httputil.SuccessResponse(c, fiber.StatusCreated, rule, "IP filter created")
}

// AdminToggleIPFilter godoc
// @Summary Toggle an IP filter rule on/off
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path int true "Rule ID"
// @Success 200
// @Router /admin/ip-filters/{id} [put]
func (h *HttpServer) AdminToggleIPFilter(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "invalid id", "Bad Request")
	}

	var rule model.IPFilter
	if err := h.DB.First(&rule, id).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, "rule not found", "Not Found")
	}

	rule.IsEnabled = !rule.IsEnabled
	if err := h.DB.Save(&rule).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to update rule")
	}

	h.bustIPFilterCache()

	adminID, _ := c.Locals("userId").(uint)
	var adminUser model.User
	h.DB.Select("username").First(&adminUser, adminID)
	h.writeAudit(c, adminID, adminUser.Username, "admin.toggle_ip_filter", "ip_filter", fmtID(rule.ID), map[string]any{
		"ip_range":   rule.IPRange,
		"is_enabled": rule.IsEnabled,
	})

	return httputil.SuccessResponse(c, fiber.StatusOK, rule, "IP filter updated")
}

// AdminDeleteIPFilter godoc
// @Summary Delete an IP filter rule
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path int true "Rule ID"
// @Success 200
// @Router /admin/ip-filters/{id} [delete]
func (h *HttpServer) AdminDeleteIPFilter(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "invalid id", "Bad Request")
	}

	var rule model.IPFilter
	if err := h.DB.First(&rule, id).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, "rule not found", "Not Found")
	}

	if err := h.DB.Delete(&rule).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to delete rule")
	}

	h.bustIPFilterCache()

	adminID, _ := c.Locals("userId").(uint)
	var adminUser model.User
	h.DB.Select("username").First(&adminUser, adminID)
	h.writeAudit(c, adminID, adminUser.Username, "admin.delete_ip_filter", "ip_filter", fmtID(rule.ID), map[string]any{
		"ip_range": rule.IPRange,
		"type":     rule.Type,
	})

	return httputil.SuccessResponse(c, fiber.StatusOK, nil, "IP filter deleted")
}

// bustIPFilterCache deletes the Redis key so the middleware reloads on next request.
func (h *HttpServer) bustIPFilterCache() {
	if err := h.Cache.Delete(context.Background(), ipFilterCacheKey); err != nil {
		h.Log.Warnf("ip filter cache bust failed: %v", err)
	}
}
