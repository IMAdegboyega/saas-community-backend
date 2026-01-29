package user

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrAlreadyFollowing = errors.New("already following this user")
	ErrNotFollowing     = errors.New("not following this user")
	ErrCannotFollowSelf = errors.New("cannot follow yourself")
	ErrAlreadyBlocked   = errors.New("user already blocked")
	ErrNotBlocked       = errors.New("user not blocked")
	ErrCannotBlockSelf  = errors.New("cannot block yourself")
	ErrBlockedByUser    = errors.New("you are blocked by this user")
)

// Repository defines user data operations
type Repository interface {
	// User operations
	GetUserByID(ctx context.Context, id int64) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	GetUserWithStats(ctx context.Context, id int64, currentUserID int64) (*UserWithStats, error)
	SearchUsers(ctx context.Context, query string, currentUserID int64, limit, offset int) ([]*FollowUser, error)
	
	// Follow operations
	Follow(ctx context.Context, followerID, followingID int64) error
	Unfollow(ctx context.Context, followerID, followingID int64) error
	IsFollowing(ctx context.Context, followerID, followingID int64) (bool, error)
	GetFollowers(ctx context.Context, userID, currentUserID int64, limit, offset int) ([]*FollowUser, int64, error)
	GetFollowing(ctx context.Context, userID, currentUserID int64, limit, offset int) ([]*FollowUser, int64, error)
	GetFollowStats(ctx context.Context, userID int64) (*FollowStats, error)
	GetMutualFollowers(ctx context.Context, userID, otherUserID int64, limit, offset int) ([]*FollowUser, error)
	
	// Block operations
	Block(ctx context.Context, blockerID, blockedID int64, reason *string) error
	Unblock(ctx context.Context, blockerID, blockedID int64) error
	IsBlocked(ctx context.Context, blockerID, blockedID int64) (bool, error)
	IsBlockedEither(ctx context.Context, userID1, userID2 int64) (bool, error)
	GetBlockedUsers(ctx context.Context, userID int64, limit, offset int) ([]*BlockedUser, int64, error)
	
	// Suggestions
	GetSuggestedUsers(ctx context.Context, userID int64, limit int) ([]*FollowUser, error)
}

// PostgresRepository implements Repository for PostgreSQL
type PostgresRepository struct {
	db *sqlx.DB
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *sqlx.DB) Repository {
	return &PostgresRepository{db: db}
}

// GetUserByID retrieves a user by ID
func (r *PostgresRepository) GetUserByID(ctx context.Context, id int64) (*User, error) {
	user := &User{}
	query := `
		SELECT id, username, display_name, profile_picture, bio, is_verified, is_online
		FROM users WHERE id = $1 AND account_status = 'active'`

	err := r.db.GetContext(ctx, user, query, id)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	return user, err
}

// GetUserByUsername retrieves a user by username
func (r *PostgresRepository) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	user := &User{}
	query := `
		SELECT id, username, display_name, profile_picture, bio, is_verified, is_online
		FROM users WHERE LOWER(username) = LOWER($1) AND account_status = 'active'`

	err := r.db.GetContext(ctx, user, query, username)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	return user, err
}

// GetUserWithStats retrieves a user with follow stats
func (r *PostgresRepository) GetUserWithStats(ctx context.Context, id int64, currentUserID int64) (*UserWithStats, error) {
	user := &UserWithStats{}
	query := `
		SELECT 
			u.id, u.username, u.display_name, u.profile_picture, u.bio, u.is_verified, u.is_online,
			(SELECT COUNT(*) FROM follows WHERE following_id = u.id) as followers_count,
			(SELECT COUNT(*) FROM follows WHERE follower_id = u.id) as following_count,
			(SELECT COUNT(*) FROM posts WHERE user_id = u.id AND is_archived = FALSE) as posts_count,
			EXISTS(SELECT 1 FROM follows WHERE follower_id = $2 AND following_id = u.id) as is_following,
			EXISTS(SELECT 1 FROM follows WHERE follower_id = u.id AND following_id = $2) as is_followed_by
		FROM users u
		WHERE u.id = $1 AND u.account_status = 'active'`

	err := r.db.GetContext(ctx, user, query, id, currentUserID)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	return user, err
}

// SearchUsers searches users by username or display name
func (r *PostgresRepository) SearchUsers(ctx context.Context, query string, currentUserID int64, limit, offset int) ([]*FollowUser, error) {
	if limit <= 0 {
		limit = 20
	}

	users := []*FollowUser{}
	sqlQuery := `
		SELECT 
			u.id, u.username, u.display_name, u.profile_picture, u.is_verified,
			EXISTS(SELECT 1 FROM follows WHERE follower_id = $2 AND following_id = u.id) as is_following,
			u.created_at as followed_at
		FROM users u
		WHERE u.account_status = 'active'
			AND u.id != $2
			AND NOT EXISTS(SELECT 1 FROM blocks WHERE blocker_id = u.id AND blocked_id = $2)
			AND NOT EXISTS(SELECT 1 FROM blocks WHERE blocker_id = $2 AND blocked_id = u.id)
			AND (
				LOWER(u.username) LIKE LOWER($1) 
				OR LOWER(u.display_name) LIKE LOWER($1)
			)
		ORDER BY 
			CASE WHEN LOWER(u.username) = LOWER($3) THEN 0 ELSE 1 END,
			u.is_verified DESC,
			u.username
		LIMIT $4 OFFSET $5`

	searchPattern := "%" + query + "%"
	err := r.db.SelectContext(ctx, &users, sqlQuery, searchPattern, currentUserID, query, limit, offset)
	return users, err
}

// Follow creates a follow relationship
func (r *PostgresRepository) Follow(ctx context.Context, followerID, followingID int64) error {
	if followerID == followingID {
		return ErrCannotFollowSelf
	}

	// Check if blocked
	blocked, err := r.IsBlockedEither(ctx, followerID, followingID)
	if err != nil {
		return err
	}
	if blocked {
		return ErrBlockedByUser
	}

	// Check if already following
	following, err := r.IsFollowing(ctx, followerID, followingID)
	if err != nil {
		return err
	}
	if following {
		return ErrAlreadyFollowing
	}

	query := `INSERT INTO follows (follower_id, following_id) VALUES ($1, $2)`
	_, err = r.db.ExecContext(ctx, query, followerID, followingID)
	return err
}

// Unfollow removes a follow relationship
func (r *PostgresRepository) Unfollow(ctx context.Context, followerID, followingID int64) error {
	query := `DELETE FROM follows WHERE follower_id = $1 AND following_id = $2`
	result, err := r.db.ExecContext(ctx, query, followerID, followingID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFollowing
	}
	return nil
}

// IsFollowing checks if a user is following another
func (r *PostgresRepository) IsFollowing(ctx context.Context, followerID, followingID int64) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND following_id = $2)`
	err := r.db.GetContext(ctx, &exists, query, followerID, followingID)
	return exists, err
}

// GetFollowers retrieves users who follow a given user
func (r *PostgresRepository) GetFollowers(ctx context.Context, userID, currentUserID int64, limit, offset int) ([]*FollowUser, int64, error) {
	if limit <= 0 {
		limit = 20
	}

	// Get total count
	var total int64
	countQuery := `SELECT COUNT(*) FROM follows WHERE following_id = $1`
	if err := r.db.GetContext(ctx, &total, countQuery, userID); err != nil {
		return nil, 0, err
	}

	// Get followers
	followers := []*FollowUser{}
	query := `
		SELECT 
			u.id, u.username, u.display_name, u.profile_picture, u.is_verified,
			EXISTS(SELECT 1 FROM follows WHERE follower_id = $2 AND following_id = u.id) as is_following,
			f.created_at as followed_at
		FROM follows f
		JOIN users u ON f.follower_id = u.id
		WHERE f.following_id = $1 AND u.account_status = 'active'
		ORDER BY f.created_at DESC
		LIMIT $3 OFFSET $4`

	err := r.db.SelectContext(ctx, &followers, query, userID, currentUserID, limit, offset)
	return followers, total, err
}

// GetFollowing retrieves users that a given user follows
func (r *PostgresRepository) GetFollowing(ctx context.Context, userID, currentUserID int64, limit, offset int) ([]*FollowUser, int64, error) {
	if limit <= 0 {
		limit = 20
	}

	// Get total count
	var total int64
	countQuery := `SELECT COUNT(*) FROM follows WHERE follower_id = $1`
	if err := r.db.GetContext(ctx, &total, countQuery, userID); err != nil {
		return nil, 0, err
	}

	// Get following
	following := []*FollowUser{}
	query := `
		SELECT 
			u.id, u.username, u.display_name, u.profile_picture, u.is_verified,
			EXISTS(SELECT 1 FROM follows WHERE follower_id = $2 AND following_id = u.id) as is_following,
			f.created_at as followed_at
		FROM follows f
		JOIN users u ON f.following_id = u.id
		WHERE f.follower_id = $1 AND u.account_status = 'active'
		ORDER BY f.created_at DESC
		LIMIT $3 OFFSET $4`

	err := r.db.SelectContext(ctx, &following, query, userID, currentUserID, limit, offset)
	return following, total, err
}

// GetFollowStats gets follow statistics for a user
func (r *PostgresRepository) GetFollowStats(ctx context.Context, userID int64) (*FollowStats, error) {
	stats := &FollowStats{}
	query := `
		SELECT 
			(SELECT COUNT(*) FROM follows WHERE following_id = $1) as followers_count,
			(SELECT COUNT(*) FROM follows WHERE follower_id = $1) as following_count`

	err := r.db.GetContext(ctx, stats, query, userID)
	return stats, err
}

// GetMutualFollowers gets users who follow both the current user and another user
func (r *PostgresRepository) GetMutualFollowers(ctx context.Context, userID, otherUserID int64, limit, offset int) ([]*FollowUser, error) {
	if limit <= 0 {
		limit = 20
	}

	mutuals := []*FollowUser{}
	query := `
		SELECT 
			u.id, u.username, u.display_name, u.profile_picture, u.is_verified,
			TRUE as is_following,
			f1.created_at as followed_at
		FROM follows f1
		JOIN follows f2 ON f1.follower_id = f2.follower_id
		JOIN users u ON f1.follower_id = u.id
		WHERE f1.following_id = $1 
			AND f2.following_id = $2
			AND u.account_status = 'active'
		ORDER BY f1.created_at DESC
		LIMIT $3 OFFSET $4`

	err := r.db.SelectContext(ctx, &mutuals, query, userID, otherUserID, limit, offset)
	return mutuals, err
}

// Block blocks a user
func (r *PostgresRepository) Block(ctx context.Context, blockerID, blockedID int64, reason *string) error {
	if blockerID == blockedID {
		return ErrCannotBlockSelf
	}

	// Check if already blocked
	blocked, err := r.IsBlocked(ctx, blockerID, blockedID)
	if err != nil {
		return err
	}
	if blocked {
		return ErrAlreadyBlocked
	}

	// Start transaction
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Remove any follow relationships
	_, err = tx.ExecContext(ctx, `DELETE FROM follows WHERE (follower_id = $1 AND following_id = $2) OR (follower_id = $2 AND following_id = $1)`, blockerID, blockedID)
	if err != nil {
		return err
	}

	// Create block
	_, err = tx.ExecContext(ctx, `INSERT INTO blocks (blocker_id, blocked_id, reason) VALUES ($1, $2, $3)`, blockerID, blockedID, reason)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Unblock unblocks a user
func (r *PostgresRepository) Unblock(ctx context.Context, blockerID, blockedID int64) error {
	query := `DELETE FROM blocks WHERE blocker_id = $1 AND blocked_id = $2`
	result, err := r.db.ExecContext(ctx, query, blockerID, blockedID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotBlocked
	}
	return nil
}

// IsBlocked checks if a user has blocked another
func (r *PostgresRepository) IsBlocked(ctx context.Context, blockerID, blockedID int64) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM blocks WHERE blocker_id = $1 AND blocked_id = $2)`
	err := r.db.GetContext(ctx, &exists, query, blockerID, blockedID)
	return exists, err
}

// IsBlockedEither checks if either user has blocked the other
func (r *PostgresRepository) IsBlockedEither(ctx context.Context, userID1, userID2 int64) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(
		SELECT 1 FROM blocks 
		WHERE (blocker_id = $1 AND blocked_id = $2) OR (blocker_id = $2 AND blocked_id = $1)
	)`
	err := r.db.GetContext(ctx, &exists, query, userID1, userID2)
	return exists, err
}

// GetBlockedUsers retrieves users blocked by a given user
func (r *PostgresRepository) GetBlockedUsers(ctx context.Context, userID int64, limit, offset int) ([]*BlockedUser, int64, error) {
	if limit <= 0 {
		limit = 20
	}

	// Get total count
	var total int64
	countQuery := `SELECT COUNT(*) FROM blocks WHERE blocker_id = $1`
	if err := r.db.GetContext(ctx, &total, countQuery, userID); err != nil {
		return nil, 0, err
	}

	// Get blocked users
	blocked := []*BlockedUser{}
	query := `
		SELECT 
			u.id, u.username, u.display_name, u.profile_picture,
			b.created_at as blocked_at
		FROM blocks b
		JOIN users u ON b.blocked_id = u.id
		WHERE b.blocker_id = $1
		ORDER BY b.created_at DESC
		LIMIT $2 OFFSET $3`

	err := r.db.SelectContext(ctx, &blocked, query, userID, limit, offset)
	return blocked, total, err
}

// GetSuggestedUsers gets suggested users to follow
// Returns ALL users except current user (for small platforms)
// Includes whether they follow you (for "Follow Back" button)
func (r *PostgresRepository) GetSuggestedUsers(ctx context.Context, userID int64, limit int) ([]*FollowUser, error) {
	if limit <= 0 {
		limit = 10
	}

	suggestions := []*FollowUser{}
	query := `
		SELECT DISTINCT
			u.id, u.username, u.display_name, u.profile_picture, u.is_verified,
			EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND following_id = u.id) as is_following,
			EXISTS(SELECT 1 FROM follows WHERE follower_id = u.id AND following_id = $1) as is_following_you,
			u.created_at as followed_at
		FROM users u
		WHERE u.id != $1
			AND u.account_status = 'active'
			AND NOT EXISTS(SELECT 1 FROM blocks WHERE blocker_id = $1 AND blocked_id = u.id)
			AND NOT EXISTS(SELECT 1 FROM blocks WHERE blocker_id = u.id AND blocked_id = $1)
		ORDER BY 
			EXISTS(SELECT 1 FROM follows WHERE follower_id = u.id AND following_id = $1) DESC,
			u.is_verified DESC,
			(SELECT COUNT(*) FROM follows WHERE following_id = u.id) DESC
		LIMIT $2`

	err := r.db.SelectContext(ctx, &suggestions, query, userID, limit)
	return suggestions, err
}
