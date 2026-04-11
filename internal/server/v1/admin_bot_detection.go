package v1

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	apperrors "github.com/zeelrupapara/seo-rank-guardian/pkg/errors"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
)

// AdminListBotDetectionRules godoc
// @Summary List bot detection rules
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param type query string false "Filter by type: block or allow"
// @Param enabled query string false "Filter by enabled status: true or false"
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Success 200 {object} PaginatedResponse
// @Router /admin/bot-detection [get]
func (h *HttpServer) AdminListBotDetectionRules(c *fiber.Ctx) error {
	page, pageSize := parsePagination(c)
	offset := (page - 1) * pageSize

	query := h.DB.Model(&model.BotDetectionRule{})

	if t := c.Query("type"); t == "block" || t == "allow" {
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

	var rules []model.BotDetectionRule
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&rules).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to list bot detection rules")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"items": rules,
		"total": total,
		"page":  page,
		"limit": pageSize,
	}, "Bot detection rules retrieved")
}

// AdminCreateBotDetectionRule godoc
// @Summary Create a bot detection rule
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 201
// @Router /admin/bot-detection [post]
func (h *HttpServer) AdminCreateBotDetectionRule(c *fiber.Ctx) error {
	var req struct {
		Pattern     string `json:"pattern"`
		MatchField  string `json:"match_field"`
		Type        string `json:"type"`
		Description string `json:"description"`
	}
	if err := c.BodyParser(&req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "invalid body", "Bad Request")
	}

	if req.Pattern == "" {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "pattern is required", "Bad Request")
	}
	if req.MatchField != "user_agent" && req.MatchField != "absent_header" {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "match_field must be 'user_agent' or 'absent_header'", "Bad Request")
	}
	if req.Type != "block" && req.Type != "allow" {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "type must be 'block' or 'allow'", "Bad Request")
	}

	adminID, _ := c.Locals("userId").(uint)

	rule := model.BotDetectionRule{
		Pattern:     req.Pattern,
		MatchField:  req.MatchField,
		Type:        req.Type,
		Description: req.Description,
		IsEnabled:   true,
	}
	rule.CreatedBy = adminID

	if err := h.DB.Create(&rule).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to create rule")
	}

	h.bustBotDetectionCache()

	var adminUser model.User
	h.DB.Select("username").First(&adminUser, adminID)
	h.writeAudit(c, adminID, adminUser.Username, "admin.create_bot_detection_rule", "bot_detection", fmtID(rule.ID), map[string]any{
		"pattern":     rule.Pattern,
		"match_field": rule.MatchField,
		"type":        rule.Type,
	})

	return httputil.SuccessResponse(c, fiber.StatusCreated, rule, "Bot detection rule created")
}

// AdminToggleBotDetectionRule godoc
// @Summary Toggle a bot detection rule on/off
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path int true "Rule ID"
// @Success 200
// @Router /admin/bot-detection/{id} [put]
func (h *HttpServer) AdminToggleBotDetectionRule(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "invalid id", "Bad Request")
	}

	var rule model.BotDetectionRule
	if err := h.DB.First(&rule, id).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, "rule not found", "Not Found")
	}

	rule.IsEnabled = !rule.IsEnabled
	if err := h.DB.Save(&rule).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to update rule")
	}

	h.bustBotDetectionCache()

	adminID, _ := c.Locals("userId").(uint)
	var adminUser model.User
	h.DB.Select("username").First(&adminUser, adminID)
	h.writeAudit(c, adminID, adminUser.Username, "admin.toggle_bot_detection_rule", "bot_detection", fmtID(rule.ID), map[string]any{
		"pattern":    rule.Pattern,
		"is_enabled": rule.IsEnabled,
	})

	return httputil.SuccessResponse(c, fiber.StatusOK, rule, "Bot detection rule updated")
}

// AdminDeleteBotDetectionRule godoc
// @Summary Delete a bot detection rule
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path int true "Rule ID"
// @Success 200
// @Router /admin/bot-detection/{id} [delete]
func (h *HttpServer) AdminDeleteBotDetectionRule(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "invalid id", "Bad Request")
	}

	var rule model.BotDetectionRule
	if err := h.DB.First(&rule, id).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, "rule not found", "Not Found")
	}

	if err := h.DB.Delete(&rule).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to delete rule")
	}

	h.bustBotDetectionCache()

	adminID, _ := c.Locals("userId").(uint)
	var adminUser model.User
	h.DB.Select("username").First(&adminUser, adminID)
	h.writeAudit(c, adminID, adminUser.Username, "admin.delete_bot_detection_rule", "bot_detection", fmtID(rule.ID), map[string]any{
		"pattern": rule.Pattern,
		"type":    rule.Type,
	})

	return httputil.SuccessResponse(c, fiber.StatusOK, nil, "Bot detection rule deleted")
}
