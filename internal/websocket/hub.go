package websocket

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for development
		// In production, restrict to specific origins
		return true
	},
}

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Inbound messages from services to broadcast
	broadcast chan []byte

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Mutex for thread-safe client access
	mu sync.RWMutex

	// Hub started time
	startedAt time.Time

	// Logger
	logger *log.Logger
}

// NewHub creates a new Hub instance
func NewHub(logger *log.Logger) *Hub {
	if logger == nil {
		logger = log.Default()
	}
	return &Hub{
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		startedAt:  time.Now(),
		logger:     logger,
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	// Start heartbeat ticker
	heartbeatTicker := time.NewTicker(pingPeriod)
	defer heartbeatTicker.Stop()

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			clientCount := len(h.clients)
			h.mu.Unlock()
			h.logger.Printf("WebSocket client connected: %s (total: %d)", client.ID, clientCount)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			clientCount := len(h.clients)
			h.mu.Unlock()
			h.logger.Printf("WebSocket client disconnected: %s (total: %d)", client.ID, clientCount)

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client buffer full, mark for removal
					go func(c *Client) {
						h.unregister <- c
					}(client)
				}
			}
			h.mu.RUnlock()

		case <-heartbeatTicker.C:
			h.sendHeartbeat()
		}
	}
}

// sendHeartbeat sends a heartbeat message to all clients
func (h *Hub) sendHeartbeat() {
	h.mu.RLock()
	clientCount := len(h.clients)
	h.mu.RUnlock()

	if clientCount == 0 {
		return
	}

	heartbeat := NewMessage(TypeHeartbeat, "ping", HeartbeatData{
		ServerTime:  time.Now().UTC(),
		ClientCount: clientCount,
	})

	data, err := heartbeat.ToJSON()
	if err != nil {
		h.logger.Printf("Error serializing heartbeat: %v", err)
		return
	}

	h.Broadcast(data)
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(message []byte) {
	select {
	case h.broadcast <- message:
	default:
		h.logger.Println("Broadcast channel full, message dropped")
	}
}

// BroadcastMessage broadcasts a Message struct to all clients
func (h *Hub) BroadcastMessage(msg *Message) error {
	data, err := msg.ToJSON()
	if err != nil {
		return err
	}
	h.Broadcast(data)
	return nil
}

// BroadcastEvent is a convenience method to broadcast an event
func (h *Hub) BroadcastEvent(msgType, event string, data interface{}) error {
	msg := NewMessage(msgType, event, data)
	return h.BroadcastMessage(msg)
}

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ServeWs handles websocket requests from the peer
func (h *Hub) ServeWs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Printf("WebSocket upgrade error: %v", err)
		return
	}

	clientID := uuid.New().String()[:8]
	client := NewClient(h, conn, clientID)
	h.register <- client

	// Send welcome message
	welcome := NewMessage(TypeHealth, "connected", map[string]interface{}{
		"client_id":    clientID,
		"server_time":  time.Now().UTC(),
		"message":      "Connected to Payment Orchestrator WebSocket",
	})
	if data, err := welcome.ToJSON(); err == nil {
		client.send <- data
	}

	// Start goroutines for reading and writing
	go client.WritePump()
	go client.ReadPump()
}

// GetStats returns hub statistics
func (h *Hub) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := make([]map[string]interface{}, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, map[string]interface{}{
			"id":           client.ID,
			"connected_at": client.ConnectedAt,
		})
	}

	return map[string]interface{}{
		"client_count": len(h.clients),
		"started_at":   h.startedAt,
		"uptime":       time.Since(h.startedAt).String(),
		"clients":      clients,
	}
}
