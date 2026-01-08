package user

import (
	"context"
	"errors"
	"fmt"
)

// Service defines user business operations
type Service interface {
	// User operations
	GetUserByID(ctx context.Context, id int64) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	GetUserProfile(ctx context.Context, userID, currentUserID int64) (*UserWithStats, error)
	SearchUsers(ctx context.Context, query string, currentUserID int64, limit, offset int) ([]*FollowUser, error)
	
	// Follow operations
	Follow(ctx context.Context, followerID, followingID int64) error
	Unfollow(ctx context.Context, followerID, followingID int64) error
	GetFollowers(ctx context.Context, userID, currentUserID int64, limit, offset int) ([]*FollowUser, int64, error)
	GetFollowing(ctx context.Context, userID, currentUserID int64, limit, offset int) ([]*FollowUser, int64, error)
	GetFollowStats(ctx context.Context, userID int64) (*FollowStats, error)
	CheckFollowStatus(ctx context.Context, followerID, followingID int64) (bool, error)
	
	// Block operations
	Block(ctx context.Context, blockerID, blockedID int64, reason *string) error
	Unblock(ctx context.Context, blockerID, blockedID int64) error
	GetBlockedUsers(ctx context.Context, userID int64, limit, offset int) ([]*BlockedUser, int64, error)
	
	// Suggestions
	GetSuggestedUsers(ctx context.Context, userID int64, limit int) ([]*FollowUser, error)
}

type service struct {
	repo Repository
}

// NewService creates a new user service
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// GetUserByID retrieves a user by ID
func (s *service) GetUserByID(ctx context.Context, id int64) (*User, error) {
	user, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// GetUserByUsername retrieves a user by username
func (s *service) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// GetUserProfile retrieves a user's full profile with stats
func (s *service) GetUserProfile(ctx context.Context, userID, currentUserID int64) (*UserWithStats, error) {
	// Check if blocked
	blocked, err := s.repo.IsBlockedEither(ctx, userID, currentUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to check block status: %w", err)
	}
	if blocked && userID != currentUserID {
		return nil, ErrUserNotFound // Don't reveal block status
	}

	user, err := s.repo.GetUserWithStats(ctx, userID, currentUserID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// SearchUsers searches for users
func (s *service) SearchUsers(ctx context.Context, query string, currentUserID int64, limit, offset int) ([]*FollowUser, error) {
	if query == "" {
		return []*FollowUser{}, nil
	}
	
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	return s.repo.SearchUsers(ctx, query, currentUserID, limit, offset)
}

// Follow creates a follow relationship
func (s *service) Follow(ctx context.Context, followerID, followingID int64) error {
	if followerID == followingID {
		return ErrCannotFollowSelf
	}

	// Verify the target user exists
	_, err := s.repo.GetUserByID(ctx, followingID)
	if err != nil {
		return err
	}

	return s.repo.Follow(ctx, followerID, followingID)
}

// Unfollow removes a follow relationship
func (s *service) Unfollow(ctx context.Context, followerID, followingID int64) error {
	return s.repo.Unfollow(ctx, followerID, followingID)
}

// GetFollowers retrieves a user's followers
func (s *service) GetFollowers(ctx context.Context, userID, currentUserID int64, limit, offset int) ([]*FollowUser, int64, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.repo.GetFollowers(ctx, userID, currentUserID, limit, offset)
}

// GetFollowing retrieves users that a user follows
func (s *service) GetFollowing(ctx context.Context, userID, currentUserID int64, limit, offset int) ([]*FollowUser, int64, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.repo.GetFollowing(ctx, userID, currentUserID, limit, offset)
}

// GetFollowStats gets follow statistics
func (s *service) GetFollowStats(ctx context.Context, userID int64) (*FollowStats, error) {
	return s.repo.GetFollowStats(ctx, userID)
}

// CheckFollowStatus checks if a user is following another
func (s *service) CheckFollowStatus(ctx context.Context, followerID, followingID int64) (bool, error) {
	return s.repo.IsFollowing(ctx, followerID, followingID)
}

// Block blocks a user
func (s *service) Block(ctx context.Context, blockerID, blockedID int64, reason *string) error {
	if blockerID == blockedID {
		return ErrCannotBlockSelf
	}

	// Verify the target user exists
	_, err := s.repo.GetUserByID(ctx, blockedID)
	if err != nil {
		return err
	}

	return s.repo.Block(ctx, blockerID, blockedID, reason)
}

// Unblock unblocks a user
func (s *service) Unblock(ctx context.Context, blockerID, blockedID int64) error {
	return s.repo.Unblock(ctx, blockerID, blockedID)
}

// GetBlockedUsers retrieves a user's blocked users list
func (s *service) GetBlockedUsers(ctx context.Context, userID int64, limit, offset int) ([]*BlockedUser, int64, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.repo.GetBlockedUsers(ctx, userID, limit, offset)
}

// GetSuggestedUsers gets users suggested to follow
func (s *service) GetSuggestedUsers(ctx context.Context, userID int64, limit int) ([]*FollowUser, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	return s.repo.GetSuggestedUsers(ctx, userID, limit)
}
