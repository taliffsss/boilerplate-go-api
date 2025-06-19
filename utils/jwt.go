package utils

import (
	"errors"
	"fmt"
	"time"

	"go-api-boilerplate/config"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims represents the JWT claims
type JWTClaims struct {
	UserID   uint   `json:"user_id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	IsActive bool   `json:"is_active"`
	jwt.RegisteredClaims
}

// GenerateTokens generates both access and refresh tokens
func GenerateTokens(userID uint, email, name, role string, isActive bool) (*TokenPair, error) {
	cfg := config.Get()

	// Generate access token
	accessToken, err := generateAccessToken(userID, email, name, role, isActive, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token
	refreshToken, err := generateRefreshToken(userID, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(cfg.JWT.Expiry.Seconds()),
	}, nil
}

// generateAccessToken generates an access token
func generateAccessToken(userID uint, email, name, role string, isActive bool, cfg *config.Config) (string, error) {
	now := time.Now()
	expiresAt := now.Add(cfg.JWT.Expiry)

	claims := JWTClaims{
		UserID:   userID,
		Email:    email,
		Name:     name,
		Role:     role,
		IsActive: isActive,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.JWT.Issuer,
			Subject:   fmt.Sprintf("%d", userID),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        GenerateUUID(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWT.Secret))
}

// generateRefreshToken generates a refresh token
func generateRefreshToken(userID uint, cfg *config.Config) (string, error) {
	now := time.Now()
	expiresAt := now.Add(cfg.JWT.RefreshExpiry)

	claims := jwt.RegisteredClaims{
		Issuer:    cfg.JWT.Issuer,
		Subject:   fmt.Sprintf("%d", userID),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		NotBefore: jwt.NewNumericDate(now),
		IssuedAt:  jwt.NewNumericDate(now),
		ID:        GenerateUUID(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWT.Secret))
}

// ValidateToken validates a JWT token and returns the claims
func ValidateToken(tokenString string) (*JWTClaims, error) {
	cfg := config.Get()

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.JWT.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	// Check if user is active
	if !claims.IsActive {
		return nil, errors.New("user account is deactivated")
	}

	return claims, nil
}

// ValidateRefreshToken validates a refresh token
func ValidateRefreshToken(tokenString string) (uint, error) {
	cfg := config.Get()

	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.JWT.Secret), nil
	})

	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return 0, errors.New("invalid refresh token")
	}

	// Parse user ID from subject
	var userID uint
	if _, err := fmt.Sscanf(claims.Subject, "%d", &userID); err != nil {
		return 0, errors.New("invalid user ID in token")
	}

	return userID, nil
}

// ExtractTokenFromHeader extracts the token from the Authorization header
func ExtractTokenFromHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("authorization header is empty")
	}

	// Check if it starts with "Bearer "
	const bearerPrefix = "Bearer "
	if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		return "", errors.New("invalid authorization header format")
	}

	token := authHeader[len(bearerPrefix):]
	if token == "" {
		return "", errors.New("token is empty")
	}

	return token, nil
}

// TokenPair represents a pair of access and refresh tokens
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// ParseTokenWithoutValidation parses a token without validating the signature
// This is useful for extracting claims from expired tokens
func ParseTokenWithoutValidation(tokenString string) (*JWTClaims, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &JWTClaims{})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

// IsTokenExpired checks if a token is expired
func IsTokenExpired(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, jwt.ErrTokenExpired)
}

// GeneratePasswordResetToken generates a token for password reset
func GeneratePasswordResetToken() string {
	return GenerateRandomString(32)
}

// GenerateEmailVerificationToken generates a token for email verification
func GenerateEmailVerificationToken() string {
	return GenerateRandomString(32)
}
