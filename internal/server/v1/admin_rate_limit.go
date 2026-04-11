package v1

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	apperrors "github.com/zeelrupapara/seo-rank-guardian/pkg/errors"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
)

// AdminListRateLimits godoc
// @Summary List rate limit rules
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param role query string false "Filter by role"
// @Param endpoint query string false "Filter by endpoint"
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Success 200 {object} PaginatedResponse
// @Router /admin/rate-limits [get]
func (h *HttpServer) AdminListRateLimits(c *fiber.Ctx) error {
	page, pageSize := parsePagination(c)
	offset := (page - 1) * pageSize

	query := h.DB.Model(&model.RateLimit{})

	if role := c.Query("role"); role != "" {
		query = query.Where("role = ?", role)
	}
	if endpoint := c.Query("endpoint"); endpoint != "" {
		query = query.Where("endpoint = ?", endpoint)
	}

	var total int64
	query.Count(&total)

	var rules []model.RateLimit
	if err := query.Order("role ASC, created_at DESC").Offset(offset).Limit(pageSize).Find(&rules).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to list rate limits")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"items": rules,
		"total": total,
		"page":  page,
		"limit": pageSize,
	}, "Rate limits retrieved")
}

// AdminCreateRateLimit godoc
// @Summary Create a rate limit rule
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 201
// @Router /admin/rate-limits [post]
func (h *HttpServer) AdminCreateRateLimit(c *fiber.Ctx) error {
	var req struct {
		Role          string `json:"role"`
		Endpoint      string `json:"endpoint"`
		MaxRequests   int    `json:"max_requests"`
		WindowSeconds int    `json:"window_seconds"`
		Description   string `json:"description"`
	}
	if err := c.BodyParser(&req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "invalid body", "Bad Request")
	}

	if req.Role == "" {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "role is required", "Bad Request")
	}
	if req.Endpoint == "" {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "endpoint is required", "Bad Request")
	}
	if req.Endpoint != "*" && !strings.HasPrefix(req.Endpoint, "/") {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "endpoint must be '*' or start with '/'", "Bad Request")
	}
	if req.MaxRequests <= 0 {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "max_requests must be greater than 0", "Bad Request")
	}
	if req.WindowSeconds <= 0 {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "window_seconds must be greater than 0", "Bad Request")
	}

	adminID, _ := c.Locals("userId").(uint)

	rule := model.RateLimit{
		Role:          req.Role,
		Endpoint:      req.Endpoint,
		MaxRequests:   req.MaxRequests,
		WindowSeconds: req.WindowSeconds,
		Description:   req.Description,
		IsEnabled:     true,
	}
	rule.CreatedBy = adminID

	if err := h.DB.Create(&rule).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to create rate limit")
	}

	h.bustRateLimitCache()

	var adminUser model.User
	h.DB.Select("username").First(&adminUser, adminID)
	h.writeAudit(c, adminID, adminUser.Username, "admin.create_rate_limit", "rate_limit", fmtID(rule.ID), map[string]any{
		"role":           rule.Role,
		"endpoint":       rule.Endpoint,
		"max_requests":   rule.MaxRequests,
		"window_seconds": rule.WindowSeconds,
	})

	return httputil.SuccessResponse(c, fiber.StatusCreated, rule, "Rate limit created")
}

// AdminToggleRateLimit godoc
// @Summary Toggle a rate limit rule on/off
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path int true "Rule ID"
// @Success 200
// @Router /admin/rate-limits/{id} [put]
func (h *HttpServer) AdminToggleRateLimit(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "invalid id", "Bad Request")
	}

	var rule model.RateLimit
	if err := h.DB.First(&rule, id).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, "rule not found", "Not Found")
	}

	rule.IsEnabled = !rule.IsEnabled
	if err := h.DB.Save(&rule).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to update rule")
	}

	h.bustRateLimitCache()

	adminID, _ := c.Locals("userId").(uint)
	var adminUser model.User
	h.DB.Select("username").First(&adminUser, adminID)
	h.writeAudit(c, adminID, adminUser.Username, "admin.toggle_rate_limit", "rate_limit", fmtID(rule.ID), map[string]any{
		"role":       rule.Role,
		"endpoint":   rule.Endpoint,
		"is_enabled": rule.IsEnabled,
	})

	return httputil.SuccessResponse(c, fiber.StatusOK, rule, "Rate limit updated")
}

// AdminDeleteRateLimit godoc
// @Summary Delete a rate limit rule
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path int true "Rule ID"
// @Success 200
// @Router /admin/rate-limits/{id} [delete]
func (h *HttpServer) AdminDeleteRateLimit(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "invalid id", "Bad Request")
	}

	var rule model.RateLimit
	if err := h.DB.First(&rule, id).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, "rule not found", "Not Found")
	}

	if err := h.DB.Delete(&rule).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to delete rule")
	}

	h.bustRateLimitCache()

	adminID, _ := c.Locals("userId").(uint)
	var adminUser model.User
	h.DB.Select("username").First(&adminUser, adminID)
	h.writeAudit(c, adminID, adminUser.Username, "admin.delete_rate_limit", "rate_limit", fmtID(rule.ID), map[string]any{
		"role":     rule.Role,
		"endpoint": rule.Endpoint,
	})

	return httputil.SuccessResponse(c, fiber.StatusOK, nil, "Rate limit deleted")
}
