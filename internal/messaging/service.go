package messaging

import (
	"context"
	"fmt"
)

type Service interface {
	// Conversations
	CreateConversation(ctx context.Context, userID int64, req *CreateConversationRequest) (*Conversation, error)
	GetConversation(ctx context.Context, convID, userID int64) (*Conversation, error)
	GetOrCreateDirectConversation(ctx context.Context, userID, otherUserID int64) (*Conversation, error)
	GetUserConversations(ctx context.Context, userID int64, limit, offset int) ([]*Conversation, int64, error)
	LeaveConversation(ctx context.Context, convID, userID int64) error

	// Messages
	SendMessage(ctx context.Context, userID, convID int64, req *SendMessageRequest) (*Message, error)
	GetMessages(ctx context.Context, convID, userID int64, limit, offset int) ([]*Message, int64, error)
	EditMessage(ctx context.Context, userID, msgID int64, req *UpdateMessageRequest) (*Message, error)
	DeleteMessage(ctx context.Context, userID, msgID int64) error
	MarkAsRead(ctx context.Context, convID, userID int64, messageID int64) error
	GetUnreadCount(ctx context.Context, userID int64) (int64, error)
}

type service struct {
	repo Repository
	hub  *Hub
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) SetHub(hub *Hub) {
	s.hub = hub
}

func (s *service) CreateConversation(ctx context.Context, userID int64, req *CreateConversationRequest) (*Conversation, error) {
	// For direct conversations, check if one already exists
	if req.Type == "direct" && len(req.ParticipantIDs) == 1 {
		otherUserID := req.ParticipantIDs[0]
		existing, err := s.repo.GetDirectConversation(ctx, userID, otherUserID)
		if err == nil {
			return existing, nil
		}
	}

	conv := &Conversation{
		Type:      req.Type,
		Name:      req.Name,
		CreatedBy: &userID,
	}

	if err := s.repo.CreateConversation(ctx, conv); err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	// Add creator as admin
	if err := s.repo.AddParticipant(ctx, conv.ID, userID, "admin"); err != nil {
		return nil, fmt.Errorf("failed to add creator: %w", err)
	}

	// Add other participants
	for _, participantID := range req.ParticipantIDs {
		if participantID != userID {
			if err := s.repo.AddParticipant(ctx, conv.ID, participantID, "member"); err != nil {
				continue // Skip failed additions
			}
		}
	}

	return s.repo.GetConversationByID(ctx, conv.ID, userID)
}

func (s *service) GetConversation(ctx context.Context, convID, userID int64) (*Conversation, error) {
	return s.repo.GetConversationByID(ctx, convID, userID)
}

func (s *service) GetOrCreateDirectConversation(ctx context.Context, userID, otherUserID int64) (*Conversation, error) {
	// Try to find existing
	conv, err := s.repo.GetDirectConversation(ctx, userID, otherUserID)
	if err == nil {
		return conv, nil
	}

	// Create new
	return s.CreateConversation(ctx, userID, &CreateConversationRequest{
		Type:           "direct",
		ParticipantIDs: []int64{otherUserID},
	})
}

func (s *service) GetUserConversations(ctx context.Context, userID int64, limit, offset int) ([]*Conversation, int64, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.repo.GetUserConversations(ctx, userID, limit, offset)
}

func (s *service) LeaveConversation(ctx context.Context, convID, userID int64) error {
	return s.repo.RemoveParticipant(ctx, convID, userID)
}

func (s *service) SendMessage(ctx context.Context, userID, convID int64, req *SendMessageRequest) (*Message, error) {
	// Verify participation
	isParticipant, err := s.repo.IsParticipant(ctx, convID, userID)
	if err != nil {
		return nil, err
	}
	if !isParticipant {
		return nil, ErrNotParticipant
	}

	msg := &Message{
		ConversationID:  convID,
		SenderID:        userID,
		Content:         req.Content,
		MessageType:     req.MessageType,
		MediaURL:        req.MediaURL,
		ParentMessageID: req.ParentMessageID,
	}

	if err := s.repo.CreateMessage(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	// Broadcast via WebSocket if hub is available
	if s.hub != nil {
		s.hub.BroadcastToConversation(convID, &WSEvent{
			Type:           WSEventNewMessage,
			ConversationID: convID,
			UserID:         userID,
			Message:        msg,
		})
	}

	return msg, nil
}

func (s *service) GetMessages(ctx context.Context, convID, userID int64, limit, offset int) ([]*Message, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.GetConversationMessages(ctx, convID, userID, limit, offset)
}

func (s *service) EditMessage(ctx context.Context, userID, msgID int64, req *UpdateMessageRequest) (*Message, error) {
	msg, err := s.repo.GetMessageByID(ctx, msgID)
	if err != nil {
		return nil, err
	}

	if msg.SenderID != userID {
		return nil, ErrUnauthorized
	}

	msg.Content = &req.Content
	if err := s.repo.UpdateMessage(ctx, msg); err != nil {
		return nil, err
	}

	// Broadcast edit
	if s.hub != nil {
		s.hub.BroadcastToConversation(msg.ConversationID, &WSEvent{
			Type:           WSEventMessageEdited,
			ConversationID: msg.ConversationID,
			Message:        msg,
		})
	}

	return msg, nil
}

func (s *service) DeleteMessage(ctx context.Context, userID, msgID int64) error {
	msg, err := s.repo.GetMessageByID(ctx, msgID)
	if err != nil {
		return err
	}

	if msg.SenderID != userID {
		return ErrUnauthorized
	}

	if err := s.repo.DeleteMessage(ctx, msgID); err != nil {
		return err
	}

	// Broadcast delete
	if s.hub != nil {
		s.hub.BroadcastToConversation(msg.ConversationID, &WSEvent{
			Type:           WSEventMessageDeleted,
			ConversationID: msg.ConversationID,
			Data:           map[string]int64{"message_id": msgID},
		})
	}

	return nil
}

func (s *service) MarkAsRead(ctx context.Context, convID, userID int64, messageID int64) error {
	if err := s.repo.MarkAsRead(ctx, convID, userID, messageID); err != nil {
		return err
	}

	// Broadcast read status
	if s.hub != nil {
		s.hub.BroadcastToConversation(convID, &WSEvent{
			Type:           WSEventRead,
			ConversationID: convID,
			UserID:         userID,
			Data:           map[string]int64{"message_id": messageID},
		})
	}

	return nil
}

func (s *service) GetUnreadCount(ctx context.Context, userID int64) (int64, error) {
	return s.repo.GetUnreadCount(ctx, userID)
}
