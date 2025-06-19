package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"go-api-boilerplate/config"
	"go-api-boilerplate/database"
	_ "go-api-boilerplate/docs" // Swagger docs
	middleware "go-api-boilerplate/middlewares"
	"go-api-boilerplate/models"
	"go-api-boilerplate/pkg/logger"
	"go-api-boilerplate/services"
	"go-api-boilerplate/utils"
)

// @title Boilerplate API
// @version 1.0
// @description A comprehensive Golang API boilerplate with REST and gRPC support
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.example.com/support
// @contact.email support@example.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	if err := logger.Init(cfg); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Connect to database
	db, err := database.Connect(cfg)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Initialize Redis
	redisService, err := services.NewRedisService()
	if err != nil {
		logger.Warnf("Failed to connect to Redis: %v", err)
		// Continue without Redis - it's optional
	}

	// Set Gin mode
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	router := setupRouter(cfg, db, redisService)

	// Create server
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%s", cfg.App.Port),
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// Start server in goroutine
	go func() {
		logger.Infof("Starting %s on port %s", cfg.App.Name, cfg.App.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatalf("Server forced to shutdown: %v", err)
	}

	logger.Info("Server exited")
}

func setupRouter(cfg *config.Config, db *database.DB, redis *services.RedisService) *gin.Engine {
	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(middleware.RequestIDMiddleware())
	router.Use(middleware.LoggerMiddleware())
	router.Use(middleware.ErrorLoggerMiddleware())
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.SecureHeadersMiddleware())

	// Initialize services
	authService := services.NewAuthService(db, redis)
	userService := services.NewUserService(db)
	uploadService := services.NewUploadService()
	wsService := services.NewWebSocketService()
	streamService := services.NewStreamService()

	// Initialize handlers
	healthHandler := controllers.NewHealthHandler(db, redis)
	authHandler := controllers.NewAuthController(authService, userService)
	userHandler := controllers.NewUserHandler(userService)
	uploadHandler := controllers.NewUploadHandler(uploadService)
	wsHandler := controllers.NewWebSocketHandler(wsService)
	streamHandler := controllers.NewStreamHandler(streamService)

	// Health check routes
	router.GET("/health", healthHandler.HealthCheck)
	router.GET("/ready", healthHandler.ReadinessCheck)

	// Swagger documentation
	if cfg.Swagger.Enabled {
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// WebSocket endpoint
	router.GET("/ws", middleware.OptionalAuthMiddleware(), wsHandler.HandleWebSocket)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Public routes
		public := v1.Group("")
		{
			// Authentication
			public.POST("/auth/register", authHandler.Register)
			public.POST("/auth/login", authHandler.Login)
			public.POST("/auth/refresh", authHandler.RefreshToken)
			public.POST("/auth/forgot-password", authHandler.ForgotPassword)
			public.POST("/auth/reset-password", authHandler.ResetPassword)
			public.GET("/auth/verify-email/:token", authHandler.VerifyEmail)
		}

		// Protected routes
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware())
		protected.Use(middleware.RequireActiveUser())
		{
			// Authentication
			protected.POST("/auth/logout", authHandler.Logout)
			protected.POST("/auth/change-password", authHandler.ChangePassword)

			// User management
			protected.GET("/users/profile", userHandler.GetProfile)
			protected.PUT("/users/profile", userHandler.UpdateProfile)
			protected.DELETE("/users/profile", userHandler.DeleteAccount)
			protected.POST("/users/avatar", uploadHandler.UploadAvatar)

			// File upload
			protected.POST("/upload", uploadHandler.UploadFile)
			protected.POST("/upload/multiple", uploadHandler.UploadMultiple)
			protected.DELETE("/upload/:id", uploadHandler.DeleteFile)
			protected.GET("/files/:id", uploadHandler.GetFileInfo)

			// Video streaming
			protected.GET("/stream/video/:id", streamHandler.StreamVideo)
			protected.GET("/stream/hls/:id/*path", streamHandler.StreamHLS)
			protected.GET("/stream/info/:id", streamHandler.GetVideoInfo)
		}

		// Admin routes
		admin := v1.Group("/admin")
		admin.Use(middleware.AuthMiddleware())
		admin.Use(middleware.RequireRole(models.RoleAdmin))
		{
			// User management
			admin.GET("/users", userHandler.ListUsers)
			admin.GET("/users/:id", userHandler.GetUser)
			admin.PUT("/users/:id", userHandler.UpdateUser)
			admin.DELETE("/users/:id", userHandler.DeleteUser)
			admin.PUT("/users/:id/role", userHandler.UpdateUserRole)
			admin.PUT("/users/:id/status", userHandler.UpdateUserStatus)

			// System management
			admin.GET("/stats", healthHandler.GetStats)
			admin.POST("/cache/flush", healthHandler.FlushCache)
		}
	}

	// Static files
	router.Static("/uploads", cfg.Upload.Path)
	router.Static("/videos", cfg.Stream.Path)

	// Metrics endpoint
	if cfg.Monitoring.MetricsEnabled {
		router.GET(cfg.Monitoring.MetricsPath, middleware.APIKeyMiddleware(), healthHandler.Metrics)
	}

	// Rate limiting for specific routes
	rateLimitedGroup := router.Group("")
	rateLimitedGroup.Use(middleware.RateLimitMiddleware(
		cfg.RateLimit.Requests,
		cfg.RateLimit.Duration,
	))
	{
		rateLimitedGroup.POST("/api/v1/auth/login", authHandler.Login)
		rateLimitedGroup.POST("/api/v1/auth/register", authHandler.Register)
		rateLimitedGroup.POST("/api/v1/auth/forgot-password", authHandler.ForgotPassword)
	}

	// 404 handler
	router.NoRoute(func(c *gin.Context) {
		utils.NotFoundResponse(c, "Route")
	})

	return router
}
