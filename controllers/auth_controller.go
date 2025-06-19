package controllers

import (
	middleware "go-api-boilerplate/middlewares"
	"go-api-boilerplate/models"
	"go-api-boilerplate/services"
	"go-api-boilerplate/utils"

	"github.com/gin-gonic/gin"
)

type AuthController struct {
	authService *services.AuthService
	userService *services.UserService
}

// NewAuthHandler creates a new auth handler
func NewAuthController(authService *services.AuthService, userService *services.UserService) *AuthController {
	return &AuthController{
		authService: authService,
		userService: userService,
	}
}

// Register godoc
// @Summary Register a new user
// @Description Create a new user account
// @Tags auth
// @Accept json
// @Produce json
// @Param input body models.RegisterInput true "Registration details"
// @Success 201 {object} models.LoginResponse
// @Failure 400 {object} utils.Response
// @Failure 409 {object} utils.Response
// @Router /auth/register [post]
func (h *AuthController) Register(c *gin.Context) {
	var input models.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ValidationErrorResponse(c, err.Error())
		return
	}

	// Check if user already exists
	exists, err := h.userService.UserExistsByEmail(input.Email)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to check user existence")
		return
	}
	if exists {
		utils.ConflictResponse(c, "Email already registered", nil)
		return
	}

	// Create user
	user, err := h.authService.Register(&input)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to register user")
		return
	}

	// Generate tokens
	tokens, err := h.authService.GenerateTokens(user)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to generate tokens")
		return
	}

	// Prepare response
	response := models.LoginResponse{
		User:   user.ToResponse(),
		Tokens: tokens,
	}

	utils.CreatedResponse(c, "Registration successful", response)
}

// Login godoc
// @Summary Login user
// @Description Authenticate user and return tokens
// @Tags auth
// @Accept json
// @Produce json
// @Param input body models.LoginInput true "Login credentials"
// @Success 200 {object} models.LoginResponse
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /auth/login [post]
func (h *AuthController) Login(c *gin.Context) {
	var input models.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ValidationErrorResponse(c, err.Error())
		return
	}

	// Authenticate user
	user, err := h.authService.Login(input.Email, input.Password, c.ClientIP())
	if err != nil {
		if err == services.ErrInvalidCredentials {
			utils.UnauthorizedResponse(c, "Invalid email or password")
			return
		}
		utils.InternalServerErrorResponse(c, "Login failed")
		return
	}

	// Generate tokens
	tokens, err := h.authService.GenerateTokens(user)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to generate tokens")
		return
	}

	// Prepare response
	response := models.LoginResponse{
		User:   user.ToResponse(),
		Tokens: tokens,
	}

	utils.SuccessResponse(c, "Login successful", response)
}

// RefreshToken godoc
// @Summary Refresh access token
// @Description Refresh access token using refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param input body models.RefreshTokenInput true "Refresh token"
// @Success 200 {object} models.AuthTokens
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /auth/refresh [post]
func (h *AuthController) RefreshToken(c *gin.Context) {
	var input models.RefreshTokenInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ValidationErrorResponse(c, err.Error())
		return
	}

	// Refresh tokens
	tokens, err := h.authService.RefreshTokens(input.RefreshToken)
	if err != nil {
		if err == services.ErrInvalidToken {
			utils.UnauthorizedResponse(c, "Invalid refresh token")
			return
		}
		utils.InternalServerErrorResponse(c, "Failed to refresh token")
		return
	}

	utils.SuccessResponse(c, "Token refreshed successfully", tokens)
}

// Logout godoc
// @Summary Logout user
// @Description Invalidate user tokens
// @Tags auth
// @Security Bearer
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /auth/logout [post]
func (h *AuthController) Logout(c *gin.Context) {
	// Get user ID from context
	userID, err := middleware.GetUserID(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "")
		return
	}

	// Get token from header
	authHeader := c.GetHeader("Authorization")
	token, _ := utils.ExtractTokenFromHeader(authHeader)

	// Logout user
	if err := h.authService.Logout(userID, token); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to logout")
		return
	}

	utils.SuccessResponse(c, "Logged out successfully", nil)
}

// ChangePassword godoc
// @Summary Change password
// @Description Change user password
// @Tags auth
// @Security Bearer
// @Accept json
// @Produce json
// @Param input body models.ChangePasswordInput true "Password change details"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /auth/change-password [post]
func (h *AuthController) ChangePassword(c *gin.Context) {
	var input models.ChangePasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ValidationErrorResponse(c, err.Error())
		return
	}

	// Get user ID from context
	userID, err := middleware.GetUserID(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "")
		return
	}

	// Change password
	if err := h.authService.ChangePassword(userID, input.OldPassword, input.NewPassword); err != nil {
		if err == services.ErrInvalidCredentials {
			utils.BadRequestResponse(c, "Current password is incorrect", nil)
			return
		}
		utils.InternalServerErrorResponse(c, "Failed to change password")
		return
	}

	utils.SuccessResponse(c, "Password changed successfully", nil)
}

// ForgotPassword godoc
// @Summary Request password reset
// @Description Send password reset email
// @Tags auth
// @Accept json
// @Produce json
// @Param input body map[string]string true "Email address"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Router /auth/forgot-password [post]
func (h *AuthController) ForgotPassword(c *gin.Context) {
	var input struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ValidationErrorResponse(c, err.Error())
		return
	}

	// Initiate password reset
	if err := h.authService.ForgotPassword(input.Email); err != nil {
		// Don't reveal if email exists or not
		utils.SuccessResponse(c, "If the email exists, a password reset link has been sent", nil)
		return
	}

	utils.SuccessResponse(c, "If the email exists, a password reset link has been sent", nil)
}

// ResetPassword godoc
// @Summary Reset password
// @Description Reset password using token
// @Tags auth
// @Accept json
// @Produce json
// @Param input body map[string]string true "Reset token and new password"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Router /auth/reset-password [post]
func (h *AuthController) ResetPassword(c *gin.Context) {
	var input struct {
		Token           string `json:"token" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required,min=8"`
		ConfirmPassword string `json:"confirm_password" binding:"required,eqfield=NewPassword"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ValidationErrorResponse(c, err.Error())
		return
	}

	// Reset password
	if err := h.authService.ResetPassword(input.Token, input.NewPassword); err != nil {
		if err == services.ErrInvalidToken {
			utils.BadRequestResponse(c, "Invalid or expired reset token", nil)
			return
		}
		utils.InternalServerErrorResponse(c, "Failed to reset password")
		return
	}

	utils.SuccessResponse(c, "Password reset successfully", nil)
}

// VerifyEmail godoc
// @Summary Verify email address
// @Description Verify user email address with token
// @Tags auth
// @Param token path string true "Verification token"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Router /auth/verify-email/{token} [get]
func (h *AuthController) VerifyEmail(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		utils.BadRequestResponse(c, "Verification token is required", nil)
		return
	}

	// Verify email
	if err := h.authService.VerifyEmail(token); err != nil {
		if err == services.ErrInvalidToken {
			utils.BadRequestResponse(c, "Invalid or expired verification token", nil)
			return
		}
		utils.InternalServerErrorResponse(c, "Failed to verify email")
		return
	}

	utils.SuccessResponse(c, "Email verified successfully", nil)
}
