package storage

import (
	"context"
	"fmt"
	"io"
	"sync"
)

// Memory is an in-process ObjectStorage for tests.
type Memory struct {
	mu      sync.Mutex
	objects map[string]object
	baseURL string
}

type object struct {
	contentType string
	data        []byte
}

func NewMemory(baseURL string) *Memory {
	if baseURL == "" {
		baseURL = "https://memory.local"
	}
	return &Memory{
		objects: make(map[string]object),
		baseURL: baseURL,
	}
}

func (m *Memory) Put(ctx context.Context, key, contentType string, body io.Reader, size int64) (string, error) {
	_ = ctx
	data, err := io.ReadAll(io.LimitReader(body, size+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > size && size >= 0 {
		// caller already enforces size; still accept what was read
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = object{contentType: contentType, data: data}
	return m.baseURL + "/" + key, nil
}

func (m *Memory) Delete(ctx context.Context, key string) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}

// Get returns stored bytes for assertions in tests.
func (m *Memory) Get(key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.objects[key]
	if !ok {
		return nil, false
	}
	out := make([]byte, len(o.data))
	copy(out, o.data)
	return out, true
}

// Has reports whether key exists.
func (m *Memory) Has(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.objects[key]
	return ok
}

// Len returns the number of stored objects.
func (m *Memory) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.objects)
}

func (m *Memory) URLFor(key string) string {
	return fmt.Sprintf("%s/%s", m.baseURL, key)
}
