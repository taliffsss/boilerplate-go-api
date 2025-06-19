package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"go-api-boilerplate/config"
	"go-api-boilerplate/database"
	"go-api-boilerplate/grpc/interceptors"
	"go-api-boilerplate/grpc/proto"
	"go-api-boilerplate/grpc/server"
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

	// Create gRPC server with interceptors
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			interceptors.LoggingInterceptor(),
			interceptors.RecoveryInterceptor(),
			interceptors.AuthInterceptor(),
			interceptors.ValidationInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			interceptors.StreamLoggingInterceptor(),
			interceptors.StreamRecoveryInterceptor(),
			interceptors.StreamAuthInterceptor(),
		),
	}

	grpcServer := grpc.NewServer(opts...)

	// Initialize services
	authService := services.NewAuthService(db, redisService)
	userService := services.NewUserService(db)

	// Register gRPC services
	authServer := server.NewAuthServer(authService, userService)
	userServer := server.NewUserServer(userService)

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
		logger.Fatalf("Failed to listen on port %s: %v", cfg.App.GRPCPort, err)
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

	logger.Info("Shutting down gRPC server...")
	grpcServer.GracefulStop()
	logger.Info("gRPC server exited")
}
