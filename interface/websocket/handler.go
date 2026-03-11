package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"web-tracker/domain"
)

// Upgrader configures the WebSocket connection upgrade
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from any origin for now
		// In production, you should validate the origin
		return true
	},
}

// Connection represents a WebSocket connection
type Connection struct {
	ID     string
	Conn   *websocket.Conn
	Send   chan []byte
	Hub    *Hub
	mu     sync.Mutex
	closed bool
}

// Hub maintains the set of active connections and broadcasts messages to them
type Hub struct {
	// Registered connections
	connections map[*Connection]bool

	// Inbound messages from connections
	broadcast chan []byte

	// Register requests from connections
	register chan *Connection

	// Unregister requests from connections
	unregister chan *Connection

	// Mutex to protect connections map
	mu sync.RWMutex

	// Context for graceful shutdown
	ctx context.Context
}

// Message represents a WebSocket message
type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// HealthCheckUpdateData represents health check update message data
type HealthCheckUpdateData struct {
	MonitorID    string    `json:"monitor_id"`
	Status       string    `json:"status"`
	StatusCode   int       `json:"status_code"`
	ResponseTime int64     `json:"response_time_ms"`
	CheckedAt    time.Time `json:"checked_at"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// AlertData represents alert message data
type AlertData struct {
	MonitorID string                    `json:"monitor_id"`
	Type      domain.AlertType          `json:"type"`
	Severity  domain.AlertSeverity      `json:"severity"`
	Message   string                    `json:"message"`
	Details   map[string]interface{}    `json:"details"`
	SentAt    time.Time                 `json:"sent_at"`
	Channels  []domain.AlertChannelType `json:"channels"`
}

// NewHub creates a new WebSocket hub
func NewHub(ctx context.Context) *Hub {
	return &Hub{
		connections: make(map[*Connection]bool),
		broadcast:   make(chan []byte, 256),
		register:    make(chan *Connection),
		unregister:  make(chan *Connection),
		ctx:         ctx,
	}
}

// Run starts the hub and handles connection management
func (h *Hub) Run() {
	for {
		select {
		case <-h.ctx.Done():
			// Graceful shutdown
			h.mu.Lock()
			for conn := range h.connections {
				conn.Close()
			}
			h.mu.Unlock()
			return

		case conn := <-h.register:
			h.mu.Lock()
			h.connections[conn] = true
			h.mu.Unlock()
			log.Printf("WebSocket connection registered: %s", conn.ID)

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.connections[conn]; ok {
				delete(h.connections, conn)
				close(conn.Send)
			}
			h.mu.Unlock()
			log.Printf("WebSocket connection unregistered: %s", conn.ID)

		case message := <-h.broadcast:
			h.mu.RLock()
			for conn := range h.connections {
				select {
				case conn.Send <- message:
				default:
					// Connection is blocked, close it
					delete(h.connections, conn)
					close(conn.Send)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastHealthCheckUpdate broadcasts a health check update to all connected clients
func (h *Hub) BroadcastHealthCheckUpdate(healthCheck *domain.HealthCheck) {
	data := HealthCheckUpdateData{
		MonitorID:    healthCheck.MonitorID,
		Status:       string(healthCheck.Status),
		StatusCode:   healthCheck.StatusCode,
		ResponseTime: healthCheck.ResponseTime.Milliseconds(),
		CheckedAt:    healthCheck.CheckedAt,
		ErrorMessage: healthCheck.ErrorMessage,
	}

	message := Message{
		Type: "health_check_update",
		Data: data,
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling health check update: %v", err)
		return
	}

	select {
	case h.broadcast <- messageBytes:
	default:
		log.Printf("Broadcast channel full, dropping health check update message")
	}
}

// BroadcastAlert broadcasts an alert to all connected clients
func (h *Hub) BroadcastAlert(alert *domain.Alert) {
	data := AlertData{
		MonitorID: alert.MonitorID,
		Type:      alert.Type,
		Severity:  alert.Severity,
		Message:   alert.Message,
		Details:   alert.Details,
		SentAt:    alert.SentAt,
		Channels:  alert.Channels,
	}

	message := Message{
		Type: "alert",
		Data: data,
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling alert: %v", err)
		return
	}

	select {
	case h.broadcast <- messageBytes:
	default:
		log.Printf("Broadcast channel full, dropping alert message")
	}
}

// GetConnectionCount returns the number of active connections
func (h *Hub) GetConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// Close gracefully closes the connection
func (c *Connection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	c.closed = true
	c.Conn.Close()
}

// writePump pumps messages from the hub to the websocket connection
func (c *Connection) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// The hub closed the channel
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *Connection) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(512)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
		// For now, we don't process incoming messages from clients
		// This is primarily a broadcast-only WebSocket
	}
}

// Handler handles WebSocket connection requests
type Handler struct {
	hub *Hub
}

// NewHandler creates a new WebSocket handler
func NewHandler(hub *Hub) *Handler {
	return &Handler{
		hub: hub,
	}
}

// HandleWebSocket handles WebSocket connection upgrade and management
func (h *Handler) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Generate connection ID
	connID := fmt.Sprintf("conn_%d", time.Now().UnixNano())

	// Create connection
	connection := &Connection{
		ID:   connID,
		Conn: conn,
		Send: make(chan []byte, 256),
		Hub:  h.hub,
	}

	// Register connection
	h.hub.register <- connection

	// Start goroutines for reading and writing
	go connection.writePump()
	go connection.readPump()
}
