package user

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/tommygebru/kiekky-backend/internal/common"
)

// Handler handles user HTTP requests
type Handler struct {
	service Service
}

// NewHandler creates a new user handler
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers user routes
func RegisterRoutes(router *mux.Router, handler *Handler, authMiddleware func(http.Handler) http.Handler) {
	// All user routes require authentication
	api := router.PathPrefix("/api/v1").Subrouter()
	api.Use(authMiddleware)

	// IMPORTANT: Specific routes must be registered BEFORE wildcard {id} routes
	// Otherwise /users/suggestions matches /users/{id} with id="suggestions"
	
	// Specific user routes (no {id} wildcard)
	api.HandleFunc("/users/search", handler.SearchUsers).Methods("GET")
	api.HandleFunc("/users/suggestions", handler.GetSuggestedUsers).Methods("GET")
	api.HandleFunc("/users/blocked", handler.GetBlockedUsers).Methods("GET")
	api.HandleFunc("/users/username/{username}", handler.GetUserByUsername).Methods("GET")

	// User profile routes with {id} wildcard - MUST come after specific routes
	api.HandleFunc("/users/{id}", handler.GetUser).Methods("GET")

	// Follow routes
	api.HandleFunc("/users/{id}/follow", handler.Follow).Methods("POST")
	api.HandleFunc("/users/{id}/unfollow", handler.Unfollow).Methods("POST")
	api.HandleFunc("/users/{id}/follow-status", handler.CheckFollowStatus).Methods("GET")
	api.HandleFunc("/users/{id}/followers", handler.GetFollowers).Methods("GET")
	api.HandleFunc("/users/{id}/following", handler.GetFollowing).Methods("GET")

	// Block routes
	api.HandleFunc("/users/{id}/block", handler.Block).Methods("POST")
	api.HandleFunc("/users/{id}/unblock", handler.Unblock).Methods("POST")
}

// GetUser returns a user's profile
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	userID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid user ID")
		return
	}

	user, err := h.service.GetUserProfile(r.Context(), userID, currentUserID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			common.NotFound(w, "User not found")
			return
		}
		common.InternalError(w, "Failed to get user")
		return
	}

	common.Success(w, "", user)
}

// GetUserByUsername returns a user by username
func (h *Handler) GetUserByUsername(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	username := vars["username"]

	user, err := h.service.GetUserByUsername(r.Context(), username)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			common.NotFound(w, "User not found")
			return
		}
		common.InternalError(w, "Failed to get user")
		return
	}

	profile, err := h.service.GetUserProfile(r.Context(), user.ID, currentUserID)
	if err != nil {
		common.InternalError(w, "Failed to get user profile")
		return
	}

	common.Success(w, "", profile)
}

// SearchUsers searches for users
func (h *Handler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		common.BadRequest(w, "Search query is required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	users, err := h.service.SearchUsers(r.Context(), query, currentUserID, limit, offset)
	if err != nil {
		common.InternalError(w, "Failed to search users")
		return
	}

	common.Success(w, "", users)
}

// GetSuggestedUsers returns suggested users to follow
func (h *Handler) GetSuggestedUsers(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	users, err := h.service.GetSuggestedUsers(r.Context(), currentUserID, limit)
	if err != nil {
		// Log the actual error for debugging
		println("GetSuggestedUsers error:", err.Error())
		common.InternalError(w, "Failed to get suggestions")
		return
	}

	common.Success(w, "", users)
}

// Follow follows a user
func (h *Handler) Follow(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	targetUserID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid user ID")
		return
	}

	if err := h.service.Follow(r.Context(), currentUserID, targetUserID); err != nil {
		switch {
		case errors.Is(err, ErrCannotFollowSelf):
			common.BadRequest(w, "You cannot follow yourself")
		case errors.Is(err, ErrAlreadyFollowing):
			common.Conflict(w, "Already following this user")
		case errors.Is(err, ErrBlockedByUser):
			common.Forbidden(w, "Cannot follow this user")
		case errors.Is(err, ErrUserNotFound):
			common.NotFound(w, "User not found")
		default:
			common.InternalError(w, "Failed to follow user")
		}
		return
	}

	common.Success(w, "Successfully followed user", nil)
}

// Unfollow unfollows a user
func (h *Handler) Unfollow(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	targetUserID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid user ID")
		return
	}

	if err := h.service.Unfollow(r.Context(), currentUserID, targetUserID); err != nil {
		if errors.Is(err, ErrNotFollowing) {
			common.BadRequest(w, "Not following this user")
			return
		}
		common.InternalError(w, "Failed to unfollow user")
		return
	}

	common.Success(w, "Successfully unfollowed user", nil)
}

// CheckFollowStatus checks if current user follows another user
func (h *Handler) CheckFollowStatus(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	targetUserID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid user ID")
		return
	}

	isFollowing, err := h.service.CheckFollowStatus(r.Context(), currentUserID, targetUserID)
	if err != nil {
		common.InternalError(w, "Failed to check follow status")
		return
	}

	common.Success(w, "", map[string]bool{"is_following": isFollowing})
}

// GetFollowers returns a user's followers
func (h *Handler) GetFollowers(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	userID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid user ID")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	followers, total, err := h.service.GetFollowers(r.Context(), userID, currentUserID, limit, offset)
	if err != nil {
		common.InternalError(w, "Failed to get followers")
		return
	}

	common.SuccessWithMeta(w, "", followers, &common.Meta{Total: total})
}

// GetFollowing returns users that a user follows
func (h *Handler) GetFollowing(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	userID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid user ID")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	following, total, err := h.service.GetFollowing(r.Context(), userID, currentUserID, limit, offset)
	if err != nil {
		common.InternalError(w, "Failed to get following")
		return
	}

	common.SuccessWithMeta(w, "", following, &common.Meta{Total: total})
}

// Block blocks a user
func (h *Handler) Block(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	targetUserID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid user ID")
		return
	}

	var req struct {
		Reason *string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.service.Block(r.Context(), currentUserID, targetUserID, req.Reason); err != nil {
		switch {
		case errors.Is(err, ErrCannotBlockSelf):
			common.BadRequest(w, "You cannot block yourself")
		case errors.Is(err, ErrAlreadyBlocked):
			common.Conflict(w, "User already blocked")
		case errors.Is(err, ErrUserNotFound):
			common.NotFound(w, "User not found")
		default:
			common.InternalError(w, "Failed to block user")
		}
		return
	}

	common.Success(w, "User blocked successfully", nil)
}

// Unblock unblocks a user
func (h *Handler) Unblock(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	targetUserID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid user ID")
		return
	}

	if err := h.service.Unblock(r.Context(), currentUserID, targetUserID); err != nil {
		if errors.Is(err, ErrNotBlocked) {
			common.BadRequest(w, "User not blocked")
			return
		}
		common.InternalError(w, "Failed to unblock user")
		return
	}

	common.Success(w, "User unblocked successfully", nil)
}

// GetBlockedUsers returns the current user's blocked users
func (h *Handler) GetBlockedUsers(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	blocked, total, err := h.service.GetBlockedUsers(r.Context(), currentUserID, limit, offset)
	if err != nil {
		common.InternalError(w, "Failed to get blocked users")
		return
	}

	common.SuccessWithMeta(w, "", blocked, &common.Meta{Total: total})
}
