package auth

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserExists        = errors.New("user already exists")
	ErrEmailExists       = errors.New("email already registered")
	ErrUsernameExists    = errors.New("username already taken")
	ErrSessionNotFound   = errors.New("session not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

// Repository defines auth data operations
type Repository interface {
	// User operations
	CreateUser(ctx context.Context, user *User) error
	GetUserByID(ctx context.Context, id int64) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	GetUserByIdentifier(ctx context.Context, identifier string) (*User, error)
	GetUserWithStats(ctx context.Context, id int64) (*UserWithStats, error)
	UpdateUser(ctx context.Context, user *User) error
	UpdatePassword(ctx context.Context, userID int64, passwordHash string) error
	UpdateVerificationStatus(ctx context.Context, userID int64, field string, status bool) error
	UpdateOnlineStatus(ctx context.Context, userID int64, isOnline bool) error
	
	// Session operations
	CreateSession(ctx context.Context, session *Session) error
	GetSessionByToken(ctx context.Context, tokenHash string) (*Session, error)
	GetSessionByRefreshToken(ctx context.Context, refreshTokenHash string) (*Session, error)
	GetUserSessions(ctx context.Context, userID int64) ([]*Session, error)
	UpdateSessionLastUsed(ctx context.Context, sessionID int64) error
	InvalidateSession(ctx context.Context, sessionID int64) error
	InvalidateAllUserSessions(ctx context.Context, userID int64) error
	CleanupExpiredSessions(ctx context.Context) error
	
	// Existence checks
	EmailExists(ctx context.Context, email string) (bool, error)
	UsernameExists(ctx context.Context, username string) (bool, error)
}

// PostgresRepository implements Repository for PostgreSQL
type PostgresRepository struct {
	db *sqlx.DB
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *sqlx.DB) Repository {
	return &PostgresRepository{db: db}
}

// CreateUser creates a new user
func (r *PostgresRepository) CreateUser(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (email, username, password_hash, phone)
		VALUES ($1, $2, $3, $4)
		RETURNING id, is_verified, email_verified, phone_verified, account_status, is_online, last_seen, created_at, updated_at`

	return r.db.QueryRowxContext(ctx, query,
		user.Email, user.Username, user.PasswordHash, user.Phone,
	).Scan(
		&user.ID, &user.IsVerified, &user.EmailVerified, &user.PhoneVerified,
		&user.AccountStatus, &user.IsOnline, &user.LastSeen, &user.CreatedAt, &user.UpdatedAt,
	)
}

// GetUserByID retrieves a user by ID
func (r *PostgresRepository) GetUserByID(ctx context.Context, id int64) (*User, error) {
	user := &User{}
	query := `
		SELECT id, email, username, password_hash, phone, is_verified, email_verified, phone_verified,
		       display_name, profile_picture, bio, account_status, is_online, last_seen, created_at, updated_at
		FROM users WHERE id = $1`

	err := r.db.GetContext(ctx, user, query, id)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	return user, err
}

// GetUserWithStats retrieves a user by ID with their profile stats
func (r *PostgresRepository) GetUserWithStats(ctx context.Context, id int64) (*UserWithStats, error) {
	user := &UserWithStats{}
	query := `
		SELECT 
			u.id, u.email, u.username, u.password_hash, u.phone, u.is_verified, 
			u.email_verified, u.phone_verified, u.display_name, u.profile_picture, 
			u.bio, u.account_status, u.is_online, u.last_seen, u.created_at, u.updated_at,
			(SELECT COUNT(*) FROM posts WHERE user_id = u.id AND deleted_at IS NULL) as posts_count,
			(SELECT COUNT(*) FROM follows WHERE following_id = u.id) as followers_count,
			(SELECT COUNT(*) FROM follows WHERE follower_id = u.id) as following_count
		FROM users u
		WHERE u.id = $1`

	err := r.db.GetContext(ctx, user, query, id)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	return user, err
}

// GetUserByEmail retrieves a user by email
func (r *PostgresRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	user := &User{}
	query := `
		SELECT id, email, username, password_hash, phone, is_verified, email_verified, phone_verified,
		       display_name, profile_picture, account_status, is_online, last_seen, created_at, updated_at
		FROM users WHERE LOWER(email) = LOWER($1)`

	err := r.db.GetContext(ctx, user, query, email)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	return user, err
}

// GetUserByUsername retrieves a user by username
func (r *PostgresRepository) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	user := &User{}
	query := `
		SELECT id, email, username, password_hash, phone, is_verified, email_verified, phone_verified,
		       display_name, profile_picture, account_status, is_online, last_seen, created_at, updated_at
		FROM users WHERE LOWER(username) = LOWER($1)`

	err := r.db.GetContext(ctx, user, query, username)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	return user, err
}

// GetUserByIdentifier retrieves a user by email or username
func (r *PostgresRepository) GetUserByIdentifier(ctx context.Context, identifier string) (*User, error) {
	user := &User{}
	query := `
		SELECT id, email, username, password_hash, phone, is_verified, email_verified, phone_verified,
		       display_name, profile_picture, account_status, is_online, last_seen, created_at, updated_at
		FROM users WHERE LOWER(email) = LOWER($1) OR LOWER(username) = LOWER($1)`

	err := r.db.GetContext(ctx, user, query, identifier)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	return user, err
}

// UpdateUser updates user information
func (r *PostgresRepository) UpdateUser(ctx context.Context, user *User) error {
	query := `
		UPDATE users SET
			display_name = $2,
			phone = $3,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, user.ID, user.DisplayName, user.Phone)
	return err
}

// UpdatePassword updates user password
func (r *PostgresRepository) UpdatePassword(ctx context.Context, userID int64, passwordHash string) error {
	query := `UPDATE users SET password_hash = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, userID, passwordHash)
	return err
}

// UpdateVerificationStatus updates verification status
func (r *PostgresRepository) UpdateVerificationStatus(ctx context.Context, userID int64, field string, status bool) error {
	var query string
	switch field {
	case "email":
		query = `UPDATE users SET email_verified = $2, is_verified = (email_verified OR $2), updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	case "phone":
		query = `UPDATE users SET phone_verified = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	default:
		return errors.New("invalid verification field")
	}
	_, err := r.db.ExecContext(ctx, query, userID, status)
	return err
}

// UpdateOnlineStatus updates user online status
func (r *PostgresRepository) UpdateOnlineStatus(ctx context.Context, userID int64, isOnline bool) error {
	query := `UPDATE users SET is_online = $2, last_seen = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, userID, isOnline)
	return err
}

// CreateSession creates a new session
func (r *PostgresRepository) CreateSession(ctx context.Context, session *Session) error {
	query := `
		INSERT INTO sessions (user_id, token_hash, refresh_token_hash, device_info, ip_address, user_agent, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, is_active, created_at, last_used_at`

	return r.db.QueryRowxContext(ctx, query,
		session.UserID, session.TokenHash, session.RefreshTokenHash,
		session.DeviceInfo, session.IPAddress, session.UserAgent, session.ExpiresAt,
	).Scan(&session.ID, &session.IsActive, &session.CreatedAt, &session.LastUsedAt)
}

// GetSessionByToken retrieves a session by token hash
func (r *PostgresRepository) GetSessionByToken(ctx context.Context, tokenHash string) (*Session, error) {
	session := &Session{}
	query := `
		SELECT id, user_id, token_hash, refresh_token_hash, device_info, ip_address, user_agent,
		       is_active, expires_at, created_at, last_used_at
		FROM sessions WHERE token_hash = $1 AND is_active = TRUE AND expires_at > CURRENT_TIMESTAMP`

	err := r.db.GetContext(ctx, session, query, tokenHash)
	if err == sql.ErrNoRows {
		return nil, ErrSessionNotFound
	}
	return session, err
}

// GetSessionByRefreshToken retrieves a session by refresh token hash
func (r *PostgresRepository) GetSessionByRefreshToken(ctx context.Context, refreshTokenHash string) (*Session, error) {
	session := &Session{}
	query := `
		SELECT id, user_id, token_hash, refresh_token_hash, device_info, ip_address, user_agent,
		       is_active, expires_at, created_at, last_used_at
		FROM sessions WHERE refresh_token_hash = $1 AND is_active = TRUE`

	err := r.db.GetContext(ctx, session, query, refreshTokenHash)
	if err == sql.ErrNoRows {
		return nil, ErrSessionNotFound
	}
	return session, err
}

// GetUserSessions retrieves all active sessions for a user
func (r *PostgresRepository) GetUserSessions(ctx context.Context, userID int64) ([]*Session, error) {
	var sessions []*Session
	query := `
		SELECT id, user_id, device_info, ip_address, user_agent, is_active, expires_at, created_at, last_used_at
		FROM sessions WHERE user_id = $1 AND is_active = TRUE
		ORDER BY last_used_at DESC`

	err := r.db.SelectContext(ctx, &sessions, query, userID)
	return sessions, err
}

// UpdateSessionLastUsed updates the last used timestamp
func (r *PostgresRepository) UpdateSessionLastUsed(ctx context.Context, sessionID int64) error {
	query := `UPDATE sessions SET last_used_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, sessionID)
	return err
}

// InvalidateSession invalidates a session
func (r *PostgresRepository) InvalidateSession(ctx context.Context, sessionID int64) error {
	query := `UPDATE sessions SET is_active = FALSE WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, sessionID)
	return err
}

// InvalidateAllUserSessions invalidates all user sessions
func (r *PostgresRepository) InvalidateAllUserSessions(ctx context.Context, userID int64) error {
	query := `UPDATE sessions SET is_active = FALSE WHERE user_id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

// CleanupExpiredSessions removes expired sessions
func (r *PostgresRepository) CleanupExpiredSessions(ctx context.Context) error {
	query := `DELETE FROM sessions WHERE expires_at < CURRENT_TIMESTAMP OR is_active = FALSE`
	_, err := r.db.ExecContext(ctx, query)
	return err
}

// EmailExists checks if email is already registered
func (r *PostgresRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(email) = LOWER($1))`
	err := r.db.GetContext(ctx, &exists, query, email)
	return exists, err
}

// UsernameExists checks if username is already taken
func (r *PostgresRepository) UsernameExists(ctx context.Context, username string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(username) = LOWER($1))`
	err := r.db.GetContext(ctx, &exists, query, username)
	return exists, err
}
