package tools

import (
	"sync"
	"time"
)

type cacheEntry[V any] struct {
	value     V
	fetchedAt time.Time
}

// TTLCache is a generic thread-safe cache with per-Get TTL.
type TTLCache[K comparable, V any] struct {
	mu      sync.Mutex
	entries map[K]cacheEntry[V]
}

func NewTTLCache[K comparable, V any]() *TTLCache[K, V] {
	return &TTLCache[K, V]{entries: make(map[K]cacheEntry[V])}
}

// Get returns the cached value for key if it was fetched within ttl, otherwise
// calls fetch, stores the result, and returns it.
func (c *TTLCache[K, V]) Get(key K, ttl time.Duration, fetch func() (V, error)) (V, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[key]; ok && time.Since(e.fetchedAt) < ttl {
		return e.value, nil
	}

	value, err := fetch()
	if err != nil {
		var zero V
		return zero, err
	}

	c.entries[key] = cacheEntry[V]{value: value, fetchedAt: time.Now()}
	return value, nil
}
