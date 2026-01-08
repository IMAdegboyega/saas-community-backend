package notification

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/tommygebru/kiekky-backend/internal/common"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func RegisterRoutes(router *mux.Router, handler *Handler, authMiddleware func(http.Handler) http.Handler) {
	api := router.PathPrefix("/api/v1").Subrouter()
	api.Use(authMiddleware)

	// Notifications
	api.HandleFunc("/notifications", handler.GetNotifications).Methods("GET")
	api.HandleFunc("/notifications/unread-count", handler.GetUnreadCount).Methods("GET")
	api.HandleFunc("/notifications/{id}/read", handler.MarkAsRead).Methods("POST")
	api.HandleFunc("/notifications/read-all", handler.MarkAllAsRead).Methods("POST")
	api.HandleFunc("/notifications/{id}", handler.DeleteNotification).Methods("DELETE")
	api.HandleFunc("/notifications", handler.DeleteAllNotifications).Methods("DELETE")

	// Push tokens
	api.HandleFunc("/notifications/push-token", handler.RegisterPushToken).Methods("POST")
	api.HandleFunc("/notifications/push-token", handler.UnregisterPushToken).Methods("DELETE")

	// Preferences
	api.HandleFunc("/notifications/preferences", handler.GetPreferences).Methods("GET")
	api.HandleFunc("/notifications/preferences", handler.UpdatePreferences).Methods("PUT")
}

func (h *Handler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	notifications, total, err := h.service.GetUserNotifications(r.Context(), userID, limit, offset)
	if err != nil {
		common.InternalError(w, "Failed to get notifications")
		return
	}

	common.SuccessWithMeta(w, "", notifications, &common.Meta{Total: total})
}

func (h *Handler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	count, err := h.service.GetUnreadCount(r.Context(), userID)
	if err != nil {
		common.InternalError(w, "Failed to get unread count")
		return
	}

	common.Success(w, "", map[string]int64{"unread_count": count})
}

func (h *Handler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	notificationID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid notification ID")
		return
	}

	if err := h.service.MarkAsRead(r.Context(), notificationID, userID); err != nil {
		common.InternalError(w, "Failed to mark as read")
		return
	}

	common.Success(w, "Marked as read", nil)
}

func (h *Handler) MarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	if err := h.service.MarkAllAsRead(r.Context(), userID); err != nil {
		common.InternalError(w, "Failed to mark all as read")
		return
	}

	common.Success(w, "All marked as read", nil)
}

func (h *Handler) DeleteNotification(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	notificationID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		common.BadRequest(w, "Invalid notification ID")
		return
	}

	if err := h.service.Delete(r.Context(), notificationID, userID); err != nil {
		common.InternalError(w, "Failed to delete notification")
		return
	}

	common.Success(w, "Notification deleted", nil)
}

func (h *Handler) DeleteAllNotifications(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	if err := h.service.DeleteAll(r.Context(), userID); err != nil {
		common.InternalError(w, "Failed to delete notifications")
		return
	}

	common.Success(w, "All notifications deleted", nil)
}

func (h *Handler) RegisterPushToken(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	var req RegisterTokenRequest
	if errs := common.DecodeAndValidate(r, &req); errs != nil {
		common.ValidationError(w, errs)
		return
	}

	if err := h.service.RegisterPushToken(r.Context(), userID, &req); err != nil {
		common.InternalError(w, "Failed to register token")
		return
	}

	common.Success(w, "Push token registered", nil)
}

func (h *Handler) UnregisterPushToken(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		common.BadRequest(w, "Token required")
		return
	}

	if err := h.service.UnregisterPushToken(r.Context(), token); err != nil {
		common.InternalError(w, "Failed to unregister token")
		return
	}

	common.Success(w, "Push token unregistered", nil)
}

func (h *Handler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	prefs, err := h.service.GetPreferences(r.Context(), userID)
	if err != nil {
		common.InternalError(w, "Failed to get preferences")
		return
	}

	common.Success(w, "", prefs)
}

func (h *Handler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	userID, err := common.GetUserID(r.Context())
	if err != nil {
		common.Unauthorized(w, "Unauthorized")
		return
	}

	var req UpdatePreferencesRequest
	if errs := common.DecodeAndValidate(r, &req); errs != nil {
		common.ValidationError(w, errs)
		return
	}

	if err := h.service.UpdatePreferences(r.Context(), userID, &req); err != nil {
		common.InternalError(w, "Failed to update preferences")
		return
	}

	common.Success(w, "Preferences updated", nil)
}
