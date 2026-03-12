package v1

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	apperrors "github.com/zeelrupapara/seo-rank-guardian/pkg/errors"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
	"github.com/zeelrupapara/seo-rank-guardian/utils"
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

// UploadAvatar godoc
// @Summary Upload avatar
// @Description Upload a profile avatar image (max 800KB, JPG/PNG only)
// @Tags users
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param avatar formData file true "Avatar image file"
// @Success 200 {object} UserResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /users/me/avatar [post]
func (h *HttpServer) UploadAvatar(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	file, err := c.FormFile("avatar")
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "No avatar file provided")
	}

	// Validate size: max 800KB
	if file.Size > 800*1024 {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "File size must be under 800KB")
	}

	// Validate extension
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Only JPG and PNG files are allowed")
	}

	// Ensure uploads directory exists
	avatarDir := "./uploads/avatars"
	if err := os.MkdirAll(avatarDir, 0755); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to create upload directory")
	}

	// Save file
	filename := fmt.Sprintf("%d_%d%s", userID, time.Now().Unix(), ext)
	savePath := filepath.Join(avatarDir, filename)
	if err := c.SaveFile(file, savePath); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to save avatar")
	}

	// Update user
	var user model.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrUserNotFound.Error(), "User not found")
	}

	// Remove old avatar file if it's a local upload
	if user.AvatarURL != "" && strings.HasPrefix(user.AvatarURL, "/uploads/") {
		_ = os.Remove("." + user.AvatarURL)
	}

	user.AvatarURL = "/uploads/avatars/" + filename
	user.UpdatedBy = userID
	if err := h.DB.Save(&user).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to update avatar")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, user, "Avatar uploaded successfully")
}

// RemoveAvatar godoc
// @Summary Remove avatar
// @Description Remove the current user's profile avatar
// @Tags users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UserResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /users/me/avatar [delete]
func (h *HttpServer) RemoveAvatar(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	var user model.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrUserNotFound.Error(), "User not found")
	}

	// Remove file from disk (only if it's a local upload, not an external URL)
	if user.AvatarURL != "" && strings.HasPrefix(user.AvatarURL, "/uploads/") {
		_ = os.Remove("." + user.AvatarURL)
	}

	// Reset to default generated avatar
	user.AvatarURL = model.DefaultAvatarURL(user.Username)
	user.UpdatedBy = userID
	if err := h.DB.Save(&user).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to remove avatar")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, user, "Avatar removed successfully")
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

// ChangePassword godoc
// @Summary Change password
// @Description Change the authenticated user's password
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ChangePasswordRequest true "Change password request"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /users/me/password [put]
func (h *HttpServer) ChangePassword(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}

	var req ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid request body")
	}

	if err := h.Validate.Struct(req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Validation failed")
	}

	var user model.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusNotFound, apperrors.ErrUserNotFound.Error(), "User not found")
	}

	// Reject if Google-only user (no password set)
	if user.Provider == "google" && user.Password == "" {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Cannot change password for Google-only accounts")
	}

	// Verify current password
	if !utils.CheckPassword(user.Password, req.CurrentPassword) {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrInvalidPassword.Error(), "Current password is incorrect")
	}

	// Hash and save new password
	hashedPassword, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to hash password")
	}

	user.Password = hashedPassword
	user.UpdatedBy = userID
	if err := h.DB.Save(&user).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to update password")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, nil, "Password changed successfully")
}
