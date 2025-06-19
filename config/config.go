package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds all configuration for our application
type Config struct {
	App        AppConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	JWT        JWTConfig
	Upload     UploadConfig
	WebSocket  WebSocketConfig
	Stream     StreamConfig
	Encryption EncryptionConfig
	CORS       CORSConfig
	RateLimit  RateLimitConfig
	Log        LogConfig
	Swagger    SwaggerConfig
	Monitoring MonitoringConfig
	AWS        AWSConfig
	SMTP       SMTPConfig
	MongoDB    MongoDBConfig
}

// AppConfig holds application specific configuration
type AppConfig struct {
	Name     string
	Env      string
	Port     string
	GRPCPort string
	Debug    bool
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Driver          string
	Host            string
	Port            string
	Name            string
	User            string
	Password        string
	SSLMode         string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	// Read replica configuration
	ReadHost     string
	ReadPort     string
	ReadUser     string
	ReadPassword string
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host         string
	Port         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret        string
	Expiry        time.Duration
	RefreshExpiry time.Duration
	Issuer        string
}

// UploadConfig holds file upload configuration
type UploadConfig struct {
	MaxSize      int64
	Path         string
	AllowedTypes []string
}

// WebSocketConfig holds WebSocket configuration
type WebSocketConfig struct {
	ReadBufferSize  int
	WriteBufferSize int
	MaxMessageSize  int64
	PingPeriod      time.Duration
	PongWait        time.Duration
}

// StreamConfig holds video streaming configuration
type StreamConfig struct {
	ChunkSize  int64
	BufferSize int64
	Path       string
}

// EncryptionConfig holds encryption configuration
type EncryptionConfig struct {
	Key string
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled  bool
	Requests int
	Duration time.Duration
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level    string
	Format   string
	Output   string
	FilePath string
}

// SwaggerConfig holds Swagger configuration
type SwaggerConfig struct {
	Enabled  bool
	Host     string
	BasePath string
}

// MonitoringConfig holds monitoring configuration
type MonitoringConfig struct {
	MetricsEnabled  bool
	MetricsPath     string
	HealthCheckPath string
}

// AWSConfig holds AWS configuration
type AWSConfig struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	S3Bucket        string
}

// SMTPConfig holds SMTP configuration
type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
}

// MongoDBConfig holds MongoDB specific configuration
type MongoDBConfig struct {
	URI            string
	Database       string
	ConnectTimeout time.Duration
	MaxPoolSize    uint64
}

var cfg *Config

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if exists
	if err := godotenv.Load(); err != nil {
		log.Printf("No .env file found: %v", err)
	}

	// Initialize viper
	viper.AutomaticEnv()
	viper.SetEnvPrefix("")

	// Set defaults
	setDefaults()

	cfg = &Config{
		App: AppConfig{
			Name:     viper.GetString("APP_NAME"),
			Env:      viper.GetString("APP_ENV"),
			Port:     viper.GetString("APP_PORT"),
			GRPCPort: viper.GetString("GRPC_PORT"),
			Debug:    viper.GetBool("APP_DEBUG"),
		},
		Database: DatabaseConfig{
			Driver:          viper.GetString("DB_DRIVER"),
			Host:            viper.GetString("DB_HOST"),
			Port:            viper.GetString("DB_PORT"),
			Name:            viper.GetString("DB_NAME"),
			User:            viper.GetString("DB_USER"),
			Password:        viper.GetString("DB_PASSWORD"),
			SSLMode:         viper.GetString("DB_SSL_MODE"),
			MaxIdleConns:    viper.GetInt("MAX_IDLE_CONNS"),
			MaxOpenConns:    viper.GetInt("MAX_OPEN_CONNS"),
			ConnMaxLifetime: viper.GetDuration("CONN_MAX_LIFETIME"),
			ConnMaxIdleTime: viper.GetDuration("CONN_MAX_IDLE_TIME"),
			ReadHost:        viper.GetString("DB_READ_HOST"),
			ReadPort:        viper.GetString("DB_READ_PORT"),
			ReadUser:        viper.GetString("DB_READ_USER"),
			ReadPassword:    viper.GetString("DB_READ_PASSWORD"),
		},
		Redis: RedisConfig{
			Host:         viper.GetString("REDIS_HOST"),
			Port:         viper.GetString("REDIS_PORT"),
			Password:     viper.GetString("REDIS_PASSWORD"),
			DB:           viper.GetInt("REDIS_DB"),
			PoolSize:     viper.GetInt("REDIS_POOL_SIZE"),
			MinIdleConns: viper.GetInt("REDIS_MIN_IDLE_CONNS"),
		},
		JWT: JWTConfig{
			Secret:        viper.GetString("JWT_SECRET"),
			Expiry:        viper.GetDuration("JWT_EXPIRY"),
			RefreshExpiry: viper.GetDuration("JWT_REFRESH_EXPIRY"),
			Issuer:        viper.GetString("JWT_ISSUER"),
		},
		Upload: UploadConfig{
			MaxSize:      viper.GetInt64("UPLOAD_MAX_SIZE"),
			Path:         viper.GetString("UPLOAD_PATH"),
			AllowedTypes: viper.GetStringSlice("UPLOAD_ALLOWED_TYPES"),
		},
		WebSocket: WebSocketConfig{
			ReadBufferSize:  viper.GetInt("WS_READ_BUFFER_SIZE"),
			WriteBufferSize: viper.GetInt("WS_WRITE_BUFFER_SIZE"),
			MaxMessageSize:  viper.GetInt64("WS_MAX_MESSAGE_SIZE"),
			PingPeriod:      viper.GetDuration("WS_PING_PERIOD"),
			PongWait:        viper.GetDuration("WS_PONG_WAIT"),
		},
		Stream: StreamConfig{
			ChunkSize:  viper.GetInt64("STREAM_CHUNK_SIZE"),
			BufferSize: viper.GetInt64("STREAM_BUFFER_SIZE"),
			Path:       viper.GetString("STREAM_PATH"),
		},
		Encryption: EncryptionConfig{
			Key: viper.GetString("ENCRYPTION_KEY"),
		},
		CORS: CORSConfig{
			AllowedOrigins:   viper.GetStringSlice("CORS_ALLOWED_ORIGINS"),
			AllowedMethods:   viper.GetStringSlice("CORS_ALLOWED_METHODS"),
			AllowedHeaders:   viper.GetStringSlice("CORS_ALLOWED_HEADERS"),
			ExposedHeaders:   viper.GetStringSlice("CORS_EXPOSE_HEADERS"),
			AllowCredentials: viper.GetBool("CORS_ALLOW_CREDENTIALS"),
			MaxAge:           viper.GetInt("CORS_MAX_AGE"),
		},
		RateLimit: RateLimitConfig{
			Enabled:  viper.GetBool("RATE_LIMIT_ENABLED"),
			Requests: viper.GetInt("RATE_LIMIT_REQUESTS"),
			Duration: viper.GetDuration("RATE_LIMIT_DURATION"),
		},
		Log: LogConfig{
			Level:    viper.GetString("LOG_LEVEL"),
			Format:   viper.GetString("LOG_FORMAT"),
			Output:   viper.GetString("LOG_OUTPUT"),
			FilePath: viper.GetString("LOG_FILE_PATH"),
		},
		Swagger: SwaggerConfig{
			Enabled:  viper.GetBool("SWAGGER_ENABLED"),
			Host:     viper.GetString("SWAGGER_HOST"),
			BasePath: viper.GetString("SWAGGER_BASE_PATH"),
		},
		Monitoring: MonitoringConfig{
			MetricsEnabled:  viper.GetBool("METRICS_ENABLED"),
			MetricsPath:     viper.GetString("METRICS_PATH"),
			HealthCheckPath: viper.GetString("HEALTH_CHECK_PATH"),
		},
		AWS: AWSConfig{
			Region:          viper.GetString("AWS_REGION"),
			AccessKeyID:     viper.GetString("AWS_ACCESS_KEY_ID"),
			SecretAccessKey: viper.GetString("AWS_SECRET_ACCESS_KEY"),
			S3Bucket:        viper.GetString("AWS_S3_BUCKET"),
		},
		SMTP: SMTPConfig{
			Host:     viper.GetString("SMTP_HOST"),
			Port:     viper.GetInt("SMTP_PORT"),
			User:     viper.GetString("SMTP_USER"),
			Password: viper.GetString("SMTP_PASSWORD"),
			From:     viper.GetString("SMTP_FROM"),
		},
		MongoDB: MongoDBConfig{
			URI:            viper.GetString("MONGODB_URI"),
			Database:       viper.GetString("MONGODB_DATABASE"),
			ConnectTimeout: viper.GetDuration("MONGODB_CONNECT_TIMEOUT"),
			MaxPoolSize:    viper.GetUint64("MONGODB_MAX_POOL_SIZE"),
		},
	}

	// Validate configuration
	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Get returns the loaded configuration
func Get() *Config {
	if cfg == nil {
		log.Fatal("Configuration not loaded. Call Load() first")
	}
	return cfg
}

// setDefaults sets default values for configuration
func setDefaults() {
	// App defaults
	viper.SetDefault("APP_NAME", "boilerplate-api")
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("APP_PORT", "8080")
	viper.SetDefault("GRPC_PORT", "50051")
	viper.SetDefault("APP_DEBUG", true)

	// Database defaults
	viper.SetDefault("DB_DRIVER", "postgres")
	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", "5432")
	viper.SetDefault("DB_SSL_MODE", "disable")
	viper.SetDefault("MAX_IDLE_CONNS", 10)
	viper.SetDefault("MAX_OPEN_CONNS", 100)
	viper.SetDefault("CONN_MAX_LIFETIME", "1h")
	viper.SetDefault("CONN_MAX_IDLE_TIME", "10m")

	// Redis defaults
	viper.SetDefault("REDIS_HOST", "localhost")
	viper.SetDefault("REDIS_PORT", "6379")
	viper.SetDefault("REDIS_DB", 0)
	viper.SetDefault("REDIS_POOL_SIZE", 10)
	viper.SetDefault("REDIS_MIN_IDLE_CONNS", 5)

	// JWT defaults
	viper.SetDefault("JWT_EXPIRY", "24h")
	viper.SetDefault("JWT_REFRESH_EXPIRY", "720h")
	viper.SetDefault("JWT_ISSUER", "boilerplate-api")

	// Upload defaults
	viper.SetDefault("UPLOAD_MAX_SIZE", 10485760) // 10MB
	viper.SetDefault("UPLOAD_PATH", "./uploads")
	viper.SetDefault("UPLOAD_ALLOWED_TYPES", []string{"image/jpeg", "image/png", "image/gif", "video/mp4", "application/pdf"})

	// WebSocket defaults
	viper.SetDefault("WS_READ_BUFFER_SIZE", 1024)
	viper.SetDefault("WS_WRITE_BUFFER_SIZE", 1024)
	viper.SetDefault("WS_MAX_MESSAGE_SIZE", 512000)
	viper.SetDefault("WS_PING_PERIOD", "54s")
	viper.SetDefault("WS_PONG_WAIT", "60s")

	// Stream defaults
	viper.SetDefault("STREAM_CHUNK_SIZE", 1048576)
	viper.SetDefault("STREAM_BUFFER_SIZE", 4194304)
	viper.SetDefault("STREAM_PATH", "./videos")

	// CORS defaults
	viper.SetDefault("CORS_ALLOWED_ORIGINS", []string{"*"})
	viper.SetDefault("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"})
	viper.SetDefault("CORS_ALLOWED_HEADERS", []string{"Origin", "Content-Type", "Accept", "Authorization"})
	viper.SetDefault("CORS_EXPOSE_HEADERS", []string{"X-Total-Count", "X-Page", "X-Per-Page"})
	viper.SetDefault("CORS_ALLOW_CREDENTIALS", true)
	viper.SetDefault("CORS_MAX_AGE", 86400)

	// Rate limit defaults
	viper.SetDefault("RATE_LIMIT_ENABLED", true)
	viper.SetDefault("RATE_LIMIT_REQUESTS", 100)
	viper.SetDefault("RATE_LIMIT_DURATION", "1m")

	// Log defaults
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("LOG_FORMAT", "json")
	viper.SetDefault("LOG_OUTPUT", "stdout")
	viper.SetDefault("LOG_FILE_PATH", "./logs/app.log")

	// Swagger defaults
	viper.SetDefault("SWAGGER_ENABLED", true)
	viper.SetDefault("SWAGGER_HOST", "localhost:8080")
	viper.SetDefault("SWAGGER_BASE_PATH", "/api/v1")

	// Monitoring defaults
	viper.SetDefault("METRICS_ENABLED", true)
	viper.SetDefault("METRICS_PATH", "/metrics")
	viper.SetDefault("HEALTH_CHECK_PATH", "/health")

	// MongoDB defaults
	viper.SetDefault("MONGODB_URI", "mongodb://localhost:27017")
	viper.SetDefault("MONGODB_DATABASE", "boilerplate")
	viper.SetDefault("MONGODB_CONNECT_TIMEOUT", "10s")
	viper.SetDefault("MONGODB_MAX_POOL_SIZE", 100)
}

// validate validates the configuration
func validate(cfg *Config) error {
	if cfg.App.Port == "" {
		return fmt.Errorf("APP_PORT is required")
	}

	if cfg.Database.Driver == "" {
		return fmt.Errorf("DB_DRIVER is required")
	}

	if cfg.JWT.Secret == "" || len(cfg.JWT.Secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}

	if cfg.Encryption.Key != "" && len(cfg.Encryption.Key) != 32 {
		return fmt.Errorf("ENCRYPTION_KEY must be exactly 32 characters")
	}

	// Create required directories
	dirs := []string{cfg.Upload.Path, cfg.Stream.Path}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// IsProduction returns true if the application is running in production
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}

// IsDevelopment returns true if the application is running in development
func (c *Config) IsDevelopment() bool {
	return c.App.Env == "development"
}

// IsDebug returns true if debug mode is enabled
func (c *Config) IsDebug() bool {
	return c.App.Debug
}
