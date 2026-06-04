package cache

import (
	"context"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// LRU is an in-process cache backed by a fixed-capacity LRU map. It is safe
// for concurrent use and tracks a reverse index from tag to keys so that
// InvalidateTags can drop related entries in bounded time.
type LRU struct {
	entries *lru.Cache[string, lruEntry]

	mu   sync.Mutex
	tags map[string]map[string]struct{} // tag -> set of keys
}

type lruEntry struct {
	value   []byte
	expires time.Time
	tags    []string
}

// NewLRU constructs an LRU cache with the given maximum number of entries.
// It panics if size is not positive because a zero-capacity cache would
// silently drop every write.
func NewLRU(size int) *LRU {
	if size <= 0 {
		panic("cache: LRU size must be positive")
	}
	c, err := lru.New[string, lruEntry](size)
	if err != nil {
		panic("cache: lru.New: " + err.Error())
	}
	return &LRU{
		entries: c,
		tags:    make(map[string]map[string]struct{}),
	}
}

// Get returns the cached bytes for key and true on an unexpired hit.
// Expired entries are evicted on access so the LRU stays tight.
func (c *LRU) Get(_ context.Context, key string) ([]byte, bool, error) {
	entry, ok := c.entries.Get(key)
	if !ok {
		return nil, false, nil
	}
	if !entry.expires.IsZero() && time.Now().After(entry.expires) {
		c.deleteKey(key)
		return nil, false, nil
	}
	return entry.value, true, nil
}

// Set stores value under key with the given TTL. A zero TTL disables
// expiry. Tags are de-duplicated and indexed for later invalidation.
func (c *LRU) Set(_ context.Context, key string, value []byte, ttl time.Duration, tags ...string) error {
	entry := lruEntry{value: value, tags: tags}
	if ttl > 0 {
		entry.expires = time.Now().Add(ttl)
	}
	c.entries.Add(key, entry)

	if len(tags) == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, tag := range tags {
		set, ok := c.tags[tag]
		if !ok {
			set = make(map[string]struct{})
			c.tags[tag] = set
		}
		set[key] = struct{}{}
	}
	return nil
}

// InvalidateTags removes every entry whose Set call included any of the
// provided tags. The tag index is also pruned.
func (c *LRU) InvalidateTags(_ context.Context, tags ...string) error {
	if len(tags) == 0 {
		return nil
	}
	c.mu.Lock()
	keysToDelete := make(map[string]struct{})
	for _, tag := range tags {
		for key := range c.tags[tag] {
			keysToDelete[key] = struct{}{}
		}
		delete(c.tags, tag)
	}
	c.mu.Unlock()

	for key := range keysToDelete {
		c.entries.Remove(key)
	}
	return nil
}

// deleteKey drops a single key and removes it from any tag sets that
// reference it. Called on lazy expiry.
func (c *LRU) deleteKey(key string) {
	entry, ok := c.entries.Peek(key)
	c.entries.Remove(key)
	if !ok || len(entry.tags) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, tag := range entry.tags {
		if set, ok := c.tags[tag]; ok {
			delete(set, key)
			if len(set) == 0 {
				delete(c.tags, tag)
			}
		}
	}
}
