package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"go-api-boilerplate/config"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware handles Cross-Origin Resource Sharing
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Get()

		// Get the origin from the request
		origin := c.GetHeader("Origin")

		// Check if origin is allowed
		if isOriginAllowed(origin, cfg.CORS.AllowedOrigins) {
			c.Header("Access-Control-Allow-Origin", origin)
		} else if contains(cfg.CORS.AllowedOrigins, "*") {
			c.Header("Access-Control-Allow-Origin", "*")
		}

		// Set other CORS headers
		c.Header("Access-Control-Allow-Methods", strings.Join(cfg.CORS.AllowedMethods, ", "))
		c.Header("Access-Control-Allow-Headers", strings.Join(cfg.CORS.AllowedHeaders, ", "))
		c.Header("Access-Control-Expose-Headers", strings.Join(cfg.CORS.ExposedHeaders, ", "))

		if cfg.CORS.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if cfg.CORS.MaxAge > 0 {
			c.Header("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.CORS.MaxAge))
		}

		// Handle preflight requests
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// isOriginAllowed checks if the origin is in the allowed list
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	for _, allowed := range allowedOrigins {
		if allowed == "*" {
			return true
		}
		if allowed == origin {
			return true
		}
		// Support wildcard subdomains
		if strings.HasPrefix(allowed, "*.") {
			domain := allowed[2:]
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}
	return false
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// SecureHeadersMiddleware adds security headers to responses
func SecureHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		c.Header("X-Frame-Options", "DENY")

		// Enable XSS protection
		c.Header("X-XSS-Protection", "1; mode=block")

		// Referrer policy
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline';")

		// Strict Transport Security (HSTS)
		if config.Get().IsProduction() {
			c.Header("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}

		// Feature Policy
		c.Header("Feature-Policy", "camera 'none'; microphone 'none'; geolocation 'self';")

		// Permissions Policy
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=(self)")

		c.Next()
	}
}

// NoCacheMiddleware prevents caching of responses
func NoCacheMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Next()
	}
}

// CacheControlMiddleware sets cache control headers
func CacheControlMiddleware(maxAge int) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAge))
		c.Next()
	}
}
