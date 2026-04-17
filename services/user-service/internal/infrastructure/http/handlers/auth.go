// Package handlers contains the Gin HTTP handlers for the user-service.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sentinel-health-engine/user-service/internal/application/auth"
	"github.com/sentinel-health-engine/user-service/internal/infrastructure/http/middleware"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	login          *auth.LoginUseCase
	refresh        *auth.RefreshUseCase
	changePassword *auth.ChangePasswordUseCase
	forgotPassword *auth.RequestPasswordResetUseCase
	verifyCode     *auth.VerifyResetCodeUseCase
	resetPassword  *auth.ResetPasswordUseCase
}

func NewAuthHandler(
	login *auth.LoginUseCase,
	refresh *auth.RefreshUseCase,
	changePassword *auth.ChangePasswordUseCase,
	forgotPassword *auth.RequestPasswordResetUseCase,
	verifyCode *auth.VerifyResetCodeUseCase,
	resetPassword *auth.ResetPasswordUseCase,
) *AuthHandler {
	return &AuthHandler{
		login:          login,
		refresh:        refresh,
		changePassword: changePassword,
		forgotPassword: forgotPassword,
		verifyCode:     verifyCode,
		resetPassword:  resetPassword,
	}
}

// Register mounts all auth routes onto the given router group.
func (h *AuthHandler) Register(rg *gin.RouterGroup) {
	rg.POST("/auth/login", h.login_)
	rg.POST("/auth/refresh", h.refreshToken)
	rg.POST("/auth/change-password", middleware.RequireAuth(), h.changePasswordHandler)
	rg.POST("/auth/forgot-password", h.forgotPasswordHandler)
	rg.POST("/auth/verify-reset-code", h.verifyResetCodeHandler)
	rg.POST("/auth/reset-password", h.resetPasswordHandler)
}

// login_ handles POST /v1/auth/login
func (h *AuthHandler) login_(c *gin.Context) {
	var body struct {
		Email    string `json:"email"    binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pair, err := h.login.Execute(c.Request.Context(), auth.LoginCommand{
		Email:    body.Email,
		Password: body.Password,
	})
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, pair)
}

// refreshToken handles POST /v1/auth/refresh
func (h *AuthHandler) refreshToken(c *gin.Context) {
	var body struct {
		RefreshToken string `json:"refreshToken" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pair, err := h.refresh.Execute(c.Request.Context(), body.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, pair)
}

// forgotPasswordHandler handles POST /v1/auth/forgot-password
// Always returns 200 to prevent email enumeration.
func (h *AuthHandler) forgotPasswordHandler(c *gin.Context) {
	var body struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_ = h.forgotPassword.Execute(c.Request.Context(), auth.RequestPasswordResetCommand{Email: body.Email})
	c.JSON(http.StatusOK, gin.H{"message": "if the email exists, a 6-digit reset code has been sent"})
}

// verifyResetCodeHandler handles POST /v1/auth/verify-reset-code
func (h *AuthHandler) verifyResetCodeHandler(c *gin.Context) {
	var body struct {
		Code string `json:"code" binding:"required,len=6"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.verifyCode.Execute(c.Request.Context(), auth.VerifyResetCodeCommand{Code: body.Code})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"maskedEmail": result.MaskedEmail})
}

// resetPasswordHandler handles POST /v1/auth/reset-password
func (h *AuthHandler) resetPasswordHandler(c *gin.Context) {
	var body struct {
		Code        string `json:"code"        binding:"required,len=6"`
		NewPassword string `json:"newPassword" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.resetPassword.Execute(c.Request.Context(), auth.ResetPasswordCommand{
		Code:        body.Code,
		NewPassword: body.NewPassword,
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password reset successfully"})
}

// changePasswordHandler handles POST /v1/auth/change-password
func (h *AuthHandler) changePasswordHandler(c *gin.Context) {
	var body struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.changePassword.Execute(c.Request.Context(), auth.ChangePasswordCommand{
		UserID:      middleware.CurrentUserID(c),
		OldPassword: body.OldPassword,
		NewPassword: body.NewPassword,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully"})
}
