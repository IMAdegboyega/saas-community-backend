package auth

import (
	"time"
)

// User represents a user in the system
type User struct {
	ID              int64     `json:"id" db:"id"`
	Email           string    `json:"email" db:"email"`
	Username        string    `json:"username" db:"username"`
	PasswordHash    string    `json:"-" db:"password_hash"`
	Phone           *string   `json:"phone,omitempty" db:"phone"`
	IsVerified      bool      `json:"is_verified" db:"is_verified"`
	EmailVerified   bool      `json:"email_verified" db:"email_verified"`
	PhoneVerified   bool      `json:"phone_verified" db:"phone_verified"`
	DisplayName     *string   `json:"display_name,omitempty" db:"display_name"`
	ProfilePicture  *string   `json:"profile_picture,omitempty" db:"profile_picture"`
	Bio             *string   `json:"bio,omitempty" db:"bio"`
	AccountStatus   string    `json:"account_status" db:"account_status"`
	IsOnline        bool      `json:"is_online" db:"is_online"`
	LastSeen        time.Time `json:"last_seen" db:"last_seen"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// UserWithStats represents a user with their profile stats
type UserWithStats struct {
	User
	PostsCount     int64 `json:"posts_count" db:"posts_count"`
	FollowersCount int64 `json:"followers_count" db:"followers_count"`
	FollowingCount int64 `json:"following_count" db:"following_count"`
}

// Session represents a user session
type Session struct {
	ID               int64     `json:"id" db:"id"`
	UserID           int64     `json:"user_id" db:"user_id"`
	TokenHash        string    `json:"-" db:"token_hash"`
	RefreshTokenHash string    `json:"-" db:"refresh_token_hash"`
	DeviceInfo       *string   `json:"device_info,omitempty" db:"device_info"`
	IPAddress        *string   `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent        *string   `json:"user_agent,omitempty" db:"user_agent"`
	IsActive         bool      `json:"is_active" db:"is_active"`
	ExpiresAt        time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	LastUsedAt       time.Time `json:"last_used_at" db:"last_used_at"`
}

// RegisterRequest represents registration request
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Username string `json:"username" validate:"required,username"`
	Password string `json:"password" validate:"required,min=8,max=72"`
	Phone    string `json:"phone,omitempty" validate:"omitempty,phone"`
}

// LoginRequest represents login request
type LoginRequest struct {
	Identifier string `json:"identifier" validate:"required"` // email or username
	Password   string `json:"password" validate:"required"`
	DeviceInfo string `json:"device_info,omitempty"`
}

// LoginResponse represents login response
type LoginResponse struct {
	User         *UserResponse `json:"user"`
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	ExpiresIn    int64         `json:"expires_in"` // seconds
}

// UserResponse is a safe user representation (no sensitive data)
type UserResponse struct {
	ID             int64   `json:"id"`
	Email          string  `json:"email"`
	Username       string  `json:"username"`
	Phone          *string `json:"phone,omitempty"`
	IsVerified     bool    `json:"is_verified"`
	EmailVerified  bool    `json:"email_verified"`
	PhoneVerified  bool    `json:"phone_verified"`
	DisplayName    *string `json:"display_name,omitempty"`
	ProfilePicture *string `json:"profile_picture,omitempty"`
	AccountStatus  string  `json:"account_status"`
	Bio            *string `json:"bio,omitempty"`
	// Profile stats
	PostsCount     int64 `json:"posts_count"`
	FollowersCount int64 `json:"followers_count"`
	FollowingCount int64 `json:"following_count"`
}

// RefreshRequest represents token refresh request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// ChangePasswordRequest represents password change request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8,max=72"`
}

// ResetPasswordRequest represents password reset request
type ResetPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ConfirmResetRequest represents password reset confirmation
type ConfirmResetRequest struct {
	Email       string `json:"email" validate:"required,email"`
	Code        string `json:"code" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=72"`
}

// VerifyEmailRequest represents email verification request
type VerifyEmailRequest struct {
	Email string `json:"email" validate:"required,email"`
	Code  string `json:"code" validate:"required"`
}

// VerifyPhoneRequest represents phone verification request
type VerifyPhoneRequest struct {
	Phone string `json:"phone" validate:"required,phone"`
	Code  string `json:"code" validate:"required"`
}

// TokenClaims represents JWT token claims
type TokenClaims struct {
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	SessionID string `json:"session_id"`
	Type      string `json:"type"` // "access" or "refresh"
}

// ToResponse converts User to UserResponse
func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID:             u.ID,
		Email:          u.Email,
		Username:       u.Username,
		Phone:          u.Phone,
		IsVerified:     u.IsVerified,
		EmailVerified:  u.EmailVerified,
		PhoneVerified:  u.PhoneVerified,
		DisplayName:    u.DisplayName,
		ProfilePicture: u.ProfilePicture,
		AccountStatus:  u.AccountStatus,
		Bio:            u.Bio,
	}
}

// ToResponse converts UserWithStats to UserResponse with stats
func (u *UserWithStats) ToResponse() *UserResponse {
	return &UserResponse{
		ID:             u.ID,
		Email:          u.Email,
		Username:       u.Username,
		Phone:          u.Phone,
		IsVerified:     u.IsVerified,
		EmailVerified:  u.EmailVerified,
		PhoneVerified:  u.PhoneVerified,
		DisplayName:    u.DisplayName,
		ProfilePicture: u.ProfilePicture,
		AccountStatus:  u.AccountStatus,
		Bio:            u.Bio,
		PostsCount:     u.PostsCount,
		FollowersCount: u.FollowersCount,
		FollowingCount: u.FollowingCount,
	}
}
