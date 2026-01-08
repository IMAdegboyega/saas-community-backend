package messaging

import (
	"encoding/json"
	"log"
	"sync"
)

// Hub maintains active WebSocket connections
type Hub struct {
	// Registered clients by user ID
	clients map[int64]map[*Client]bool

	// Conversation subscriptions: conversation_id -> set of clients
	conversations map[int64]map[*Client]bool

	// Channel for registering clients
	register chan *Client

	// Channel for unregistering clients
	unregister chan *Client

	// Channel for subscribing to conversations
	subscribe chan *Subscription

	// Channel for unsubscribing from conversations
	unsubscribe chan *Subscription

	// Channel for broadcasting to conversations
	broadcast chan *BroadcastMessage

	// Mutex for thread safety
	mu sync.RWMutex
}

// Client represents a WebSocket client
type Client struct {
	UserID int64
	Conn   WSConn
	Send   chan []byte
	Hub    *Hub
}

// WSConn interface for WebSocket connection
type WSConn interface {
	WriteMessage(messageType int, data []byte) error
	ReadMessage() (messageType int, p []byte, err error)
	Close() error
}

// Subscription represents a conversation subscription request
type Subscription struct {
	Client         *Client
	ConversationID int64
}

// BroadcastMessage represents a message to broadcast
type BroadcastMessage struct {
	ConversationID int64
	Event          *WSEvent
}

// NewHub creates a new Hub
func NewHub() *Hub {
	return &Hub{
		clients:       make(map[int64]map[*Client]bool),
		conversations: make(map[int64]map[*Client]bool),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		subscribe:     make(chan *Subscription),
		unsubscribe:   make(chan *Subscription),
		broadcast:     make(chan *BroadcastMessage, 256),
	}
}

// Run starts the hub's event loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case sub := <-h.subscribe:
			h.subscribeToConversation(sub)

		case sub := <-h.unsubscribe:
			h.unsubscribeFromConversation(sub)

		case msg := <-h.broadcast:
			h.broadcastToConversation(msg)
		}
	}
}

func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[client.UserID] == nil {
		h.clients[client.UserID] = make(map[*Client]bool)
	}
	h.clients[client.UserID][client] = true

	log.Printf("Client registered: user %d", client.UserID)
}

func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Remove from clients
	if clients, ok := h.clients[client.UserID]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.clients, client.UserID)
		}
	}

	// Remove from all conversation subscriptions
	for convID, clients := range h.conversations {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.conversations, convID)
		}
	}

	close(client.Send)
	log.Printf("Client unregistered: user %d", client.UserID)
}

func (h *Hub) subscribeToConversation(sub *Subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.conversations[sub.ConversationID] == nil {
		h.conversations[sub.ConversationID] = make(map[*Client]bool)
	}
	h.conversations[sub.ConversationID][sub.Client] = true

	log.Printf("User %d subscribed to conversation %d", sub.Client.UserID, sub.ConversationID)
}

func (h *Hub) unsubscribeFromConversation(sub *Subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.conversations[sub.ConversationID]; ok {
		delete(clients, sub.Client)
		if len(clients) == 0 {
			delete(h.conversations, sub.ConversationID)
		}
	}
}

func (h *Hub) broadcastToConversation(msg *BroadcastMessage) {
	h.mu.RLock()
	clients := h.conversations[msg.ConversationID]
	h.mu.RUnlock()

	data, err := json.Marshal(msg.Event)
	if err != nil {
		log.Printf("Failed to marshal event: %v", err)
		return
	}

	for client := range clients {
		select {
		case client.Send <- data:
		default:
			// Client's send buffer is full, skip
			log.Printf("Client send buffer full for user %d", client.UserID)
		}
	}
}

// BroadcastToConversation sends an event to all participants in a conversation
func (h *Hub) BroadcastToConversation(convID int64, event *WSEvent) {
	h.broadcast <- &BroadcastMessage{
		ConversationID: convID,
		Event:          event,
	}
}

// BroadcastToUser sends an event to all connections of a specific user
func (h *Hub) BroadcastToUser(userID int64, event *WSEvent) {
	h.mu.RLock()
	clients := h.clients[userID]
	h.mu.RUnlock()

	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	for client := range clients {
		select {
		case client.Send <- data:
		default:
		}
	}
}

// IsUserOnline checks if a user has any active connections
func (h *Hub) IsUserOnline(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[userID]) > 0
}

// GetOnlineUsers returns a list of online user IDs from a given list
func (h *Hub) GetOnlineUsers(userIDs []int64) []int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	online := make([]int64, 0)
	for _, userID := range userIDs {
		if len(h.clients[userID]) > 0 {
			online = append(online, userID)
		}
	}
	return online
}

// Register registers a client
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister unregisters a client
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Subscribe subscribes a client to a conversation
func (h *Hub) Subscribe(client *Client, convID int64) {
	h.subscribe <- &Subscription{Client: client, ConversationID: convID}
}

// Unsubscribe unsubscribes a client from a conversation
func (h *Hub) Unsubscribe(client *Client, convID int64) {
	h.unsubscribe <- &Subscription{Client: client, ConversationID: convID}
}
