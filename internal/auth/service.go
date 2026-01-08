package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Config holds auth configuration
type Config struct {
	JWTSecret          string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
	BCryptCost         int
}

// Service defines auth business operations
type Service interface {
	// Authentication
	Register(ctx context.Context, req *RegisterRequest) (*User, error)
	Login(ctx context.Context, req *LoginRequest, ipAddress, userAgent string) (*LoginResponse, error)
	Logout(ctx context.Context, userID int64, sessionID string) error
	LogoutAll(ctx context.Context, userID int64) error
	RefreshToken(ctx context.Context, refreshToken string) (*LoginResponse, error)
	
	// Password management
	ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error
	
	// Token validation
	ValidateAccessToken(token string) (*TokenClaims, error)
	ValidateRefreshToken(token string) (*TokenClaims, error)
	
	// User retrieval
	GetUserByID(ctx context.Context, id int64) (*User, error)
	
	// Session management
	GetUserSessions(ctx context.Context, userID int64) ([]*Session, error)
	InvalidateSession(ctx context.Context, userID int64, sessionID int64) error
	
	// Online status
	UpdateOnlineStatus(ctx context.Context, userID int64, isOnline bool) error
}

type service struct {
	repo   Repository
	config *Config
}

// NewService creates a new auth service
func NewService(repo Repository, config *Config) Service {
	return &service{
		repo:   repo,
		config: config,
	}
}

// Register creates a new user account
func (s *service) Register(ctx context.Context, req *RegisterRequest) (*User, error) {
	// Check if email exists
	exists, err := s.repo.EmailExists(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if exists {
		return nil, ErrEmailExists
	}

	// Check if username exists
	exists, err = s.repo.UsernameExists(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to check username: %w", err)
	}
	if exists {
		return nil, ErrUsernameExists
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), s.config.BCryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &User{
		Email:        req.Email,
		Username:     req.Username,
		PasswordHash: string(passwordHash),
	}
	if req.Phone != "" {
		user.Phone = &req.Phone
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// Login authenticates a user and returns tokens
func (s *service) Login(ctx context.Context, req *LoginRequest, ipAddress, userAgent string) (*LoginResponse, error) {
	// Find user
	user, err := s.repo.GetUserByIdentifier(ctx, req.Identifier)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// Check account status
	if user.AccountStatus != "active" {
		return nil, errors.New("account is not active")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Generate session ID
	sessionID := generateSecureToken(32)

	// Generate tokens
	accessToken, err := s.generateAccessToken(user, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.generateRefreshToken(user, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Create session
	session := &Session{
		UserID:           user.ID,
		TokenHash:        hashToken(accessToken),
		RefreshTokenHash: hashToken(refreshToken),
		IPAddress:        &ipAddress,
		UserAgent:        &userAgent,
		ExpiresAt:        time.Now().Add(s.config.RefreshTokenExpiry),
	}
	if req.DeviceInfo != "" {
		session.DeviceInfo = &req.DeviceInfo
	}

	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Update online status
	_ = s.repo.UpdateOnlineStatus(ctx, user.ID, true)

	return &LoginResponse{
		User:         user.ToResponse(),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.config.AccessTokenExpiry.Seconds()),
	}, nil
}

// Logout invalidates the current session
func (s *service) Logout(ctx context.Context, userID int64, sessionID string) error {
	// Get session by token hash
	sessions, err := s.repo.GetUserSessions(ctx, userID)
	if err != nil {
		return err
	}

	// Find and invalidate the session
	for _, session := range sessions {
		if err := s.repo.InvalidateSession(ctx, session.ID); err != nil {
			return err
		}
	}

	// Update online status
	_ = s.repo.UpdateOnlineStatus(ctx, userID, false)

	return nil
}

// LogoutAll invalidates all user sessions
func (s *service) LogoutAll(ctx context.Context, userID int64) error {
	if err := s.repo.InvalidateAllUserSessions(ctx, userID); err != nil {
		return err
	}
	return s.repo.UpdateOnlineStatus(ctx, userID, false)
}

// RefreshToken generates new tokens using a refresh token
func (s *service) RefreshToken(ctx context.Context, refreshToken string) (*LoginResponse, error) {
	// Validate refresh token
	claims, err := s.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	// Get session
	session, err := s.repo.GetSessionByRefreshToken(ctx, hashToken(refreshToken))
	if err != nil {
		return nil, ErrSessionNotFound
	}

	// Get user
	user, err := s.repo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	// Generate new session ID
	newSessionID := generateSecureToken(32)

	// Generate new tokens
	newAccessToken, err := s.generateAccessToken(user, newSessionID)
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := s.generateRefreshToken(user, newSessionID)
	if err != nil {
		return nil, err
	}

	// Invalidate old session
	_ = s.repo.InvalidateSession(ctx, session.ID)

	// Create new session
	newSession := &Session{
		UserID:           user.ID,
		TokenHash:        hashToken(newAccessToken),
		RefreshTokenHash: hashToken(newRefreshToken),
		DeviceInfo:       session.DeviceInfo,
		IPAddress:        session.IPAddress,
		UserAgent:        session.UserAgent,
		ExpiresAt:        time.Now().Add(s.config.RefreshTokenExpiry),
	}

	if err := s.repo.CreateSession(ctx, newSession); err != nil {
		return nil, err
	}

	return &LoginResponse{
		User:         user.ToResponse(),
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int64(s.config.AccessTokenExpiry.Seconds()),
	}, nil
}

// ChangePassword changes user password
func (s *service) ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return errors.New("current password is incorrect")
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.config.BCryptCost)
	if err != nil {
		return err
	}

	// Update password
	if err := s.repo.UpdatePassword(ctx, userID, string(newHash)); err != nil {
		return err
	}

	// Invalidate all sessions (force re-login)
	return s.repo.InvalidateAllUserSessions(ctx, userID)
}

// ValidateAccessToken validates an access token
func (s *service) ValidateAccessToken(tokenString string) (*TokenClaims, error) {
	return s.validateToken(tokenString, "access")
}

// ValidateRefreshToken validates a refresh token
func (s *service) ValidateRefreshToken(tokenString string) (*TokenClaims, error) {
	return s.validateToken(tokenString, "refresh")
}

// GetUserByID retrieves a user by ID
func (s *service) GetUserByID(ctx context.Context, id int64) (*User, error) {
	return s.repo.GetUserByID(ctx, id)
}

// GetUserSessions retrieves all sessions for a user
func (s *service) GetUserSessions(ctx context.Context, userID int64) ([]*Session, error) {
	return s.repo.GetUserSessions(ctx, userID)
}

// InvalidateSession invalidates a specific session
func (s *service) InvalidateSession(ctx context.Context, userID int64, sessionID int64) error {
	return s.repo.InvalidateSession(ctx, sessionID)
}

// UpdateOnlineStatus updates user online status
func (s *service) UpdateOnlineStatus(ctx context.Context, userID int64, isOnline bool) error {
	return s.repo.UpdateOnlineStatus(ctx, userID, isOnline)
}

// Helper functions

func (s *service) generateAccessToken(user *User, sessionID string) (string, error) {
	claims := jwt.MapClaims{
		"user_id":    user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"session_id": sessionID,
		"type":       "access",
		"exp":        time.Now().Add(s.config.AccessTokenExpiry).Unix(),
		"iat":        time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

func (s *service) generateRefreshToken(user *User, sessionID string) (string, error) {
	claims := jwt.MapClaims{
		"user_id":    user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"session_id": sessionID,
		"type":       "refresh",
		"exp":        time.Now().Add(s.config.RefreshTokenExpiry).Unix(),
		"iat":        time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

func (s *service) validateToken(tokenString string, expectedType string) (*TokenClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	// Verify token type
	tokenType, _ := claims["type"].(string)
	if tokenType != expectedType {
		return nil, fmt.Errorf("invalid token type: expected %s, got %s", expectedType, tokenType)
	}

	return &TokenClaims{
		UserID:    int64(claims["user_id"].(float64)),
		Username:  claims["username"].(string),
		Email:     claims["email"].(string),
		SessionID: claims["session_id"].(string),
		Type:      tokenType,
	}, nil
}

func generateSecureToken(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
