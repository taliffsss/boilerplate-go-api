package server

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go-api-boilerplate/grpc/interceptors"
	"go-api-boilerplate/grpc/proto"
	"go-api-boilerplate/models"
	"go-api-boilerplate/services"
)

// UserServer implements the gRPC UserService
type UserServer struct {
	proto.UnimplementedUserServiceServer
	userService *services.UserService
}

// NewUserServer creates a new user server
func NewUserServer(userService *services.UserService) proto.UserServiceServer {
	return &UserServer{
		userService: userService,
	}
}

// GetUser retrieves a user by ID
func (s *UserServer) GetUser(ctx context.Context, req *proto.GetUserRequest) (*proto.User, error) {
	if req.Id == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user ID is required")
	}

	// Check permissions
	currentUserID, _ := interceptors.GetUserIDFromContext(ctx)
	currentUserRole, _ := interceptors.GetUserRoleFromContext(ctx)

	// Users can only get their own profile unless they're admin/moderator
	if currentUserID != uint(req.Id) && currentUserRole != models.RoleAdmin && currentUserRole != models.RoleModerator {
		return nil, status.Errorf(codes.PermissionDenied, "insufficient permissions")
	}

	// Get user
	user, err := s.userService.FindByID(ctx, uint(req.Id))
	if err != nil {
		if err == services.ErrUserNotFound {
			return nil, status.Errorf(codes.NotFound, "user not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to retrieve user")
	}

	return modelUserToProto(user), nil
}

// GetUserByEmail retrieves a user by email
func (s *UserServer) GetUserByEmail(ctx context.Context, req *proto.GetUserByEmailRequest) (*proto.User, error) {
	if req.Email == "" {
		return nil, status.Errorf(codes.InvalidArgument, "email is required")
	}

	// Check permissions - only admins can search by email
	currentUserRole, err := interceptors.GetUserRoleFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if currentUserRole != models.RoleAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "insufficient permissions")
	}

	// Get user
	user, err := s.userService.FindByEmail(ctx, req.Email)
	if err != nil {
		if err == services.ErrUserNotFound {
			return nil, status.Errorf(codes.NotFound, "user not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to retrieve user")
	}

	return modelUserToProto(user), nil
}

// ListUsers retrieves a list of users with pagination
func (s *UserServer) ListUsers(ctx context.Context, req *proto.ListUsersRequest) (*proto.ListUsersResponse, error) {
	// Check permissions - only admins and moderators can list users
	currentUserRole, err := interceptors.GetUserRoleFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if currentUserRole != models.RoleAdmin && currentUserRole != models.RoleModerator {
		return nil, status.Errorf(codes.PermissionDenied, "insufficient permissions")
	}

	// Set defaults
	page := int(req.Page)
	if page < 1 {
		page = 1
	}
	perPage := int(req.PerPage)
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	// Build filter
	filter := &services.UserFilter{
		SortBy:    req.SortBy,
		SortOrder: req.SortOrder,
	}

	if req.Filter != nil {
		filter.Search = req.Filter.Search
		filter.Role = req.Filter.Role
		if req.Filter.IsActive {
			isActive := true
			filter.IsActive = &isActive
		}
		if req.Filter.EmailVerified {
			emailVerified := true
			filter.EmailVerified = &emailVerified
		}
	}

	// Get users
	meta, users, err := s.userService.FindPaginated(ctx, page, perPage, filter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve users")
	}

	// Convert to proto
	protoUsers := make([]*proto.User, len(users))
	for i, user := range users {
		protoUsers[i] = modelUserToProto(&user)
	}

	return &proto.ListUsersResponse{
		Users: protoUsers,
		Pagination: &proto.PaginationMeta{
			Page:       int32(meta.Page),
			PerPage:    int32(meta.PerPage),
			Total:      meta.Total,
			TotalPages: int32(meta.TotalPages),
			HasNext:    meta.HasNext,
			HasPrev:    meta.HasPrev,
		},
	}, nil
}

// CreateUser creates a new user
func (s *UserServer) CreateUser(ctx context.Context, req *proto.CreateUserRequest) (*proto.User, error) {
	// Check permissions - only admins can create users
	currentUserRole, err := interceptors.GetUserRoleFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if currentUserRole != models.RoleAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "insufficient permissions")
	}

	// Validate request
	if err := validateCreateUserRequest(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	// Create user
	input := &models.CreateUserInput{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
		Role:     req.Role,
	}

	user, err := s.userService.Create(ctx, input)
	if err != nil {
		if err == services.ErrUserAlreadyExists {
			return nil, status.Errorf(codes.AlreadyExists, "user already exists")
		}
		return nil, status.Errorf(codes.Internal, "failed to create user")
	}

	return modelUserToProto(user), nil
}

// UpdateUser updates an existing user
func (s *UserServer) UpdateUser(ctx context.Context, req *proto.UpdateUserRequest) (*proto.User, error) {
	if req.Id == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user ID is required")
	}

	// Check permissions
	currentUserID, _ := interceptors.GetUserIDFromContext(ctx)
	currentUserRole, _ := interceptors.GetUserRoleFromContext(ctx)

	// Users can only update their own profile (limited fields)
	// Admins can update any user
	isOwnProfile := currentUserID == uint(req.Id)
	isAdmin := currentUserRole == models.RoleAdmin

	if !isOwnProfile && !isAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "insufficient permissions")
	}

	// Build update input
	input := &models.UpdateUserInput{
		Name:   req.Name,
		Avatar: req.Avatar,
	}

	// Only admins can update these fields
	if isAdmin {
		input.Role = req.Role
		if req.IsActive {
			input.IsActive = &req.IsActive
		}
		if req.EmailVerified {
			input.EmailVerified = &req.EmailVerified
		}
	}

	// Update user
	user, err := s.userService.Update(ctx, uint(req.Id), input)
	if err != nil {
		if err == services.ErrUserNotFound {
			return nil, status.Errorf(codes.NotFound, "user not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to update user")
	}

	return modelUserToProto(user), nil
}

// DeleteUser deletes a user
func (s *UserServer) DeleteUser(ctx context.Context, req *proto.DeleteUserRequest) (*emptypb.Empty, error) {
	if req.Id == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user ID is required")
	}

	// Check permissions - only admins can delete users
	currentUserRole, err := interceptors.GetUserRoleFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if currentUserRole != models.RoleAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "insufficient permissions")
	}

	// Prevent self-deletion
	currentUserID, _ := interceptors.GetUserIDFromContext(ctx)
	if currentUserID == uint(req.Id) {
		return nil, status.Errorf(codes.InvalidArgument, "cannot delete your own account")
	}

	// Delete user
	if err := s.userService.Delete(ctx, uint(req.Id)); err != nil {
		if err == services.ErrUserNotFound {
			return nil, status.Errorf(codes.NotFound, "user not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to delete user")
	}

	return &emptypb.Empty{}, nil
}

// StreamUsers streams users in real-time
func (s *UserServer) StreamUsers(req *proto.StreamUsersRequest, stream proto.UserService_StreamUsersServer) error {
	// Check permissions
	currentUserRole, err := interceptors.GetUserRoleFromContext(stream.Context())
	if err != nil {
		return err
	}
	if currentUserRole != models.RoleAdmin {
		return status.Errorf(codes.PermissionDenied, "insufficient permissions")
	}

	// Build filter
	filter := &services.UserFilter{}
	if req.Filter != nil {
		filter.Search = req.Filter.Search
		filter.Role = req.Filter.Role
		if req.Filter.IsActive {
			isActive := true
			filter.IsActive = &isActive
		}
		if req.Filter.EmailVerified {
			emailVerified := true
			filter.EmailVerified = &emailVerified
		}
	}

	// This is a simplified implementation
	// In a real system, you might:
	// 1. Subscribe to database change events
	// 2. Use a message queue for real-time updates
	// 3. Implement server-sent events

	// For now, we'll send current users and simulate updates
	ctx := stream.Context()

	// Send initial batch of users
	users, err := s.userService.FindAll(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to retrieve users")
	}

	for _, user := range users {
		// Apply filter
		if !matchesFilter(&user, filter) {
			continue
		}

		if err := stream.Send(modelUserToProto(&user)); err != nil {
			return err
		}
	}

	// Keep connection alive and send updates
	// In production, subscribe to real events
	<-ctx.Done()
	return nil
}

// Helper functions

// modelUserToProto converts a model user to proto user
func modelUserToProto(user *models.User) *proto.User {
	protoUser := &proto.User{
		Id:            uint64(user.ID),
		Email:         user.Email,
		Name:          user.Name,
		Avatar:        user.Avatar,
		Role:          user.Role,
		IsActive:      user.IsActive,
		EmailVerified: user.EmailVerified,
		CreatedAt:     timestamppb.New(user.CreatedAt),
		UpdatedAt:     timestamppb.New(user.UpdatedAt),
	}

	if user.EmailVerifiedAt != nil {
		protoUser.EmailVerifiedAt = timestamppb.New(*user.EmailVerifiedAt)
	}
	if user.LastLoginAt != nil {
		protoUser.LastLoginAt = timestamppb.New(*user.LastLoginAt)
	}

	return protoUser
}

// validateCreateUserRequest validates create user request
func validateCreateUserRequest(req *proto.CreateUserRequest) error {
	if req.Email == "" {
		return fmt.Errorf("email is required")
	}
	if req.Password == "" || len(req.Password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if req.Role == "" {
		req.Role = models.RoleUser
	}
	return nil
}

// matchesFilter checks if a user matches the filter criteria
func matchesFilter(user *models.User, filter *services.UserFilter) bool {
	if filter.Search != "" {
		if !strings.Contains(strings.ToLower(user.Name), strings.ToLower(filter.Search)) &&
			!strings.Contains(strings.ToLower(user.Email), strings.ToLower(filter.Search)) {
			return false
		}
	}

	if filter.Role != "" && user.Role != filter.Role {
		return false
	}

	if filter.IsActive != nil && user.IsActive != *filter.IsActive {
		return false
	}

	if filter.EmailVerified != nil && user.EmailVerified != *filter.EmailVerified {
		return false
	}

	return true
}
