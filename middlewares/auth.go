package middleware

import (
	"fmt"
	"net/http"
	"time"

	"go-api-boilerplate/models"
	"go-api-boilerplate/services"
	"go-api-boilerplate/utils"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates JWT tokens and adds user info to context
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.UnauthorizedResponse(c, "Authorization header is required")
			c.Abort()
			return
		}

		// Extract token from header
		token, err := utils.ExtractTokenFromHeader(authHeader)
		if err != nil {
			utils.UnauthorizedResponse(c, err.Error())
			c.Abort()
			return
		}

		// Validate token
		claims, err := utils.ValidateToken(token)
		if err != nil {
			if utils.IsTokenExpired(err) {
				utils.UnauthorizedResponse(c, "Token has expired")
			} else {
				utils.UnauthorizedResponse(c, "Invalid token")
			}
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_name", claims.Name)
		c.Set("user_role", claims.Role)
		c.Set("is_active", claims.IsActive)

		c.Next()
	}
}

// OptionalAuthMiddleware validates JWT tokens if present but doesn't require them
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		// Extract token from header
		token, err := utils.ExtractTokenFromHeader(authHeader)
		if err != nil {
			c.Next()
			return
		}

		// Validate token
		claims, err := utils.ValidateToken(token)
		if err != nil {
			c.Next()
			return
		}

		// Set user info in context
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_name", claims.Name)
		c.Set("user_role", claims.Role)
		c.Set("is_active", claims.IsActive)

		c.Next()
	}
}

// RequireRole checks if the user has the required role
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user role from context
		userRole, exists := c.Get("user_role")
		if !exists {
			utils.ForbiddenResponse(c, "Role information not found")
			c.Abort()
			return
		}

		// Check if user has any of the required roles
		hasRole := false
		for _, role := range roles {
			if userRole == role {
				hasRole = true
				break
			}
		}

		if !hasRole {
			utils.ForbiddenResponse(c, "Insufficient permissions")
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireActiveUser ensures the user account is active
func RequireActiveUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		isActive, exists := c.Get("is_active")
		if !exists || !isActive.(bool) {
			utils.ForbiddenResponse(c, "Account is not active")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RateLimitMiddleware implements rate limiting using Redis
func RateLimitMiddleware(limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Initialize Redis service
		redisService, err := services.NewRedisService()
		if err != nil {
			// If Redis is not available, allow the request
			c.Next()
			return
		}

		// Create rate limit key
		var key string
		userID, exists := c.Get("user_id")
		if exists {
			key = fmt.Sprintf("rate_limit:user:%d:%s", userID, c.FullPath())
		} else {
			key = fmt.Sprintf("rate_limit:ip:%s:%s", c.ClientIP(), c.FullPath())
		}

		// Check rate limit
		allowed, remaining, err := redisService.RateLimitCheck(key, limit, window)
		if err != nil {
			// If there's an error, allow the request
			c.Next()
			return
		}

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(window).Unix()))

		if !allowed {
			utils.ErrorResponse(c, http.StatusTooManyRequests, "Rate limit exceeded", "RATE_LIMIT_EXCEEDED", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// APIKeyMiddleware validates API key authentication
func APIKeyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			// Try to get from query parameter
			apiKey = c.Query("api_key")
		}

		if apiKey == "" {
			utils.UnauthorizedResponse(c, "API key is required")
			c.Abort()
			return
		}

		// Validate API key (implement your API key validation logic)
		// This is a placeholder - implement actual API key validation
		if !validateAPIKey(apiKey) {
			utils.UnauthorizedResponse(c, "Invalid API key")
			c.Abort()
			return
		}

		c.Next()
	}
}

// validateAPIKey validates the API key (placeholder implementation)
func validateAPIKey(apiKey string) bool {
	// Implement your API key validation logic here
	// This could involve checking against a database, cache, etc.
	return apiKey != ""
}

// GetUserID gets the user ID from context
func GetUserID(c *gin.Context) (uint, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, fmt.Errorf("user ID not found in context")
	}

	id, ok := userID.(uint)
	if !ok {
		return 0, fmt.Errorf("invalid user ID type")
	}

	return id, nil
}

// GetUserEmail gets the user email from context
func GetUserEmail(c *gin.Context) (string, error) {
	email, exists := c.Get("user_email")
	if !exists {
		return "", fmt.Errorf("user email not found in context")
	}

	emailStr, ok := email.(string)
	if !ok {
		return "", fmt.Errorf("invalid user email type")
	}

	return emailStr, nil
}

// GetUserRole gets the user role from context
func GetUserRole(c *gin.Context) (string, error) {
	role, exists := c.Get("user_role")
	if !exists {
		return "", fmt.Errorf("user role not found in context")
	}

	roleStr, ok := role.(string)
	if !ok {
		return "", fmt.Errorf("invalid user role type")
	}

	return roleStr, nil
}

// IsAuthenticated checks if the user is authenticated
func IsAuthenticated(c *gin.Context) bool {
	_, exists := c.Get("user_id")
	return exists
}

// IsAdmin checks if the user is an admin
func IsAdmin(c *gin.Context) bool {
	role, _ := GetUserRole(c)
	return role == models.RoleAdmin
}

// IsModerator checks if the user is a moderator
func IsModerator(c *gin.Context) bool {
	role, _ := GetUserRole(c)
	return role == models.RoleModerator
}

// HasPermission checks if the user has a specific permission
func HasPermission(c *gin.Context, permission string) bool {
	// Implement permission checking logic here
	// This is a placeholder implementation
	role, _ := GetUserRole(c)

	// Admin has all permissions
	if role == models.RoleAdmin {
		return true
	}

	// Implement specific permission checking based on your requirements
	return false
}

// RequirePermission checks if the user has the required permission
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !HasPermission(c, permission) {
			utils.ForbiddenResponse(c, "Insufficient permissions")
			c.Abort()
			return
		}
		c.Next()
	}
}

// SessionMiddleware validates session-based authentication
func SessionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get session ID from cookie
		sessionID, err := c.Cookie("session_id")
		if err != nil || sessionID == "" {
			utils.UnauthorizedResponse(c, "Session not found")
			c.Abort()
			return
		}

		// Initialize Redis service
		redisService, err := services.NewRedisService()
		if err != nil {
			utils.InternalServerErrorResponse(c, "Failed to connect to session store")
			c.Abort()
			return
		}

		// Get session data
		var sessionData map[string]interface{}
		if err := redisService.SessionGet(sessionID, &sessionData); err != nil {
			utils.UnauthorizedResponse(c, "Invalid or expired session")
			c.Abort()
			return
		}

		// Set user info in context
		if userID, ok := sessionData["user_id"].(float64); ok {
			c.Set("user_id", uint(userID))
		}
		if email, ok := sessionData["email"].(string); ok {
			c.Set("user_email", email)
		}
		if name, ok := sessionData["name"].(string); ok {
			c.Set("user_name", name)
		}
		if role, ok := sessionData["role"].(string); ok {
			c.Set("user_role", role)
		}
		if isActive, ok := sessionData["is_active"].(bool); ok {
			c.Set("is_active", isActive)
		}

		// Extend session expiration
		_ = redisService.SessionExtend(sessionID, 24*time.Hour)

		c.Next()
	}
}
