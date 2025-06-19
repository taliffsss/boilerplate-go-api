package services

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go-api-boilerplate/config"
	"go-api-boilerplate/pkg/logger"

	"github.com/gin-gonic/gin"
)

// StreamService handles video streaming operations
type StreamService struct {
	config *config.Config
}

// NewStreamService creates a new stream service
func NewStreamService() *StreamService {
	return &StreamService{
		config: config.Get(),
	}
}

// StreamVideo handles video streaming with range requests
func (s *StreamService) StreamVideo(c *gin.Context, videoPath string) error {
	// Validate video path
	if err := s.validateVideoPath(videoPath); err != nil {
		return err
	}

	// Open video file
	video, err := os.Open(videoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("video not found")
		}
		return fmt.Errorf("failed to open video: %w", err)
	}
	defer video.Close()

	// Get file info
	stat, err := video.Stat()
	if err != nil {
		return fmt.Errorf("failed to get video info: %w", err)
	}

	// Get file size
	fileSize := stat.Size()

	// Parse range header
	rangeHeader := c.GetHeader("Range")
	if rangeHeader == "" {
		// No range requested, send entire file
		s.serveFullVideo(c, video, fileSize)
		return nil
	}

	// Parse range values
	start, end, err := s.parseRange(rangeHeader, fileSize)
	if err != nil {
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return err
	}

	// Serve partial content
	return s.servePartialVideo(c, video, start, end, fileSize)
}

// validateVideoPath validates and sanitizes the video path
func (s *StreamService) validateVideoPath(videoPath string) error {
	// Get absolute path
	absPath, err := filepath.Abs(videoPath)
	if err != nil {
		return fmt.Errorf("invalid video path: %w", err)
	}

	// Get stream directory absolute path
	streamDir, err := filepath.Abs(s.config.Stream.Path)
	if err != nil {
		return fmt.Errorf("invalid stream directory: %w", err)
	}

	// Ensure video is within stream directory
	if !strings.HasPrefix(absPath, streamDir) {
		return fmt.Errorf("video path is outside stream directory")
	}

	// Check if file exists
	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("video not found")
		}
		return fmt.Errorf("failed to access video: %w", err)
	}

	return nil
}

// parseRange parses the Range header
func (s *StreamService) parseRange(rangeHeader string, fileSize int64) (start, end int64, err error) {
	// Range format: bytes=start-end
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return 0, 0, fmt.Errorf("invalid range format")
	}

	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(rangeSpec, "-")

	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range format")
	}

	// Parse start
	if parts[0] != "" {
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid start position")
		}
	}

	// Parse end
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid end position")
		}
	} else {
		// If end is not specified, use chunk size or file size
		end = start + s.config.Stream.ChunkSize - 1
		if end >= fileSize {
			end = fileSize - 1
		}
	}

	// Validate range
	if start > end || start < 0 || end >= fileSize {
		return 0, 0, fmt.Errorf("invalid range")
	}

	return start, end, nil
}

// serveFullVideo serves the entire video file
func (s *StreamService) serveFullVideo(c *gin.Context, video *os.File, fileSize int64) {
	// Set headers
	c.Header("Content-Type", s.getContentType(video.Name()))
	c.Header("Content-Length", fmt.Sprintf("%d", fileSize))
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "no-cache")

	// Copy file to response
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, video); err != nil {
		logger.WithError(err).Error("Failed to stream video")
	}
}

// servePartialVideo serves a partial video content
func (s *StreamService) servePartialVideo(c *gin.Context, video *os.File, start, end, fileSize int64) error {
	// Seek to start position
	if _, err := video.Seek(start, 0); err != nil {
		return fmt.Errorf("failed to seek video: %w", err)
	}

	// Set headers for partial content
	c.Header("Content-Type", s.getContentType(video.Name()))
	c.Header("Content-Length", fmt.Sprintf("%d", end-start+1))
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "no-cache")

	// Set status code for partial content
	c.Status(http.StatusPartialContent)

	// Create limited reader
	limitedReader := io.LimitReader(video, end-start+1)

	// Copy content to response
	written, err := io.Copy(c.Writer, limitedReader)
	if err != nil {
		logger.WithError(err).Error("Failed to stream video chunk")
		return fmt.Errorf("failed to stream video: %w", err)
	}

	logger.Debugf("Streamed %d bytes for range %d-%d", written, start, end)
	return nil
}

// getContentType determines the content type based on file extension
func (s *StreamService) getContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".ogg":
		return "video/ogg"
	case ".avi":
		return "video/x-msvideo"
	case ".mov":
		return "video/quicktime"
	case ".mkv":
		return "video/x-matroska"
	case ".flv":
		return "video/x-flv"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".m3u8":
		return "application/x-mpegURL"
	case ".ts":
		return "video/MP2T"
	default:
		return "application/octet-stream"
	}
}

// StreamHLS handles HLS (HTTP Live Streaming) requests
func (s *StreamService) StreamHLS(c *gin.Context, playlistPath string) error {
	// Validate playlist path
	if err := s.validateVideoPath(playlistPath); err != nil {
		return err
	}

	// Check if it's a playlist or segment request
	if strings.HasSuffix(playlistPath, ".m3u8") {
		return s.serveHLSPlaylist(c, playlistPath)
	} else if strings.HasSuffix(playlistPath, ".ts") {
		return s.serveHLSSegment(c, playlistPath)
	}

	return fmt.Errorf("unsupported HLS file type")
}

// serveHLSPlaylist serves HLS playlist files
func (s *StreamService) serveHLSPlaylist(c *gin.Context, playlistPath string) error {
	// Read playlist file
	content, err := os.ReadFile(playlistPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("playlist not found")
		}
		return fmt.Errorf("failed to read playlist: %w", err)
	}

	// Set headers
	c.Header("Content-Type", "application/x-mpegURL")
	c.Header("Cache-Control", "no-cache")

	// Send content
	c.Data(http.StatusOK, "application/x-mpegURL", content)
	return nil
}

// serveHLSSegment serves HLS segment files
func (s *StreamService) serveHLSSegment(c *gin.Context, segmentPath string) error {
	// Open segment file
	segment, err := os.Open(segmentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("segment not found")
		}
		return fmt.Errorf("failed to open segment: %w", err)
	}
	defer segment.Close()

	// Get file info
	stat, err := segment.Stat()
	if err != nil {
		return fmt.Errorf("failed to get segment info: %w", err)
	}

	// Set headers
	c.Header("Content-Type", "video/MP2T")
	c.Header("Content-Length", fmt.Sprintf("%d", stat.Size()))
	c.Header("Cache-Control", "max-age=3600")

	// Copy segment to response
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, segment); err != nil {
		logger.WithError(err).Error("Failed to serve HLS segment")
		return fmt.Errorf("failed to serve segment: %w", err)
	}

	return nil
}

// GenerateHLS converts a video file to HLS format
func (s *StreamService) GenerateHLS(inputPath, outputDir string, segmentDuration int) error {
	// This is a placeholder - you would need to use ffmpeg or similar tool
	// Example command:
	// ffmpeg -i input.mp4 -c:v h264 -c:a aac -hls_time 10 -hls_list_size 0 -f hls output.m3u8

	return fmt.Errorf("HLS generation not implemented - requires ffmpeg integration")
}

// GetVideoInfo retrieves information about a video file
type VideoInfo struct {
	Filename     string  `json:"filename"`
	Size         int64   `json:"size"`
	Duration     float64 `json:"duration"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	Codec        string  `json:"codec"`
	Bitrate      int     `json:"bitrate"`
	FrameRate    float64 `json:"frame_rate"`
	HasAudio     bool    `json:"has_audio"`
	AudioCodec   string  `json:"audio_codec,omitempty"`
	AudioBitrate int     `json:"audio_bitrate,omitempty"`
}

// GetVideoInfo retrieves metadata about a video file
func (s *StreamService) GetVideoInfo(videoPath string) (*VideoInfo, error) {
	// Validate path
	if err := s.validateVideoPath(videoPath); err != nil {
		return nil, err
	}

	// Get file info
	stat, err := os.Stat(videoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Basic info
	info := &VideoInfo{
		Filename: filepath.Base(videoPath),
		Size:     stat.Size(),
	}

	// For full metadata extraction, you would need to use a library like:
	// - github.com/alfg/mp4
	// - ffprobe integration

	return info, nil
}

// GenerateThumbnail generates a thumbnail for a video
func (s *StreamService) GenerateThumbnail(videoPath string, outputPath string, timestamp float64) error {
	// This is a placeholder - you would need to use ffmpeg or similar tool
	// Example command:
	// ffmpeg -i input.mp4 -ss 00:00:10 -vframes 1 -q:v 2 output.jpg

	return fmt.Errorf("thumbnail generation not implemented - requires ffmpeg integration")
}

// StreamLive handles live streaming (WebRTC/RTMP)
func (s *StreamService) StreamLive(c *gin.Context, streamKey string) error {
	// This is a placeholder for live streaming implementation
	// You would need to integrate with:
	// - WebRTC for browser-based streaming
	// - RTMP server for traditional streaming
	// - HLS/DASH for adaptive streaming

	return fmt.Errorf("live streaming not implemented")
}

// BufferedReader implements a buffered reader for smooth streaming
type BufferedReader struct {
	reader    io.Reader
	buffer    []byte
	bufferPos int
	bufferLen int
}

// NewBufferedReader creates a new buffered reader
func NewBufferedReader(reader io.Reader, bufferSize int) *BufferedReader {
	return &BufferedReader{
		reader: reader,
		buffer: make([]byte, bufferSize),
	}
}

// Read implements io.Reader interface with buffering
func (br *BufferedReader) Read(p []byte) (n int, err error) {
	if br.bufferPos >= br.bufferLen {
		// Refill buffer
		br.bufferLen, err = br.reader.Read(br.buffer)
		if err != nil {
			return 0, err
		}
		br.bufferPos = 0
	}

	// Copy from buffer to p
	n = copy(p, br.buffer[br.bufferPos:br.bufferLen])
	br.bufferPos += n
	return n, nil
}

// StreamWithBuffer streams video with buffering for smooth playback
func (s *StreamService) StreamWithBuffer(c *gin.Context, videoPath string) error {
	// Validate video path
	if err := s.validateVideoPath(videoPath); err != nil {
		return err
	}

	// Open video file
	video, err := os.Open(videoPath)
	if err != nil {
		return fmt.Errorf("failed to open video: %w", err)
	}
	defer video.Close()

	// Get file info
	stat, err := video.Stat()
	if err != nil {
		return fmt.Errorf("failed to get video info: %w", err)
	}

	// Create buffered reader
	bufferedReader := NewBufferedReader(video, int(s.config.Stream.BufferSize))

	// Set headers
	c.Header("Content-Type", s.getContentType(video.Name()))
	c.Header("Content-Length", fmt.Sprintf("%d", stat.Size()))
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "no-cache")

	// Stream with buffer
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, bufferedReader); err != nil {
		logger.WithError(err).Error("Failed to stream video with buffer")
		return fmt.Errorf("streaming failed: %w", err)
	}

	return nil
}

// AdaptiveBitrate handles adaptive bitrate streaming
type QualityLevel struct {
	Name    string `json:"name"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Bitrate int    `json:"bitrate"`
	Path    string `json:"path"`
}

// GetAvailableQualities returns available quality levels for a video
func (s *StreamService) GetAvailableQualities(videoID string) ([]QualityLevel, error) {
	// This would check for pre-transcoded versions of the video
	// in different qualities (e.g., 360p, 720p, 1080p, 4K)

	qualities := []QualityLevel{
		{Name: "360p", Width: 640, Height: 360, Bitrate: 800000, Path: fmt.Sprintf("%s_360p.mp4", videoID)},
		{Name: "720p", Width: 1280, Height: 720, Bitrate: 2500000, Path: fmt.Sprintf("%s_720p.mp4", videoID)},
		{Name: "1080p", Width: 1920, Height: 1080, Bitrate: 5000000, Path: fmt.Sprintf("%s_1080p.mp4", videoID)},
	}

	// Filter out non-existent files
	var available []QualityLevel
	for _, q := range qualities {
		path := filepath.Join(s.config.Stream.Path, q.Path)
		if _, err := os.Stat(path); err == nil {
			available = append(available, q)
		}
	}

	return available, nil
}

// TranscodeVideo transcodes video to different qualities
func (s *StreamService) TranscodeVideo(inputPath string, qualities []QualityLevel) error {
	// This is a placeholder for video transcoding
	// You would use ffmpeg or similar tool to transcode videos
	// Example:
	// ffmpeg -i input.mp4 -vf scale=1280:720 -c:v h264 -b:v 2500k -c:a copy output_720p.mp4

	return fmt.Errorf("video transcoding not implemented - requires ffmpeg integration")
}

// CleanupOldStreams removes old streaming files
func (s *StreamService) CleanupOldStreams(olderThan time.Duration) error {
	cutoffTime := time.Now().Add(-olderThan)

	return filepath.Walk(s.config.Stream.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file is a temporary streaming file
		if strings.Contains(path, ".tmp") || strings.Contains(path, ".ts") {
			if info.ModTime().Before(cutoffTime) {
				if err := os.Remove(path); err != nil {
					logger.WithError(err).Warnf("Failed to remove old stream file: %s", path)
				}
			}
		}

		return nil
	})
}
