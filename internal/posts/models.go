package posts

import (
	"time"
)

// Post represents a social media post
type Post struct {
	ID            int64       `json:"id" db:"id"`
	UserID        int64       `json:"user_id" db:"user_id"`
	Caption       *string     `json:"caption,omitempty" db:"caption"`
	Location      *string     `json:"location,omitempty" db:"location"`
	Latitude      *float64    `json:"latitude,omitempty" db:"latitude"`
	Longitude     *float64    `json:"longitude,omitempty" db:"longitude"`
	Visibility    string      `json:"visibility" db:"visibility"`
	IsPinned      bool        `json:"is_pinned" db:"is_pinned"`
	IsArchived    bool        `json:"is_archived" db:"is_archived"`
	LikesCount    int         `json:"likes_count" db:"likes_count"`
	CommentsCount int         `json:"comments_count" db:"comments_count"`
	SharesCount   int         `json:"shares_count" db:"shares_count"`
	CreatedAt     time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at" db:"updated_at"`
	Media         []PostMedia `json:"media,omitempty"`
	User          *PostUser   `json:"user,omitempty"`
	IsLiked       bool        `json:"is_liked,omitempty"`
	IsSaved       bool        `json:"is_saved,omitempty"`
}

// PostMedia represents media attached to a post
type PostMedia struct {
	ID           int64     `json:"id" db:"id"`
	PostID       int64     `json:"post_id" db:"post_id"`
	MediaURL     string    `json:"media_url" db:"media_url"`
	MediaType    string    `json:"media_type" db:"media_type"`
	ThumbnailURL *string   `json:"thumbnail_url,omitempty" db:"thumbnail_url"`
	Width        *int      `json:"width,omitempty" db:"width"`
	Height       *int      `json:"height,omitempty" db:"height"`
	Duration     *int      `json:"duration,omitempty" db:"duration"`
	Position     int       `json:"position" db:"position"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// PostUser represents the user info in a post response
type PostUser struct {
	ID             int64   `json:"id" db:"id"`
	Username       string  `json:"username" db:"username"`
	DisplayName    *string `json:"display_name,omitempty" db:"display_name"`
	ProfilePicture *string `json:"profile_picture,omitempty" db:"profile_picture"`
	IsVerified     bool    `json:"is_verified" db:"is_verified"`
}

// Comment represents a comment on a post
type Comment struct {
	ID         int64        `json:"id" db:"id"`
	PostID     int64        `json:"post_id" db:"post_id"`
	UserID     int64        `json:"user_id" db:"user_id"`
	ParentID   *int64       `json:"parent_id,omitempty" db:"parent_id"`
	Content    string       `json:"content" db:"content"`
	LikesCount int          `json:"likes_count" db:"likes_count"`
	IsEdited   bool         `json:"is_edited" db:"is_edited"`
	CreatedAt  time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at" db:"updated_at"`
	User       *PostUser    `json:"user,omitempty"`
	Replies    []Comment    `json:"replies,omitempty"`
	IsLiked    bool         `json:"is_liked,omitempty"`
}

// PostLike represents a like on a post
type PostLike struct {
	ID        int64     `json:"id" db:"id"`
	PostID    int64     `json:"post_id" db:"post_id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// SavedPost represents a saved post
type SavedPost struct {
	ID        int64     `json:"id" db:"id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	PostID    int64     `json:"post_id" db:"post_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// CreatePostRequest represents a request to create a post
type CreatePostRequest struct {
	Caption    *string  `json:"caption" validate:"omitempty,max=2000"`
	Location   *string  `json:"location" validate:"omitempty,max=200"`
	Latitude   *float64 `json:"latitude" validate:"omitempty"`
	Longitude  *float64 `json:"longitude" validate:"omitempty"`
	Visibility string   `json:"visibility" validate:"omitempty,oneof=public followers private"`
}

// UpdatePostRequest represents a request to update a post
type UpdatePostRequest struct {
	Caption    *string `json:"caption" validate:"omitempty,max=2000"`
	Location   *string `json:"location" validate:"omitempty,max=200"`
	Visibility *string `json:"visibility" validate:"omitempty,oneof=public followers private"`
}

// CreateCommentRequest represents a request to create a comment
type CreateCommentRequest struct {
	Content  string `json:"content" validate:"required,min=1,max=1000"`
	ParentID *int64 `json:"parent_id" validate:"omitempty"`
}

// UpdateCommentRequest represents a request to update a comment
type UpdateCommentRequest struct {
	Content string `json:"content" validate:"required,min=1,max=1000"`
}

// FeedRequest represents parameters for getting feed
type FeedRequest struct {
	Limit  int    `json:"limit" validate:"omitempty,min=1,max=50"`
	Offset int    `json:"offset" validate:"omitempty,min=0"`
	Type   string `json:"type" validate:"omitempty,oneof=following explore"` // following = from followed users, explore = all public
}

// UserPostsRequest represents parameters for getting user posts
type UserPostsRequest struct {
	UserID int64 `json:"-"`
	Limit  int   `json:"limit" validate:"omitempty,min=1,max=50"`
	Offset int   `json:"offset" validate:"omitempty,min=0"`
}
