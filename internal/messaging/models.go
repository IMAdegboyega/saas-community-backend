package messaging

import (
	"time"
)

// Conversation represents a chat conversation
type Conversation struct {
	ID            int64        `json:"id" db:"id"`
	Type          string       `json:"type" db:"type"` // direct, group
	Name          *string      `json:"name,omitempty" db:"name"`
	ImageURL      *string      `json:"image_url,omitempty" db:"image_url"`
	CreatedBy     *int64       `json:"created_by,omitempty" db:"created_by"`
	LastMessageID *int64       `json:"last_message_id,omitempty" db:"last_message_id"`
	LastMessageAt *time.Time   `json:"last_message_at,omitempty" db:"last_message_at"`
	CreatedAt     time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at" db:"updated_at"`
	Participants  []*Participant `json:"participants,omitempty"`
	LastMessage   *Message     `json:"last_message,omitempty"`
	UnreadCount   int          `json:"unread_count,omitempty"`
}

// Participant represents a conversation participant
type Participant struct {
	ID                int64      `json:"id" db:"id"`
	ConversationID    int64      `json:"conversation_id" db:"conversation_id"`
	UserID            int64      `json:"user_id" db:"user_id"`
	Role              string     `json:"role" db:"role"` // admin, member
	JoinedAt          time.Time  `json:"joined_at" db:"joined_at"`
	LeftAt            *time.Time `json:"left_at,omitempty" db:"left_at"`
	LastReadAt        *time.Time `json:"last_read_at,omitempty" db:"last_read_at"`
	LastReadMessageID *int64     `json:"last_read_message_id,omitempty" db:"last_read_message_id"`
	IsMuted           bool       `json:"is_muted" db:"is_muted"`
	IsArchived        bool       `json:"is_archived" db:"is_archived"`
	UnreadCount       int        `json:"unread_count" db:"unread_count"`
	User              *ChatUser  `json:"user,omitempty"`
}

// ChatUser represents a user in chat context
type ChatUser struct {
	ID             int64   `json:"id" db:"id"`
	Username       string  `json:"username" db:"username"`
	DisplayName    *string `json:"display_name,omitempty" db:"display_name"`
	ProfilePicture *string `json:"profile_picture,omitempty" db:"profile_picture"`
	IsVerified     bool    `json:"is_verified" db:"is_verified"`
	IsOnline       bool    `json:"is_online" db:"is_online"`
}

// Message represents a chat message
type Message struct {
	ID                int64      `json:"id" db:"id"`
	ConversationID    int64      `json:"conversation_id" db:"conversation_id"`
	SenderID          int64      `json:"sender_id" db:"sender_id"`
	ParentMessageID   *int64     `json:"parent_message_id,omitempty" db:"parent_message_id"`
	Content           *string    `json:"content,omitempty" db:"content"`
	MessageType       string     `json:"message_type" db:"message_type"` // text, image, video, audio, file
	MediaURL          *string    `json:"media_url,omitempty" db:"media_url"`
	MediaThumbnailURL *string    `json:"media_thumbnail_url,omitempty" db:"media_thumbnail_url"`
	MediaSize         *int       `json:"media_size,omitempty" db:"media_size"`
	MediaDuration     *int       `json:"media_duration,omitempty" db:"media_duration"`
	IsEdited          bool       `json:"is_edited" db:"is_edited"`
	EditedAt          *time.Time `json:"edited_at,omitempty" db:"edited_at"`
	IsDeleted         bool       `json:"is_deleted" db:"is_deleted"`
	DeletedAt         *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	Sender            *ChatUser  `json:"sender,omitempty"`
	ParentMessage     *Message   `json:"parent_message,omitempty"`
}

// MessageReceipt represents message delivery/read status
type MessageReceipt struct {
	ID          int64      `json:"id" db:"id"`
	MessageID   int64      `json:"message_id" db:"message_id"`
	UserID      int64      `json:"user_id" db:"user_id"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty" db:"delivered_at"`
	ReadAt      *time.Time `json:"read_at,omitempty" db:"read_at"`
}

// CreateConversationRequest for starting a new conversation
type CreateConversationRequest struct {
	Type         string  `json:"type" validate:"required,oneof=direct group"`
	ParticipantIDs []int64 `json:"participant_ids" validate:"required,min=1"`
	Name         *string `json:"name" validate:"omitempty,max=100"`
}

// SendMessageRequest for sending a message
type SendMessageRequest struct {
	Content         *string `json:"content" validate:"omitempty,max=5000"`
	MessageType     string  `json:"message_type" validate:"required,oneof=text image video audio file"`
	MediaURL        *string `json:"media_url" validate:"omitempty,url"`
	ParentMessageID *int64  `json:"parent_message_id" validate:"omitempty"`
}

// UpdateMessageRequest for editing a message
type UpdateMessageRequest struct {
	Content string `json:"content" validate:"required,max=5000"`
}

// WebSocket event types
type WSEventType string

const (
	WSEventNewMessage      WSEventType = "new_message"
	WSEventMessageEdited   WSEventType = "message_edited"
	WSEventMessageDeleted  WSEventType = "message_deleted"
	WSEventTyping          WSEventType = "typing"
	WSEventStopTyping      WSEventType = "stop_typing"
	WSEventRead            WSEventType = "read"
	WSEventOnlineStatus    WSEventType = "online_status"
	WSEventConversationUpdated WSEventType = "conversation_updated"
)

// WSEvent represents a WebSocket event
type WSEvent struct {
	Type           WSEventType `json:"type"`
	ConversationID int64       `json:"conversation_id,omitempty"`
	UserID         int64       `json:"user_id,omitempty"`
	Message        *Message    `json:"message,omitempty"`
	Data           interface{} `json:"data,omitempty"`
}

// WSIncomingMessage represents an incoming WebSocket message
type WSIncomingMessage struct {
	Type           string `json:"type"`
	ConversationID int64  `json:"conversation_id,omitempty"`
	Content        string `json:"content,omitempty"`
	MessageID      int64  `json:"message_id,omitempty"`
}
