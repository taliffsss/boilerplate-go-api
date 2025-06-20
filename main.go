package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"go-api-boilerplate/config"
	"go-api-boilerplate/controllers"
	"go-api-boilerplate/database"
	grpcinterceptors "go-api-boilerplate/grpc/interceptors"
	"go-api-boilerplate/grpc/proto"
	grpcserver "go-api-boilerplate/grpc/server"
	middleware "go-api-boilerplate/middlewares"
	"go-api-boilerplate/pkg/logger"
	"go-api-boilerplate/services"
)

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

	// Initialize services
	authService := services.NewAuthService(db, redisService)
	userService := services.NewUserService(db)
	uploadService := services.NewUploadService()
	wsService := services.NewWebSocketService()
	streamService := services.NewStreamService()

	// Wait group for graceful shutdown
	var wg sync.WaitGroup

	// Start REST API server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := startRESTServer(cfg, db, redisService, authService, userService, uploadService, wsService, streamService); err != nil {
			logger.Fatalf("REST server failed: %v", err)
		}
	}()

	// Start gRPC server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := startGRPCServer(cfg, authService, userService); err != nil {
			logger.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down servers...")

	// Wait for all servers to shut down
	wg.Wait()

	logger.Info("All servers exited")
}

func startRESTServer(
	cfg *config.Config,
	db *database.DB,
	redis *services.RedisService,
	authService *services.AuthService,
	userService *services.UserService,
	uploadService *services.UploadService,
	wsService *services.WebSocketService,
	streamService *services.StreamService,
) error {
	// Create router (reuse from api/main.go)
	router := setupRouter(cfg, db, redis, authService, userService, uploadService, wsService, streamService)

	// Create HTTP server
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%s", cfg.App.Port),
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// Start server in goroutine
	go func() {
		logger.Infof("Starting REST API on port %s", cfg.App.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start REST server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("REST server forced to shutdown: %w", err)
	}

	return nil
}

func startGRPCServer(
	cfg *config.Config,
	authService *services.AuthService,
	userService *services.UserService,
) error {
	// Create gRPC server with interceptors
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			grpcinterceptors.LoggingInterceptor(),
			grpcinterceptors.RecoveryInterceptor(),
			grpcinterceptors.AuthInterceptor(),
			grpcinterceptors.ValidationInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			grpcinterceptors.StreamLoggingInterceptor(),
			grpcinterceptors.StreamRecoveryInterceptor(),
			grpcinterceptors.StreamAuthInterceptor(),
		),
	}

	grpcServer := grpc.NewServer(opts...)

	// Register gRPC services
	authServer := grpcserver.NewAuthServer(authService, userService)
	userServer := grpcserver.NewUserServer(userService)

	proto.RegisterAuthServiceServer(grpcServer, authServer)
	proto.RegisterUserServiceServer(grpcServer, userServer)

	// Register health check
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	// Register reflection service for debugging
	reflection.Register(grpcServer)

	// Create listener
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.App.GRPCPort))
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %w", cfg.App.GRPCPort, err)
	}

	// Start server in goroutine
	go func() {
		logger.Infof("Starting gRPC server on port %s", cfg.App.GRPCPort)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful stop
	grpcServer.GracefulStop()

	return nil
}

// setupRouter creates and configures the Gin router
func setupRouter(
	cfg *config.Config,
	db *database.DB,
	redis *services.RedisService,
	authService *services.AuthService,
	userService *services.UserService,
	uploadService *services.UploadService,
	wsService *services.WebSocketService,
	streamService *services.StreamService,
) *gin.Engine {
	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(middleware.RequestIDMiddleware())
	router.Use(middleware.LoggerMiddleware())
	router.Use(middleware.ErrorLoggerMiddleware())
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.SecureHeadersMiddleware())

	// Initialize handlers
	healthHandler := controllers.NewHealthHandler(db, redis)
	authHandler := controllers.NewAuthHandler(authService, userService)
	userHandler := controllers.NewUserHandler(userService)
	uploadHandler := controllers.NewUploadHandler(uploadService)
	wsHandler := controllers.WebSocketController(wsService)
	StreamController := controllers.NewStreamController(streamService)

	// Routes setup (same as api/main.go)
	// ... (copy route setup from api/main.go)

	return router
}
