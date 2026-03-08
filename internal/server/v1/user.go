package v1

import (
	"github.com/gofiber/fiber/v2"
	apperrors "github.com/zeelrupapara/seo-rank-guardian/pkg/errors"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
	"github.com/zeelrupapara/seo-rank-guardian/model"
)

type UpdateProfileRequest struct {
	Username string `json:"username" validate:"omitempty,min=3,max=50"`
}

// GetProfile godoc
// @Summary Get current user profile
// @Description Get the authenticated user's profile
// @Tags users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UserResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /users/me [get]
func (h *HttpServer) GetProfile(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	var user model.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrUserNotFound.Error(), "User not found")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, user, "Profile retrieved")
}

// UpdateProfile godoc
// @Summary Update current user profile
// @Description Update the authenticated user's profile
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body UpdateProfileRequest true "Update profile request"
// @Success 200 {object} UserResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /users/me [put]
func (h *HttpServer) UpdateProfile(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	var req UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid request body")
	}

	if err := h.Validate.Struct(req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, err.Error(), "Validation failed")
	}

	var user model.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrUserNotFound.Error(), "User not found")
	}

	if req.Username != "" {
		user.Username = req.Username
	}
	user.UpdatedBy = userID

	if err := h.DB.Save(&user).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to update profile")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, user, "Profile updated")
}
