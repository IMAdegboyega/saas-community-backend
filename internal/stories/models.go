package stories

import (
	"time"
)

// Story represents a 24-hour story
type Story struct {
	ID           int64      `json:"id" db:"id"`
	UserID       int64      `json:"user_id" db:"user_id"`
	MediaURL     string     `json:"media_url" db:"media_url"`
	MediaType    string     `json:"media_type" db:"media_type"` // image, video
	ThumbnailURL *string    `json:"thumbnail_url,omitempty" db:"thumbnail_url"`
	Caption      *string    `json:"caption,omitempty" db:"caption"`
	Duration     int        `json:"duration" db:"duration"` // display duration in seconds
	ViewsCount   int        `json:"views_count" db:"views_count"`
	ExpiresAt    time.Time  `json:"expires_at" db:"expires_at"`
	IsHighlighted bool      `json:"is_highlighted" db:"is_highlighted"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	User         *StoryUser `json:"user,omitempty"`
	IsViewed     bool       `json:"is_viewed,omitempty"`
}

// StoryUser represents user info in story
type StoryUser struct {
	ID             int64   `json:"id" db:"id"`
	Username       string  `json:"username" db:"username"`
	DisplayName    *string `json:"display_name,omitempty" db:"display_name"`
	ProfilePicture *string `json:"profile_picture,omitempty" db:"profile_picture"`
	IsVerified     bool    `json:"is_verified" db:"is_verified"`
}

// StoryView represents a story view
type StoryView struct {
	ID        int64     `json:"id" db:"id"`
	StoryID   int64     `json:"story_id" db:"story_id"`
	ViewerID  int64     `json:"viewer_id" db:"viewer_id"`
	ViewedAt  time.Time `json:"viewed_at" db:"viewed_at"`
	Viewer    *StoryUser `json:"viewer,omitempty"`
}

// UserStories represents a user's stories grouped
type UserStories struct {
	User      *StoryUser `json:"user"`
	Stories   []*Story   `json:"stories"`
	HasUnread bool       `json:"has_unread"`
	LastStoryAt time.Time `json:"last_story_at"`
}

// StoryHighlight represents a highlight collection
type StoryHighlight struct {
	ID         int64     `json:"id" db:"id"`
	UserID     int64     `json:"user_id" db:"user_id"`
	Title      string    `json:"title" db:"title"`
	CoverImage *string   `json:"cover_image,omitempty" db:"cover_image"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
	Stories    []*Story  `json:"stories,omitempty"`
}

// CreateStoryRequest represents request to create a story
type CreateStoryRequest struct {
	MediaURL     string  `json:"media_url" validate:"required,url"`
	MediaType    string  `json:"media_type" validate:"required,oneof=image video"`
	ThumbnailURL *string `json:"thumbnail_url" validate:"omitempty,url"`
	Caption      *string `json:"caption" validate:"omitempty,max=500"`
	Duration     int     `json:"duration" validate:"omitempty,min=1,max=30"`
}

// CreateHighlightRequest represents request to create a highlight
type CreateHighlightRequest struct {
	Title      string  `json:"title" validate:"required,min=1,max=100"`
	CoverImage *string `json:"cover_image" validate:"omitempty,url"`
	StoryIDs   []int64 `json:"story_ids" validate:"required,min=1"`
}

// AddToHighlightRequest represents request to add stories to highlight
type AddToHighlightRequest struct {
	StoryIDs []int64 `json:"story_ids" validate:"required,min=1"`
}
