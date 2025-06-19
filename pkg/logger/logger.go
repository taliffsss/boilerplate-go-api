package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go-api-boilerplate/config"

	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

// Init initializes the logger
func Init(cfg *config.Config) error {
	log = logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(cfg.Log.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	log.SetLevel(level)

	// Set log format
	if cfg.Log.Format == "json" {
		log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
			ForceColors:     true,
		})
	}

	// Set output
	switch cfg.Log.Output {
	case "file":
		file, err := setupLogFile(cfg.Log.FilePath)
		if err != nil {
			return fmt.Errorf("failed to setup log file: %w", err)
		}
		log.SetOutput(file)
	case "stdout":
		log.SetOutput(os.Stdout)
	default:
		// Use multi-writer for both stdout and file
		file, err := setupLogFile(cfg.Log.FilePath)
		if err != nil {
			return fmt.Errorf("failed to setup log file: %w", err)
		}
		multiWriter := io.MultiWriter(os.Stdout, file)
		log.SetOutput(multiWriter)
	}

	// Add caller information in debug mode
	if cfg.IsDebug() {
		log.SetReportCaller(true)
		log.Formatter = &CustomFormatter{
			Formatter: log.Formatter,
		}
	}

	// Add default fields
	log = log.WithFields(logrus.Fields{
		"app":         cfg.App.Name,
		"environment": cfg.App.Env,
	}).Logger

	return nil
}

// setupLogFile creates and opens a log file
func setupLogFile(filePath string) (*os.File, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	// Open file with appropriate flags
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// Get returns the logger instance
func Get() *logrus.Logger {
	if log == nil {
		// Return a default logger if not initialized
		defaultLogger := logrus.New()
		defaultLogger.SetLevel(logrus.InfoLevel)
		defaultLogger.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		})
		return defaultLogger
	}
	return log
}

// WithField creates an entry with a single field
func WithField(key string, value interface{}) *logrus.Entry {
	return Get().WithField(key, value)
}

// WithFields creates an entry with multiple fields
func WithFields(fields logrus.Fields) *logrus.Entry {
	return Get().WithFields(fields)
}

// WithError adds an error field to the log entry
func WithError(err error) *logrus.Entry {
	return Get().WithError(err)
}

// Debug logs a debug message
func Debug(args ...interface{}) {
	Get().Debug(args...)
}

// Debugf logs a formatted debug message
func Debugf(format string, args ...interface{}) {
	Get().Debugf(format, args...)
}

// Info logs an info message
func Info(args ...interface{}) {
	Get().Info(args...)
}

// Infof logs a formatted info message
func Infof(format string, args ...interface{}) {
	Get().Infof(format, args...)
}

// Warn logs a warning message
func Warn(args ...interface{}) {
	Get().Warn(args...)
}

// Warnf logs a formatted warning message
func Warnf(format string, args ...interface{}) {
	Get().Warnf(format, args...)
}

// Error logs an error message
func Error(args ...interface{}) {
	Get().Error(args...)
}

// Errorf logs a formatted error message
func Errorf(format string, args ...interface{}) {
	Get().Errorf(format, args...)
}

// Fatal logs a fatal message and exits
func Fatal(args ...interface{}) {
	Get().Fatal(args...)
}

// Fatalf logs a formatted fatal message and exits
func Fatalf(format string, args ...interface{}) {
	Get().Fatalf(format, args...)
}

// Panic logs a panic message and panics
func Panic(args ...interface{}) {
	Get().Panic(args...)
}

// Panicf logs a formatted panic message and panics
func Panicf(format string, args ...interface{}) {
	Get().Panicf(format, args...)
}

// CustomFormatter adds caller information to logs
type CustomFormatter struct {
	Formatter logrus.Formatter
}

// Format formats the log entry
func (f *CustomFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// Add caller information
	if entry.HasCaller() {
		frame := getCaller()
		entry.Data["caller"] = fmt.Sprintf("%s:%d %s()", frame.File, frame.Line, frame.Function)
	}

	// Use the underlying formatter
	return f.Formatter.Format(entry)
}

// getCaller retrieves the caller information
func getCaller() *runtime.Frame {
	// Skip the logrus stack frames
	skip := 7
	for {
		pc, file, line, ok := runtime.Caller(skip)
		if !ok {
			break
		}

		// Skip logrus internal calls
		if !strings.Contains(file, "sirupsen/logrus") &&
			!strings.Contains(file, "pkg/logger") {
			f := runtime.Frame{
				PC:       pc,
				File:     filepath.Base(file),
				Line:     line,
				Function: runtime.FuncForPC(pc).Name(),
			}
			return &f
		}

		skip++
	}

	return &runtime.Frame{}
}

// NewLogger creates a new logger instance with custom configuration
func NewLogger(name string) *logrus.Logger {
	logger := logrus.New()

	// Copy configuration from main logger
	if log != nil {
		logger.SetLevel(log.Level)
		logger.SetFormatter(log.Formatter)
		logger.SetOutput(log.Out)
	}

	// Add name field
	logger = logger.WithField("logger", name).Logger

	return logger
}

// RotateLogFile rotates the log file
func RotateLogFile(filePath string) error {
	// Close current file if it's open
	if file, ok := log.Out.(*os.File); ok && file != os.Stdout && file != os.Stderr {
		file.Close()
	}

	// Rename current log file
	timestamp := time.Now().Format("20060102_150405")
	rotatedPath := fmt.Sprintf("%s.%s", filePath, timestamp)

	if err := os.Rename(filePath, rotatedPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to rotate log file: %w", err)
	}

	// Open new log file
	file, err := setupLogFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to create new log file: %w", err)
	}

	log.SetOutput(file)
	return nil
}
