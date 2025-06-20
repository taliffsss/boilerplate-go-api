package server

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go-api-boilerplate/grpc/interceptors"
	"go-api-boilerplate/grpc/proto"
	"go-api-boilerplate/models"
	"go-api-boilerplate/services"
	"go-api-boilerplate/utils"
)

// AuthServer implements the gRPC AuthService
type AuthServer struct {
	proto.UnimplementedAuthServiceServer
	authService *services.AuthService
	userService *services.UserService
}

// NewAuthServer creates a new auth server
func NewAuthServer(authService *services.AuthService, userService *services.UserService) proto.AuthServiceServer {
	return &AuthServer{
		authService: authService,
		userService: userService,
	}
}

// Login authenticates a user and returns tokens
func (s *AuthServer) Login(ctx context.Context, req *proto.LoginRequest) (*proto.LoginResponse, error) {
	// Validate request
	if req.Email == "" || req.Password == "" {
		return nil, status.Errorf(codes.InvalidArgument, "email and password are required")
	}

	// Extract client IP (if available)
	clientIP := "unknown"
	// In a real implementation, extract from metadata

	// Authenticate user
	user, err := s.authService.Login(req.Email, req.Password, clientIP)
	if err != nil {
		if err == services.ErrInvalidCredentials {
			return nil, status.Errorf(codes.Unauthenticated, "invalid email or password")
		}
		if err == services.ErrUserNotActive {
			return nil, status.Errorf(codes.PermissionDenied, "account is not active")
		}
		return nil, status.Errorf(codes.Internal, "login failed")
	}

	// Generate tokens
	tokens, err := s.authService.GenerateTokens(user)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate tokens")
	}

	// Build response
	return &proto.LoginResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		TokenType:    tokens.TokenType,
		ExpiresIn:    tokens.ExpiresIn,
		User:         userToProto(user),
	}, nil
}

// Register creates a new user account
func (s *AuthServer) Register(ctx context.Context, req *proto.RegisterRequest) (*proto.RegisterResponse, error) {
	// Validate request
	if err := validateRegisterRequest(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	// Check if passwords match
	if req.Password != req.ConfirmPassword {
		return nil, status.Errorf(codes.InvalidArgument, "passwords do not match")
	}

	// Check if user already exists
	exists, err := s.userService.UserExistsByEmail(req.Email)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check user existence")
	}
	if exists {
		return nil, status.Errorf(codes.AlreadyExists, "email already registered")
	}

	// Register user
	input := &models.RegisterInput{
		Email:           req.Email,
		Password:        req.Password,
		ConfirmPassword: req.ConfirmPassword,
		Name:            req.Name,
	}

	user, err := s.authService.Register(input)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "registration failed")
	}

	// Generate tokens
	tokens, err := s.authService.GenerateTokens(user)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate tokens")
	}

	// Build response
	return &proto.RegisterResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		TokenType:    tokens.TokenType,
		ExpiresIn:    tokens.ExpiresIn,
		User:         userToProto(user),
	}, nil
}

// RefreshToken refreshes authentication tokens
func (s *AuthServer) RefreshToken(ctx context.Context, req *proto.RefreshTokenRequest) (*proto.RefreshTokenResponse, error) {
	if req.RefreshToken == "" {
		return nil, status.Errorf(codes.InvalidArgument, "refresh token is required")
	}

	// Refresh tokens
	tokens, err := s.authService.RefreshTokens(req.RefreshToken)
	if err != nil {
		if err == services.ErrInvalidToken {
			return nil, status.Errorf(codes.Unauthenticated, "invalid refresh token")
		}
		return nil, status.Errorf(codes.Internal, "failed to refresh token")
	}

	return &proto.RefreshTokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		TokenType:    tokens.TokenType,
		ExpiresIn:    tokens.ExpiresIn,
	}, nil
}

// Logout invalidates user tokens
func (s *AuthServer) Logout(ctx context.Context, req *proto.LogoutRequest) (*emptypb.Empty, error) {
	// Get user ID from context
	userID, err := interceptors.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Logout user
	if err := s.authService.Logout(userID, req.AccessToken); err != nil {
		return nil, status.Errorf(codes.Internal, "logout failed")
	}

	return &emptypb.Empty{}, nil
}

// ChangePassword changes user password
func (s *AuthServer) ChangePassword(ctx context.Context, req *proto.ChangePasswordRequest) (*emptypb.Empty, error) {
	// Validate request
	if err := validateChangePasswordRequest(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	// Get user ID from context
	userID, err := interceptors.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Change password
	if err := s.authService.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		if err == services.ErrInvalidCredentials {
			return nil, status.Errorf(codes.InvalidArgument, "current password is incorrect")
		}
		return nil, status.Errorf(codes.Internal, "failed to change password")
	}

	return &emptypb.Empty{}, nil
}

// ForgotPassword initiates password reset
func (s *AuthServer) ForgotPassword(ctx context.Context, req *proto.ForgotPasswordRequest) (*emptypb.Empty, error) {
	if req.Email == "" {
		return nil, status.Errorf(codes.InvalidArgument, "email is required")
	}

	// Initiate password reset
	_ = s.authService.ForgotPassword(req.Email)
	// Always return success to prevent email enumeration

	return &emptypb.Empty{}, nil
}

// ResetPassword resets user password with token
func (s *AuthServer) ResetPassword(ctx context.Context, req *proto.ResetPasswordRequest) (*emptypb.Empty, error) {
	// Validate request
	if req.Token == "" || req.NewPassword == "" {
		return nil, status.Errorf(codes.InvalidArgument, "token and new password are required")
	}

	if req.NewPassword != req.ConfirmPassword {
		return nil, status.Errorf(codes.InvalidArgument, "passwords do not match")
	}

	// Reset password
	if err := s.authService.ResetPassword(req.Token, req.NewPassword); err != nil {
		if err == services.ErrInvalidToken {
			return nil, status.Errorf(codes.InvalidArgument, "invalid or expired reset token")
		}
		return nil, status.Errorf(codes.Internal, "failed to reset password")
	}

	return &emptypb.Empty{}, nil
}

// VerifyEmail verifies user email address
func (s *AuthServer) VerifyEmail(ctx context.Context, req *proto.VerifyEmailRequest) (*emptypb.Empty, error) {
	if req.Token == "" {
		return nil, status.Errorf(codes.InvalidArgument, "verification token is required")
	}

	// Verify email
	if err := s.authService.VerifyEmail(req.Token); err != nil {
		if err == services.ErrInvalidToken {
			return nil, status.Errorf(codes.InvalidArgument, "invalid or expired verification token")
		}
		return nil, status.Errorf(codes.Internal, "failed to verify email")
	}

	return &emptypb.Empty{}, nil
}

// ValidateToken validates an access token
func (s *AuthServer) ValidateToken(ctx context.Context, req *proto.ValidateTokenRequest) (*proto.ValidateTokenResponse, error) {
	if req.AccessToken == "" {
		return nil, status.Errorf(codes.InvalidArgument, "access token is required")
	}

	// Validate token
	user, err := s.authService.ValidateAccessToken(req.AccessToken)
	if err != nil {
		return &proto.ValidateTokenResponse{
			Valid: false,
		}, nil
	}

	// Get token expiration
	claims, _ := utils.ParseTokenWithoutValidation(req.AccessToken)

	return &proto.ValidateTokenResponse{
		Valid:     true,
		User:      userToProto(user),
		ExpiresAt: timestamppb.New(claims.ExpiresAt.Time),
	}, nil
}

// Helper functions

// userToProto converts a model user to proto user
func userToProto(user *models.User) *proto.UserInfo {
	return &proto.UserInfo{
		Id:            uint64(user.ID),
		Email:         user.Email,
		Name:          user.Name,
		Role:          user.Role,
		IsActive:      user.IsActive,
		EmailVerified: user.EmailVerified,
	}
}

// validateRegisterRequest validates registration request
func validateRegisterRequest(req *proto.RegisterRequest) error {
	if req.Email == "" {
		return fmt.Errorf("email is required")
	}
	if req.Password == "" || len(req.Password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// validateChangePasswordRequest validates change password request
func validateChangePasswordRequest(req *proto.ChangePasswordRequest) error {
	if req.OldPassword == "" {
		return fmt.Errorf("current password is required")
	}
	if req.NewPassword == "" || len(req.NewPassword) < 8 {
		return fmt.Errorf("new password must be at least 8 characters")
	}
	if req.NewPassword != req.ConfirmNewPassword {
		return fmt.Errorf("passwords do not match")
	}
	return nil
}
