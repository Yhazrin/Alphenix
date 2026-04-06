package realtime

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/multica-ai/multicode/server/internal/auth"
)

// MembershipChecker verifies a user belongs to a workspace.
type MembershipChecker interface {
	IsMember(ctx context.Context, userID, workspaceID string) bool
}

// checkOrigin validates the WebSocket origin against a set of allowed origins.
// If no allowed origins are configured, all origins are rejected.
// Use "*" to allow all origins (not recommended for production).
func checkOrigin(allowedOrigins []string) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Same-origin or non-browser client
		}
		for _, allowed := range allowedOrigins {
			if allowed == "*" || strings.EqualFold(origin, allowed) {
				return true
			}
		}
		return false
	}
}

// Client represents a single WebSocket connection with identity.
type Client struct {
	hub         *Hub
	conn        *websocket.Conn
	send        chan []byte
	userID      string
	workspaceID string
	pongWait    chan struct{} // closed when a pong is received or connection dies
}

// Hub manages WebSocket connections organized by workspace rooms.
type Hub struct {
	rooms          map[string]map[*Client]bool // workspaceID -> clients
	broadcast      chan []byte                  // global broadcast (daemon events)
	register       chan *Client
	unregister     chan *Client
	mu             sync.RWMutex
	allowedOrigins []string
	closeOnce      sync.Once // protects broadcast channel close
}

// NewHub creates a new Hub instance.
func NewHub(allowedOrigins []string) *Hub {
	return &Hub{
		rooms:          make(map[string]map[*Client]bool),
		broadcast:      make(chan []byte, 64),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		allowedOrigins: allowedOrigins,
	}
}

// Run starts the hub event loop.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			room := client.workspaceID
			if h.rooms[room] == nil {
				h.rooms[room] = make(map[*Client]bool)
			}
			h.rooms[room][client] = true
			total := 0
			for _, r := range h.rooms {
				total += len(r)
			}
			h.mu.Unlock()
			slog.Info("ws client connected", "workspace_id", room, "total_clients", total)

		case client := <-h.unregister:
			h.mu.Lock()
			room := client.workspaceID
			if clients, ok := h.rooms[room]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					close(client.send)
					if len(clients) == 0 {
						delete(h.rooms, room)
					}
				}
			}
			total := 0
			for _, r := range h.rooms {
				total += len(r)
			}
			h.mu.Unlock()
			slog.Info("ws client disconnected", "workspace_id", room, "total_clients", total)

		case message, ok := <-h.broadcast:
			if !ok {
				return // broadcast channel closed
			}
			// Global broadcast for daemon events (no workspace filtering)
			h.mu.RLock()
			var slow []*Client
			for _, clients := range h.rooms {
				for client := range clients {
					select {
					case client.send <- message:
					default:
						slow = append(slow, client)
					}
				}
			}
			h.mu.RUnlock()
			if len(slow) > 0 {
				h.mu.Lock()
				for _, client := range slow {
					room := client.workspaceID
					if clients, ok := h.rooms[room]; ok {
						if _, exists := clients[client]; exists {
							delete(clients, client)
							close(client.send)
							if len(clients) == 0 {
								delete(h.rooms, room)
							}
						}
					}
				}
				h.mu.Unlock()
			}
		}
	}
}

// BroadcastToWorkspace sends a message only to clients in the given workspace.
func (h *Hub) BroadcastToWorkspace(workspaceID string, message []byte) {
	h.mu.RLock()
	clients := h.rooms[workspaceID]
	var slow []*Client
	for client := range clients {
		select {
		case client.send <- message:
		default:
			slow = append(slow, client)
		}
	}
	h.mu.RUnlock()

	// Remove slow clients under write lock
	if len(slow) > 0 {
		h.mu.Lock()
		for _, client := range slow {
			if room, ok := h.rooms[workspaceID]; ok {
				if _, exists := room[client]; exists {
					delete(room, client)
					close(client.send)
					if len(room) == 0 {
						delete(h.rooms, workspaceID)
					}
				}
			}
		}
		h.mu.Unlock()
	}
}

// SendToUser sends a message to all connections belonging to a specific user,
// regardless of which workspace room they are in. Connections in excludeWorkspace
// are skipped (they already receive the message via BroadcastToWorkspace).
func (h *Hub) SendToUser(userID string, message []byte, excludeWorkspace ...string) {
	exclude := ""
	if len(excludeWorkspace) > 0 {
		exclude = excludeWorkspace[0]
	}

	h.mu.RLock()
	type target struct {
		client      *Client
		workspaceID string
	}
	var targets []target
	for wsID, clients := range h.rooms {
		if wsID == exclude {
			continue
		}
		for client := range clients {
			if client.userID == userID {
				targets = append(targets, target{client, wsID})
			}
		}
	}
	h.mu.RUnlock()

	var slow []target
	for _, t := range targets {
		select {
		case t.client.send <- message:
		default:
			slow = append(slow, t)
		}
	}

	// Remove slow clients under write lock (same pattern as BroadcastToWorkspace)
	if len(slow) > 0 {
		h.mu.Lock()
		for _, t := range slow {
			if room, ok := h.rooms[t.workspaceID]; ok {
				if _, exists := room[t.client]; exists {
					delete(room, t.client)
					close(t.client.send)
					if len(room) == 0 {
						delete(h.rooms, t.workspaceID)
					}
				}
			}
		}
		h.mu.Unlock()
	}
}

// Broadcast sends a message to all connected clients (used for daemon events).
func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

// CloseBroadcast safely closes the broadcast channel exactly once.
func (h *Hub) CloseBroadcast() {
	h.closeOnce.Do(func() {
		close(h.broadcast)
	})
}

// HandleWebSocket upgrades an HTTP connection to WebSocket with JWT or ticket auth.
func HandleWebSocket(hub *Hub, mc MembershipChecker, w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	ticketStr := r.URL.Query().Get("ticket")
	tokenStr := r.URL.Query().Get("token")

	var userID string
	var err error

	if ticketStr != "" && workspaceID != "" {
		// Ticket-based auth (new flow)
		store := TicketStoreFor()
		if store == nil {
			http.Error(w, `{"error":"ticket store not available"}`, http.StatusServiceUnavailable)
			return
		}
		wsID, uid, ok := store.Validate(ticketStr, workspaceID)
		if !ok {
			http.Error(w, `{"error":"invalid or expired ticket"}`, http.StatusUnauthorized)
			return
		}
		workspaceID = wsID
		userID = uid
	} else if tokenStr != "" && workspaceID != "" {
		// Token-based auth (legacy fallback)
		token, parseErr := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return auth.JWTSecret(), nil
		})
		if parseErr != nil || !token.Valid {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, `{"error":"invalid claims"}`, http.StatusUnauthorized)
			return
		}
		userID, ok = claims["sub"].(string)
		if !ok || strings.TrimSpace(userID) == "" {
			http.Error(w, `{"error":"invalid claims"}`, http.StatusUnauthorized)
			return
		}
	} else {
		http.Error(w, `{"error":"ticket or token and workspace_id required"}`, http.StatusUnauthorized)
		return
	}

	// Verify user is a member of the workspace
	if !mc.IsMember(r.Context(), userID, workspaceID) {
		http.Error(w, `{"error":"not a member of this workspace"}`, http.StatusForbidden)
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: checkOrigin(hub.allowedOrigins),
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}

	client := &Client{
		hub:         hub,
		conn:        conn,
		send:        make(chan []byte, 256),
		userID:      userID,
		workspaceID: workspaceID,
		pongWait:    make(chan struct{}),
	}
	hub.register <- client

	go client.writePump()
	go client.readPump()
}

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = 30 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
)

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		// Signal that a pong was received
		select {
		case c.pongWait <- struct{}{}:
		default:
		}
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Debug("websocket read error", "error", err, "user_id", c.userID, "workspace_id", c.workspaceID)
			}
			break
		}
		// Route inbound messages to appropriate handlers
		slog.Debug("ws message received", "user_id", c.userID, "workspace_id", c.workspaceID, "message", string(message))
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				slog.Warn("websocket write error", "error", err)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
