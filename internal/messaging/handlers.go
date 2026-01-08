package messaging

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/tommygebru/kiekky-backend/internal/common"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Handler struct {
	service Service
	hub     *Hub
}

func NewHandler(service Service, hub *Hub) *Handler {
	if svc, ok := service.(*service); ok {
		svc.hub = hub
	}
	return &Handler{service: service, hub: hub}
}

func RegisterRoutes(router *mux.Router, handler *Handler, authMiddleware func(http.Handler) http.Handler) {
	api := router.PathPrefix("/api/v1").Subrouter()
	api.Use(authMiddleware)

	api.HandleFunc("/conversations", handler.CreateConversation).Methods("POST")
	api.HandleFunc("/conversations", handler.GetConversations).Methods("GET")
	api.HandleFunc("/conversations/{id}", handler.GetConversation).Methods("GET")
	api.HandleFunc("/conversations/{id}/leave", handler.LeaveConversation).Methods("POST")
	api.HandleFunc("/conversations/direct/{user_id}", handler.GetOrCreateDirect).Methods("POST")
	api.HandleFunc("/conversations/{id}/messages", handler.SendMessage).Methods("POST")
	api.HandleFunc("/conversations/{id}/messages", handler.GetMessages).Methods("GET")
	api.HandleFunc("/conversations/{id}/read", handler.MarkAsRead).Methods("POST")
	api.HandleFunc("/messages/{id}", handler.EditMessage).Methods("PUT")
	api.HandleFunc("/messages/{id}", handler.DeleteMessage).Methods("DELETE")
	api.HandleFunc("/messages/unread", handler.GetUnreadCount).Methods("GET")

	router.HandleFunc("/ws", handler.HandleWebSocket)
}

func (h *Handler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	userID, _ := common.GetUserID(r.Context())
	var req CreateConversationRequest
	if errs := common.DecodeAndValidate(r, &req); errs != nil {
		common.ValidationError(w, errs)
		return
	}
	conv, err := h.service.CreateConversation(r.Context(), userID, &req)
	if err != nil {
		common.InternalError(w, "Failed to create conversation")
		return
	}
	common.Created(w, "Conversation created", conv)
}

func (h *Handler) GetConversations(w http.ResponseWriter, r *http.Request) {
	userID, _ := common.GetUserID(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	conversations, total, err := h.service.GetUserConversations(r.Context(), userID, limit, offset)
	if err != nil {
		common.InternalError(w, "Failed to get conversations")
		return
	}
	common.SuccessWithMeta(w, "", conversations, &common.Meta{Total: total})
}

func (h *Handler) GetConversation(w http.ResponseWriter, r *http.Request) {
	userID, _ := common.GetUserID(r.Context())
	convID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	conv, err := h.service.GetConversation(r.Context(), convID, userID)
	if err != nil {
		if errors.Is(err, ErrNotParticipant) {
			common.NotFound(w, "Conversation not found")
			return
		}
		common.InternalError(w, "Failed to get conversation")
		return
	}
	common.Success(w, "", conv)
}

func (h *Handler) GetOrCreateDirect(w http.ResponseWriter, r *http.Request) {
	userID, _ := common.GetUserID(r.Context())
	otherUserID, _ := strconv.ParseInt(mux.Vars(r)["user_id"], 10, 64)
	conv, err := h.service.GetOrCreateDirectConversation(r.Context(), userID, otherUserID)
	if err != nil {
		common.InternalError(w, "Failed to get/create conversation")
		return
	}
	common.Success(w, "", conv)
}

func (h *Handler) LeaveConversation(w http.ResponseWriter, r *http.Request) {
	userID, _ := common.GetUserID(r.Context())
	convID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	h.service.LeaveConversation(r.Context(), convID, userID)
	common.Success(w, "Left conversation", nil)
}

func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID, _ := common.GetUserID(r.Context())
	convID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	var req SendMessageRequest
	if errs := common.DecodeAndValidate(r, &req); errs != nil {
		common.ValidationError(w, errs)
		return
	}
	msg, err := h.service.SendMessage(r.Context(), userID, convID, &req)
	if err != nil {
		if errors.Is(err, ErrNotParticipant) {
			common.Forbidden(w, "Not a participant")
			return
		}
		common.InternalError(w, "Failed to send message")
		return
	}
	common.Created(w, "Message sent", msg)
}

func (h *Handler) GetMessages(w http.ResponseWriter, r *http.Request) {
	userID, _ := common.GetUserID(r.Context())
	convID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	messages, total, err := h.service.GetMessages(r.Context(), convID, userID, limit, offset)
	if err != nil {
		common.InternalError(w, "Failed to get messages")
		return
	}
	common.SuccessWithMeta(w, "", messages, &common.Meta{Total: total})
}

func (h *Handler) EditMessage(w http.ResponseWriter, r *http.Request) {
	userID, _ := common.GetUserID(r.Context())
	msgID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	var req UpdateMessageRequest
	if errs := common.DecodeAndValidate(r, &req); errs != nil {
		common.ValidationError(w, errs)
		return
	}
	msg, err := h.service.EditMessage(r.Context(), userID, msgID, &req)
	if err != nil {
		common.InternalError(w, "Failed to edit message")
		return
	}
	common.Success(w, "Message edited", msg)
}

func (h *Handler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	userID, _ := common.GetUserID(r.Context())
	msgID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err := h.service.DeleteMessage(r.Context(), userID, msgID); err != nil {
		common.InternalError(w, "Failed to delete message")
		return
	}
	common.Success(w, "Message deleted", nil)
}

func (h *Handler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	userID, _ := common.GetUserID(r.Context())
	convID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	var req struct {
		MessageID int64 `json:"message_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	h.service.MarkAsRead(r.Context(), convID, userID, req.MessageID)
	common.Success(w, "Marked as read", nil)
}

func (h *Handler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	userID, _ := common.GetUserID(r.Context())
	count, _ := h.service.GetUnreadCount(r.Context(), userID)
	common.Success(w, "", map[string]int64{"unread_count": count})
}

func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Token required", http.StatusUnauthorized)
		return
	}

	// TODO: Validate token and get userID
	// For now, get from query param (in production, validate JWT)
	userIDStr := r.URL.Query().Get("user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid user", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		UserID: userID,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		Hub:    h.hub,
	}

	h.hub.Register(client)

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.Hub.Unregister(c)
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var incoming WSIncomingMessage
		if err := json.Unmarshal(message, &incoming); err != nil {
			continue
		}

		switch incoming.Type {
		case "subscribe":
			c.Hub.Subscribe(c, incoming.ConversationID)
		case "unsubscribe":
			c.Hub.Unsubscribe(c, incoming.ConversationID)
		case "typing":
			c.Hub.BroadcastToConversation(incoming.ConversationID, &WSEvent{
				Type:           WSEventTyping,
				ConversationID: incoming.ConversationID,
				UserID:         c.UserID,
			})
		case "stop_typing":
			c.Hub.BroadcastToConversation(incoming.ConversationID, &WSEvent{
				Type:           WSEventStopTyping,
				ConversationID: incoming.ConversationID,
				UserID:         c.UserID,
			})
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Conn.WriteMessage(websocket.TextMessage, message)
		case <-ticker.C:
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
