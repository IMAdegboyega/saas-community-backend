package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/tommygebru/kiekky-backend/internal/common"
)

// Handler handles auth HTTP requests
type Handler struct {
	service Service
}

// NewHandler creates a new auth handler
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers auth routes
func (h *Handler) RegisterRoutes(router *mux.Router, authMiddleware *Middleware) {
	// Public routes
	router.HandleFunc("/api/v1/auth/register", h.Register).Methods("POST")
	router.HandleFunc("/api/v1/auth/login", h.Login).Methods("POST")
	router.HandleFunc("/api/v1/auth/refresh", h.RefreshToken).Methods("POST")

	// Protected routes
	protected := router.PathPrefix("/api/v1/auth").Subrouter()
	protected.Use(authMiddleware.Authenticate)
	protected.HandleFunc("/me", h.GetMe).Methods("GET")
	protected.HandleFunc("/logout", h.Logout).Methods("POST")
	protected.HandleFunc("/logout-all", h.LogoutAll).Methods("POST")
	protected.HandleFunc("/change-password", h.ChangePassword).Methods("POST")
	protected.HandleFunc("/sessions", h.GetSessions).Methods("GET")
	protected.HandleFunc("/sessions/{id}", h.RevokeSession).Methods("DELETE")
}

// Register handles user registration
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if errs := common.DecodeAndValidate(r, &req); errs != nil {
		common.ValidationError(w, errs)
		return
	}

	// Sanitize inputs
	req.Email = common.SanitizeEmail(req.Email)
	req.Username = common.SanitizeUsername(req.Username)

	user, err := h.service.Register(r.Context(), &req)
	if err != nil {
		if errors.Is(err, ErrEmailExists) {
			common.Conflict(w, "Email already registered")
			return
		}
		if errors.Is(err, ErrUsernameExists) {
			common.Conflict(w, "Username already taken")
			return
		}
		common.InternalError(w, fmt.Sprintf("Failed to create account: %v", err))
		return
	}

	common.Created(w, "Account created successfully", user.ToResponse())
}

// Login handles user login
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if errs := common.DecodeAndValidate(r, &req); errs != nil {
		common.ValidationError(w, errs)
		return
	}

	// Get client info
	ipAddress := getClientIP(r)
	userAgent := r.UserAgent()

	response, err := h.service.Login(r.Context(), &req, ipAddress, userAgent)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			common.Unauthorized(w, "Invalid email/username or password")
			return
		}
		common.InternalError(w, "Login failed")
		return
	}

	common.Success(w, "Login successful", response)
}

// Logout handles user logout
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	sessionID, _ := common.GetSessionID(r.Context())

	if err := h.service.Logout(r.Context(), userID, sessionID); err != nil {
		common.InternalError(w, "Logout failed")
		return
	}

	common.Success(w, "Logged out successfully", nil)
}

// LogoutAll handles logout from all devices
func (h *Handler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	if err := h.service.LogoutAll(r.Context(), userID); err != nil {
		common.InternalError(w, "Logout failed")
		return
	}

	common.Success(w, "Logged out from all devices", nil)
}

// RefreshToken handles token refresh
func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if errs := common.DecodeAndValidate(r, &req); errs != nil {
		common.ValidationError(w, errs)
		return
	}

	response, err := h.service.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		common.Unauthorized(w, "Invalid refresh token")
		return
	}

	common.Success(w, "Token refreshed", response)
}

// GetMe returns current user info
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	user, err := h.service.GetUserWithStats(r.Context(), userID)
	if err != nil {
		common.NotFound(w, "User not found")
		return
	}

	common.Success(w, "", user.ToResponse())
}

// ChangePassword handles password change
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	var req ChangePasswordRequest
	if errs := common.DecodeAndValidate(r, &req); errs != nil {
		common.ValidationError(w, errs)
		return
	}

	if err := h.service.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		if err.Error() == "current password is incorrect" {
			common.BadRequest(w, err.Error())
			return
		}
		common.InternalError(w, "Failed to change password")
		return
	}

	common.Success(w, "Password changed successfully. Please login again.", nil)
}

// GetSessions returns user's active sessions
func (h *Handler) GetSessions(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	sessions, err := h.service.GetUserSessions(r.Context(), userID)
	if err != nil {
		common.InternalError(w, "Failed to get sessions")
		return
	}

	common.Success(w, "", sessions)
}

// RevokeSession revokes a specific session
func (h *Handler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	sessionID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid session ID")
		return
	}

	if err := h.service.InvalidateSession(r.Context(), userID, sessionID); err != nil {
		common.InternalError(w, "Failed to revoke session")
		return
	}

	common.Success(w, "Session revoked", nil)
}

// Helper function to get client IP
func getClientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return forwarded
	}

	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	return r.RemoteAddr
}
