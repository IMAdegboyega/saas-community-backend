package stories

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

var (
	ErrStoryNotFound     = errors.New("story not found")
	ErrHighlightNotFound = errors.New("highlight not found")
	ErrStoryExpired      = errors.New("story has expired")
	ErrUnauthorized      = errors.New("unauthorized")
)

type Repository interface {
	// Story operations
	CreateStory(ctx context.Context, story *Story) error
	GetStoryByID(ctx context.Context, storyID, currentUserID int64) (*Story, error)
	DeleteStory(ctx context.Context, storyID int64) error
	GetUserStories(ctx context.Context, userID, currentUserID int64) ([]*Story, error)
	GetFeedStories(ctx context.Context, userID int64) ([]*UserStories, error)
	GetActiveStoryCount(ctx context.Context, userID int64) (int, error)

	// View operations
	ViewStory(ctx context.Context, storyID, viewerID int64) error
	GetStoryViewers(ctx context.Context, storyID int64, limit, offset int) ([]*StoryView, int64, error)
	HasViewedStory(ctx context.Context, storyID, viewerID int64) (bool, error)

	// Highlight operations
	CreateHighlight(ctx context.Context, highlight *StoryHighlight) error
	GetHighlightByID(ctx context.Context, highlightID int64) (*StoryHighlight, error)
	GetUserHighlights(ctx context.Context, userID int64) ([]*StoryHighlight, error)
	UpdateHighlight(ctx context.Context, highlight *StoryHighlight) error
	DeleteHighlight(ctx context.Context, highlightID int64) error
	AddStoriesToHighlight(ctx context.Context, highlightID int64, storyIDs []int64) error
	RemoveStoryFromHighlight(ctx context.Context, highlightID, storyID int64) error

	// Cleanup
	DeleteExpiredStories(ctx context.Context) (int64, error)
}

type PostgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) Repository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateStory(ctx context.Context, story *Story) error {
	if story.Duration <= 0 {
		story.Duration = 5
	}
	story.ExpiresAt = time.Now().Add(24 * time.Hour)

	query := `
		INSERT INTO stories (user_id, media_url, media_type, thumbnail_url, caption, duration, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, views_count, is_highlighted, created_at`

	return r.db.QueryRowxContext(ctx, query,
		story.UserID, story.MediaURL, story.MediaType, story.ThumbnailURL,
		story.Caption, story.Duration, story.ExpiresAt,
	).Scan(&story.ID, &story.ViewsCount, &story.IsHighlighted, &story.CreatedAt)
}

func (r *PostgresRepository) GetStoryByID(ctx context.Context, storyID, currentUserID int64) (*Story, error) {
	story := &Story{}
	query := `
		SELECT s.id, s.user_id, s.media_url, s.media_type, s.thumbnail_url, s.caption,
			s.duration, s.views_count, s.expires_at, s.is_highlighted, s.created_at,
			EXISTS(SELECT 1 FROM story_views WHERE story_id = s.id AND viewer_id = $2) as is_viewed
		FROM stories s
		WHERE s.id = $1`

	err := r.db.QueryRowxContext(ctx, query, storyID, currentUserID).Scan(
		&story.ID, &story.UserID, &story.MediaURL, &story.MediaType, &story.ThumbnailURL,
		&story.Caption, &story.Duration, &story.ViewsCount, &story.ExpiresAt,
		&story.IsHighlighted, &story.CreatedAt, &story.IsViewed,
	)
	if err == sql.ErrNoRows {
		return nil, ErrStoryNotFound
	}
	if err != nil {
		return nil, err
	}

	// Get user info
	user := &StoryUser{}
	if err := r.db.GetContext(ctx, user,
		`SELECT id, username, display_name, profile_picture, is_verified FROM users WHERE id = $1`,
		story.UserID); err == nil {
		story.User = user
	}

	return story, nil
}

func (r *PostgresRepository) DeleteStory(ctx context.Context, storyID int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM stories WHERE id = $1`, storyID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrStoryNotFound
	}
	return nil
}

func (r *PostgresRepository) GetUserStories(ctx context.Context, userID, currentUserID int64) ([]*Story, error) {
	stories := []*Story{}
	query := `
		SELECT s.id, s.user_id, s.media_url, s.media_type, s.thumbnail_url, s.caption,
			s.duration, s.views_count, s.expires_at, s.is_highlighted, s.created_at,
			EXISTS(SELECT 1 FROM story_views WHERE story_id = s.id AND viewer_id = $2) as is_viewed
		FROM stories s
		WHERE s.user_id = $1 AND s.expires_at > CURRENT_TIMESTAMP
		ORDER BY s.created_at ASC`

	rows, err := r.db.QueryxContext(ctx, query, userID, currentUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		story := &Story{}
		if err := rows.Scan(&story.ID, &story.UserID, &story.MediaURL, &story.MediaType,
			&story.ThumbnailURL, &story.Caption, &story.Duration, &story.ViewsCount,
			&story.ExpiresAt, &story.IsHighlighted, &story.CreatedAt, &story.IsViewed); err != nil {
			continue
		}
		stories = append(stories, story)
	}
	return stories, nil
}

func (r *PostgresRepository) GetFeedStories(ctx context.Context, userID int64) ([]*UserStories, error) {
	// Get users with active stories that the current user follows
	// Simplified query to avoid prepared statement parameter issues
	query := `
		SELECT 
			u.id, u.username, u.display_name, u.profile_picture, u.is_verified,
			MAX(s.created_at) as last_story_at,
			COALESCE(
				(SELECT COUNT(*) = 0 FROM stories s2 
				 WHERE s2.user_id = u.id 
				 AND s2.expires_at > CURRENT_TIMESTAMP
				 AND NOT EXISTS(SELECT 1 FROM story_views sv WHERE sv.story_id = s2.id AND sv.viewer_id = $1)),
				true
			) as all_viewed
		FROM users u
		JOIN stories s ON u.id = s.user_id
		LEFT JOIN follows f ON u.id = f.following_id AND f.follower_id = $1
		WHERE s.expires_at > CURRENT_TIMESTAMP
			AND (f.follower_id = $1 OR u.id = $1)
			AND NOT EXISTS(SELECT 1 FROM blocks b1 WHERE b1.blocker_id = u.id AND b1.blocked_id = $1)
			AND NOT EXISTS(SELECT 1 FROM blocks b2 WHERE b2.blocker_id = $1 AND b2.blocked_id = u.id)
		GROUP BY u.id, u.username, u.display_name, u.profile_picture, u.is_verified
		ORDER BY all_viewed ASC, last_story_at DESC`

	rows, err := r.db.QueryxContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	userStoriesMap := make(map[int64]*UserStories)
	var userIDs []int64

	for rows.Next() {
		var user StoryUser
		var lastStoryAt time.Time
		var allViewed bool

		if err := rows.Scan(&user.ID, &user.Username, &user.DisplayName, &user.ProfilePicture,
			&user.IsVerified, &lastStoryAt, &allViewed); err != nil {
			continue
		}

		userStoriesMap[user.ID] = &UserStories{
			User:        &user,
			Stories:     []*Story{},
			HasUnread:   !allViewed,
			LastStoryAt: lastStoryAt,
		}
		userIDs = append(userIDs, user.ID)
	}

	// Get stories for each user
	for _, uid := range userIDs {
		stories, err := r.GetUserStories(ctx, uid, userID)
		if err == nil {
			userStoriesMap[uid].Stories = stories
		}
	}

	// Convert to slice
	result := make([]*UserStories, 0, len(userStoriesMap))
	for _, us := range userStoriesMap {
		result = append(result, us)
	}

	return result, nil
}

func (r *PostgresRepository) GetActiveStoryCount(ctx context.Context, userID int64) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM stories WHERE user_id = $1 AND expires_at > CURRENT_TIMESTAMP`, userID)
	return count, err
}

func (r *PostgresRepository) ViewStory(ctx context.Context, storyID, viewerID int64) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO story_views (story_id, viewer_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		storyID, viewerID)
	return err
}

func (r *PostgresRepository) GetStoryViewers(ctx context.Context, storyID int64, limit, offset int) ([]*StoryView, int64, error) {
	if limit <= 0 {
		limit = 20
	}

	var total int64
	r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM story_views WHERE story_id = $1`, storyID)

	views := []*StoryView{}
	query := `
		SELECT sv.id, sv.story_id, sv.viewer_id, sv.viewed_at,
			u.id, u.username, u.display_name, u.profile_picture, u.is_verified
		FROM story_views sv
		JOIN users u ON sv.viewer_id = u.id
		WHERE sv.story_id = $1
		ORDER BY sv.viewed_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryxContext(ctx, query, storyID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		view := &StoryView{Viewer: &StoryUser{}}
		if err := rows.Scan(&view.ID, &view.StoryID, &view.ViewerID, &view.ViewedAt,
			&view.Viewer.ID, &view.Viewer.Username, &view.Viewer.DisplayName,
			&view.Viewer.ProfilePicture, &view.Viewer.IsVerified); err != nil {
			continue
		}
		views = append(views, view)
	}

	return views, total, nil
}

func (r *PostgresRepository) HasViewedStory(ctx context.Context, storyID, viewerID int64) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists,
		`SELECT EXISTS(SELECT 1 FROM story_views WHERE story_id = $1 AND viewer_id = $2)`,
		storyID, viewerID)
	return exists, err
}

func (r *PostgresRepository) CreateHighlight(ctx context.Context, highlight *StoryHighlight) error {
	query := `
		INSERT INTO story_highlights (user_id, title, cover_image)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at`

	return r.db.QueryRowxContext(ctx, query,
		highlight.UserID, highlight.Title, highlight.CoverImage,
	).Scan(&highlight.ID, &highlight.CreatedAt, &highlight.UpdatedAt)
}

func (r *PostgresRepository) GetHighlightByID(ctx context.Context, highlightID int64) (*StoryHighlight, error) {
	highlight := &StoryHighlight{}
	err := r.db.GetContext(ctx, highlight,
		`SELECT id, user_id, title, cover_image, created_at, updated_at 
		FROM story_highlights WHERE id = $1`, highlightID)
	if err == sql.ErrNoRows {
		return nil, ErrHighlightNotFound
	}
	return highlight, err
}

func (r *PostgresRepository) GetUserHighlights(ctx context.Context, userID int64) ([]*StoryHighlight, error) {
	highlights := []*StoryHighlight{}
	err := r.db.SelectContext(ctx, &highlights,
		`SELECT id, user_id, title, cover_image, created_at, updated_at 
		FROM story_highlights WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	return highlights, err
}

func (r *PostgresRepository) UpdateHighlight(ctx context.Context, highlight *StoryHighlight) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE story_highlights SET title = $2, cover_image = $3, updated_at = CURRENT_TIMESTAMP WHERE id = $1`,
		highlight.ID, highlight.Title, highlight.CoverImage)
	return err
}

func (r *PostgresRepository) DeleteHighlight(ctx context.Context, highlightID int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM story_highlights WHERE id = $1`, highlightID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrHighlightNotFound
	}
	return nil
}

func (r *PostgresRepository) AddStoriesToHighlight(ctx context.Context, highlightID int64, storyIDs []int64) error {
	for i, storyID := range storyIDs {
		_, err := r.db.ExecContext(ctx,
			`INSERT INTO highlight_stories (highlight_id, story_id, position) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			highlightID, storyID, i)
		if err != nil {
			return err
		}
		// Mark story as highlighted
		r.db.ExecContext(ctx, `UPDATE stories SET is_highlighted = TRUE WHERE id = $1`, storyID)
	}
	return nil
}

func (r *PostgresRepository) RemoveStoryFromHighlight(ctx context.Context, highlightID, storyID int64) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM highlight_stories WHERE highlight_id = $1 AND story_id = $2`,
		highlightID, storyID)
	return err
}

func (r *PostgresRepository) DeleteExpiredStories(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM stories WHERE expires_at < CURRENT_TIMESTAMP AND is_highlighted = FALSE`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
