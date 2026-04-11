package v1

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	apperrors "github.com/zeelrupapara/seo-rank-guardian/pkg/errors"
	httputil "github.com/zeelrupapara/seo-rank-guardian/pkg/http"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/oauth2"
	"github.com/zeelrupapara/seo-rank-guardian/utils"
)

type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// createSession creates a DB session record and generates a token pair.
// Called by Login, Register, and GoogleCallback to avoid duplication.
func (h *HttpServer) createSession(c *fiber.Ctx, userID uint, role, loginMethod string) (*oauth2.TokenPair, *model.Session, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, nil, err
	}
	sessionID := hex.EncodeToString(b)

	now := time.Now()
	session := model.Session{
		ID:           sessionID,
		UserID:       userID,
		IPAddress:    c.IP(),
		UserAgent:    c.Get("User-Agent"),
		DeviceInfo:   utils.ParseDeviceInfo(c.Get("User-Agent")),
		LoginMethod:  loginMethod,
		LastActiveAt: now.UnixNano(),
		ExpiresAt:    now.Add(h.OAuth2.RefreshExpiry()).UnixNano(),
	}
	if err := h.DB.Create(&session).Error; err != nil {
		return nil, nil, err
	}

	tokens, err := h.OAuth2.GenerateTokenPair(userID, role, sessionID)
	if err != nil {
		h.DB.Delete(&session)
		return nil, nil, err
	}

	return tokens, &session, nil
}

// Register godoc
// @Summary Register a new user
// @Description Create a new user with email and password
// @Tags auth
// @Accept json
// @Produce json
// @Param body body RegisterRequest true "Register request"
// @Success 201 {object} AuthResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /auth/register [post]
func (h *HttpServer) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid request body")
	}

	if err := h.Validate.Struct(req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Validation failed")
	}

	var existing model.User
	if err := h.DB.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		return httputil.ErrorResponse(c, fiber.StatusConflict, apperrors.ErrBadRequest.Error(), "An account with this email already exists")
	}
	if err := h.DB.Where("username = ?", req.Username).First(&existing).Error; err == nil {
		return httputil.ErrorResponse(c, fiber.StatusConflict, apperrors.ErrBadRequest.Error(), "This username is already taken")
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to hash password")
	}

	user := model.User{
		Username:  req.Username,
		Email:     req.Email,
		Password:  hashedPassword,
		Role:      "user",
		IsActive:  true,
		AvatarURL: model.DefaultAvatarURL(req.Username),
	}

	if err := h.DB.Create(&user).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to create user")
	}

	tokens, _, err := h.createSession(c, user.ID, user.Role, "password")
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to generate tokens")
	}

	h.writeAudit(c, user.ID, user.Username, "auth.register", "user", fmtID(user.ID), nil)
	return httputil.SuccessResponse(c, fiber.StatusCreated, fiber.Map{
		"user":   user,
		"tokens": tokens,
	}, "User registered successfully")
}

// Login godoc
// @Summary Login
// @Description Authenticate with email and password
// @Tags auth
// @Accept json
// @Produce json
// @Param body body LoginRequest true "Login request"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/login [post]
func (h *HttpServer) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid request body")
	}

	if err := h.Validate.Struct(req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Validation failed")
	}

	var user model.User
	if err := h.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUserNotFound.Error(), "Invalid credentials")
	}

	if !utils.CheckPassword(user.Password, req.Password) {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrInvalidPassword.Error(), "Invalid credentials")
	}

	if !user.IsActive {
		return httputil.ErrorResponse(c, fiber.StatusForbidden, apperrors.ErrForbidden.Error(), "Account is deactivated")
	}

	tokens, _, err := h.createSession(c, user.ID, user.Role, "password")
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to generate tokens")
	}

	h.writeAudit(c, user.ID, user.Username, "auth.login", "user", fmtID(user.ID), nil)
	return httputil.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"user":   user,
		"tokens": tokens,
	}, "Login successful")
}

// RefreshToken godoc
// @Summary Refresh token
// @Description Get a new access token using a refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param body body RefreshRequest true "Refresh request"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/refresh [post]
func (h *HttpServer) RefreshToken(c *fiber.Ctx) error {
	var req RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Invalid request body")
	}

	if err := h.Validate.Struct(req); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Validation failed")
	}

	claims, err := h.OAuth2.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrInvalidToken.Error(), "Invalid refresh token")
	}

	// Lazy update: track last activity only on refresh, not every request
	h.DB.Model(&model.Session{}).
		Where("id = ? AND revoked_at IS NULL", claims.SessionID).
		Update("last_active_at", time.Now().UnixNano())

	// Re-issue tokens with the same session ID so the session persists
	tokens, err := h.OAuth2.GenerateTokenPair(claims.UserID, claims.Role, claims.SessionID)
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to generate tokens")
	}

	return httputil.SuccessResponse(c, fiber.StatusOK, tokens, "Token refreshed successfully")
}

// GoogleLogin godoc
// @Summary Google OAuth login
// @Description Redirect to Google consent screen for authentication
// @Tags auth
// @Produce json
// @Success 307 {string} string "Redirect to Google"
// @Router /auth/google [get]
func (h *HttpServer) GoogleLogin(c *fiber.Ctx) error {
	if h.GoogleOAuth == nil {
		return httputil.ErrorResponse(c, fiber.StatusNotImplemented, "google_oauth_not_configured", "Google OAuth is not configured")
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to generate state")
	}
	state := hex.EncodeToString(b)

	ctx := context.Background()
	if err := h.Cache.Set(ctx, "oauth_state:"+state, state, 10*time.Minute); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to store state")
	}

	url := h.GoogleOAuth.GetAuthURL(state)
	return c.Redirect(url, fiber.StatusTemporaryRedirect)
}

// GoogleCallback godoc
// @Summary Google OAuth callback
// @Description Handle Google OAuth callback, create or find user, return JWT tokens
// @Tags auth
// @Produce json
// @Param state query string true "OAuth state"
// @Param code query string true "Authorization code"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /auth/google/callback [get]
func (h *HttpServer) GoogleCallback(c *fiber.Ctx) error {
	if h.GoogleOAuth == nil {
		return httputil.ErrorResponse(c, fiber.StatusNotImplemented, "google_oauth_not_configured", "Google OAuth is not configured")
	}

	state := c.Query("state")
	code := c.Query("code")

	if state == "" || code == "" {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Missing state or code")
	}

	ctx := context.Background()
	var storedState string
	if err := h.Cache.Get(ctx, "oauth_state:"+state, &storedState); err != nil {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, "invalid_state", "Invalid or expired state")
	}
	_ = h.Cache.Delete(ctx, "oauth_state:"+state)

	token, err := h.GoogleOAuth.Exchange(ctx, code)
	if err != nil {
		h.Log.Errorf("Google OAuth exchange failed: %v", err)
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to exchange code")
	}

	info, err := h.GoogleOAuth.GetUserInfo(ctx, token)
	if err != nil {
		h.Log.Errorf("Google get user info failed: %v", err)
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to get user info")
	}

	if !info.EmailVerified {
		return httputil.ErrorResponse(c, fiber.StatusBadRequest, apperrors.ErrBadRequest.Error(), "Google email is not verified")
	}

	var user model.User

	// 1. Look up by provider + provider_id
	result := h.DB.Where("provider = ? AND provider_id = ?", "google", info.Sub).First(&user)
	if result.Error == nil {
		// Existing Google user — sync avatar from Google if they don't have a custom upload
		if info.Picture != "" && !strings.HasPrefix(user.AvatarURL, "/uploads/") {
			user.AvatarURL = info.Picture
			h.DB.Model(&user).Update("avatar_url", user.AvatarURL)
		}
	}
	if result.Error != nil {
		// 2. Look up by email — only link if account was created via Google (prevent takeover of local accounts)
		result = h.DB.Where("email = ?", info.Email).First(&user)
		if result.Error != nil {
			// 3. Create new user
			avatarURL := info.Picture
			if avatarURL == "" {
				avatarURL = model.DefaultAvatarURL(info.Name)
			}
			// Ensure unique username — if taken, append random suffix
			username := info.Name
			var usernameCheck model.User
			if h.DB.Where("username = ?", username).First(&usernameCheck).Error == nil {
				suffix := make([]byte, 3)
				rand.Read(suffix)
				username = username + "_" + hex.EncodeToString(suffix)
			}
			user = model.User{
				Username:   username,
				Email:      info.Email,
				Role:       "user",
				IsActive:   true,
				Provider:   "google",
				ProviderID: info.Sub,
				AvatarURL:  avatarURL,
			}
			if err := h.DB.Create(&user).Error; err != nil {
				h.Log.Errorf("Failed to create Google user: %v", err)
				return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to create user")
			}
		} else if user.Provider == "google" {
			// Link Google account only if user was already a Google user (different Google account)
			user.ProviderID = info.Sub
			if err := h.DB.Save(&user).Error; err != nil {
				return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to link account")
			}
		} else {
			// Local account exists with this email — do NOT auto-link, require manual linking
			return httputil.ErrorResponse(c, fiber.StatusConflict, apperrors.ErrConflict.Error(), "An account with this email already exists. Please login with your password.")
		}
	}

	if !user.IsActive {
		return httputil.ErrorResponse(c, fiber.StatusForbidden, apperrors.ErrForbidden.Error(), "Account is deactivated")
	}

	tokens, _, err := h.createSession(c, user.ID, user.Role, "google")
	if err != nil {
		return httputil.ErrorResponse(c, fiber.StatusInternalServerError, apperrors.ErrInternalServer.Error(), "Failed to generate tokens")
	}

	h.writeAudit(c, user.ID, user.Username, "auth.oauth_login", "user", fmtID(user.ID), nil)
	// Redirect to frontend with tokens as query params
	frontendURL := "/auth/google/callback?access_token=" + tokens.AccessToken + "&refresh_token=" + tokens.RefreshToken
	return c.Redirect(frontendURL, fiber.StatusTemporaryRedirect)
}

// Logout godoc
// @Summary Logout
// @Description Revoke the current user session
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} MessageResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/logout [delete]
func (h *HttpServer) Logout(c *fiber.Ctx) error {
	userID, ok := c.Locals("userId").(uint)
	if !ok {
		return httputil.ErrorResponse(c, fiber.StatusUnauthorized, apperrors.ErrUnauthorized.Error(), "Unauthorized")
	}
	sessionID, _ := c.Locals("sessionId").(string)

	// Remove refresh token from Redis
	// NOTE: We do NOT call MarkSessionRevoked here — the caller is handing in
	// their own token right now, so there is no replay risk for the access token.
	_ = h.OAuth2.RevokeSession(sessionID)

	// Mark session as revoked in DB for audit trail
	now := time.Now()
	h.DB.Model(&model.Session{}).
		Where("id = ?", sessionID).
		Updates(map[string]any{
			"revoked_at": now,
			"revoked_by": userID,
		})

	var actingUser model.User
	h.DB.Select("username").First(&actingUser, userID)
	h.writeAudit(c, userID, actingUser.Username, "auth.logout", "user", fmtID(userID), nil)
	return httputil.SuccessResponse(c, fiber.StatusOK, nil, "Logged out successfully")
}
