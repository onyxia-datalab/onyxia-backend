package tools

import (
	"errors"
	"sync"
	"time"
)

var errFetchPanicked = errors.New("cache fetch panicked")

type cacheEntry[V any] struct {
	value     V
	fetchedAt time.Time
}

// inflightFetch is a fetch in progress for one key; concurrent callers of
// that key wait for it instead of fetching again.
type inflightFetch[V any] struct {
	done  chan struct{}
	value V
	err   error
}

// TTLCache is a generic thread-safe cache with per-Get TTL.
//
// The lock only guards the maps, never a fetch: a slow fetch (e.g. a Helm
// index download) delays only the callers of the same key, and concurrent
// callers of that key share one fetch. Errors are not cached.
type TTLCache[K comparable, V any] struct {
	mu       sync.Mutex
	entries  map[K]cacheEntry[V]
	inflight map[K]*inflightFetch[V]
}

func NewTTLCache[K comparable, V any]() *TTLCache[K, V] {
	return &TTLCache[K, V]{
		entries:  make(map[K]cacheEntry[V]),
		inflight: make(map[K]*inflightFetch[V]),
	}
}

// Get returns the cached value for key if it was fetched within ttl, otherwise
// calls fetch, stores the result, and returns it.
func (c *TTLCache[K, V]) Get(key K, ttl time.Duration, fetch func() (V, error)) (V, error) {
	c.mu.Lock()
	if e, ok := c.entries[key]; ok && time.Since(e.fetchedAt) < ttl {
		c.mu.Unlock()
		return e.value, nil
	}
	if f, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		<-f.done
		return f.value, f.err
	}
	f := &inflightFetch[V]{done: make(chan struct{})}
	c.inflight[key] = f
	c.mu.Unlock()

	completed := false
	// Release the waiters even if fetch panics (they get errFetchPanicked).
	defer func() {
		if !completed {
			f.err = errFetchPanicked
		}
		c.mu.Lock()
		if f.err == nil {
			c.entries[key] = cacheEntry[V]{value: f.value, fetchedAt: time.Now()}
		}
		delete(c.inflight, key)
		c.mu.Unlock()
		close(f.done)
	}()

	f.value, f.err = fetch()
	completed = true

	return f.value, f.err
}
