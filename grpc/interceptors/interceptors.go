package interceptors

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go-api-boilerplate/pkg/logger"
	"go-api-boilerplate/utils"
)

// LoggingInterceptor logs gRPC requests
func LoggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		// Extract request ID from metadata
		md, _ := metadata.FromIncomingContext(ctx)
		requestID := md.Get("request-id")
		if len(requestID) == 0 {
			requestID = []string{utils.GenerateUUID()}
		}

		// Log request
		logger.WithFields(map[string]interface{}{
			"method":     info.FullMethod,
			"request_id": requestID[0],
		}).Info("gRPC request started")

		// Call handler
		resp, err := handler(ctx, req)

		// Log response
		duration := time.Since(start)
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		logger.WithFields(map[string]interface{}{
			"method":     info.FullMethod,
			"request_id": requestID[0],
			"duration":   duration.Milliseconds(),
			"status":     code.String(),
		}).Info("gRPC request completed")

		return resp, err
	}
}

// StreamLoggingInterceptor logs streaming gRPC requests
func StreamLoggingInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()

		// Extract request ID
		md, _ := metadata.FromIncomingContext(ss.Context())
		requestID := md.Get("request-id")
		if len(requestID) == 0 {
			requestID = []string{utils.GenerateUUID()}
		}

		logger.WithFields(map[string]interface{}{
			"method":           info.FullMethod,
			"request_id":       requestID[0],
			"is_client_stream": info.IsClientStream,
			"is_server_stream": info.IsServerStream,
		}).Info("gRPC stream started")

		// Call handler
		err := handler(srv, ss)

		// Log completion
		duration := time.Since(start)
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		logger.WithFields(map[string]interface{}{
			"method":     info.FullMethod,
			"request_id": requestID[0],
			"duration":   duration.Milliseconds(),
			"status":     code.String(),
		}).Info("gRPC stream completed")

		return err
	}
}

// RecoveryInterceptor recovers from panics
func RecoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				// Log the panic
				logger.WithFields(map[string]interface{}{
					"method": info.FullMethod,
					"panic":  r,
					"stack":  string(debug.Stack()),
				}).Error("gRPC handler panic")

				// Return internal error
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()

		return handler(ctx, req)
	}
}

// StreamRecoveryInterceptor recovers from panics in streams
func StreamRecoveryInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.WithFields(map[string]interface{}{
					"method": info.FullMethod,
					"panic":  r,
					"stack":  string(debug.Stack()),
				}).Error("gRPC stream handler panic")

				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()

		return handler(srv, ss)
	}
}

// AuthInterceptor validates authentication
func AuthInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Skip auth for certain methods
		if shouldSkipAuth(info.FullMethod) {
			return handler(ctx, req)
		}

		// Extract token from metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
		}

		authorization := md.Get("authorization")
		if len(authorization) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "missing authorization token")
		}

		// Extract token
		token, err := extractToken(authorization[0])
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid authorization format")
		}

		// Validate token
		claims, err := utils.ValidateToken(token)
		if err != nil {
			if utils.IsTokenExpired(err) {
				return nil, status.Errorf(codes.Unauthenticated, "token expired")
			}
			return nil, status.Errorf(codes.Unauthenticated, "invalid token")
		}

		// Add user info to context
		ctx = context.WithValue(ctx, "user_id", claims.UserID)
		ctx = context.WithValue(ctx, "user_email", claims.Email)
		ctx = context.WithValue(ctx, "user_role", claims.Role)

		return handler(ctx, req)
	}
}

// StreamAuthInterceptor validates authentication for streams
func StreamAuthInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Skip auth for certain methods
		if shouldSkipAuth(info.FullMethod) {
			return handler(srv, ss)
		}

		// Extract token from metadata
		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			return status.Errorf(codes.Unauthenticated, "missing metadata")
		}

		authorization := md.Get("authorization")
		if len(authorization) == 0 {
			return status.Errorf(codes.Unauthenticated, "missing authorization token")
		}

		// Extract token
		token, err := extractToken(authorization[0])
		if err != nil {
			return status.Errorf(codes.Unauthenticated, "invalid authorization format")
		}

		// Validate token
		claims, err := utils.ValidateToken(token)
		if err != nil {
			if utils.IsTokenExpired(err) {
				return status.Errorf(codes.Unauthenticated, "token expired")
			}
			return status.Errorf(codes.Unauthenticated, "invalid token")
		}

		// Create wrapped stream with auth context
		wrappedStream := &authenticatedServerStream{
			ServerStream: ss,
			ctx: context.WithValue(
				context.WithValue(
					context.WithValue(ss.Context(), "user_id", claims.UserID),
					"user_email", claims.Email,
				),
				"user_role", claims.Role,
			),
		}

		return handler(srv, wrappedStream)
	}
}

// ValidationInterceptor validates requests
func ValidationInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Check if request implements validator interface
		if v, ok := req.(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
			}
		}

		return handler(ctx, req)
	}
}

// RateLimitInterceptor implements rate limiting
func RateLimitInterceptor(limit int, window time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Extract client identifier
		var key string
		if userID := ctx.Value("user_id"); userID != nil {
			key = fmt.Sprintf("grpc_rate:%v:%s", userID, info.FullMethod)
		} else {
			// Use IP address
			md, _ := metadata.FromIncomingContext(ctx)
			if forwarded := md.Get("x-forwarded-for"); len(forwarded) > 0 {
				key = fmt.Sprintf("grpc_rate:%s:%s", forwarded[0], info.FullMethod)
			} else {
				key = fmt.Sprintf("grpc_rate:unknown:%s", info.FullMethod)
			}
		}

		fmt.Printf("Key %s\n", key)

		// Check rate limit (would need Redis service)
		// This is a simplified implementation
		// In production, use Redis for distributed rate limiting

		return handler(ctx, req)
	}
}

// Helper functions

// shouldSkipAuth checks if a method should skip authentication
func shouldSkipAuth(method string) bool {
	// Methods that don't require authentication
	publicMethods := []string{
		"/boilerplate.v1.AuthService/Login",
		"/boilerplate.v1.AuthService/Register",
		"/boilerplate.v1.AuthService/ForgotPassword",
		"/boilerplate.v1.AuthService/ResetPassword",
		"/boilerplate.v1.AuthService/VerifyEmail",
		"/grpc.health.v1.Health/Check",
		"/grpc.health.v1.Health/Watch",
	}

	for _, publicMethod := range publicMethods {
		if method == publicMethod {
			return true
		}
	}

	return false
}

// extractToken extracts token from authorization header
func extractToken(auth string) (string, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return "", fmt.Errorf("invalid authorization format")
	}
	return strings.TrimPrefix(auth, prefix), nil
}

// authenticatedServerStream wraps ServerStream with authenticated context
type authenticatedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authenticatedServerStream) Context() context.Context {
	return s.ctx
}

// MetricsInterceptor collects metrics
func MetricsInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		// Call handler
		resp, err := handler(ctx, req)

		// Collect metrics
		duration := time.Since(start)
		status := "success"
		if err != nil {
			status = "error"
		}

		// Log metrics (in production, send to metrics system)
		logger.WithFields(map[string]interface{}{
			"type":     "grpc_metrics",
			"method":   info.FullMethod,
			"duration": duration.Milliseconds(),
			"status":   status,
		}).Debug("gRPC metrics")

		return resp, err
	}
}

// GetUserIDFromContext extracts user ID from context
func GetUserIDFromContext(ctx context.Context) (uint, error) {
	userID := ctx.Value("user_id")
	if userID == nil {
		return 0, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	id, ok := userID.(uint)
	if !ok {
		return 0, status.Errorf(codes.Internal, "invalid user ID type")
	}

	return id, nil
}

// GetUserRoleFromContext extracts user role from context
func GetUserRoleFromContext(ctx context.Context) (string, error) {
	role := ctx.Value("user_role")
	if role == nil {
		return "", status.Errorf(codes.Unauthenticated, "user role not found")
	}

	roleStr, ok := role.(string)
	if !ok {
		return "", status.Errorf(codes.Internal, "invalid role type")
	}

	return roleStr, nil
}

// RequireRole creates an interceptor that requires specific roles
func RequireRole(roles ...string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		userRole, err := GetUserRoleFromContext(ctx)
		if err != nil {
			return nil, err
		}

		// Check if user has required role
		hasRole := false
		for _, role := range roles {
			if userRole == role {
				hasRole = true
				break
			}
		}

		if !hasRole {
			return nil, status.Errorf(codes.PermissionDenied, "insufficient permissions")
		}

		return handler(ctx, req)
	}
}
