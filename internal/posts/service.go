package posts

import (
	"context"
	"errors"
	"fmt"
)

// Service defines post business operations
type Service interface {
	CreatePost(ctx context.Context, userID int64, req *CreatePostRequest) (*Post, error)
	GetPost(ctx context.Context, postID, currentUserID int64) (*Post, error)
	UpdatePost(ctx context.Context, userID, postID int64, req *UpdatePostRequest) (*Post, error)
	DeletePost(ctx context.Context, userID, postID int64) error
	GetUserPosts(ctx context.Context, userID, currentUserID int64, limit, offset int) ([]*Post, int64, error)
	GetFeed(ctx context.Context, userID int64, feedType string, limit, offset int) ([]*Post, error)
	AddPostMedia(ctx context.Context, userID, postID int64, media *PostMedia) error
	LikePost(ctx context.Context, userID, postID int64) error
	UnlikePost(ctx context.Context, userID, postID int64) error
	SavePost(ctx context.Context, userID, postID int64) error
	UnsavePost(ctx context.Context, userID, postID int64) error
	GetSavedPosts(ctx context.Context, userID int64, limit, offset int) ([]*Post, int64, error)
	CreateComment(ctx context.Context, userID, postID int64, req *CreateCommentRequest) (*Comment, error)
	GetPostComments(ctx context.Context, postID, currentUserID int64, limit, offset int) ([]*Comment, int64, error)
	DeleteComment(ctx context.Context, userID, commentID int64) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) CreatePost(ctx context.Context, userID int64, req *CreatePostRequest) (*Post, error) {
	visibility := req.Visibility
	if visibility == "" {
		visibility = "public"
	}

	post := &Post{
		UserID:     userID,
		Caption:    req.Caption,
		Location:   req.Location,
		Latitude:   req.Latitude,
		Longitude:  req.Longitude,
		Visibility: visibility,
	}

	if err := s.repo.CreatePost(ctx, post); err != nil {
		return nil, fmt.Errorf("failed to create post: %w", err)
	}

	return post, nil
}

func (s *service) GetPost(ctx context.Context, postID, currentUserID int64) (*Post, error) {
	post, err := s.repo.GetPostByID(ctx, postID, currentUserID)
	if err != nil {
		return nil, err
	}

	// Check visibility permissions
	if post.Visibility == "private" && post.UserID != currentUserID {
		return nil, ErrPostNotFound
	}

	return post, nil
}

func (s *service) UpdatePost(ctx context.Context, userID, postID int64, req *UpdatePostRequest) (*Post, error) {
	post, err := s.repo.GetPostByID(ctx, postID, userID)
	if err != nil {
		return nil, err
	}

	if post.UserID != userID {
		return nil, ErrUnauthorized
	}

	if req.Caption != nil {
		post.Caption = req.Caption
	}
	if req.Location != nil {
		post.Location = req.Location
	}
	if req.Visibility != nil {
		post.Visibility = *req.Visibility
	}

	if err := s.repo.UpdatePost(ctx, post); err != nil {
		return nil, fmt.Errorf("failed to update post: %w", err)
	}

	return post, nil
}

func (s *service) DeletePost(ctx context.Context, userID, postID int64) error {
	post, err := s.repo.GetPostByID(ctx, postID, userID)
	if err != nil {
		return err
	}

	if post.UserID != userID {
		return ErrUnauthorized
	}

	return s.repo.DeletePost(ctx, postID)
}

func (s *service) GetUserPosts(ctx context.Context, userID, currentUserID int64, limit, offset int) ([]*Post, int64, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.repo.GetUserPosts(ctx, userID, currentUserID, limit, offset)
}

func (s *service) GetFeed(ctx context.Context, userID int64, feedType string, limit, offset int) ([]*Post, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if feedType == "" {
		feedType = "following"
	}
	return s.repo.GetFeed(ctx, userID, feedType, limit, offset)
}

func (s *service) AddPostMedia(ctx context.Context, userID, postID int64, media *PostMedia) error {
	post, err := s.repo.GetPostByID(ctx, postID, userID)
	if err != nil {
		return err
	}

	if post.UserID != userID {
		return ErrUnauthorized
	}

	media.PostID = postID
	return s.repo.AddPostMedia(ctx, media)
}

func (s *service) LikePost(ctx context.Context, userID, postID int64) error {
	_, err := s.repo.GetPostByID(ctx, postID, userID)
	if err != nil {
		return err
	}
	return s.repo.LikePost(ctx, postID, userID)
}

func (s *service) UnlikePost(ctx context.Context, userID, postID int64) error {
	return s.repo.UnlikePost(ctx, postID, userID)
}

func (s *service) SavePost(ctx context.Context, userID, postID int64) error {
	_, err := s.repo.GetPostByID(ctx, postID, userID)
	if err != nil {
		return err
	}
	return s.repo.SavePost(ctx, postID, userID)
}

func (s *service) UnsavePost(ctx context.Context, userID, postID int64) error {
	return s.repo.UnsavePost(ctx, postID, userID)
}

func (s *service) GetSavedPosts(ctx context.Context, userID int64, limit, offset int) ([]*Post, int64, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.repo.GetSavedPosts(ctx, userID, limit, offset)
}

func (s *service) CreateComment(ctx context.Context, userID, postID int64, req *CreateCommentRequest) (*Comment, error) {
	_, err := s.repo.GetPostByID(ctx, postID, userID)
	if err != nil {
		return nil, err
	}

	comment := &Comment{
		PostID:   postID,
		UserID:   userID,
		ParentID: req.ParentID,
		Content:  req.Content,
	}

	if err := s.repo.CreateComment(ctx, comment); err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	return comment, nil
}

func (s *service) GetPostComments(ctx context.Context, postID, currentUserID int64, limit, offset int) ([]*Comment, int64, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.repo.GetPostComments(ctx, postID, currentUserID, limit, offset)
}

func (s *service) DeleteComment(ctx context.Context, userID, commentID int64) error {
	comment, err := s.repo.GetCommentByID(ctx, commentID)
	if err != nil {
		return err
	}

	// Check if user owns the comment or the post
	post, err := s.repo.GetPostByID(ctx, comment.PostID, userID)
	if err != nil && !errors.Is(err, ErrPostNotFound) {
		return err
	}

	if comment.UserID != userID && (post == nil || post.UserID != userID) {
		return ErrUnauthorized
	}

	return s.repo.DeleteComment(ctx, commentID)
}
