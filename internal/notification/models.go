package notification

import (
	"time"
)

// NotificationType represents types of notifications
type NotificationType string

const (
	TypeFollow     NotificationType = "follow"
	TypeLike       NotificationType = "like"
	TypeComment    NotificationType = "comment"
	TypeMention    NotificationType = "mention"
	TypeMessage    NotificationType = "message"
	TypeStoryView  NotificationType = "story_view"
	TypeStoryReply NotificationType = "story_reply"
)

// Notification represents a notification
type Notification struct {
	ID        int64            `json:"id" db:"id"`
	UserID    int64            `json:"user_id" db:"user_id"`
	Type      NotificationType `json:"type" db:"type"`
	Title     string           `json:"title" db:"title"`
	Message   string           `json:"message" db:"message"`
	Data      map[string]interface{} `json:"data,omitempty" db:"data"`
	ActionURL *string          `json:"action_url,omitempty" db:"action_url"`
	IsRead    bool             `json:"is_read" db:"is_read"`
	ReadAt    *time.Time       `json:"read_at,omitempty" db:"read_at"`
	CreatedAt time.Time        `json:"created_at" db:"created_at"`
	Actor     *NotificationActor `json:"actor,omitempty"`
}

// NotificationActor represents the user who triggered the notification
type NotificationActor struct {
	ID             int64   `json:"id" db:"id"`
	Username       string  `json:"username" db:"username"`
	DisplayName    *string `json:"display_name,omitempty" db:"display_name"`
	ProfilePicture *string `json:"profile_picture,omitempty" db:"profile_picture"`
}

// PushToken represents a device push token
type PushToken struct {
	ID         int64     `json:"id" db:"id"`
	UserID     int64     `json:"user_id" db:"user_id"`
	Token      string    `json:"token" db:"token"`
	Platform   string    `json:"platform" db:"platform"` // ios, android, web
	DeviceID   *string   `json:"device_id,omitempty" db:"device_id"`
	IsActive   bool      `json:"is_active" db:"is_active"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// NotificationPreferences represents user notification settings
type NotificationPreferences struct {
	UserID            int64 `json:"user_id" db:"user_id"`
	PushEnabled       bool  `json:"push_enabled" db:"push_enabled"`
	EmailEnabled      bool  `json:"email_enabled" db:"email_enabled"`
	Likes             bool  `json:"likes" db:"likes"`
	Comments          bool  `json:"comments" db:"comments"`
	Follows           bool  `json:"follows" db:"follows"`
	Messages          bool  `json:"messages" db:"messages"`
	StoryViews        bool  `json:"story_views" db:"story_views"`
	Mentions          bool  `json:"mentions" db:"mentions"`
}

// CreateNotificationRequest for creating a notification
type CreateNotificationRequest struct {
	UserID    int64            `json:"user_id" validate:"required"`
	Type      NotificationType `json:"type" validate:"required"`
	Title     string           `json:"title" validate:"required,max=200"`
	Message   string           `json:"message" validate:"required,max=500"`
	Data      map[string]interface{} `json:"data,omitempty"`
	ActionURL *string          `json:"action_url,omitempty"`
	ActorID   *int64           `json:"actor_id,omitempty"`
}

// RegisterTokenRequest for registering push token
type RegisterTokenRequest struct {
	Token    string  `json:"token" validate:"required"`
	Platform string  `json:"platform" validate:"required,oneof=ios android web"`
	DeviceID *string `json:"device_id,omitempty"`
}

// UpdatePreferencesRequest for updating notification preferences
type UpdatePreferencesRequest struct {
	PushEnabled  *bool `json:"push_enabled,omitempty"`
	EmailEnabled *bool `json:"email_enabled,omitempty"`
	Likes        *bool `json:"likes,omitempty"`
	Comments     *bool `json:"comments,omitempty"`
	Follows      *bool `json:"follows,omitempty"`
	Messages     *bool `json:"messages,omitempty"`
	StoryViews   *bool `json:"story_views,omitempty"`
	Mentions     *bool `json:"mentions,omitempty"`
}
