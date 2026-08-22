package chat

import (
	"context"
	"sync"
)

// PubSub abstracts chat fan-out. Redis implementation is used in production;
// memory implementation is used in tests and as fallback.
type PubSub interface {
	Publish(ctx context.Context, channel, message string) error
	Subscribe(ctx context.Context, channel string) (<-chan string, func(), error)
	Close() error
}

// MemoryPubSub is an in-process pub/sub that mimics Redis for tests/single-instance.
type MemoryPubSub struct {
	mu   sync.RWMutex
	subs map[string]map[chan string]struct{}
}

func NewMemoryPubSub() *MemoryPubSub {
	return &MemoryPubSub{subs: make(map[string]map[chan string]struct{})}
}

func (m *MemoryPubSub) Publish(_ context.Context, channel, message string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for ch := range m.subs[channel] {
		select {
		case ch <- message:
		default:
			// drop if slow consumer; chat is best-effort
		}
	}
	return nil
}

func (m *MemoryPubSub) Subscribe(_ context.Context, channel string) (<-chan string, func(), error) {
	ch := make(chan string, 64)
	m.mu.Lock()
	if m.subs[channel] == nil {
		m.subs[channel] = make(map[chan string]struct{})
	}
	m.subs[channel][ch] = struct{}{}
	m.mu.Unlock()

	unsub := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if subs, ok := m.subs[channel]; ok {
			delete(subs, ch)
			if len(subs) == 0 {
				delete(m.subs, channel)
			}
		}
		close(ch)
	}
	return ch, unsub, nil
}

func (m *MemoryPubSub) Close() error { return nil }
