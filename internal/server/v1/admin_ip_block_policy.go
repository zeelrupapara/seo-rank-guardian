package v1

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	apperrors "github.com/zeelrupapara/seo-rank-guardian/pkg/errors"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
)

// AdminListIPBlockPolicies godoc
// @Summary List IP block escalation policies
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Success 200 {object} PaginatedResponse
// @Router /admin/ip-block-policies [get]
func (h *HttpServer) AdminListIPBlockPolicies(c *fiber.Ctx) error {
	page, pageSize := parsePagination(c)
	offset := (page - 1) * pageSize

	var total int64
	h.DB.Model(&model.IPBlockPolicy{}).Count(&total)

	var policies []model.IPBlockPolicy
	if err := h.DB.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&policies).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to list policies")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"items": policies,
		"total": total,
		"page":  page,
		"limit": pageSize,
	}, "IP block policies retrieved")
}

// AdminCreateIPBlockPolicy godoc
// @Summary Create an IP block escalation policy
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 201
// @Router /admin/ip-block-policies [post]
func (h *HttpServer) AdminCreateIPBlockPolicy(c *fiber.Ctx) error {
	var req struct {
		Name                 string `json:"name"`
		Description          string `json:"description"`
		L1ViolationThreshold int    `json:"l1_violation_threshold"`
		L1BlockSeconds       int    `json:"l1_block_seconds"`
		L2ThresholdBlocks    int    `json:"l2_threshold_blocks"`
		L2BlockSeconds       int    `json:"l2_block_seconds"`
		L3ThresholdBlocks    int    `json:"l3_threshold_blocks"`
		L3BlockSeconds       int    `json:"l3_block_seconds"`
	}
	if err := c.BodyParser(&req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "invalid body", "Bad Request")
	}

	if req.Name == "" {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "name is required", "Bad Request")
	}
	if req.L1ViolationThreshold <= 0 || req.L1BlockSeconds <= 0 ||
		req.L2ThresholdBlocks <= 0 || req.L2BlockSeconds <= 0 ||
		req.L3ThresholdBlocks <= 0 || req.L3BlockSeconds <= 0 {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "all threshold and block-second values must be > 0", "Bad Request")
	}

	adminID, _ := c.Locals("userId").(uint)

	policy := model.IPBlockPolicy{
		Name:                 req.Name,
		Description:          req.Description,
		IsEnabled:            true,
		L1ViolationThreshold: req.L1ViolationThreshold,
		L1BlockSeconds:       req.L1BlockSeconds,
		L2ThresholdBlocks:    req.L2ThresholdBlocks,
		L2BlockSeconds:       req.L2BlockSeconds,
		L3ThresholdBlocks:    req.L3ThresholdBlocks,
		L3BlockSeconds:       req.L3BlockSeconds,
	}
	policy.CreatedBy = adminID

	if err := h.DB.Create(&policy).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to create policy")
	}

	h.bustIPBlockPolicyCache()

	var adminUser model.User
	h.DB.Select("username").First(&adminUser, adminID)
	h.writeAudit(c, adminID, adminUser.Username, "admin.create_ip_block_policy", "ip_block_policy", fmtID(policy.ID), map[string]any{
		"name": policy.Name,
	})

	return httputil.SuccessResponse(c, fiber.StatusCreated, policy, "IP block policy created")
}

// AdminUpdateIPBlockPolicy godoc
// @Summary Update an IP block escalation policy
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Policy ID"
// @Success 200
// @Router /admin/ip-block-policies/{id} [put]
func (h *HttpServer) AdminUpdateIPBlockPolicy(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "invalid id", "Bad Request")
	}

	var policy model.IPBlockPolicy
	if err := h.DB.First(&policy, id).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, "policy not found", "Not Found")
	}

	var req struct {
		Name                 string `json:"name"`
		Description          string `json:"description"`
		L1ViolationThreshold int    `json:"l1_violation_threshold"`
		L1BlockSeconds       int    `json:"l1_block_seconds"`
		L2ThresholdBlocks    int    `json:"l2_threshold_blocks"`
		L2BlockSeconds       int    `json:"l2_block_seconds"`
		L3ThresholdBlocks    int    `json:"l3_threshold_blocks"`
		L3BlockSeconds       int    `json:"l3_block_seconds"`
	}
	if err := c.BodyParser(&req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "invalid body", "Bad Request")
	}

	if req.Name != "" {
		policy.Name = req.Name
	}
	policy.Description = req.Description

	if req.L1ViolationThreshold > 0 {
		policy.L1ViolationThreshold = req.L1ViolationThreshold
	}
	if req.L1BlockSeconds > 0 {
		policy.L1BlockSeconds = req.L1BlockSeconds
	}
	if req.L2ThresholdBlocks > 0 {
		policy.L2ThresholdBlocks = req.L2ThresholdBlocks
	}
	if req.L2BlockSeconds > 0 {
		policy.L2BlockSeconds = req.L2BlockSeconds
	}
	if req.L3ThresholdBlocks > 0 {
		policy.L3ThresholdBlocks = req.L3ThresholdBlocks
	}
	if req.L3BlockSeconds > 0 {
		policy.L3BlockSeconds = req.L3BlockSeconds
	}

	adminID, _ := c.Locals("userId").(uint)
	policy.UpdatedBy = adminID

	if err := h.DB.Save(&policy).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to update policy")
	}

	h.bustIPBlockPolicyCache()

	var adminUser model.User
	h.DB.Select("username").First(&adminUser, adminID)
	h.writeAudit(c, adminID, adminUser.Username, "admin.update_ip_block_policy", "ip_block_policy", fmtID(policy.ID), map[string]any{
		"name": policy.Name,
	})

	return httputil.SuccessResponse(c, fiber.StatusOK, policy, "IP block policy updated")
}

// AdminToggleIPBlockPolicy godoc
// @Summary Toggle an IP block policy on/off
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path int true "Policy ID"
// @Success 200
// @Router /admin/ip-block-policies/{id}/toggle [patch]
func (h *HttpServer) AdminToggleIPBlockPolicy(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "invalid id", "Bad Request")
	}

	var policy model.IPBlockPolicy
	if err := h.DB.First(&policy, id).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, "policy not found", "Not Found")
	}

	policy.IsEnabled = !policy.IsEnabled
	adminID, _ := c.Locals("userId").(uint)
	policy.UpdatedBy = adminID

	if err := h.DB.Save(&policy).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to toggle policy")
	}

	h.bustIPBlockPolicyCache()

	var adminUser model.User
	h.DB.Select("username").First(&adminUser, adminID)
	h.writeAudit(c, adminID, adminUser.Username, "admin.toggle_ip_block_policy", "ip_block_policy", fmtID(policy.ID), map[string]any{
		"name":       policy.Name,
		"is_enabled": policy.IsEnabled,
	})

	return httputil.SuccessResponse(c, fiber.StatusOK, policy, "IP block policy toggled")
}

// AdminDeleteIPBlockPolicy godoc
// @Summary Delete an IP block escalation policy
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path int true "Policy ID"
// @Success 200
// @Router /admin/ip-block-policies/{id} [delete]
func (h *HttpServer) AdminDeleteIPBlockPolicy(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "invalid id", "Bad Request")
	}

	var policy model.IPBlockPolicy
	if err := h.DB.First(&policy, id).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, "policy not found", "Not Found")
	}

	if err := h.DB.Delete(&policy).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to delete policy")
	}

	h.bustIPBlockPolicyCache()

	adminID, _ := c.Locals("userId").(uint)
	var adminUser model.User
	h.DB.Select("username").First(&adminUser, adminID)
	h.writeAudit(c, adminID, adminUser.Username, "admin.delete_ip_block_policy", "ip_block_policy", fmtID(policy.ID), map[string]any{
		"name": policy.Name,
	})

	return httputil.SuccessResponse(c, fiber.StatusOK, nil, "IP block policy deleted")
}
