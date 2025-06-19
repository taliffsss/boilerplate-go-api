package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go-api-boilerplate/config"
	"go-api-boilerplate/pkg/logger"
	"go-api-boilerplate/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocketService manages WebSocket connections
type WebSocketService struct {
	config    *config.Config
	upgrader  websocket.Upgrader
	hub       *Hub
	broadcast chan *Message
}

// Hub maintains active WebSocket connections
type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// Client represents a WebSocket client
type Client struct {
	ID      string
	UserID  uint
	conn    *websocket.Conn
	send    chan []byte
	hub     *Hub
	service *WebSocketService
	rooms   map[string]bool
	mu      sync.RWMutex
}

// Message represents a WebSocket message
type Message struct {
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
	UserID    uint            `json:"user_id,omitempty"`
	Room      string          `json:"room,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// NewWebSocketService creates a new WebSocket service
func NewWebSocketService() *WebSocketService {
	cfg := config.Get()

	hub := &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}

	service := &WebSocketService{
		config: cfg,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  cfg.WebSocket.ReadBufferSize,
			WriteBufferSize: cfg.WebSocket.WriteBufferSize,
			CheckOrigin: func(r *http.Request) bool {
				// Configure origin checking based on your security requirements
				return true
			},
		},
		hub:       hub,
		broadcast: make(chan *Message, 256),
	}

	// Start hub
	go hub.run()
	go service.runBroadcast()

	return service
}

// HandleWebSocket handles WebSocket upgrade requests
func (s *WebSocketService) HandleWebSocket(c *gin.Context) {
	// Get user ID from context (if authenticated)
	var userID uint
	if id, exists := c.Get("user_id"); exists {
		userID = id.(uint)
	}

	// Upgrade connection
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.WithError(err).Error("Failed to upgrade WebSocket connection")
		return
	}

	// Create client
	client := &Client{
		ID:      utils.GenerateUUID(),
		UserID:  userID,
		conn:    conn,
		send:    make(chan []byte, 256),
		hub:     s.hub,
		service: s,
		rooms:   make(map[string]bool),
	}

	// Register client
	s.hub.register <- client

	// Start client routines
	go client.writePump()
	go client.readPump()

	// Send welcome message
	welcome := map[string]interface{}{
		"message":   "Connected to WebSocket server",
		"client_id": client.ID,
	}
	client.SendJSON("welcome", welcome)
}

// run manages the hub
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			logger.Infof("Client registered: %s", client.ID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.mu.Unlock()
				logger.Infof("Client unregistered: %s", client.ID)
			} else {
				h.mu.Unlock()
			}
		}
	}
}

// runBroadcast handles broadcast messages
func (s *WebSocketService) runBroadcast() {
	for {
		message := <-s.broadcast
		s.hub.broadcast(message)
	}
}

// broadcast sends a message to all clients or specific room
func (h *Hub) broadcast(message *Message) {
	data, err := json.Marshal(message)
	if err != nil {
		logger.WithError(err).Error("Failed to marshal broadcast message")
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		// If room is specified, only send to clients in that room
		if message.Room != "" {
			client.mu.RLock()
			inRoom := client.rooms[message.Room]
			client.mu.RUnlock()

			if !inRoom {
				continue
			}
		}

		select {
		case client.send <- data:
		default:
			// Client's send channel is full, close it
			close(client.send)
			delete(h.clients, client)
		}
	}
}

// readPump reads messages from the WebSocket connection
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(c.service.config.WebSocket.MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(c.service.config.WebSocket.PongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.service.config.WebSocket.PongWait))
		return nil
	})

	for {
		var message Message
		err := c.conn.ReadJSON(&message)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.WithError(err).Errorf("WebSocket error for client %s", c.ID)
			}
			break
		}

		// Add metadata
		message.UserID = c.UserID
		message.Timestamp = time.Now()

		// Handle message based on type
		c.handleMessage(&message)
	}
}

// writePump writes messages to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(c.service.config.WebSocket.PingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.conn.WriteMessage(websocket.TextMessage, message)

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming messages
func (c *Client) handleMessage(message *Message) {
	switch message.Type {
	case "ping":
		c.SendJSON("pong", map[string]interface{}{"timestamp": time.Now()})

	case "join_room":
		var data map[string]string
		if err := json.Unmarshal(message.Data, &data); err != nil {
			c.SendError("Invalid join_room data")
			return
		}
		c.JoinRoom(data["room"])

	case "leave_room":
		var data map[string]string
		if err := json.Unmarshal(message.Data, &data); err != nil {
			c.SendError("Invalid leave_room data")
			return
		}
		c.LeaveRoom(data["room"])

	case "broadcast":
		// Forward to broadcast channel
		c.service.broadcast <- message

	default:
		// Custom message handling
		c.service.handleCustomMessage(c, message)
	}
}

// handleCustomMessage handles custom message types
func (s *WebSocketService) handleCustomMessage(client *Client, message *Message) {
	// Implement custom message handling based on your application needs
	logger.Debugf("Received custom message type: %s from client: %s", message.Type, client.ID)
}

// SendJSON sends a JSON message to the client
func (c *Client) SendJSON(messageType string, data interface{}) error {
	message := Message{
		Type:      messageType,
		Timestamp: time.Now(),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	message.Data = jsonData

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	select {
	case c.send <- messageBytes:
		return nil
	default:
		return fmt.Errorf("client send buffer is full")
	}
}

// SendError sends an error message to the client
func (c *Client) SendError(errorMsg string) {
	c.SendJSON("error", map[string]string{"error": errorMsg})
}

// JoinRoom adds the client to a room
func (c *Client) JoinRoom(room string) {
	c.mu.Lock()
	c.rooms[room] = true
	c.mu.Unlock()

	c.SendJSON("room_joined", map[string]string{"room": room})
	logger.Infof("Client %s joined room %s", c.ID, room)
}

// LeaveRoom removes the client from a room
func (c *Client) LeaveRoom(room string) {
	c.mu.Lock()
	delete(c.rooms, room)
	c.mu.Unlock()

	c.SendJSON("room_left", map[string]string{"room": room})
	logger.Infof("Client %s left room %s", c.ID, room)
}

// BroadcastToRoom sends a message to all clients in a room
func (s *WebSocketService) BroadcastToRoom(room string, messageType string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	message := &Message{
		Type:      messageType,
		Data:      jsonData,
		Room:      room,
		Timestamp: time.Now(),
	}

	s.broadcast <- message
	return nil
}

// BroadcastToUser sends a message to a specific user
func (s *WebSocketService) BroadcastToUser(userID uint, messageType string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	message := Message{
		Type:      messageType,
		Data:      jsonData,
		Timestamp: time.Now(),
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	s.hub.mu.RLock()
	defer s.hub.mu.RUnlock()

	sent := false
	for client := range s.hub.clients {
		if client.UserID == userID {
			select {
			case client.send <- messageBytes:
				sent = true
			default:
				// Client buffer is full
			}
		}
	}

	if !sent {
		return fmt.Errorf("user %d is not connected", userID)
	}

	return nil
}

// GetConnectedClients returns the number of connected clients
func (s *WebSocketService) GetConnectedClients() int {
	s.hub.mu.RLock()
	defer s.hub.mu.RUnlock()
	return len(s.hub.clients)
}

// GetRoomClients returns the number of clients in a specific room
func (s *WebSocketService) GetRoomClients(room string) int {
	s.hub.mu.RLock()
	defer s.hub.mu.RUnlock()

	count := 0
	for client := range s.hub.clients {
		client.mu.RLock()
		if client.rooms[room] {
			count++
		}
		client.mu.RUnlock()
	}

	return count
}

// Close gracefully shuts down the WebSocket service
func (s *WebSocketService) Close() {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()

	// Close all client connections
	for client := range s.hub.clients {
		client.conn.Close()
		delete(s.hub.clients, client)
	}

	close(s.broadcast)
}
