package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

var (
	ErrNotificationNotFound = errors.New("notification not found")
)

type Repository interface {
	// Notification operations
	Create(ctx context.Context, n *Notification) error
	GetByID(ctx context.Context, id int64) (*Notification, error)
	GetUserNotifications(ctx context.Context, userID int64, limit, offset int) ([]*Notification, int64, error)
	MarkAsRead(ctx context.Context, id, userID int64) error
	MarkAllAsRead(ctx context.Context, userID int64) error
	Delete(ctx context.Context, id, userID int64) error
	DeleteAll(ctx context.Context, userID int64) error
	GetUnreadCount(ctx context.Context, userID int64) (int64, error)

	// Push token operations
	SavePushToken(ctx context.Context, token *PushToken) error
	GetUserPushTokens(ctx context.Context, userID int64) ([]*PushToken, error)
	DeletePushToken(ctx context.Context, token string) error
	DeactivatePushToken(ctx context.Context, token string) error

	// Preferences
	GetPreferences(ctx context.Context, userID int64) (*NotificationPreferences, error)
	UpdatePreferences(ctx context.Context, prefs *NotificationPreferences) error
}

type PostgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) Repository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, n *Notification) error {
	dataJSON, _ := json.Marshal(n.Data)

	query := `
		INSERT INTO notifications (user_id, type, title, message, data, action_url)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, is_read, created_at`

	err := r.db.QueryRowxContext(ctx, query,
		n.UserID, n.Type, n.Title, n.Message, dataJSON, n.ActionURL,
	).Scan(&n.ID, &n.IsRead, &n.CreatedAt)
	
	if err != nil {
		fmt.Printf("ERROR: notification insert failed - UserID: %d, Type: %s, Error: %v\n", n.UserID, n.Type, err)
	}
	return err
}

func (r *PostgresRepository) GetByID(ctx context.Context, id int64) (*Notification, error) {
	n := &Notification{}
	var dataJSON []byte

	query := `SELECT id, user_id, type, title, message, data, action_url, is_read, read_at, created_at
		FROM notifications WHERE id = $1`

	err := r.db.QueryRowxContext(ctx, query, id).Scan(
		&n.ID, &n.UserID, &n.Type, &n.Title, &n.Message, &dataJSON, &n.ActionURL, &n.IsRead, &n.ReadAt, &n.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotificationNotFound
	}
	if err != nil {
		return nil, err
	}

	if len(dataJSON) > 0 {
		json.Unmarshal(dataJSON, &n.Data)
	}

	return n, nil
}

func (r *PostgresRepository) GetUserNotifications(ctx context.Context, userID int64, limit, offset int) ([]*Notification, int64, error) {
	if limit <= 0 {
		limit = 20
	}

	var total int64
	err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM notifications WHERE user_id = $1`, userID)
	if err != nil {
		fmt.Printf("ERROR: Failed to count notifications for user %d: %v\n", userID, err)
	}
	fmt.Printf("INFO: GetUserNotifications - UserID: %d, Total: %d\n", userID, total)

	notifications := []*Notification{}
	query := `
		SELECT n.id, n.user_id, n.type, n.title, n.message, n.data, n.action_url, n.is_read, n.read_at, n.created_at
		FROM notifications n
		WHERE n.user_id = $1
		ORDER BY n.created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryxContext(ctx, query, userID, limit, offset)
	if err != nil {
		fmt.Printf("ERROR: Failed to query notifications: %v\n", err)
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		n := &Notification{}
		var dataJSON []byte
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Message, &dataJSON, &n.ActionURL, &n.IsRead, &n.ReadAt, &n.CreatedAt); err != nil {
			fmt.Printf("ERROR: Failed to scan notification row: %v\n", err)
			continue
		}
		if len(dataJSON) > 0 {
			json.Unmarshal(dataJSON, &n.Data)
		}

		// Try to get actor info from data
		if actorID, ok := n.Data["actor_id"].(float64); ok {
			actor := &NotificationActor{}
			if err := r.db.GetContext(ctx, actor,
				`SELECT id, username, display_name, profile_picture FROM users WHERE id = $1`,
				int64(actorID)); err == nil {
				n.Actor = actor
			}
		}

		notifications = append(notifications, n)
	}

	fmt.Printf("INFO: Returning %d notifications for user %d\n", len(notifications), userID)
	return notifications, total, nil
}

func (r *PostgresRepository) MarkAsRead(ctx context.Context, id, userID int64) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE notifications SET is_read = TRUE, read_at = $3 WHERE id = $1 AND user_id = $2`,
		id, userID, now)
	return err
}

func (r *PostgresRepository) MarkAllAsRead(ctx context.Context, userID int64) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE notifications SET is_read = TRUE, read_at = $2 WHERE user_id = $1 AND is_read = FALSE`,
		userID, now)
	return err
}

func (r *PostgresRepository) Delete(ctx context.Context, id, userID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM notifications WHERE id = $1 AND user_id = $2`, id, userID)
	return err
}

func (r *PostgresRepository) DeleteAll(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM notifications WHERE user_id = $1`, userID)
	return err
}

func (r *PostgresRepository) GetUnreadCount(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = FALSE`, userID)
	return count, err
}

func (r *PostgresRepository) SavePushToken(ctx context.Context, token *PushToken) error {
	query := `
		INSERT INTO push_tokens (user_id, token, platform, device_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (token) DO UPDATE SET user_id = $1, platform = $3, device_id = $4, is_active = TRUE, updated_at = CURRENT_TIMESTAMP
		RETURNING id, is_active, created_at, updated_at`

	return r.db.QueryRowxContext(ctx, query,
		token.UserID, token.Token, token.Platform, token.DeviceID,
	).Scan(&token.ID, &token.IsActive, &token.CreatedAt, &token.UpdatedAt)
}

func (r *PostgresRepository) GetUserPushTokens(ctx context.Context, userID int64) ([]*PushToken, error) {
	tokens := []*PushToken{}
	err := r.db.SelectContext(ctx, &tokens,
		`SELECT id, user_id, token, platform, device_id, is_active, last_used_at, created_at, updated_at
		FROM push_tokens WHERE user_id = $1 AND is_active = TRUE`, userID)
	return tokens, err
}

func (r *PostgresRepository) DeletePushToken(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM push_tokens WHERE token = $1`, token)
	return err
}

func (r *PostgresRepository) DeactivatePushToken(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE push_tokens SET is_active = FALSE WHERE token = $1`, token)
	return err
}

func (r *PostgresRepository) GetPreferences(ctx context.Context, userID int64) (*NotificationPreferences, error) {
	prefs := &NotificationPreferences{
		UserID:       userID,
		PushEnabled:  true,
		EmailEnabled: true,
		Likes:        true,
		Comments:     true,
		Follows:      true,
		Messages:     true,
		StoryViews:   true,
		Mentions:     true,
	}
	// In a real implementation, you'd fetch from a preferences table
	// For now, return defaults
	return prefs, nil
}

func (r *PostgresRepository) UpdatePreferences(ctx context.Context, prefs *NotificationPreferences) error {
	// In a real implementation, upsert to preferences table
	return nil
}
