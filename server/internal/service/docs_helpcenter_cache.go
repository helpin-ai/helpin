package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/cache"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// Help-center cache TTLs. Short enough that stale content stays bounded;
// long enough that a hot space amortizes its DB cost across many readers.
const (
	hcCacheNamespace     = "hc-icons-v2"
	hcConfigCacheTTL     = 5 * time.Minute
	hcNavCacheTTL        = 5 * time.Minute
	hcSpacesCacheTTL     = 5 * time.Minute
	hcCollectionCacheTTL = 5 * time.Minute
	hcArticleCacheTTL    = 2 * time.Minute
	hcRedirectCacheTTL   = 5 * time.Minute
)

type publicLocalizedCollectionCacheEntry struct {
	Collection *model.PublicNavCollection `json:"collection"`
	Articles   []model.PublicNavArticle   `json:"articles"`
}

type publicCollectionCacheEntry struct {
	Collection *model.DocsCollection    `json:"collection"`
	Articles   []model.PublicNavArticle `json:"articles"`
	SpaceSlug  string                   `json:"space_slug"`
}

type publicPathResolveCacheEntry struct {
	Target string `json:"target"`
}

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

// ResolveConfig resolves a help center config by subdomain first, then by
// custom domain. It is hot for every public route, so cache the enriched
// public config by host identifier.
func (s *DocsHelpcenterService) ResolveConfig(ctx context.Context, identifier string) (*model.DocsHelpcenterConfig, error) {
	if s.hcCache == nil {
		return s.resolveConfigUncached(ctx, identifier)
	}

	key := hcCacheKey("hc", "config", identifier)
	var out model.DocsHelpcenterConfig
	if s.getHelpcenterCachedJSON(ctx, key, &out) {
		return &out, nil
	}

	cfg, err := s.resolveConfigUncached(ctx, identifier)
	if err != nil || cfg == nil {
		return cfg, err
	}
	s.setHelpcenterCachedJSON(ctx, key, cfg, hcConfigCacheTTL, HelpcenterCacheTagWorkspace(cfg.WorkspaceID))
	return cfg, nil
}

// ListPublicSpaces returns external-capable spaces for the public help center.
func (s *DocsHelpcenterService) ListPublicSpaces(ctx context.Context, workspaceID, requestedLocale string) ([]model.PublicSpaceResponse, error) {
	if s.hcCache == nil {
		return s.listPublicSpacesUncached(ctx, workspaceID, requestedLocale)
	}

	key := hcCacheKey("hc", "spaces", workspaceID, requestedLocale)
	var out []model.PublicSpaceResponse
	if s.getHelpcenterCachedJSON(ctx, key, &out) {
		return out, nil
	}

	result, err := s.listPublicSpacesUncached(ctx, workspaceID, requestedLocale)
	if err != nil {
		return nil, err
	}
	s.setHelpcenterCachedJSON(ctx, key, result, hcSpacesCacheTTL, HelpcenterCacheTagWorkspace(workspaceID))
	return result, nil
}

// GetSpaceNavigation is the public entry point for the navigation tree. It
// serves responses from cache when available, falling back to a live read
// that populates the cache under a workspace-scoped tag so any HC write in
// the same workspace can invalidate it cheaply.
func (s *DocsHelpcenterService) GetSpaceNavigation(ctx context.Context, workspaceID, requestedLocale, spaceSlug string) ([]model.PublicNavCollection, error) {
	if s.hcCache == nil {
		return s.getSpaceNavigationUncached(ctx, workspaceID, requestedLocale, spaceSlug)
	}

	key := hcCacheKey("hc", "nav", workspaceID, requestedLocale, spaceSlug)
	var out []model.PublicNavCollection
	if s.getHelpcenterCachedJSON(ctx, key, &out) {
		return out, nil
	}

	result, err := s.getSpaceNavigationUncached(ctx, workspaceID, requestedLocale, spaceSlug)
	if err != nil {
		return nil, err
	}

	s.setHelpcenterCachedJSON(ctx, key, result, hcNavCacheTTL, HelpcenterCacheTagWorkspace(workspaceID))
	return result, nil
}

// GetPublicArticle returns the full article detail for the locale-aware public help center.
func (s *DocsHelpcenterService) GetPublicArticle(ctx context.Context, workspaceID, requestedLocale, spaceSlug, collectionSlug, articleSlug string) (*model.PublicArticleResponse, error) {
	return s.getCachedPublicArticle(ctx, hcCacheKey("hc", "article", "localized-path", workspaceID, requestedLocale, spaceSlug, collectionSlug, articleSlug), workspaceID, true, func() (*model.PublicArticleResponse, error) {
		return s.getPublicArticleUncached(ctx, workspaceID, requestedLocale, spaceSlug, collectionSlug, articleSlug)
	})
}

func (s *DocsHelpcenterService) GetPublicArticleByLocalizedCanonicalPath(ctx context.Context, workspaceID, requestedLocale, collectionSlug, articleSlug string) (*model.PublicArticleResponse, error) {
	return s.getCachedPublicArticle(ctx, hcCacheKey("hc", "article", "localized-canonical-path", workspaceID, requestedLocale, collectionSlug, articleSlug), workspaceID, true, func() (*model.PublicArticleResponse, error) {
		return s.getPublicArticleByLocalizedCanonicalPathUncached(ctx, workspaceID, requestedLocale, collectionSlug, articleSlug)
	})
}

func (s *DocsHelpcenterService) GetPublicArticleByLocalizedCanonicalKey(ctx context.Context, workspaceID, requestedLocale, articleKey string) (*model.PublicArticleResponse, error) {
	return s.getCachedPublicArticle(ctx, hcCacheKey("hc", "article", "localized-key", workspaceID, requestedLocale, articleKey), workspaceID, true, func() (*model.PublicArticleResponse, error) {
		return s.getPublicArticleByLocalizedCanonicalKeyUncached(ctx, workspaceID, requestedLocale, articleKey)
	})
}

// GetPublicArticleByCanonicalPath returns a public article by collection slug and article slug.
func (s *DocsHelpcenterService) GetPublicArticleByCanonicalPath(ctx context.Context, workspaceID, collectionSlug, articleSlug string) (*model.PublicArticleResponse, error) {
	return s.getCachedPublicArticle(ctx, hcCacheKey("hc", "article", "canonical-path", workspaceID, collectionSlug, articleSlug), workspaceID, false, func() (*model.PublicArticleResponse, error) {
		return s.getPublicArticleByCanonicalPathUncached(ctx, workspaceID, collectionSlug, articleSlug)
	})
}

func (s *DocsHelpcenterService) GetPublicArticleByCanonicalKey(ctx context.Context, workspaceID, articleKey string) (*model.PublicArticleResponse, error) {
	return s.getCachedPublicArticle(ctx, hcCacheKey("hc", "article", "canonical-key", workspaceID, articleKey), workspaceID, false, func() (*model.PublicArticleResponse, error) {
		return s.getPublicArticleByCanonicalKeyUncached(ctx, workspaceID, articleKey)
	})
}

// GetPublicLocalizedCollection returns a translated collection page and its translated articles.
func (s *DocsHelpcenterService) GetPublicLocalizedCollection(ctx context.Context, workspaceID, requestedLocale, spaceSlug, collectionSlug string) (*model.PublicNavCollection, []model.PublicNavArticle, error) {
	if s.hcCache == nil {
		return s.getPublicLocalizedCollectionUncached(ctx, workspaceID, requestedLocale, spaceSlug, collectionSlug)
	}

	key := hcCacheKey("hc", "collection", "localized", workspaceID, requestedLocale, spaceSlug, collectionSlug)
	var out publicLocalizedCollectionCacheEntry
	if s.getHelpcenterCachedJSON(ctx, key, &out) {
		return out.Collection, out.Articles, nil
	}

	collection, articles, err := s.getPublicLocalizedCollectionUncached(ctx, workspaceID, requestedLocale, spaceSlug, collectionSlug)
	if err != nil {
		return nil, nil, err
	}
	s.setHelpcenterCachedJSON(ctx, key, publicLocalizedCollectionCacheEntry{Collection: collection, Articles: articles}, hcCollectionCacheTTL, HelpcenterCacheTagWorkspace(workspaceID))
	return collection, articles, nil
}

func (s *DocsHelpcenterService) GetPublicLocalizedCollectionByCanonicalPath(ctx context.Context, workspaceID, requestedLocale, collectionSlug string) (*model.PublicNavCollection, []model.PublicNavArticle, error) {
	if s.hcCache == nil {
		return s.getPublicLocalizedCollectionByCanonicalPathUncached(ctx, workspaceID, requestedLocale, collectionSlug)
	}

	key := hcCacheKey("hc", "collection", "localized-canonical", workspaceID, requestedLocale, collectionSlug)
	var out publicLocalizedCollectionCacheEntry
	if s.getHelpcenterCachedJSON(ctx, key, &out) {
		return out.Collection, out.Articles, nil
	}

	collection, articles, err := s.getPublicLocalizedCollectionByCanonicalPathUncached(ctx, workspaceID, requestedLocale, collectionSlug)
	if err != nil {
		return nil, nil, err
	}
	s.setHelpcenterCachedJSON(ctx, key, publicLocalizedCollectionCacheEntry{Collection: collection, Articles: articles}, hcCollectionCacheTTL, HelpcenterCacheTagWorkspace(workspaceID))
	return collection, articles, nil
}

// GetPublicCollection returns a collection, its published articles, and the parent space slug by workspace and collection key.
func (s *DocsHelpcenterService) GetPublicCollection(ctx context.Context, workspaceID, collectionKey string) (*model.DocsCollection, []model.PublicNavArticle, string, error) {
	if s.hcCache == nil {
		return s.getPublicCollectionUncached(ctx, workspaceID, collectionKey)
	}

	key := hcCacheKey("hc", "collection", "canonical", workspaceID, collectionKey)
	var out publicCollectionCacheEntry
	if s.getHelpcenterCachedJSON(ctx, key, &out) {
		return out.Collection, out.Articles, out.SpaceSlug, nil
	}

	collection, articles, spaceSlug, err := s.getPublicCollectionUncached(ctx, workspaceID, collectionKey)
	if err != nil {
		return nil, nil, "", err
	}
	s.setHelpcenterCachedJSON(ctx, key, publicCollectionCacheEntry{Collection: collection, Articles: articles, SpaceSlug: spaceSlug}, hcCollectionCacheTTL, HelpcenterCacheTagWorkspace(workspaceID))
	return collection, articles, spaceSlug, nil
}

// ResolvePublicPath resolves a legacy or imported URL path to a redirect target.
func (s *DocsHelpcenterService) ResolvePublicPath(ctx context.Context, workspaceID, path string) (string, error) {
	if s.hcCache == nil {
		return s.resolvePublicPathUncached(ctx, workspaceID, path)
	}

	normalizedPath := normalizeDocsRedirectSourcePath(path)
	if normalizedPath == "" {
		return "", nil
	}
	key := hcCacheKey("hc", "redirect", workspaceID, normalizedPath)
	var out publicPathResolveCacheEntry
	if s.getHelpcenterCachedJSON(ctx, key, &out) {
		return out.Target, nil
	}

	target, err := s.resolvePublicPathUncached(ctx, workspaceID, normalizedPath)
	if err != nil || target == "" {
		return target, err
	}
	s.setHelpcenterCachedJSON(ctx, key, publicPathResolveCacheEntry{Target: target}, hcRedirectCacheTTL, HelpcenterCacheTagWorkspace(workspaceID))
	return target, nil
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

func (s *DocsHelpcenterService) getCachedPublicArticle(ctx context.Context, key, workspaceID string, localized bool, fetch func() (*model.PublicArticleResponse, error)) (*model.PublicArticleResponse, error) {
	if s.hcCache == nil {
		return fetch()
	}

	var out model.PublicArticleResponse
	if s.getHelpcenterCachedJSON(ctx, key, &out) {
		s.incrementCachedArticleView(ctx, &out, localized)
		return &out, nil
	}

	article, err := fetch()
	if err != nil || article == nil {
		return article, err
	}
	s.setHelpcenterCachedJSON(ctx, key, article, hcArticleCacheTTL, HelpcenterCacheTagWorkspace(workspaceID))
	return article, nil
}

func (s *DocsHelpcenterService) incrementCachedArticleView(ctx context.Context, article *model.PublicArticleResponse, localized bool) {
	if article == nil || article.ID == "" {
		return
	}
	go func() {
		defer func() { recover() }()
		if localized && article.Locale != "" {
			_ = s.hcRepo.IncrementTranslatedViewCount(ctx, article.ID, article.Locale)
			return
		}
		_ = s.hcRepo.IncrementViewCount(ctx, article.ID)
	}()
}

func (s *DocsHelpcenterService) getHelpcenterCachedJSON(ctx context.Context, key string, out any) bool {
	raw, hit, err := s.hcCache.Get(ctx, key)
	if err != nil || !hit {
		if err != nil {
			slog.WarnContext(ctx, "hc cache: get failed", "error", err, "key", key)
		}
		return false
	}
	if err := json.Unmarshal(raw, out); err != nil {
		slog.WarnContext(ctx, "hc cache: decode failed, refreshing", "error", err, "key", key)
		return false
	}
	return true
}

func (s *DocsHelpcenterService) setHelpcenterCachedJSON(ctx context.Context, key string, value any, ttl time.Duration, tags ...string) {
	raw, err := json.Marshal(value)
	if err != nil {
		slog.WarnContext(ctx, "hc cache: marshal failed", "error", err, "key", key)
		return
	}
	if err := s.hcCache.Set(ctx, key, raw, ttl, tags...); err != nil {
		slog.WarnContext(ctx, "hc cache: set failed", "error", err, "key", key)
	}
}

func hcCacheKey(parts ...string) string {
	encoded := make([]string, 0, len(parts)+1)
	encoded = append(encoded, base64.RawURLEncoding.EncodeToString([]byte(hcCacheNamespace)))
	for _, part := range parts {
		encoded = append(encoded, base64.RawURLEncoding.EncodeToString([]byte(part)))
	}
	return strings.Join(encoded, ":")
}
