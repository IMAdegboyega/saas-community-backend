package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/tommygebru/kiekky-backend/internal/common"
)

// Middleware handles authentication middleware
type Middleware struct {
	service Service
}

// NewMiddleware creates a new auth middleware
func NewMiddleware(service Service) *Middleware {
	return &Middleware{service: service}
}

// Authenticate middleware validates JWT token and sets user context
func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			common.Unauthorized(w, "Authorization header required")
			return
		}

		// Check Bearer prefix
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			common.Unauthorized(w, "Invalid authorization format")
			return
		}

		token := parts[1]

		// Validate token
		claims, err := m.service.ValidateAccessToken(token)
		if err != nil {
			common.Unauthorized(w, "Invalid or expired token")
			return
		}

		// Set user context
		ctx := context.WithValue(r.Context(), common.UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, common.UsernameKey, claims.Username)
		ctx = context.WithValue(ctx, common.EmailKey, claims.Email)
		ctx = context.WithValue(ctx, common.SessionIDKey, claims.SessionID)

		// Continue with enriched context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthenticateFunc is a function version of Authenticate middleware
func (m *Middleware) AuthenticateFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m.Authenticate(http.HandlerFunc(next)).ServeHTTP(w, r)
	}
}

// OptionalAuth middleware tries to authenticate but doesn't fail if no token
func (m *Middleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			next.ServeHTTP(w, r)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			next.ServeHTTP(w, r)
			return
		}

		token := parts[1]
		claims, err := m.service.ValidateAccessToken(token)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		// Set user context if valid
		ctx := context.WithValue(r.Context(), common.UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, common.UsernameKey, claims.Username)
		ctx = context.WithValue(ctx, common.EmailKey, claims.Email)
		ctx = context.WithValue(ctx, common.SessionIDKey, claims.SessionID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
