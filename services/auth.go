package services

import (
	"errors"
	"fmt"
	"time"

	"go-api-boilerplate/database"
	"go-api-boilerplate/models"
	"go-api-boilerplate/pkg/logger"
	"go-api-boilerplate/utils"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrUserNotActive      = errors.New("user account is not active")
	ErrEmailNotVerified   = errors.New("email not verified")
)

// AuthService handles authentication logic
type AuthService struct {
	db    *database.DB
	redis *RedisService
}

// NewAuthService creates a new auth service
func NewAuthService(db *database.DB, redis *RedisService) *AuthService {
	return &AuthService{
		db:    db,
		redis: redis,
	}
}

// Register creates a new user account
func (s *AuthService) Register(input *models.RegisterInput) (*models.User, error) {
	// Hash password
	hashedPassword, err := utils.HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &models.User{
		Email:    input.Email,
		Password: hashedPassword,
		Name:     input.Name,
		Role:     models.RoleUser,
	}

	// Save to database
	if err := s.db.Write.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Send verification email (implement email service)
	go s.sendVerificationEmail(user)

	return user, nil
}

// Login authenticates a user
func (s *AuthService) Login(email, password, ipAddress string) (*models.User, error) {
	// Find user by email
	var user models.User
	if err := s.db.Read.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, ErrInvalidCredentials
	}

	// Check password
	if !utils.CheckPassword(password, user.Password) {
		return nil, ErrInvalidCredentials
	}

	// Check if user is active
	if !user.IsActive {
		return nil, ErrUserNotActive
	}

	// Update last login
	now := time.Now()
	user.LastLoginAt = &now
	s.db.Write.Save(&user)

	// Log login attempt (implement audit logging)
	go s.logLoginAttempt(user.ID, ipAddress, true)

	return &user, nil
}

// GenerateTokens generates JWT tokens for a user
func (s *AuthService) GenerateTokens(user *models.User) (*models.AuthTokens, error) {
	// Generate tokens
	tokenPair, err := utils.GenerateTokens(
		user.ID,
		user.Email,
		user.Name,
		user.Role,
		user.IsActive,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Store refresh token
	user.RefreshToken = tokenPair.RefreshToken
	if err := s.db.Write.Save(user).Error; err != nil {
		return nil, fmt.Errorf("failed to save refresh token: %w", err)
	}

	// Cache user data in Redis
	if s.redis != nil {
		cacheKey := fmt.Sprintf("user:%d", user.ID)
		s.redis.CacheSet("auth", cacheKey, user.ToResponse(), 24*time.Hour)
	}

	return &models.AuthTokens{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    tokenPair.ExpiresIn,
	}, nil
}

// RefreshTokens refreshes authentication tokens
func (s *AuthService) RefreshTokens(refreshToken string) (*models.AuthTokens, error) {
	// Validate refresh token
	userID, err := utils.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Find user
	var user models.User
	if err := s.db.Read.First(&user, userID).Error; err != nil {
		return nil, ErrInvalidToken
	}

	// Verify stored refresh token matches
	if user.RefreshToken != refreshToken {
		return nil, ErrInvalidToken
	}

	// Check if user is active
	if !user.IsActive {
		return nil, ErrUserNotActive
	}

	// Generate new tokens
	return s.GenerateTokens(&user)
}

// Logout invalidates user tokens
func (s *AuthService) Logout(userID uint, token string) error {
	// Clear refresh token from database
	if err := s.db.Write.Model(&models.User{}).Where("id = ?", userID).
		Update("refresh_token", "").Error; err != nil {
		return fmt.Errorf("failed to clear refresh token: %w", err)
	}

	// Blacklist the access token in Redis
	if s.redis != nil {
		// Parse token to get expiration
		claims, _ := utils.ParseTokenWithoutValidation(token)
		if claims != nil && claims.ExpiresAt != nil {
			ttl := time.Until(claims.ExpiresAt.Time)
			if ttl > 0 {
				s.redis.CacheSet("blacklist", token, true, ttl)
			}
		}

		// Clear user cache
		cacheKey := fmt.Sprintf("user:%d", userID)
		s.redis.CacheDelete("auth", cacheKey)
	}

	return nil
}

// ChangePassword changes user password
func (s *AuthService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	// Find user
	var user models.User
	if err := s.db.Read.First(&user, userID).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Verify old password
	if !utils.CheckPassword(oldPassword, user.Password) {
		return ErrInvalidCredentials
	}

	// Hash new password
	hashedPassword, err := utils.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	user.Password = hashedPassword
	if err := s.db.Write.Save(&user).Error; err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Invalidate all tokens (force re-login)
	if s.redis != nil {
		s.redis.CacheDelete("auth", fmt.Sprintf("user:%d", userID))
	}

	return nil
}

// ForgotPassword initiates password reset process
func (s *AuthService) ForgotPassword(email string) error {
	// Find user by email
	var user models.User
	if err := s.db.Read.Where("email = ?", email).First(&user).Error; err != nil {
		// Don't reveal if user exists
		return nil
	}

	// Generate reset token
	token := utils.GeneratePasswordResetToken()

	// Save reset token
	resetRequest := &models.PasswordReset{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	if err := s.db.Write.Create(resetRequest).Error; err != nil {
		return fmt.Errorf("failed to save reset token: %w", err)
	}

	// Send reset email
	go s.sendPasswordResetEmail(&user, token)

	return nil
}

// ResetPassword resets user password with token
func (s *AuthService) ResetPassword(token, newPassword string) error {
	// Find valid reset request
	var resetRequest models.PasswordReset
	if err := s.db.Read.Where("token = ? AND used_at IS NULL", token).
		Preload("User").First(&resetRequest).Error; err != nil {
		return ErrInvalidToken
	}

	// Check if expired
	if resetRequest.IsExpired() {
		return ErrInvalidToken
	}

	// Hash new password
	hashedPassword, err := utils.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password in transaction
	tx := s.db.Write.Begin()

	// Update user password
	if err := tx.Model(&models.User{}).Where("id = ?", resetRequest.UserID).
		Update("password", hashedPassword).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Mark reset token as used
	now := time.Now()
	resetRequest.UsedAt = &now
	if err := tx.Save(&resetRequest).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update reset token: %w", err)
	}

	tx.Commit()

	// Clear any cached data
	if s.redis != nil {
		s.redis.CacheDelete("auth", fmt.Sprintf("user:%d", resetRequest.UserID))
	}

	return nil
}

// VerifyEmail verifies user email address
func (s *AuthService) VerifyEmail(token string) error {
	// This is a simplified implementation
	// In production, you'd have a proper email verification token system

	// Find user by verification token (implement proper token storage)
	var user models.User
	// This is placeholder logic - implement proper verification token lookup
	if err := s.db.Write.Where("email_verified = false").First(&user).Error; err != nil {
		return ErrInvalidToken
	}

	// Mark email as verified
	now := time.Now()
	user.EmailVerified = true
	user.EmailVerifiedAt = &now

	if err := s.db.Write.Save(&user).Error; err != nil {
		return fmt.Errorf("failed to verify email: %w", err)
	}

	return nil
}

// ValidateAccessToken validates an access token
func (s *AuthService) ValidateAccessToken(token string) (*models.User, error) {
	// Check if token is blacklisted
	if s.redis != nil {
		blacklisted, err := s.redis.Exists(fmt.Sprintf("blacklist:%s", token))
		if err == nil && blacklisted > 0 {
			return nil, ErrInvalidToken
		}
	}

	// Validate token
	claims, err := utils.ValidateToken(token)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Try to get user from cache first
	if s.redis != nil {
		cacheKey := fmt.Sprintf("user:%d", claims.UserID)
		var userResp models.UserResponse
		if err := s.redis.CacheGetJSON("auth", cacheKey, &userResp); err == nil {
			// Convert response back to user (simplified)
			return &models.User{
				ID:            userResp.ID,
				Email:         userResp.Email,
				Name:          userResp.Name,
				Role:          userResp.Role,
				IsActive:      userResp.IsActive,
				EmailVerified: userResp.EmailVerified,
			}, nil
		}
	}

	// Get from database
	var user models.User
	if err := s.db.Read.First(&user, claims.UserID).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Cache for future requests
	if s.redis != nil {
		cacheKey := fmt.Sprintf("user:%d", user.ID)
		s.redis.CacheSet("auth", cacheKey, user.ToResponse(), 24*time.Hour)
	}

	return &user, nil
}

// Helper methods

func (s *AuthService) sendVerificationEmail(user *models.User) {
	// Implement email sending logic
	// This would integrate with an email service like SendGrid, AWS SES, etc.
	token := utils.GenerateEmailVerificationToken()
	// Store token and send email
	logger.Infof("Sending verification email to %s with token: %s", user.Email, token)
}

func (s *AuthService) sendPasswordResetEmail(user *models.User, token string) {
	// Implement email sending logic
	resetURL := fmt.Sprintf("https://example.com/reset-password?token=%s", token)
	logger.Infof("Sending password reset email to %s with URL: %s", user.Email, resetURL)
}

func (s *AuthService) logLoginAttempt(userID uint, ipAddress string, success bool) {
	// Implement audit logging
	logger.Infof("Login attempt for user %d from IP %s: success=%v", userID, ipAddress, success)
}

// IsTokenBlacklisted checks if a token is blacklisted
func (s *AuthService) IsTokenBlacklisted(token string) bool {
	if s.redis == nil {
		return false
	}

	exists, err := s.redis.Exists(fmt.Sprintf("blacklist:%s", token))
	return err == nil && exists > 0
}
