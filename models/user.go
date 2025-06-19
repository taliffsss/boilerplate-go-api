package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"gorm.io/gorm"
)

// User represents a user in the system
type User struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	Email           string         `gorm:"uniqueIndex;not null" json:"email"`
	Password        string         `gorm:"not null" json:"-"`
	Name            string         `gorm:"not null" json:"name"`
	Avatar          string         `json:"avatar,omitempty"`
	Role            string         `gorm:"default:'user'" json:"role"`
	IsActive        bool           `gorm:"default:true" json:"is_active"`
	EmailVerified   bool           `gorm:"default:false" json:"email_verified"`
	EmailVerifiedAt *time.Time     `json:"email_verified_at,omitempty"`
	LastLoginAt     *time.Time     `json:"last_login_at,omitempty"`
	RefreshToken    string         `json:"-"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

// UserMongo represents a user in MongoDB
type UserMongo struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email           string             `bson:"email" json:"email"`
	Password        string             `bson:"password" json:"-"`
	Name            string             `bson:"name" json:"name"`
	Avatar          string             `bson:"avatar,omitempty" json:"avatar,omitempty"`
	Role            string             `bson:"role" json:"role"`
	IsActive        bool               `bson:"is_active" json:"is_active"`
	EmailVerified   bool               `bson:"email_verified" json:"email_verified"`
	EmailVerifiedAt *time.Time         `bson:"email_verified_at,omitempty" json:"email_verified_at,omitempty"`
	LastLoginAt     *time.Time         `bson:"last_login_at,omitempty" json:"last_login_at,omitempty"`
	RefreshToken    string             `bson:"refresh_token,omitempty" json:"-"`
	CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt       time.Time          `bson:"updated_at" json:"updated_at"`
	DeletedAt       *time.Time         `bson:"deleted_at,omitempty" json:"-"`
}

// TableName specifies the table name for the User model
func (User) TableName() string {
	return "users"
}

// BeforeCreate hook for User
func (u *User) BeforeCreate(tx *gorm.DB) error {
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()
	return nil
}

// BeforeUpdate hook for User
func (u *User) BeforeUpdate(tx *gorm.DB) error {
	u.UpdatedAt = time.Now()
	return nil
}

// UserRole constants
const (
	RoleAdmin     = "admin"
	RoleModerator = "moderator"
	RoleUser      = "user"
)

// CreateUserInput represents the input for creating a user
type CreateUserInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Role     string `json:"role,omitempty"`
}

// UpdateUserInput represents the input for updating a user
type UpdateUserInput struct {
	Name          string `json:"name,omitempty" binding:"omitempty,min=2,max=100"`
	Avatar        string `json:"avatar,omitempty"`
	Role          string `json:"role,omitempty"`
	IsActive      *bool  `json:"is_active,omitempty"`
	EmailVerified *bool  `json:"email_verified,omitempty"`
}

// LoginInput represents the input for user login
type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RegisterInput represents the input for user registration
type RegisterInput struct {
	Email           string `json:"email" binding:"required,email"`
	Password        string `json:"password" binding:"required,min=8"`
	ConfirmPassword string `json:"confirm_password" binding:"required,eqfield=Password"`
	Name            string `json:"name" binding:"required,min=2,max=100"`
}

// ChangePasswordInput represents the input for changing password
type ChangePasswordInput struct {
	OldPassword        string `json:"old_password" binding:"required"`
	NewPassword        string `json:"new_password" binding:"required,min=8"`
	ConfirmNewPassword string `json:"confirm_new_password" binding:"required,eqfield=NewPassword"`
}

// RefreshTokenInput represents the input for refreshing tokens
type RefreshTokenInput struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// UserResponse represents the user response structure
type UserResponse struct {
	ID              uint       `json:"id"`
	Email           string     `json:"email"`
	Name            string     `json:"name"`
	Avatar          string     `json:"avatar,omitempty"`
	Role            string     `json:"role"`
	IsActive        bool       `json:"is_active"`
	EmailVerified   bool       `json:"email_verified"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	LastLoginAt     *time.Time `json:"last_login_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ToResponse converts User to UserResponse
func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID:              u.ID,
		Email:           u.Email,
		Name:            u.Name,
		Avatar:          u.Avatar,
		Role:            u.Role,
		IsActive:        u.IsActive,
		EmailVerified:   u.EmailVerified,
		EmailVerifiedAt: u.EmailVerifiedAt,
		LastLoginAt:     u.LastLoginAt,
		CreatedAt:       u.CreatedAt,
		UpdatedAt:       u.UpdatedAt,
	}
}

// ToResponseMongo converts UserMongo to UserResponse
func (u *UserMongo) ToResponse() *UserResponse {
	return &UserResponse{
		ID:              uint(u.ID.Timestamp().Unix()), // Convert ObjectID to uint
		Email:           u.Email,
		Name:            u.Name,
		Avatar:          u.Avatar,
		Role:            u.Role,
		IsActive:        u.IsActive,
		EmailVerified:   u.EmailVerified,
		EmailVerifiedAt: u.EmailVerifiedAt,
		LastLoginAt:     u.LastLoginAt,
		CreatedAt:       u.CreatedAt,
		UpdatedAt:       u.UpdatedAt,
	}
}

// AuthTokens represents the authentication tokens
type AuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	User   *UserResponse `json:"user"`
	Tokens *AuthTokens   `json:"tokens"`
}

// Permission represents a permission
type Permission struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	Name        string         `gorm:"uniqueIndex;not null" json:"name"`
	Description string         `json:"description"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// Session represents a user session
type Session struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Token     string    `gorm:"uniqueIndex;not null" json:"token"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	ExpiresAt time.Time `gorm:"not null" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	User      *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// IsExpired checks if the session is expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// PasswordReset represents a password reset request
type PasswordReset struct {
	ID        uint       `gorm:"primarykey" json:"id"`
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	Token     string     `gorm:"uniqueIndex;not null" json:"token"`
	ExpiresAt time.Time  `gorm:"not null" json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	User      *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// IsExpired checks if the password reset token is expired
func (pr *PasswordReset) IsExpired() bool {
	return time.Now().After(pr.ExpiresAt)
}

// IsUsed checks if the password reset token has been used
func (pr *PasswordReset) IsUsed() bool {
	return pr.UsedAt != nil
}
