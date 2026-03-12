package dpop

import (
	"sync"
	"time"
)

// JTICache is a thread-safe in-memory store for DPoP proof JTIs.
// It prevents replay attacks by tracking seen JTIs for jtiTTL.
type JTICache struct {
	mu      sync.Mutex
	entries map[string]time.Time
}

func NewJTICache() *JTICache {
	return &JTICache{entries: make(map[string]time.Time)}
}

// Seen returns true if the jti was already seen (replay). Otherwise it records
// the jti and returns false.
func (c *JTICache) Seen(jti string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evict()
	if _, exists := c.entries[jti]; exists {
		return true
	}
	c.entries[jti] = time.Now().Add(jtiTTL)
	return false
}

// evict removes expired entries. Must be called with c.mu held.
func (c *JTICache) evict() {
	now := time.Now()
	for k, exp := range c.entries {
		if now.After(exp) {
			delete(c.entries, k)
		}
	}
}
