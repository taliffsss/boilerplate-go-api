package services

import (
	"crypto/md5"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-api-boilerplate/config"
	"go-api-boilerplate/utils"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
)

// UploadService handles file upload operations
type UploadService struct {
	config *config.Config
}

// NewUploadService creates a new upload service
func NewUploadService() *UploadService {
	return &UploadService{
		config: config.Get(),
	}
}

// FileInfo represents uploaded file information
type FileInfo struct {
	Filename     string    `json:"filename"`
	OriginalName string    `json:"original_name"`
	Size         int64     `json:"size"`
	MimeType     string    `json:"mime_type"`
	Extension    string    `json:"extension"`
	Path         string    `json:"path"`
	URL          string    `json:"url"`
	Hash         string    `json:"hash"`
	UploadedAt   time.Time `json:"uploaded_at"`
}

// UploadFile handles single file upload
func (s *UploadService) UploadFile(c *gin.Context, formField string) (*FileInfo, error) {
	// Get file from form
	file, header, err := c.Request.FormFile(formField)
	if err != nil {
		return nil, fmt.Errorf("failed to get file from form: %w", err)
	}
	defer file.Close()

	// Validate file size
	if header.Size > s.config.Upload.MaxSize {
		return nil, fmt.Errorf("file size exceeds maximum allowed size of %d bytes", s.config.Upload.MaxSize)
	}

	// Detect MIME type
	mtype, err := s.detectMimeType(file)
	if err != nil {
		return nil, fmt.Errorf("failed to detect file type: %w", err)
	}

	// Validate MIME type
	if !s.isAllowedType(mtype.String()) {
		return nil, fmt.Errorf("file type %s is not allowed", mtype.String())
	}

	// Generate unique filename
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = mtype.Extension()
	}
	filename := s.generateUniqueFilename(ext)

	// Create upload directory
	uploadPath := s.getUploadPath()
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Save file
	filePath := filepath.Join(uploadPath, filename)
	fileInfo, err := s.saveFile(file, filePath, header)
	if err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// Update file info
	fileInfo.OriginalName = header.Filename
	fileInfo.MimeType = mtype.String()
	fileInfo.Extension = ext
	fileInfo.URL = s.getFileURL(filename)

	return fileInfo, nil
}

// UploadMultipleFiles handles multiple file uploads
func (s *UploadService) UploadMultipleFiles(c *gin.Context, formField string) ([]*FileInfo, error) {
	form, err := c.MultipartForm()
	if err != nil {
		return nil, fmt.Errorf("failed to parse multipart form: %w", err)
	}

	files := form.File[formField]
	if len(files) == 0 {
		return nil, fmt.Errorf("no files found in form field: %s", formField)
	}

	var uploadedFiles []*FileInfo
	var errors []string

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: failed to open", fileHeader.Filename))
			continue
		}
		defer file.Close()

		// Process each file
		fileInfo, err := s.processUploadedFile(file, fileHeader)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", fileHeader.Filename, err))
			continue
		}

		uploadedFiles = append(uploadedFiles, fileInfo)
	}

	if len(errors) > 0 {
		return uploadedFiles, fmt.Errorf("some files failed to upload: %s", strings.Join(errors, "; "))
	}

	return uploadedFiles, nil
}

// processUploadedFile processes a single uploaded file
func (s *UploadService) processUploadedFile(file multipart.File, header *multipart.FileHeader) (*FileInfo, error) {
	// Validate file size
	if header.Size > s.config.Upload.MaxSize {
		return nil, fmt.Errorf("file size exceeds maximum allowed size")
	}

	// Detect MIME type
	mtype, err := s.detectMimeType(file)
	if err != nil {
		return nil, fmt.Errorf("failed to detect file type: %w", err)
	}

	// Validate MIME type
	if !s.isAllowedType(mtype.String()) {
		return nil, fmt.Errorf("file type not allowed")
	}

	// Generate unique filename
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = mtype.Extension()
	}
	filename := s.generateUniqueFilename(ext)

	// Create upload directory
	uploadPath := s.getUploadPath()
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Save file
	filePath := filepath.Join(uploadPath, filename)
	fileInfo, err := s.saveFile(file, filePath, header)
	if err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// Update file info
	fileInfo.OriginalName = header.Filename
	fileInfo.MimeType = mtype.String()
	fileInfo.Extension = ext
	fileInfo.URL = s.getFileURL(filename)

	return fileInfo, nil
}

// saveFile saves the uploaded file to disk
func (s *UploadService) saveFile(src multipart.File, destPath string, header *multipart.FileHeader) (*FileInfo, error) {
	// Create destination file
	dst, err := os.Create(destPath)
	if err != nil {
		return nil, err
	}
	defer dst.Close()

	// Reset file pointer
	if _, err := src.Seek(0, 0); err != nil {
		return nil, err
	}

	// Create hash calculator
	hash := md5.New()

	// Copy file and calculate hash
	writer := io.MultiWriter(dst, hash)
	size, err := io.Copy(writer, src)
	if err != nil {
		os.Remove(destPath) // Clean up on error
		return nil, err
	}

	// Calculate final hash
	hashSum := fmt.Sprintf("%x", hash.Sum(nil))

	return &FileInfo{
		Filename:   filepath.Base(destPath),
		Size:       size,
		Path:       destPath,
		Hash:       hashSum,
		UploadedAt: time.Now(),
	}, nil
}

// detectMimeType detects the MIME type of a file
func (s *UploadService) detectMimeType(file multipart.File) (*mimetype.MIME, error) {
	// Reset file pointer
	if _, err := file.Seek(0, 0); err != nil {
		return nil, err
	}

	// Read first 512 bytes for detection
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return nil, err
	}

	// Reset file pointer again
	if _, err := file.Seek(0, 0); err != nil {
		return nil, err
	}

	// Detect MIME type
	return mimetype.Detect(buffer[:n]), nil
}

// isAllowedType checks if a MIME type is allowed
func (s *UploadService) isAllowedType(mimeType string) bool {
	for _, allowed := range s.config.Upload.AllowedTypes {
		if allowed == mimeType {
			return true
		}
		// Check for wildcard patterns
		if strings.Contains(allowed, "*") {
			pattern := strings.Replace(allowed, "*", "", -1)
			if strings.HasPrefix(mimeType, pattern) {
				return true
			}
		}
	}
	return false
}

// generateUniqueFilename generates a unique filename
func (s *UploadService) generateUniqueFilename(extension string) string {
	timestamp := time.Now().Unix()
	random := utils.GenerateRandomString(8)
	return fmt.Sprintf("%d_%s%s", timestamp, random, extension)
}

// getUploadPath returns the upload directory path
func (s *UploadService) getUploadPath() string {
	// Create date-based subdirectory
	now := time.Now()
	subDir := fmt.Sprintf("%d/%02d/%02d", now.Year(), now.Month(), now.Day())
	return filepath.Join(s.config.Upload.Path, subDir)
}

// getFileURL returns the URL for accessing a file
func (s *UploadService) getFileURL(filename string) string {
	// This should be configured based on your setup
	// For example, if serving files through a CDN or specific route
	return fmt.Sprintf("/uploads/%s", filename)
}

// DeleteFile deletes a file from storage
func (s *UploadService) DeleteFile(filePath string) error {
	// Ensure the file is within the upload directory
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	uploadDir, err := filepath.Abs(s.config.Upload.Path)
	if err != nil {
		return fmt.Errorf("invalid upload directory: %w", err)
	}

	// Security check: ensure file is within upload directory
	if !strings.HasPrefix(absPath, uploadDir) {
		return fmt.Errorf("file path is outside upload directory")
	}

	// Delete the file
	if err := os.Remove(absPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found")
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// GetFileInfo retrieves information about a file
func (s *UploadService) GetFileInfo(filePath string) (*FileInfo, error) {
	// Get file stats
	stat, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found")
		}
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Open file for MIME type detection
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Detect MIME type
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	mtype := mimetype.Detect(buffer[:n])

	// Calculate file hash
	file.Seek(0, 0)
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, fmt.Errorf("failed to calculate hash: %w", err)
	}
	hashSum := fmt.Sprintf("%x", hash.Sum(nil))

	return &FileInfo{
		Filename:   filepath.Base(filePath),
		Size:       stat.Size(),
		MimeType:   mtype.String(),
		Extension:  filepath.Ext(filePath),
		Path:       filePath,
		URL:        s.getFileURL(filepath.Base(filePath)),
		Hash:       hashSum,
		UploadedAt: stat.ModTime(),
	}, nil
}

// ValidateFile validates a file before processing
func (s *UploadService) ValidateFile(file multipart.File, header *multipart.FileHeader) error {
	// Check file size
	if header.Size > s.config.Upload.MaxSize {
		return fmt.Errorf("file size %d exceeds maximum allowed size of %d bytes", header.Size, s.config.Upload.MaxSize)
	}

	// Check file type
	mtype, err := s.detectMimeType(file)
	if err != nil {
		return fmt.Errorf("failed to detect file type: %w", err)
	}

	if !s.isAllowedType(mtype.String()) {
		return fmt.Errorf("file type %s is not allowed", mtype.String())
	}

	return nil
}

// ResizeImage resizes an uploaded image (requires additional image processing library)
func (s *UploadService) ResizeImage(filePath string, width, height int) (string, error) {
	// This is a placeholder - implement actual image resizing logic
	// You might want to use libraries like:
	// - github.com/disintegration/imaging
	// - github.com/nfnt/resize

	// For now, return the original path
	return filePath, nil
}

// CreateThumbnail creates a thumbnail for an image
func (s *UploadService) CreateThumbnail(filePath string, maxWidth, maxHeight int) (string, error) {
	// This is a placeholder - implement actual thumbnail creation
	// You might want to use libraries like:
	// - github.com/disintegration/imaging

	// For now, return the original path
	return filePath, nil
}

// CleanupOldFiles removes files older than specified duration
func (s *UploadService) CleanupOldFiles(olderThan time.Duration) error {
	cutoffTime := time.Now().Add(-olderThan)

	return filepath.Walk(s.config.Upload.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file is older than cutoff time
		if info.ModTime().Before(cutoffTime) {
			if err := os.Remove(path); err != nil {
				// Log error but continue with other files
				return nil
			}
		}

		return nil
	})
}

// GetUploadProgress returns upload progress (for chunked uploads)
type UploadProgress struct {
	FileID       string  `json:"file_id"`
	TotalSize    int64   `json:"total_size"`
	UploadedSize int64   `json:"uploaded_size"`
	Percentage   float64 `json:"percentage"`
	IsComplete   bool    `json:"is_complete"`
}

// ChunkedUpload handles chunked file uploads
func (s *UploadService) ChunkedUpload(c *gin.Context, fileID string, chunkNumber int, totalChunks int) (*FileInfo, error) {
	// This is a placeholder for chunked upload implementation
	// You would need to:
	// 1. Store chunks temporarily
	// 2. Merge chunks when all are received
	// 3. Process the complete file

	return nil, fmt.Errorf("chunked upload not implemented")
}

// ServeFile serves a file for download
func (s *UploadService) ServeFile(c *gin.Context, filePath string) error {
	// Security check
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	uploadDir, err := filepath.Abs(s.config.Upload.Path)
	if err != nil {
		return fmt.Errorf("invalid upload directory: %w", err)
	}

	if !strings.HasPrefix(absPath, uploadDir) {
		return fmt.Errorf("file path is outside upload directory")
	}

	// Check if file exists
	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found")
		}
		return fmt.Errorf("failed to access file: %w", err)
	}

	// Serve the file
	c.File(absPath)
	return nil
}
