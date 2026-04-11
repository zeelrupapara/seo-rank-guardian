package v1

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	apperrors "github.com/zeelrupapara/seo-rank-guardian/pkg/errors"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
	"gorm.io/datatypes"
)

// writeAudit inserts an audit log entry after a successful mutating operation.
// Failures are only warned — audit log issues must never break the user's request.
func (h *HttpServer) writeAudit(c *fiber.Ctx, userID uint, username, action, resource, resourceID string, meta map[string]any) {
	raw, _ := json.Marshal(meta)
	entry := model.AuditLog{
		UserID:     userID,
		Username:   username,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		IPAddress:  c.IP(),
		Meta:       datatypes.JSON(raw),
	}
	if err := h.DB.Create(&entry).Error; err != nil {
		h.Log.Warnf("audit log write failed [%s]: %v", action, err)
	}
}

// AdminListAuditLogs godoc
// @Summary List audit logs
// @Description Paginated list of all audit log entries with optional filters
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param user_id query int false "Filter by acting user ID"
// @Param action query string false "Filter by action prefix (e.g. 'admin.')"
// @Param resource query string false "Filter by resource type"
// @Param from query int false "Start time as Unix milliseconds"
// @Param to query int false "End time as Unix milliseconds"
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Success 200 {object} PaginatedResponse
// @Router /admin/audit [get]
func (h *HttpServer) AdminListAuditLogs(c *fiber.Ctx) error {
	page, pageSize := parsePagination(c)
	offset := (page - 1) * pageSize

	query := h.DB.Model(&model.AuditLog{})

	if userIDStr := c.Query("user_id"); userIDStr != "" {
		uid, err := strconv.ParseUint(userIDStr, 10, 64)
		if err == nil {
			query = query.Where("user_id = ?", uid)
		}
	}

	if action := c.Query("action"); action != "" {
		query = query.Where("action LIKE ?", action+"%")
	}

	if resource := c.Query("resource"); resource != "" {
		query = query.Where("resource = ?", resource)
	}

	// from/to are Unix milliseconds from the frontend; DB stores nanoseconds
	if fromStr := c.Query("from"); fromStr != "" {
		fromMs, err := strconv.ParseInt(fromStr, 10, 64)
		if err == nil {
			query = query.Where("created_at >= ?", fromMs*int64(time.Millisecond))
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		toMs, err := strconv.ParseInt(toStr, 10, 64)
		if err == nil {
			query = query.Where("created_at <= ?", toMs*int64(time.Millisecond))
		}
	}

	var total int64
	query.Count(&total)

	var logs []model.AuditLog
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to list audit logs")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"items": logs,
		"total": total,
		"page":  page,
		"limit": pageSize,
	}, "Audit logs retrieved")
}

// fmtID converts a uint to a string for use as ResourceID in audit log entries.
func fmtID(id uint) string {
	return fmt.Sprintf("%d", id)
}

// fmtID64 converts a uint64 to a string for use as ResourceID.
func fmtID64(id uint64) string {
	return strconv.FormatUint(id, 10)
}
