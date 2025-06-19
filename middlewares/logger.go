package middleware

import (
	"bytes"
	"io"
	"strings"
	"time"

	"go-api-boilerplate/pkg/logger"
	"go-api-boilerplate/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// LoggerMiddleware logs HTTP requests
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get status code
		statusCode := c.Writer.Status()

		// Get client IP
		clientIP := c.ClientIP()

		// Get user ID if authenticated
		userID, _ := c.Get("user_id")

		// Create log fields
		fields := logrus.Fields{
			"status":     statusCode,
			"method":     c.Request.Method,
			"path":       path,
			"query":      raw,
			"ip":         clientIP,
			"user_agent": c.Request.UserAgent(),
			"latency":    latency.String(),
			"latency_ms": latency.Milliseconds(),
		}

		if userID != nil {
			fields["user_id"] = userID
		}

		// Add error if exists
		if len(c.Errors) > 0 {
			fields["error"] = c.Errors.String()
		}

		// Log based on status code
		log := logger.Get().WithFields(fields)
		msg := "HTTP Request"

		if statusCode >= 500 {
			log.Error(msg)
		} else if statusCode >= 400 {
			log.Warn(msg)
		} else {
			log.Info(msg)
		}
	}
}

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request ID exists in header
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Set request ID in context and response header
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		// Add request ID to logger
		logger.Get().WithField("request_id", requestID)

		c.Next()
	}
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return utils.GenerateUUID()
}

// ResponseBodyLoggerMiddleware logs response bodies (use with caution in production)
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// BodyLoggerMiddleware logs request and response bodies (use with caution)
func BodyLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip for file uploads and downloads
		contentType := c.GetHeader("Content-Type")
		if strings.Contains(contentType, "multipart/form-data") ||
			strings.Contains(c.Request.URL.Path, "/download") ||
			strings.Contains(c.Request.URL.Path, "/stream") {
			c.Next()
			return
		}

		// Read request body
		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// Create response body writer
		w := &responseBodyWriter{body: &bytes.Buffer{}, ResponseWriter: c.Writer}
		c.Writer = w

		// Process request
		c.Next()

		// Log request and response
		fields := logrus.Fields{
			"method":        c.Request.Method,
			"path":          c.Request.URL.Path,
			"request_body":  maskSensitiveData(string(requestBody)),
			"response_body": maskSensitiveData(w.body.String()),
			"status":        c.Writer.Status(),
		}

		logger.Get().WithFields(fields).Debug("Request/Response Body")
	}
}

// maskSensitiveData masks sensitive information in logs
func maskSensitiveData(data string) string {
	// Define sensitive field patterns
	sensitiveFields := []string{
		"password",
		"token",
		"secret",
		"authorization",
		"api_key",
		"credit_card",
		"ssn",
	}

	masked := data
	for _, field := range sensitiveFields {
		// Simple masking - in production, use more sophisticated masking
		if strings.Contains(strings.ToLower(masked), field) {
			// This is a basic implementation - improve based on your needs
			masked = strings.ReplaceAll(masked, field, field+"_MASKED")
		}
	}

	return masked
}

// ErrorLoggerMiddleware logs errors with stack traces
func ErrorLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Log errors if any
		for _, err := range c.Errors {
			fields := logrus.Fields{
				"error":  err.Error(),
				"type":   err.Type,
				"meta":   err.Meta,
				"path":   c.Request.URL.Path,
				"method": c.Request.Method,
				"status": c.Writer.Status(),
				"ip":     c.ClientIP(),
			}

			// Add user context if available
			if userID, exists := c.Get("user_id"); exists {
				fields["user_id"] = userID
			}

			// Add request ID if available
			if requestID, exists := c.Get("request_id"); exists {
				fields["request_id"] = requestID
			}

			logger.Get().WithFields(fields).Error("Request error")
		}
	}
}

// AuditLogMiddleware logs security-relevant events
func AuditLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Define paths that should be audited
		auditPaths := []string{
			"/auth/login",
			"/auth/logout",
			"/auth/register",
			"/users/delete",
			"/admin",
		}

		path := c.Request.URL.Path
		shouldAudit := false

		for _, auditPath := range auditPaths {
			if strings.Contains(path, auditPath) {
				shouldAudit = true
				break
			}
		}

		if shouldAudit {
			// Capture request info before processing
			method := c.Request.Method
			ip := c.ClientIP()
			userAgent := c.Request.UserAgent()

			// Process request
			c.Next()

			// Log audit event
			fields := logrus.Fields{
				"event_type": "audit",
				"method":     method,
				"path":       path,
				"ip":         ip,
				"user_agent": userAgent,
				"status":     c.Writer.Status(),
				"timestamp":  time.Now().UTC(),
			}

			// Add user context
			if userID, exists := c.Get("user_id"); exists {
				fields["user_id"] = userID
			}
			if userEmail, exists := c.Get("user_email"); exists {
				fields["user_email"] = userEmail
			}

			logger.Get().WithFields(fields).Info("Audit log")
		} else {
			c.Next()
		}
	}
}

// MetricsMiddleware collects metrics for monitoring
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Collect metrics
		duration := time.Since(start)
		status := c.Writer.Status()
		path := c.FullPath()
		method := c.Request.Method

		// Log metrics (in production, send to metrics system)
		fields := logrus.Fields{
			"type":        "metrics",
			"method":      method,
			"path":        path,
			"status":      status,
			"duration_ms": duration.Milliseconds(),
		}

		logger.Get().WithFields(fields).Debug("Request metrics")
	}
}
