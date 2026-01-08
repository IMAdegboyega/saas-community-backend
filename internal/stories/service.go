package stories

import (
	"context"
	"fmt"
	"time"
)

type Service interface {
	CreateStory(ctx context.Context, userID int64, req *CreateStoryRequest) (*Story, error)
	GetStory(ctx context.Context, storyID, currentUserID int64) (*Story, error)
	DeleteStory(ctx context.Context, userID, storyID int64) error
	GetUserStories(ctx context.Context, userID, currentUserID int64) ([]*Story, error)
	GetFeedStories(ctx context.Context, userID int64) ([]*UserStories, error)
	ViewStory(ctx context.Context, storyID, viewerID int64) error
	GetStoryViewers(ctx context.Context, userID, storyID int64, limit, offset int) ([]*StoryView, int64, error)
	CreateHighlight(ctx context.Context, userID int64, req *CreateHighlightRequest) (*StoryHighlight, error)
	GetUserHighlights(ctx context.Context, userID int64) ([]*StoryHighlight, error)
	DeleteHighlight(ctx context.Context, userID, highlightID int64) error
	AddToHighlight(ctx context.Context, userID, highlightID int64, req *AddToHighlightRequest) error
	CleanupExpiredStories(ctx context.Context) (int64, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) CreateStory(ctx context.Context, userID int64, req *CreateStoryRequest) (*Story, error) {
	duration := req.Duration
	if duration <= 0 {
		duration = 5
	}

	story := &Story{
		UserID:       userID,
		MediaURL:     req.MediaURL,
		MediaType:    req.MediaType,
		ThumbnailURL: req.ThumbnailURL,
		Caption:      req.Caption,
		Duration:     duration,
	}

	if err := s.repo.CreateStory(ctx, story); err != nil {
		return nil, fmt.Errorf("failed to create story: %w", err)
	}

	return story, nil
}

func (s *service) GetStory(ctx context.Context, storyID, currentUserID int64) (*Story, error) {
	story, err := s.repo.GetStoryByID(ctx, storyID, currentUserID)
	if err != nil {
		return nil, err
	}

	// Check if story has expired (unless it's highlighted)
	if !story.IsHighlighted && story.ExpiresAt.Before(time.Now()) {
		return nil, ErrStoryExpired
	}

	return story, nil
}

func (s *service) DeleteStory(ctx context.Context, userID, storyID int64) error {
	story, err := s.repo.GetStoryByID(ctx, storyID, userID)
	if err != nil {
		return err
	}

	if story.UserID != userID {
		return ErrUnauthorized
	}

	return s.repo.DeleteStory(ctx, storyID)
}

func (s *service) GetUserStories(ctx context.Context, userID, currentUserID int64) ([]*Story, error) {
	return s.repo.GetUserStories(ctx, userID, currentUserID)
}

func (s *service) GetFeedStories(ctx context.Context, userID int64) ([]*UserStories, error) {
	return s.repo.GetFeedStories(ctx, userID)
}

func (s *service) ViewStory(ctx context.Context, storyID, viewerID int64) error {
	story, err := s.repo.GetStoryByID(ctx, storyID, viewerID)
	if err != nil {
		return err
	}

	// Don't record view if viewer is the owner
	if story.UserID == viewerID {
		return nil
	}

	// Check if expired
	if !story.IsHighlighted && story.ExpiresAt.Before(time.Now()) {
		return ErrStoryExpired
	}

	return s.repo.ViewStory(ctx, storyID, viewerID)
}

func (s *service) GetStoryViewers(ctx context.Context, userID, storyID int64, limit, offset int) ([]*StoryView, int64, error) {
	// Verify ownership
	story, err := s.repo.GetStoryByID(ctx, storyID, userID)
	if err != nil {
		return nil, 0, err
	}

	if story.UserID != userID {
		return nil, 0, ErrUnauthorized
	}

	if limit <= 0 || limit > 50 {
		limit = 20
	}

	return s.repo.GetStoryViewers(ctx, storyID, limit, offset)
}

func (s *service) CreateHighlight(ctx context.Context, userID int64, req *CreateHighlightRequest) (*StoryHighlight, error) {
	highlight := &StoryHighlight{
		UserID:     userID,
		Title:      req.Title,
		CoverImage: req.CoverImage,
	}

	if err := s.repo.CreateHighlight(ctx, highlight); err != nil {
		return nil, fmt.Errorf("failed to create highlight: %w", err)
	}

	// Add stories to highlight
	if len(req.StoryIDs) > 0 {
		if err := s.repo.AddStoriesToHighlight(ctx, highlight.ID, req.StoryIDs); err != nil {
			return nil, fmt.Errorf("failed to add stories to highlight: %w", err)
		}
	}

	return highlight, nil
}

func (s *service) GetUserHighlights(ctx context.Context, userID int64) ([]*StoryHighlight, error) {
	return s.repo.GetUserHighlights(ctx, userID)
}

func (s *service) DeleteHighlight(ctx context.Context, userID, highlightID int64) error {
	highlight, err := s.repo.GetHighlightByID(ctx, highlightID)
	if err != nil {
		return err
	}

	if highlight.UserID != userID {
		return ErrUnauthorized
	}

	return s.repo.DeleteHighlight(ctx, highlightID)
}

func (s *service) AddToHighlight(ctx context.Context, userID, highlightID int64, req *AddToHighlightRequest) error {
	highlight, err := s.repo.GetHighlightByID(ctx, highlightID)
	if err != nil {
		return err
	}

	if highlight.UserID != userID {
		return ErrUnauthorized
	}

	return s.repo.AddStoriesToHighlight(ctx, highlightID, req.StoryIDs)
}

func (s *service) CleanupExpiredStories(ctx context.Context) (int64, error) {
	return s.repo.DeleteExpiredStories(ctx)
}
