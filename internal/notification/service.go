package notification

import (
	"context"
	"fmt"
)

type Service interface {
	// Notifications
	Create(ctx context.Context, req *CreateNotificationRequest) (*Notification, error)
	GetUserNotifications(ctx context.Context, userID int64, limit, offset int) ([]*Notification, int64, error)
	MarkAsRead(ctx context.Context, id, userID int64) error
	MarkAllAsRead(ctx context.Context, userID int64) error
	Delete(ctx context.Context, id, userID int64) error
	DeleteAll(ctx context.Context, userID int64) error
	GetUnreadCount(ctx context.Context, userID int64) (int64, error)

	// Push tokens
	RegisterPushToken(ctx context.Context, userID int64, req *RegisterTokenRequest) error
	UnregisterPushToken(ctx context.Context, token string) error

	// Preferences
	GetPreferences(ctx context.Context, userID int64) (*NotificationPreferences, error)
	UpdatePreferences(ctx context.Context, userID int64, req *UpdatePreferencesRequest) error

	// Helper methods for creating specific notification types
	NotifyFollow(ctx context.Context, followerID, followedID int64, followerUsername string) error
	NotifyLike(ctx context.Context, likerID, postOwnerID, postID int64, likerUsername string) error
	NotifyComment(ctx context.Context, commenterID, postOwnerID, postID, commentID int64, commenterUsername, commentPreview string) error
	NotifyMention(ctx context.Context, mentionerID, mentionedID, postID int64, mentionerUsername string) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, req *CreateNotificationRequest) (*Notification, error) {
	n := &Notification{
		UserID:    req.UserID,
		Type:      req.Type,
		Title:     req.Title,
		Message:   req.Message,
		Data:      req.Data,
		ActionURL: req.ActionURL,
	}

	if req.ActorID != nil {
		if n.Data == nil {
			n.Data = make(map[string]interface{})
		}
		n.Data["actor_id"] = *req.ActorID
	}

	if err := s.repo.Create(ctx, n); err != nil {
		fmt.Printf("ERROR: Failed to create notification in DB: %v\n", err)
		return nil, fmt.Errorf("failed to create notification: %w", err)
	}

	fmt.Printf("INFO: Notification created - ID: %d, Type: %s, UserID: %d\n", n.ID, n.Type, n.UserID)

	// TODO: Send push notification if enabled

	return n, nil
}

func (s *service) GetUserNotifications(ctx context.Context, userID int64, limit, offset int) ([]*Notification, int64, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.repo.GetUserNotifications(ctx, userID, limit, offset)
}

func (s *service) MarkAsRead(ctx context.Context, id, userID int64) error {
	return s.repo.MarkAsRead(ctx, id, userID)
}

func (s *service) MarkAllAsRead(ctx context.Context, userID int64) error {
	return s.repo.MarkAllAsRead(ctx, userID)
}

func (s *service) Delete(ctx context.Context, id, userID int64) error {
	return s.repo.Delete(ctx, id, userID)
}

func (s *service) DeleteAll(ctx context.Context, userID int64) error {
	return s.repo.DeleteAll(ctx, userID)
}

func (s *service) GetUnreadCount(ctx context.Context, userID int64) (int64, error) {
	return s.repo.GetUnreadCount(ctx, userID)
}

func (s *service) RegisterPushToken(ctx context.Context, userID int64, req *RegisterTokenRequest) error {
	token := &PushToken{
		UserID:   userID,
		Token:    req.Token,
		Platform: req.Platform,
		DeviceID: req.DeviceID,
	}
	return s.repo.SavePushToken(ctx, token)
}

func (s *service) UnregisterPushToken(ctx context.Context, token string) error {
	return s.repo.DeletePushToken(ctx, token)
}

func (s *service) GetPreferences(ctx context.Context, userID int64) (*NotificationPreferences, error) {
	return s.repo.GetPreferences(ctx, userID)
}

func (s *service) UpdatePreferences(ctx context.Context, userID int64, req *UpdatePreferencesRequest) error {
	prefs, err := s.repo.GetPreferences(ctx, userID)
	if err != nil {
		return err
	}

	if req.PushEnabled != nil {
		prefs.PushEnabled = *req.PushEnabled
	}
	if req.EmailEnabled != nil {
		prefs.EmailEnabled = *req.EmailEnabled
	}
	if req.Likes != nil {
		prefs.Likes = *req.Likes
	}
	if req.Comments != nil {
		prefs.Comments = *req.Comments
	}
	if req.Follows != nil {
		prefs.Follows = *req.Follows
	}
	if req.Messages != nil {
		prefs.Messages = *req.Messages
	}
	if req.StoryViews != nil {
		prefs.StoryViews = *req.StoryViews
	}
	if req.Mentions != nil {
		prefs.Mentions = *req.Mentions
	}

	return s.repo.UpdatePreferences(ctx, prefs)
}

// Helper notification methods

func (s *service) NotifyFollow(ctx context.Context, followerID, followedID int64, followerUsername string) error {
	_, err := s.Create(ctx, &CreateNotificationRequest{
		UserID:  followedID,
		Type:    TypeFollow,
		Title:   "New Follower",
		Message: fmt.Sprintf("%s started following you", followerUsername),
		ActorID: &followerID,
		Data: map[string]interface{}{
			"follower_id": followerID,
		},
	})
	return err
}

func (s *service) NotifyLike(ctx context.Context, likerID, postOwnerID, postID int64, likerUsername string) error {
	if likerID == postOwnerID {
		return nil // Don't notify yourself
	}

	actionURL := fmt.Sprintf("/posts/%d", postID)
	_, err := s.Create(ctx, &CreateNotificationRequest{
		UserID:    postOwnerID,
		Type:      TypeLike,
		Title:     "New Like",
		Message:   fmt.Sprintf("%s liked your post", likerUsername),
		ActorID:   &likerID,
		ActionURL: &actionURL,
		Data: map[string]interface{}{
			"post_id":  postID,
			"liker_id": likerID,
		},
	})
	return err
}

func (s *service) NotifyComment(ctx context.Context, commenterID, postOwnerID, postID, commentID int64, commenterUsername, commentPreview string) error {
	if commenterID == postOwnerID {
		return nil
	}

	actionURL := fmt.Sprintf("/posts/%d", postID)
	message := fmt.Sprintf("%s commented: %s", commenterUsername, commentPreview)
	if len(message) > 100 {
		message = message[:97] + "..."
	}

	_, err := s.Create(ctx, &CreateNotificationRequest{
		UserID:    postOwnerID,
		Type:      TypeComment,
		Title:     "New Comment",
		Message:   message,
		ActorID:   &commenterID,
		ActionURL: &actionURL,
		Data: map[string]interface{}{
			"post_id":      postID,
			"comment_id":   commentID,
			"commenter_id": commenterID,
		},
	})
	return err
}

func (s *service) NotifyMention(ctx context.Context, mentionerID, mentionedID, postID int64, mentionerUsername string) error {
	if mentionerID == mentionedID {
		return nil
	}

	actionURL := fmt.Sprintf("/posts/%d", postID)
	_, err := s.Create(ctx, &CreateNotificationRequest{
		UserID:    mentionedID,
		Type:      TypeMention,
		Title:     "You were mentioned",
		Message:   fmt.Sprintf("%s mentioned you in a post", mentionerUsername),
		ActorID:   &mentionerID,
		ActionURL: &actionURL,
		Data: map[string]interface{}{
			"post_id":      postID,
			"mentioner_id": mentionerID,
		},
	})
	return err
}
