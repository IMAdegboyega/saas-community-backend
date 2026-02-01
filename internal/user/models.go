package user

import (
	"time"
)

// User represents a user (simplified view for user operations)
type User struct {
	ID             int64   `json:"id" db:"id"`
	Username       string  `json:"username" db:"username"`
	DisplayName    *string `json:"display_name,omitempty" db:"display_name"`
	ProfilePicture *string `json:"profile_picture,omitempty" db:"profile_picture"`
	Bio            *string `json:"bio,omitempty" db:"bio"`
	IsVerified     bool    `json:"is_verified" db:"is_verified"`
	IsOnline       bool    `json:"is_online" db:"is_online"`
}

// UserWithStats includes follower/following counts
type UserWithStats struct {
	User
	FollowersCount int64 `json:"followers_count" db:"followers_count"`
	FollowingCount int64 `json:"following_count" db:"following_count"`
	PostsCount     int64 `json:"posts_count" db:"posts_count"`
	IsFollowing    bool  `json:"is_following,omitempty" db:"is_following"` // Whether current user follows this user
	IsFollowedBy   bool  `json:"is_followed_by,omitempty" db:"is_followed_by"` // Whether this user follows current user
}

// Follow represents a follow relationship
type Follow struct {
	ID          int64     `json:"id" db:"id"`
	FollowerID  int64     `json:"follower_id" db:"follower_id"`
	FollowingID int64     `json:"following_id" db:"following_id"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// FollowUser represents a user in follow lists
type FollowUser struct {
	ID             int64     `json:"id" db:"id"`
	Username       string    `json:"username" db:"username"`
	DisplayName    *string   `json:"display_name,omitempty" db:"display_name"`
	ProfilePicture *string   `json:"profile_picture,omitempty" db:"profile_picture"`
	IsVerified     bool      `json:"is_verified" db:"is_verified"`
	IsFollowing    bool      `json:"is_following" db:"is_following"` // Whether current user follows this user
	IsFollowingYou bool      `json:"is_following_you" db:"is_following_you"` // Whether this user follows current user (for "Follow Back")
	FollowedAt     time.Time `json:"followed_at" db:"followed_at"`
}

// Block represents a block relationship
type Block struct {
	ID        int64     `json:"id" db:"id"`
	BlockerID int64     `json:"blocker_id" db:"blocker_id"`
	BlockedID int64     `json:"blocked_id" db:"blocked_id"`
	Reason    *string   `json:"reason,omitempty" db:"reason"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// BlockedUser represents a blocked user in lists
type BlockedUser struct {
	ID             int64     `json:"id" db:"id"`
	Username       string    `json:"username" db:"username"`
	DisplayName    *string   `json:"display_name,omitempty" db:"display_name"`
	ProfilePicture *string   `json:"profile_picture,omitempty" db:"profile_picture"`
	BlockedAt      time.Time `json:"blocked_at" db:"blocked_at"`
}

// SearchRequest represents user search parameters
type SearchRequest struct {
	Query  string `json:"query" validate:"required,min=1,max=100"`
	Limit  int    `json:"limit" validate:"omitempty,min=1,max=50"`
	Offset int    `json:"offset" validate:"omitempty,min=0"`
}

// FollowListRequest represents pagination for follow lists
type FollowListRequest struct {
	UserID int64 `json:"-"`
	Limit  int   `json:"limit" validate:"omitempty,min=1,max=50"`
	Offset int   `json:"offset" validate:"omitempty,min=0"`
}

// BlockRequest represents block request
type BlockRequest struct {
	UserID int64   `json:"user_id" validate:"required"`
	Reason *string `json:"reason,omitempty" validate:"omitempty,max=500"`
}

// FollowStats represents follow statistics
type FollowStats struct {
	FollowersCount int64 `json:"followers_count"`
	FollowingCount int64 `json:"following_count"`
}
