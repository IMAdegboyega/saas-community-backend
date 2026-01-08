package posts

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
)

var (
	ErrPostNotFound    = errors.New("post not found")
	ErrCommentNotFound = errors.New("comment not found")
	ErrAlreadyLiked    = errors.New("already liked")
	ErrNotLiked        = errors.New("not liked")
	ErrAlreadySaved    = errors.New("already saved")
	ErrNotSaved        = errors.New("not saved")
	ErrUnauthorized    = errors.New("unauthorized")
)

// Repository defines post data operations
type Repository interface {
	CreatePost(ctx context.Context, post *Post) error
	GetPostByID(ctx context.Context, postID, currentUserID int64) (*Post, error)
	UpdatePost(ctx context.Context, post *Post) error
	DeletePost(ctx context.Context, postID int64) error
	GetUserPosts(ctx context.Context, userID, currentUserID int64, limit, offset int) ([]*Post, int64, error)
	GetFeed(ctx context.Context, userID int64, feedType string, limit, offset int) ([]*Post, error)
	AddPostMedia(ctx context.Context, media *PostMedia) error
	GetPostMedia(ctx context.Context, postID int64) ([]PostMedia, error)
	LikePost(ctx context.Context, postID, userID int64) error
	UnlikePost(ctx context.Context, postID, userID int64) error
	SavePost(ctx context.Context, postID, userID int64) error
	UnsavePost(ctx context.Context, postID, userID int64) error
	GetSavedPosts(ctx context.Context, userID int64, limit, offset int) ([]*Post, int64, error)
	CreateComment(ctx context.Context, comment *Comment) error
	GetPostComments(ctx context.Context, postID, currentUserID int64, limit, offset int) ([]*Comment, int64, error)
	DeleteComment(ctx context.Context, commentID int64) error
	GetCommentByID(ctx context.Context, commentID int64) (*Comment, error)
}

type PostgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) Repository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreatePost(ctx context.Context, post *Post) error {
	query := `
		INSERT INTO posts (user_id, caption, location, latitude, longitude, visibility)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, is_pinned, is_archived, likes_count, comments_count, shares_count, created_at, updated_at`
	return r.db.QueryRowxContext(ctx, query,
		post.UserID, post.Caption, post.Location, post.Latitude, post.Longitude, post.Visibility,
	).Scan(&post.ID, &post.IsPinned, &post.IsArchived, &post.LikesCount, &post.CommentsCount, &post.SharesCount, &post.CreatedAt, &post.UpdatedAt)
}

func (r *PostgresRepository) GetPostByID(ctx context.Context, postID, currentUserID int64) (*Post, error) {
	post := &Post{}
	query := `
		SELECT p.id, p.user_id, p.caption, p.location, p.latitude, p.longitude,
			p.visibility, p.is_pinned, p.is_archived, p.likes_count, p.comments_count, p.shares_count,
			p.created_at, p.updated_at,
			EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $2) as is_liked,
			EXISTS(SELECT 1 FROM saved_posts WHERE post_id = p.id AND user_id = $2) as is_saved
		FROM posts p WHERE p.id = $1 AND p.is_archived = FALSE`

	err := r.db.QueryRowxContext(ctx, query, postID, currentUserID).Scan(
		&post.ID, &post.UserID, &post.Caption, &post.Location, &post.Latitude, &post.Longitude,
		&post.Visibility, &post.IsPinned, &post.IsArchived, &post.LikesCount, &post.CommentsCount, &post.SharesCount,
		&post.CreatedAt, &post.UpdatedAt, &post.IsLiked, &post.IsSaved,
	)
	if err == sql.ErrNoRows {
		return nil, ErrPostNotFound
	}
	if err != nil {
		return nil, err
	}

	user := &PostUser{}
	if err := r.db.GetContext(ctx, user, `SELECT id, username, display_name, profile_picture, is_verified FROM users WHERE id = $1`, post.UserID); err == nil {
		post.User = user
	}
	media, _ := r.GetPostMedia(ctx, postID)
	post.Media = media
	return post, nil
}

func (r *PostgresRepository) UpdatePost(ctx context.Context, post *Post) error {
	query := `UPDATE posts SET caption = $2, location = $3, visibility = $4, updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, post.ID, post.Caption, post.Location, post.Visibility)
	return err
}

func (r *PostgresRepository) DeletePost(ctx context.Context, postID int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM posts WHERE id = $1`, postID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrPostNotFound
	}
	return nil
}

func (r *PostgresRepository) GetUserPosts(ctx context.Context, userID, currentUserID int64, limit, offset int) ([]*Post, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	var total int64
	r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM posts WHERE user_id = $1 AND is_archived = FALSE`, userID)

	posts := []*Post{}
	query := `
		SELECT p.id, p.user_id, p.caption, p.location, p.visibility, p.is_pinned,
			p.likes_count, p.comments_count, p.created_at,
			EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $2) as is_liked,
			EXISTS(SELECT 1 FROM saved_posts WHERE post_id = p.id AND user_id = $2) as is_saved
		FROM posts p
		WHERE p.user_id = $1 AND p.is_archived = FALSE
		ORDER BY p.is_pinned DESC, p.created_at DESC
		LIMIT $3 OFFSET $4`

	rows, err := r.db.QueryxContext(ctx, query, userID, currentUserID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		post := &Post{}
		if err := rows.Scan(&post.ID, &post.UserID, &post.Caption, &post.Location, &post.Visibility, &post.IsPinned,
			&post.LikesCount, &post.CommentsCount, &post.CreatedAt, &post.IsLiked, &post.IsSaved); err != nil {
			continue
		}
		media, _ := r.GetPostMedia(ctx, post.ID)
		post.Media = media
		posts = append(posts, post)
	}
	return posts, total, nil
}

func (r *PostgresRepository) GetFeed(ctx context.Context, userID int64, feedType string, limit, offset int) ([]*Post, error) {
	if limit <= 0 {
		limit = 20
	}

	var query string
	if feedType == "following" {
		query = `
			SELECT p.id, p.user_id, p.caption, p.location, p.visibility,
				p.likes_count, p.comments_count, p.created_at,
				u.id, u.username, u.display_name, u.profile_picture, u.is_verified,
				EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $1) as is_liked,
				EXISTS(SELECT 1 FROM saved_posts WHERE post_id = p.id AND user_id = $1) as is_saved
			FROM posts p
			JOIN users u ON p.user_id = u.id
			JOIN follows f ON p.user_id = f.following_id
			WHERE f.follower_id = $1 AND p.is_archived = FALSE AND p.visibility IN ('public', 'followers')
			ORDER BY p.created_at DESC
			LIMIT $2 OFFSET $3`
	} else {
		query = `
			SELECT p.id, p.user_id, p.caption, p.location, p.visibility,
				p.likes_count, p.comments_count, p.created_at,
				u.id, u.username, u.display_name, u.profile_picture, u.is_verified,
				EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $1) as is_liked,
				EXISTS(SELECT 1 FROM saved_posts WHERE post_id = p.id AND user_id = $1) as is_saved
			FROM posts p
			JOIN users u ON p.user_id = u.id
			WHERE p.is_archived = FALSE AND p.visibility = 'public'
				AND NOT EXISTS(SELECT 1 FROM blocks WHERE blocker_id = p.user_id AND blocked_id = $1)
				AND NOT EXISTS(SELECT 1 FROM blocks WHERE blocker_id = $1 AND blocked_id = p.user_id)
			ORDER BY p.created_at DESC
			LIMIT $2 OFFSET $3`
	}

	rows, err := r.db.QueryxContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := []*Post{}
	for rows.Next() {
		post := &Post{User: &PostUser{}}
		if err := rows.Scan(&post.ID, &post.UserID, &post.Caption, &post.Location, &post.Visibility,
			&post.LikesCount, &post.CommentsCount, &post.CreatedAt,
			&post.User.ID, &post.User.Username, &post.User.DisplayName, &post.User.ProfilePicture, &post.User.IsVerified,
			&post.IsLiked, &post.IsSaved); err != nil {
			continue
		}
		media, _ := r.GetPostMedia(ctx, post.ID)
		post.Media = media
		posts = append(posts, post)
	}
	return posts, nil
}

func (r *PostgresRepository) AddPostMedia(ctx context.Context, media *PostMedia) error {
	query := `
		INSERT INTO post_media (post_id, media_url, media_type, thumbnail_url, width, height, duration, position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at`
	return r.db.QueryRowxContext(ctx, query,
		media.PostID, media.MediaURL, media.MediaType, media.ThumbnailURL, media.Width, media.Height, media.Duration, media.Position,
	).Scan(&media.ID, &media.CreatedAt)
}

func (r *PostgresRepository) GetPostMedia(ctx context.Context, postID int64) ([]PostMedia, error) {
	media := []PostMedia{}
	query := `SELECT id, post_id, media_url, media_type, thumbnail_url, width, height, duration, position, created_at
		FROM post_media WHERE post_id = $1 ORDER BY position`
	err := r.db.SelectContext(ctx, &media, query, postID)
	return media, err
}

func (r *PostgresRepository) LikePost(ctx context.Context, postID, userID int64) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO post_likes (post_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, postID, userID)
	return err
}

func (r *PostgresRepository) UnlikePost(ctx context.Context, postID, userID int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM post_likes WHERE post_id = $1 AND user_id = $2`, postID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotLiked
	}
	return nil
}

func (r *PostgresRepository) SavePost(ctx context.Context, postID, userID int64) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO saved_posts (post_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, postID, userID)
	return err
}

func (r *PostgresRepository) UnsavePost(ctx context.Context, postID, userID int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM saved_posts WHERE post_id = $1 AND user_id = $2`, postID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotSaved
	}
	return nil
}

func (r *PostgresRepository) GetSavedPosts(ctx context.Context, userID int64, limit, offset int) ([]*Post, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	var total int64
	r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM saved_posts WHERE user_id = $1`, userID)

	posts := []*Post{}
	query := `
		SELECT p.id, p.user_id, p.caption, p.location, p.visibility,
			p.likes_count, p.comments_count, p.created_at, TRUE as is_saved
		FROM posts p
		JOIN saved_posts sp ON p.id = sp.post_id
		WHERE sp.user_id = $1 AND p.is_archived = FALSE
		ORDER BY sp.created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryxContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		post := &Post{}
		if err := rows.Scan(&post.ID, &post.UserID, &post.Caption, &post.Location, &post.Visibility,
			&post.LikesCount, &post.CommentsCount, &post.CreatedAt, &post.IsSaved); err != nil {
			continue
		}
		media, _ := r.GetPostMedia(ctx, post.ID)
		post.Media = media
		posts = append(posts, post)
	}
	return posts, total, nil
}

func (r *PostgresRepository) CreateComment(ctx context.Context, comment *Comment) error {
	query := `
		INSERT INTO comments (post_id, user_id, parent_id, content)
		VALUES ($1, $2, $3, $4)
		RETURNING id, likes_count, is_edited, created_at, updated_at`
	return r.db.QueryRowxContext(ctx, query, comment.PostID, comment.UserID, comment.ParentID, comment.Content,
	).Scan(&comment.ID, &comment.LikesCount, &comment.IsEdited, &comment.CreatedAt, &comment.UpdatedAt)
}

func (r *PostgresRepository) GetCommentByID(ctx context.Context, commentID int64) (*Comment, error) {
	comment := &Comment{}
	err := r.db.GetContext(ctx, comment, `SELECT id, post_id, user_id, parent_id, content, likes_count, is_edited, created_at, updated_at FROM comments WHERE id = $1`, commentID)
	if err == sql.ErrNoRows {
		return nil, ErrCommentNotFound
	}
	return comment, err
}

func (r *PostgresRepository) GetPostComments(ctx context.Context, postID, currentUserID int64, limit, offset int) ([]*Comment, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	var total int64
	r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM comments WHERE post_id = $1 AND parent_id IS NULL`, postID)

	comments := []*Comment{}
	query := `
		SELECT c.id, c.post_id, c.user_id, c.content, c.likes_count, c.is_edited, c.created_at,
			u.id, u.username, u.display_name, u.profile_picture, u.is_verified
		FROM comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.post_id = $1 AND c.parent_id IS NULL
		ORDER BY c.created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryxContext(ctx, query, postID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		comment := &Comment{User: &PostUser{}}
		if err := rows.Scan(&comment.ID, &comment.PostID, &comment.UserID, &comment.Content, &comment.LikesCount, &comment.IsEdited, &comment.CreatedAt,
			&comment.User.ID, &comment.User.Username, &comment.User.DisplayName, &comment.User.ProfilePicture, &comment.User.IsVerified); err != nil {
			continue
		}
		comments = append(comments, comment)
	}
	return comments, total, nil
}

func (r *PostgresRepository) DeleteComment(ctx context.Context, commentID int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM comments WHERE id = $1`, commentID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrCommentNotFound
	}
	return nil
}
