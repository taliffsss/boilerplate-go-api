package utils

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Response represents a standard API response
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
}

// ErrorInfo represents error information
type ErrorInfo struct {
	Code    string                 `json:"code,omitempty"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// PaginationMeta represents pagination metadata
type PaginationMeta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Success    bool           `json:"success"`
	Message    string         `json:"message,omitempty"`
	Data       interface{}    `json:"data,omitempty"`
	Pagination PaginationMeta `json:"pagination,omitempty"`
	Error      *ErrorInfo     `json:"error,omitempty"`
}

// SuccessResponse sends a success response
func SuccessResponse(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// CreatedResponse sends a created response
func CreatedResponse(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// NoContentResponse sends a no content response
func NoContentResponse(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// ErrorResponse sends an error response
func ErrorResponse(c *gin.Context, statusCode int, message string, errorCode string, details map[string]interface{}) {
	c.JSON(statusCode, Response{
		Success: false,
		Message: message,
		Error: &ErrorInfo{
			Code:    errorCode,
			Message: message,
			Details: details,
		},
	})
}

// BadRequestResponse sends a bad request response
func BadRequestResponse(c *gin.Context, message string, details map[string]interface{}) {
	ErrorResponse(c, http.StatusBadRequest, message, "BAD_REQUEST", details)
}

// UnauthorizedResponse sends an unauthorized response
func UnauthorizedResponse(c *gin.Context, message string) {
	if message == "" {
		message = "Unauthorized access"
	}
	ErrorResponse(c, http.StatusUnauthorized, message, "UNAUTHORIZED", nil)
}

// ForbiddenResponse sends a forbidden response
func ForbiddenResponse(c *gin.Context, message string) {
	if message == "" {
		message = "Access forbidden"
	}
	ErrorResponse(c, http.StatusForbidden, message, "FORBIDDEN", nil)
}

// NotFoundResponse sends a not found response
func NotFoundResponse(c *gin.Context, resource string) {
	message := "Resource not found"
	if resource != "" {
		message = resource + " not found"
	}
	ErrorResponse(c, http.StatusNotFound, message, "NOT_FOUND", nil)
}

// ConflictResponse sends a conflict response
func ConflictResponse(c *gin.Context, message string, details map[string]interface{}) {
	ErrorResponse(c, http.StatusConflict, message, "CONFLICT", details)
}

// ValidationErrorResponse sends a validation error response
func ValidationErrorResponse(c *gin.Context, errors interface{}) {
	ErrorResponse(c, http.StatusUnprocessableEntity, "Validation failed", "VALIDATION_ERROR", map[string]interface{}{
		"validation_errors": errors,
	})
}

// InternalServerErrorResponse sends an internal server error response
func InternalServerErrorResponse(c *gin.Context, message string) {
	if message == "" {
		message = "Internal server error"
	}
	ErrorResponse(c, http.StatusInternalServerError, message, "INTERNAL_ERROR", nil)
}

// PaginatedSuccessResponse sends a paginated success response
func PaginatedSuccessResponse(c *gin.Context, message string, data interface{}, pagination PaginationMeta) {
	c.JSON(http.StatusOK, PaginatedResponse{
		Success:    true,
		Message:    message,
		Data:       data,
		Pagination: pagination,
	})
}

// CalculatePaginationMeta calculates pagination metadata
func CalculatePaginationMeta(page, perPage int, total int64) PaginationMeta {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	return PaginationMeta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}

// GetPaginationParams extracts pagination parameters from request
func GetPaginationParams(c *gin.Context) (page, perPage int) {
	page = 1
	perPage = 10

	if p, exists := c.GetQuery("page"); exists {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	if pp, exists := c.GetQuery("per_page"); exists {
		if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 && parsed <= 100 {
			perPage = parsed
		}
	}

	return page, perPage
}

// GetOffset calculates the offset for pagination
func GetOffset(page, perPage int) int {
	if page < 1 {
		page = 1
	}
	return (page - 1) * perPage
}

// FileResponse sends a file response
func FileResponse(c *gin.Context, filePath string, fileName string) {
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Header("Content-Type", "application/octet-stream")
	c.File(filePath)
}

// StreamResponse prepares headers for streaming response
func StreamResponse(c *gin.Context, contentType string, contentLength int64) {
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", fmt.Sprintf("%d", contentLength))
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "no-cache")
}

// CustomResponse sends a custom response with any status code
func CustomResponse(c *gin.Context, statusCode int, response interface{}) {
	c.JSON(statusCode, response)
}
