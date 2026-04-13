package cache

import (
	"container/list"
	"sync"
	"time"
)

// memEntry is one slot in the in-process LRU cache.
type memEntry struct {
	key       string
	data      []byte
	expiresAt time.Time
	elem      *list.Element
}

// MemoryCache is a bounded, TTL-aware in-process LRU cache (L1 layer).
// It stores raw JSON bytes to avoid a circular import with the search package.
// All operations are O(1) amortised.
type MemoryCache struct {
	mu    sync.Mutex
	cap   int
	items map[string]*memEntry
	order *list.List // front = most-recently used
}

// NewMemoryCache creates a MemoryCache limited to capacity entries.
// Capacity <= 0 defaults to 1 000.
func NewMemoryCache(capacity int) *MemoryCache {
	if capacity <= 0 {
		capacity = 1000
	}
	return &MemoryCache{
		cap:   capacity,
		items: make(map[string]*memEntry, capacity),
		order: list.New(),
	}
}

// Get returns the raw bytes for key if present and not expired.
func (m *MemoryCache) Get(key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	e, ok := m.items[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiresAt) {
		m.remove(e)
		return nil, false
	}
	m.order.MoveToFront(e.elem)
	return e.data, true
}

// Set stores data under key with the given TTL, evicting the LRU entry when full.
func (m *MemoryCache) Set(key string, data []byte, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if e, ok := m.items[key]; ok {
		e.data = data
		e.expiresAt = time.Now().Add(ttl)
		m.order.MoveToFront(e.elem)
		return
	}

	if len(m.items) >= m.cap {
		m.evict()
	}

	e := &memEntry{
		key:       key,
		data:      data,
		expiresAt: time.Now().Add(ttl),
	}
	e.elem = m.order.PushFront(e)
	m.items[key] = e
}

// InvalidatePrefix removes all entries whose key begins with prefix.
func (m *MemoryCache) InvalidatePrefix(prefix string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, e := range m.items {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			m.remove(e)
		}
	}
}

func (m *MemoryCache) remove(e *memEntry) {
	m.order.Remove(e.elem)
	delete(m.items, e.key)
}

func (m *MemoryCache) evict() {
	if back := m.order.Back(); back != nil {
		m.remove(back.Value.(*memEntry))
	}
}
