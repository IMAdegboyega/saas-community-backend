package messaging

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

var (
	ErrConversationNotFound = errors.New("conversation not found")
	ErrMessageNotFound      = errors.New("message not found")
	ErrNotParticipant       = errors.New("not a participant of this conversation")
	ErrUnauthorized         = errors.New("unauthorized")
)

type Repository interface {
	// Conversation operations
	CreateConversation(ctx context.Context, conv *Conversation) error
	GetConversationByID(ctx context.Context, convID, userID int64) (*Conversation, error)
	GetDirectConversation(ctx context.Context, userID1, userID2 int64) (*Conversation, error)
	GetUserConversations(ctx context.Context, userID int64, limit, offset int) ([]*Conversation, int64, error)
	UpdateConversation(ctx context.Context, conv *Conversation) error
	DeleteConversation(ctx context.Context, convID int64) error

	// Participant operations
	AddParticipant(ctx context.Context, convID, userID int64, role string) error
	RemoveParticipant(ctx context.Context, convID, userID int64) error
	GetParticipants(ctx context.Context, convID int64) ([]*Participant, error)
	IsParticipant(ctx context.Context, convID, userID int64) (bool, error)
	UpdateParticipant(ctx context.Context, p *Participant) error

	// Message operations
	CreateMessage(ctx context.Context, msg *Message) error
	GetMessageByID(ctx context.Context, msgID int64) (*Message, error)
	GetConversationMessages(ctx context.Context, convID, userID int64, limit, offset int) ([]*Message, int64, error)
	UpdateMessage(ctx context.Context, msg *Message) error
	DeleteMessage(ctx context.Context, msgID int64) error
	MarkAsRead(ctx context.Context, convID, userID int64, messageID int64) error
	GetUnreadCount(ctx context.Context, userID int64) (int64, error)
}

type PostgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) Repository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateConversation(ctx context.Context, conv *Conversation) error {
	query := `
		INSERT INTO conversations (type, name, image_url, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id, last_message_id, last_message_at, created_at, updated_at`

	return r.db.QueryRowxContext(ctx, query,
		conv.Type, conv.Name, conv.ImageURL, conv.CreatedBy,
	).Scan(&conv.ID, &conv.LastMessageID, &conv.LastMessageAt, &conv.CreatedAt, &conv.UpdatedAt)
}

func (r *PostgresRepository) GetConversationByID(ctx context.Context, convID, userID int64) (*Conversation, error) {
	// First check if user is a participant
	isParticipant, err := r.IsParticipant(ctx, convID, userID)
	if err != nil {
		return nil, err
	}
	if !isParticipant {
		return nil, ErrNotParticipant
	}

	conv := &Conversation{}
	query := `
		SELECT c.id, c.type, c.name, c.image_url, c.created_by, 
			c.last_message_id, c.last_message_at, c.created_at, c.updated_at,
			COALESCE(cp.unread_count, 0) as unread_count
		FROM conversations c
		LEFT JOIN conversation_participants cp ON c.id = cp.conversation_id AND cp.user_id = $2
		WHERE c.id = $1`

	err = r.db.QueryRowxContext(ctx, query, convID, userID).Scan(
		&conv.ID, &conv.Type, &conv.Name, &conv.ImageURL, &conv.CreatedBy,
		&conv.LastMessageID, &conv.LastMessageAt, &conv.CreatedAt, &conv.UpdatedAt,
		&conv.UnreadCount,
	)
	if err == sql.ErrNoRows {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, err
	}

	// Get participants
	participants, _ := r.GetParticipants(ctx, convID)
	conv.Participants = participants

	return conv, nil
}

func (r *PostgresRepository) GetDirectConversation(ctx context.Context, userID1, userID2 int64) (*Conversation, error) {
	var convID int64
	query := `
		SELECT c.id FROM conversations c
		JOIN conversation_participants cp1 ON c.id = cp1.conversation_id AND cp1.user_id = $1
		JOIN conversation_participants cp2 ON c.id = cp2.conversation_id AND cp2.user_id = $2
		WHERE c.type = 'direct'
		LIMIT 1`

	err := r.db.GetContext(ctx, &convID, query, userID1, userID2)
	if err == sql.ErrNoRows {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, err
	}

	return r.GetConversationByID(ctx, convID, userID1)
}

func (r *PostgresRepository) GetUserConversations(ctx context.Context, userID int64, limit, offset int) ([]*Conversation, int64, error) {
	if limit <= 0 {
		limit = 20
	}

	var total int64
	r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM conversation_participants WHERE user_id = $1 AND left_at IS NULL`, userID)

	conversations := []*Conversation{}
	query := `
		SELECT c.id, c.type, c.name, c.image_url, c.created_by,
			c.last_message_id, c.last_message_at, c.created_at, c.updated_at,
			COALESCE(cp.unread_count, 0) as unread_count
		FROM conversations c
		JOIN conversation_participants cp ON c.id = cp.conversation_id
		WHERE cp.user_id = $1 AND cp.left_at IS NULL AND cp.is_archived = FALSE
		ORDER BY COALESCE(c.last_message_at, c.created_at) DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryxContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		conv := &Conversation{}
		if err := rows.Scan(&conv.ID, &conv.Type, &conv.Name, &conv.ImageURL, &conv.CreatedBy,
			&conv.LastMessageID, &conv.LastMessageAt, &conv.CreatedAt, &conv.UpdatedAt,
			&conv.UnreadCount); err != nil {
			continue
		}

		// Get participants for each conversation
		participants, _ := r.GetParticipants(ctx, conv.ID)
		conv.Participants = participants

		conversations = append(conversations, conv)
	}

	return conversations, total, nil
}

func (r *PostgresRepository) UpdateConversation(ctx context.Context, conv *Conversation) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE conversations SET name = $2, image_url = $3, last_message_id = $4, 
		last_message_at = $5, updated_at = CURRENT_TIMESTAMP WHERE id = $1`,
		conv.ID, conv.Name, conv.ImageURL, conv.LastMessageID, conv.LastMessageAt)
	return err
}

func (r *PostgresRepository) DeleteConversation(ctx context.Context, convID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM conversations WHERE id = $1`, convID)
	return err
}

func (r *PostgresRepository) AddParticipant(ctx context.Context, convID, userID int64, role string) error {
	if role == "" {
		role = "member"
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO conversation_participants (conversation_id, user_id, role) VALUES ($1, $2, $3)
		ON CONFLICT (conversation_id, user_id) DO UPDATE SET left_at = NULL`,
		convID, userID, role)
	return err
}

func (r *PostgresRepository) RemoveParticipant(ctx context.Context, convID, userID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE conversation_participants SET left_at = CURRENT_TIMESTAMP WHERE conversation_id = $1 AND user_id = $2`,
		convID, userID)
	return err
}

func (r *PostgresRepository) GetParticipants(ctx context.Context, convID int64) ([]*Participant, error) {
	participants := []*Participant{}
	query := `
		SELECT cp.id, cp.conversation_id, cp.user_id, cp.role, cp.joined_at,
			cp.left_at, cp.last_read_at, cp.last_read_message_id, cp.is_muted, cp.is_archived, cp.unread_count,
			u.id, u.username, u.display_name, u.profile_picture, u.is_verified, u.is_online
		FROM conversation_participants cp
		JOIN users u ON cp.user_id = u.id
		WHERE cp.conversation_id = $1 AND cp.left_at IS NULL`

	rows, err := r.db.QueryxContext(ctx, query, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		p := &Participant{User: &ChatUser{}}
		if err := rows.Scan(&p.ID, &p.ConversationID, &p.UserID, &p.Role, &p.JoinedAt,
			&p.LeftAt, &p.LastReadAt, &p.LastReadMessageID, &p.IsMuted, &p.IsArchived, &p.UnreadCount,
			&p.User.ID, &p.User.Username, &p.User.DisplayName, &p.User.ProfilePicture,
			&p.User.IsVerified, &p.User.IsOnline); err != nil {
			continue
		}
		participants = append(participants, p)
	}

	return participants, nil
}

func (r *PostgresRepository) IsParticipant(ctx context.Context, convID, userID int64) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists,
		`SELECT EXISTS(SELECT 1 FROM conversation_participants WHERE conversation_id = $1 AND user_id = $2 AND left_at IS NULL)`,
		convID, userID)
	return exists, err
}

func (r *PostgresRepository) UpdateParticipant(ctx context.Context, p *Participant) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE conversation_participants SET is_muted = $3, is_archived = $4 WHERE conversation_id = $1 AND user_id = $2`,
		p.ConversationID, p.UserID, p.IsMuted, p.IsArchived)
	return err
}

func (r *PostgresRepository) CreateMessage(ctx context.Context, msg *Message) error {
	query := `
		INSERT INTO messages (conversation_id, sender_id, parent_message_id, content, message_type, media_url, media_thumbnail_url, media_size, media_duration)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, is_edited, is_deleted, created_at`

	err := r.db.QueryRowxContext(ctx, query,
		msg.ConversationID, msg.SenderID, msg.ParentMessageID, msg.Content, msg.MessageType,
		msg.MediaURL, msg.MediaThumbnailURL, msg.MediaSize, msg.MediaDuration,
	).Scan(&msg.ID, &msg.IsEdited, &msg.IsDeleted, &msg.CreatedAt)

	if err != nil {
		return err
	}

	// Update conversation
	now := time.Now()
	r.db.ExecContext(ctx,
		`UPDATE conversations SET last_message_id = $2, last_message_at = $3, updated_at = $3 WHERE id = $1`,
		msg.ConversationID, msg.ID, now)

	// Increment unread count for other participants
	r.db.ExecContext(ctx,
		`UPDATE conversation_participants SET unread_count = unread_count + 1 WHERE conversation_id = $1 AND user_id != $2`,
		msg.ConversationID, msg.SenderID)

	return nil
}

func (r *PostgresRepository) GetMessageByID(ctx context.Context, msgID int64) (*Message, error) {
	msg := &Message{}
	query := `
		SELECT m.id, m.conversation_id, m.sender_id, m.parent_message_id, m.content, m.message_type,
			m.media_url, m.media_thumbnail_url, m.media_size, m.media_duration,
			m.is_edited, m.edited_at, m.is_deleted, m.deleted_at, m.created_at
		FROM messages m WHERE m.id = $1`

	err := r.db.GetContext(ctx, msg, query, msgID)
	if err == sql.ErrNoRows {
		return nil, ErrMessageNotFound
	}
	return msg, err
}

func (r *PostgresRepository) GetConversationMessages(ctx context.Context, convID, userID int64, limit, offset int) ([]*Message, int64, error) {
	// Check participation
	isParticipant, err := r.IsParticipant(ctx, convID, userID)
	if err != nil {
		return nil, 0, err
	}
	if !isParticipant {
		return nil, 0, ErrNotParticipant
	}

	if limit <= 0 {
		limit = 50
	}

	var total int64
	r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM messages WHERE conversation_id = $1 AND is_deleted = FALSE`, convID)

	messages := []*Message{}
	query := `
		SELECT m.id, m.conversation_id, m.sender_id, m.parent_message_id, m.content, m.message_type,
			m.media_url, m.media_thumbnail_url, m.media_size, m.media_duration,
			m.is_edited, m.edited_at, m.is_deleted, m.deleted_at, m.created_at,
			u.id, u.username, u.display_name, u.profile_picture, u.is_verified, u.is_online
		FROM messages m
		JOIN users u ON m.sender_id = u.id
		WHERE m.conversation_id = $1 AND m.is_deleted = FALSE
		ORDER BY m.created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryxContext(ctx, query, convID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		msg := &Message{Sender: &ChatUser{}}
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.SenderID, &msg.ParentMessageID,
			&msg.Content, &msg.MessageType, &msg.MediaURL, &msg.MediaThumbnailURL,
			&msg.MediaSize, &msg.MediaDuration, &msg.IsEdited, &msg.EditedAt,
			&msg.IsDeleted, &msg.DeletedAt, &msg.CreatedAt,
			&msg.Sender.ID, &msg.Sender.Username, &msg.Sender.DisplayName,
			&msg.Sender.ProfilePicture, &msg.Sender.IsVerified, &msg.Sender.IsOnline); err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	return messages, total, nil
}

func (r *PostgresRepository) UpdateMessage(ctx context.Context, msg *Message) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE messages SET content = $2, is_edited = TRUE, edited_at = $3 WHERE id = $1`,
		msg.ID, msg.Content, now)
	return err
}

func (r *PostgresRepository) DeleteMessage(ctx context.Context, msgID int64) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE messages SET is_deleted = TRUE, deleted_at = $2, content = NULL WHERE id = $1`,
		msgID, now)
	return err
}

func (r *PostgresRepository) MarkAsRead(ctx context.Context, convID, userID int64, messageID int64) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE conversation_participants SET last_read_at = $3, last_read_message_id = $4, unread_count = 0 
		WHERE conversation_id = $1 AND user_id = $2`,
		convID, userID, now, messageID)
	return err
}

func (r *PostgresRepository) GetUnreadCount(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.db.GetContext(ctx, &count,
		`SELECT COALESCE(SUM(unread_count), 0) FROM conversation_participants WHERE user_id = $1 AND left_at IS NULL`,
		userID)
	return count, err
}
