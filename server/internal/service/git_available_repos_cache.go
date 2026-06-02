package service

import (
	"strings"
	"sync"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const availableReposCacheTTL = 5 * time.Minute

type availableReposCacheEntry struct {
	value     []model.GitAvailableRepo
	expiresAt time.Time
}

// availableReposCache is a tiny TTL store for ListAvailableRepos results,
// keyed by (integration_id, search_query). It exists to absorb re-render
// bursts and accidental double-clicks on the picker without re-hitting GitLab.
type availableReposCache struct {
	mu      sync.Mutex
	entries map[string]availableReposCacheEntry
}

func newAvailableReposCache() *availableReposCache {
	return &availableReposCache{entries: map[string]availableReposCacheEntry{}}
}

func availableReposCacheKey(integrationID, search string) string {
	return integrationID + "|" + strings.ToLower(strings.TrimSpace(search))
}

// Get returns the cached value if present and unexpired. Expired entries are
// removed on access; no background sweeper.
func (c *availableReposCache) Get(integrationID, search string) ([]model.GitAvailableRepo, bool) {
	if c == nil {
		return nil, false
	}
	key := availableReposCacheKey(integrationID, search)
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return entry.value, true
}

// Set stores the value for the configured TTL.
func (c *availableReposCache) Set(integrationID, search string, value []model.GitAvailableRepo) {
	if c == nil {
		return
	}
	key := availableReposCacheKey(integrationID, search)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = availableReposCacheEntry{
		value:     value,
		expiresAt: time.Now().Add(availableReposCacheTTL),
	}
}

// InvalidateIntegration drops every cached entry for the given integration,
// across all search keys. Called when the integration's repos change (sync,
// delete) or when the caller explicitly requested a fresh fetch.
func (c *availableReposCache) InvalidateIntegration(integrationID string) {
	if c == nil {
		return
	}
	prefix := integrationID + "|"
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.entries {
		if strings.HasPrefix(key, prefix) {
			delete(c.entries, key)
		}
	}
}
