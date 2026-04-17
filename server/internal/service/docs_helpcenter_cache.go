package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/cache"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// Help-center cache TTLs. Short enough that stale content stays bounded;
// long enough that a hot space amortizes its DB cost across many readers.
const (
	hcNavCacheTTL = 5 * time.Minute
)

// HelpcenterCacheTagWorkspace returns the tag used to invalidate every cached
// help-center read for a workspace. Write-path handlers import this so the
// tag vocabulary stays in one place.
func HelpcenterCacheTagWorkspace(workspaceID string) string {
	return "hc:ws:" + workspaceID
}

// SetHelpcenterCache wires a cache into the service. A nil argument leaves the
// service uncached — the safe default for tests and single-pod runs.
func (s *DocsHelpcenterService) SetHelpcenterCache(c cache.Cache) {
	s.hcCache = c
}

// GetSpaceNavigation is the public entry point for the navigation tree. It
// serves responses from cache when available, falling back to a live read
// that populates the cache under a workspace-scoped tag so any HC write in
// the same workspace can invalidate it cheaply.
func (s *DocsHelpcenterService) GetSpaceNavigation(ctx context.Context, workspaceID, requestedLocale, spaceSlug string) ([]model.PublicNavCollection, error) {
	if s.hcCache == nil {
		return s.getSpaceNavigationUncached(ctx, workspaceID, requestedLocale, spaceSlug)
	}

	key := "hc:nav:" + workspaceID + ":" + requestedLocale + ":" + spaceSlug
	if raw, hit, err := s.hcCache.Get(ctx, key); err == nil && hit {
		var out []model.PublicNavCollection
		if err := json.Unmarshal(raw, &out); err == nil {
			return out, nil
		}
		// Malformed entry — fall through and refresh.
		slog.WarnContext(ctx, "hc cache: decode failed, refreshing", "key", key)
	}

	result, err := s.getSpaceNavigationUncached(ctx, workspaceID, requestedLocale, spaceSlug)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(result)
	if err != nil {
		slog.WarnContext(ctx, "hc cache: marshal failed", "error", err, "key", key)
		return result, nil
	}
	if err := s.hcCache.Set(ctx, key, raw, hcNavCacheTTL, HelpcenterCacheTagWorkspace(workspaceID)); err != nil {
		slog.WarnContext(ctx, "hc cache: set failed", "error", err, "key", key)
	}
	return result, nil
}

// InvalidateHelpcenterCacheForWorkspace drops every cached help-center entry
// associated with a workspace. Safe to call when the cache is nil.
func (s *DocsHelpcenterService) InvalidateHelpcenterCacheForWorkspace(ctx context.Context, workspaceID string) {
	if s.hcCache == nil || workspaceID == "" {
		return
	}
	if err := s.hcCache.InvalidateTags(ctx, HelpcenterCacheTagWorkspace(workspaceID)); err != nil {
		slog.WarnContext(ctx, "hc cache: invalidate failed", "error", err, "workspace_id", workspaceID)
	}
}
