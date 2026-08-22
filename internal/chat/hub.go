package chat

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

var ViewerCountInterval = 15 * time.Second

type Hub struct {
	mu         sync.RWMutex
	clients    map[string]map[*Client]struct{}
	pubsub     PubSub
	subscribed map[string]bool
	subMu      sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewHub(pubsub PubSub) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	h := &Hub{
		clients:    make(map[string]map[*Client]struct{}),
		pubsub:     pubsub,
		subscribed: make(map[string]bool),
		ctx:        ctx,
		cancel:     cancel,
	}
	h.startViewerCountBroadcaster()
	return h
}

func (h *Hub) Count(channel string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[channel])
}

func (h *Hub) Counts() map[string]int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	m := make(map[string]int, len(h.clients))
	for ch, clients := range h.clients {
		m[ch] = len(clients)
	}
	return m
}

func (h *Hub) startViewerCountBroadcaster() {
	go func() {
		interval := ViewerCountInterval
		if interval <= 0 {
			interval = 15 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				h.mu.RLock()
				channels := make([]string, 0, len(h.clients))
				for ch := range h.clients {
					channels = append(channels, ch)
				}
				h.mu.RUnlock()
				for _, ch := range channels {
					count := h.Count(ch)
					msg, _ := json.Marshal(map[string]any{"type": "viewer_count", "viewer_count": count, "count": count})
					h.Broadcast(ch, string(msg))
				}
			case <-h.ctx.Done():
				return
			}
		}
	}()
}

func (h *Hub) Add(channel string, c *Client) {
	h.mu.Lock()
	if h.clients[channel] == nil {
		h.clients[channel] = make(map[*Client]struct{})
	}
	h.clients[channel][c] = struct{}{}
	h.mu.Unlock()
	h.ensureSubscribed(channel)
}

func (h *Hub) Remove(channel string, c *Client) {
	h.mu.Lock()
	if m, ok := h.clients[channel]; ok {
		delete(m, c)
		if len(m) == 0 {
			delete(h.clients, channel)
		}
	}
	h.mu.Unlock()
}

func (h *Hub) Broadcast(channel, message string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	// fmt.Printf("Broadcast channel=%s clients=%d msg=%.20s\n", channel, len(h.clients[channel]), message)
	for c := range h.clients[channel] {
		// safe send: avoid panic on closed channel
		func() {
			defer func() { _ = recover() }()
			select {
			case c.send <- []byte(message):
			default:
			}
		}()
	}
}

func (h *Hub) Publish(ctx context.Context, channel, message string) error {
	return h.pubsub.Publish(ctx, pubSubChannel(channel), message)
}

func (h *Hub) ensureSubscribed(channel string) {
	h.subMu.Lock()
	defer h.subMu.Unlock()
	if h.subscribed[channel] {
		return
	}
	h.subscribed[channel] = true
	ch, unsub, err := h.pubsub.Subscribe(h.ctx, pubSubChannel(channel))
	if err != nil {
		// allow retry on next add
		delete(h.subscribed, channel)
		return
	}
	go func() {
		defer unsub()
		for msg := range ch {
			h.Broadcast(channel, msg)
		}
	}()
}

func pubSubChannel(channel string) string { return "chat:" + channel }

func (h *Hub) Close() {
	h.cancel()
	_ = h.pubsub.Close()
}
