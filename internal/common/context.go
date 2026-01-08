package common

import (
	"context"
	"errors"
)

// Context keys
type contextKey string

const (
	UserIDKey    contextKey = "user_id"
	UsernameKey  contextKey = "username"
	EmailKey     contextKey = "email"
	SessionIDKey contextKey = "session_id"
)

// GetUserID extracts user ID from context
func GetUserID(ctx context.Context) (int64, error) {
	userID, ok := ctx.Value(UserIDKey).(int64)
	if !ok {
		return 0, errors.New("user ID not found in context")
	}
	return userID, nil
}

// GetUsername extracts username from context
func GetUsername(ctx context.Context) (string, error) {
	username, ok := ctx.Value(UsernameKey).(string)
	if !ok {
		return "", errors.New("username not found in context")
	}
	return username, nil
}

// GetEmail extracts email from context
func GetEmail(ctx context.Context) (string, error) {
	email, ok := ctx.Value(EmailKey).(string)
	if !ok {
		return "", errors.New("email not found in context")
	}
	return email, nil
}

// GetSessionID extracts session ID from context
func GetSessionID(ctx context.Context) (string, error) {
	sessionID, ok := ctx.Value(SessionIDKey).(string)
	if !ok {
		return "", errors.New("session ID not found in context")
	}
	return sessionID, nil
}

// SetUserContext creates a new context with user information
func SetUserContext(ctx context.Context, userID int64, username, email string) context.Context {
	ctx = context.WithValue(ctx, UserIDKey, userID)
	ctx = context.WithValue(ctx, UsernameKey, username)
	ctx = context.WithValue(ctx, EmailKey, email)
	return ctx
}
