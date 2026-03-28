package cache

import (
	"strings"
	"sync"
	"time"

	"twchfetch/internal/api"
)

// Entry holds the cached data for one streamer.
type Entry struct {
	Details    *api.StreamDetails // nil when offline
	Followers  int                // channel follower count (0 if unavailable)
	VODs       []api.VOD
	Chapters   map[string][]api.Chapter
	VODHasMore bool   // whether more VODs may exist beyond the currently cached list
	LastSeen   *time.Time // approximate end of last stream
	FetchedAt  time.Time
}

// Cache is a simple thread-safe per-streamer store with TTL.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*Entry
	ttl     time.Duration
}

// New creates a Cache with the given TTL.
func New(ttl time.Duration) *Cache {
	return &Cache{
		entries: make(map[string]*Entry),
		ttl:     ttl,
	}
}

// Get returns the cached entry for username if it exists and is still fresh.
func (c *Cache) Get(username string) (*Entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[username]
	if !ok {
		return nil, false
	}
	if c.ttl > 0 && time.Since(e.FetchedAt) > c.ttl {
		return nil, false
	}
	return e, true
}

// Set stores an entry for username, recording FetchedAt as now.
func (c *Cache) Set(username string, e *Entry) {
	e.FetchedAt = time.Now()
	c.mu.Lock()
	c.entries[username] = e
	c.mu.Unlock()
}

// Invalidate removes the cached entry for username.
func (c *Cache) Invalidate(username string) {
	c.mu.Lock()
	delete(c.entries, username)
	c.mu.Unlock()
}

// InvalidateAll clears the entire cache.
func (c *Cache) InvalidateAll() {
	c.mu.Lock()
	c.entries = make(map[string]*Entry)
	c.mu.Unlock()
}

// InvalidateIfChanged removes a streamer's cache entry when their live status
// or game/category has changed since the entry was stored.  Returns true if
// an entry was present and was invalidated.
func (c *Cache) InvalidateIfChanged(username string, isLive bool, game string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[username]
	if !ok {
		return false
	}
	wasLive := e.Details != nil
	if wasLive != isLive {
		delete(c.entries, username)
		return true
	}
	if isLive && e.Details != nil && !strings.EqualFold(e.Details.Game, game) {
		delete(c.entries, username)
		return true
	}
	return false
}

// SetTTL updates the cache TTL for subsequent Get calls.
func (c *Cache) SetTTL(d time.Duration) {
	c.mu.Lock()
	c.ttl = d
	c.mu.Unlock()
}
