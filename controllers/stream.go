package controllers

import (
	"path/filepath"
	"strings"

	"go-api-boilerplate/config"
	"go-api-boilerplate/services"
	"go-api-boilerplate/utils"

	"github.com/gin-gonic/gin"
)

// StreamController handles video streaming requests
type StreamController struct {
	streamService *services.StreamService
}

// NewStreamController creates a new stream handler
func NewStreamController(streamService *services.StreamService) *StreamController {
	return &StreamController{
		streamService: streamService,
	}
}

// StreamVideo godoc
// @Summary Stream video
// @Description Stream a video file with range support
// @Tags streaming
// @Security Bearer
// @Param id path string true "Video ID"
// @Param Range header string false "Range header for partial content"
// @Success 200 {file} binary
// @Success 206 {file} binary
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /stream/video/{id} [get]
func (h *StreamController) StreamVideo(c *gin.Context) {
	videoID := c.Param("id")
	if videoID == "" {
		utils.BadRequestResponse(c, "Video ID is required", nil)
		return
	}

	// In a real implementation, you would:
	// 1. Validate user has access to this video
	// 2. Get video path from database
	// 3. Check if video exists

	// For now, construct path from ID
	cfg := config.Get()
	videoPath := filepath.Join(cfg.Stream.Path, videoID+".mp4")

	// Stream the video
	if err := h.streamService.StreamVideo(c, videoPath); err != nil {
		if strings.Contains(err.Error(), "not found") {
			utils.NotFoundResponse(c, "Video")
			return
		}
		utils.InternalServerErrorResponse(c, "Failed to stream video")
		return
	}
}

// StreamHLS godoc
// @Summary Stream HLS content
// @Description Stream HLS playlist or segments
// @Tags streaming
// @Security Bearer
// @Param id path string true "Video ID"
// @Param path path string true "HLS file path"
// @Success 200 {file} binary
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /stream/hls/{id}/{path} [get]
func (h *StreamController) StreamHLS(c *gin.Context) {
	videoID := c.Param("id")
	hlsPath := c.Param("path")

	if videoID == "" || hlsPath == "" {
		utils.BadRequestResponse(c, "Video ID and path are required", nil)
		return
	}

	// Construct full path
	cfg := config.Get()
	fullPath := filepath.Join(cfg.Stream.Path, "hls", videoID, hlsPath)

	// Stream HLS content
	if err := h.streamService.StreamHLS(c, fullPath); err != nil {
		if strings.Contains(err.Error(), "not found") {
			utils.NotFoundResponse(c, "HLS content")
			return
		}
		utils.InternalServerErrorResponse(c, "Failed to stream HLS content")
		return
	}
}

// GetVideoInfo godoc
// @Summary Get video information
// @Description Get metadata about a video file
// @Tags streaming
// @Security Bearer
// @Param id path string true "Video ID"
// @Success 200 {object} services.VideoInfo
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /stream/info/{id} [get]
func (h *StreamController) GetVideoInfo(c *gin.Context) {
	videoID := c.Param("id")
	if videoID == "" {
		utils.BadRequestResponse(c, "Video ID is required", nil)
		return
	}

	// Construct path
	cfg := config.Get()
	videoPath := filepath.Join(cfg.Stream.Path, videoID+".mp4")

	// Get video info
	info, err := h.streamService.GetVideoInfo(videoPath)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			utils.NotFoundResponse(c, "Video")
			return
		}
		utils.InternalServerErrorResponse(c, "Failed to get video info")
		return
	}

	utils.SuccessResponse(c, "Video info retrieved successfully", info)
}
