package chat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"

	"github.com/igustavo11/livestreaming-clone/internal/auth"
	"github.com/igustavo11/livestreaming-clone/internal/db"
	"github.com/igustavo11/livestreaming-clone/internal/httputil"
)

const (
	maxMessageLen = 200
	rateLimit     = time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Handler struct {
	queries *db.Queries
	hub     *Hub
}

func NewHandler(queries *db.Queries, pubsub PubSub) *Handler {
	if pubsub == nil {
		pubsub = NewMemoryPubSub()
	}
	return &Handler{queries: queries, hub: NewHub(pubsub)}
}

func (h *Handler) Count(channel string) int {
	if h == nil || h.hub == nil {
		return 0
	}
	return h.hub.Count(channel)
}

func (h *Handler) Counts() map[string]int {
	if h == nil || h.hub == nil {
		return nil
	}
	return h.hub.Counts()
}

func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if !isValidUsername(username) {
		http.NotFound(w, r)
		return
	}
	if _, err := h.queries.GetPublicChannelByUsername(r.Context(), username); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	var user *auth.SessionUser
	if c, err := r.Cookie(auth.SessionCookieName); err == nil && c.Value != "" {
		if u, err := h.resolveUser(r.Context(), c.Value); err == nil {
			user = u
		}
	}

	client := newClient(conn, username, user, h.hub)
	h.hub.Add(username, client)
	go client.writePump()
	client.readPump(h)
	h.hub.Remove(username, client)
	_ = conn.Close()
}

func (h *Handler) resolveUser(ctx context.Context, token string) (*auth.SessionUser, error) {
	hash := auth.HashSessionToken(token)
	su, err := h.queries.GetSessionUser(ctx, hash)
	if err != nil {
		return nil, err
	}
	if !su.ExpiresAt.Valid || !su.ExpiresAt.Time.After(time.Now()) {
		return nil, context.Canceled
	}
	return &auth.SessionUser{
		ID:       httputil.UUIDString(su.ID),
		Email:    su.Email,
		Username: su.Username,
	}, nil
}

type Client struct {
	conn     *websocket.Conn
	send     chan []byte
	channel  string
	user     *auth.SessionUser
	hub      *Hub
	mu       sync.Mutex
	lastSend time.Time
}

func newClient(conn *websocket.Conn, channel string, user *auth.SessionUser, hub *Hub) *Client {
	return &Client{
		conn:    conn,
		send:    make(chan []byte, 64),
		channel: channel,
		user:    user,
		hub:     hub,
	}
}

func (c *Client) readPump(h *Handler) {
	defer close(c.send)
	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		var in struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(data, &in); err != nil {
			_ = c.writeError("invalid message")
			continue
		}
		msg := strings.TrimSpace(in.Message)
		if msg == "" {
			_ = c.writeError("message empty")
			continue
		}
		if len([]rune(msg)) > maxMessageLen {
			_ = c.writeError("message too long: max 200 characters")
			continue
		}
		if c.user == nil {
			_ = c.writeError("authentication required")
			continue
		}
		c.mu.Lock()
		if !c.lastSend.IsZero() && time.Since(c.lastSend) < rateLimit {
			c.mu.Unlock()
			_ = c.writeError("rate limited: 1 message per second")
			continue
		}
		c.lastSend = time.Now()
		c.mu.Unlock()

		out, _ := json.Marshal(map[string]string{
			"type":     "chat",
			"username": c.user.Username,
			"message":  msg,
		})
		_ = h.hub.Publish(context.Background(), c.channel, string(out))
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) writeError(errMsg string) error {
	b, _ := json.Marshal(map[string]string{"type": "error", "error": errMsg})
	select {
	case c.send <- b:
	default:
	}
	return nil
}

func isValidUsername(s string) bool {
	if len(s) < 3 || len(s) > 25 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}
