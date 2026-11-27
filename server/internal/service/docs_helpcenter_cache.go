package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/helpin-ai/helpin/server/internal/cache"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrPublicArticleNotFound means no published article matches a public key.
var ErrPublicArticleNotFound = errors.New("article not found")

// hcCacheTTL bounds every help-center cache entry. Writes invalidate entries
// by workspace tag, so the TTL is only a backstop; one value for every entry
// keeps a shared tag set from expiring before the entries it tracks (each
// Set resets the tag's expiry to 2x its own TTL).
const hcCacheTTL = 24 * time.Hour

// hcReinvalidateDelay is how long after an invalidation it runs again, to
// drop entries written by reads that started before the first invalidation
// and finished after it.
const hcReinvalidateDelay = 30 * time.Second

// hcCacheNamespace prefixes every key. It includes a fingerprint of this
// build so a deploy never reads JSON cached by a binary with another shape.
var hcCacheNamespace = sync.OnceValue(func() string {
	return "hc-v3-" + buildFingerprint()
})

// buildFingerprint identifies the running executable by size and modification
// time: unique per build, stable across restarts of the same image, and cheap
// (no need to pass a version in at build time or hash the binary).
func buildFingerprint() string {
	path, err := os.Executable()
	if err != nil {
		return "unknown"
	}
	info, err := os.Stat(path)
	if err != nil {
		return "unknown"
	}
	sum := sha256.Sum256([]byte(strconv.FormatInt(info.Size(), 10) + ":" +
		strconv.FormatInt(info.ModTime().UnixNano(), 10)))
	return hex.EncodeToString(sum[:])[:12]
}

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
	s.setHelpcenterCachedJSON(ctx, key, cfg, hcCacheTTL, HelpcenterCacheTagWorkspace(cfg.WorkspaceID))
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
	s.setHelpcenterCachedJSON(ctx, key, result, hcCacheTTL, HelpcenterCacheTagWorkspace(workspaceID))
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

	s.setHelpcenterCachedJSON(ctx, key, result, hcCacheTTL, HelpcenterCacheTagWorkspace(workspaceID))
	return result, nil
}

// GetPublicArticle returns the full article detail for the locale-aware public help center.
func (s *DocsHelpcenterService) GetPublicArticle(ctx context.Context, workspaceID, requestedLocale, spaceSlug, collectionSlug, articleSlug string) (*model.PublicArticleResponse, error) {
	return s.getCachedPublicArticle(ctx, hcCacheKey("hc", "article", "localized-path", workspaceID, requestedLocale, spaceSlug, collectionSlug, articleSlug), workspaceID, func() (*model.PublicArticleResponse, error) {
		return s.getPublicArticleUncached(ctx, workspaceID, requestedLocale, spaceSlug, collectionSlug, articleSlug)
	})
}

func (s *DocsHelpcenterService) GetPublicArticleByLocalizedCanonicalPath(ctx context.Context, workspaceID, requestedLocale, collectionSlug, articleSlug string) (*model.PublicArticleResponse, error) {
	return s.getCachedPublicArticle(ctx, hcCacheKey("hc", "article", "localized-canonical-path", workspaceID, requestedLocale, collectionSlug, articleSlug), workspaceID, func() (*model.PublicArticleResponse, error) {
		return s.getPublicArticleByLocalizedCanonicalPathUncached(ctx, workspaceID, requestedLocale, collectionSlug, articleSlug)
	})
}

func (s *DocsHelpcenterService) GetPublicArticleByLocalizedCanonicalKey(ctx context.Context, workspaceID, requestedLocale, articleKey string) (*model.PublicArticleResponse, error) {
	return s.getCachedPublicArticle(ctx, hcCacheKey("hc", "article", "localized-key", workspaceID, requestedLocale, articleKey), workspaceID, func() (*model.PublicArticleResponse, error) {
		return s.getPublicArticleByLocalizedCanonicalKeyUncached(ctx, workspaceID, requestedLocale, articleKey)
	})
}

// GetPublicArticleByCanonicalPath returns a public article by collection slug and article slug.
func (s *DocsHelpcenterService) GetPublicArticleByCanonicalPath(ctx context.Context, workspaceID, collectionSlug, articleSlug string) (*model.PublicArticleResponse, error) {
	return s.getCachedPublicArticle(ctx, hcCacheKey("hc", "article", "canonical-path", workspaceID, collectionSlug, articleSlug), workspaceID, func() (*model.PublicArticleResponse, error) {
		return s.getPublicArticleByCanonicalPathUncached(ctx, workspaceID, collectionSlug, articleSlug)
	})
}

func (s *DocsHelpcenterService) GetPublicArticleByCanonicalKey(ctx context.Context, workspaceID, articleKey string) (*model.PublicArticleResponse, error) {
	return s.getCachedPublicArticle(ctx, hcCacheKey("hc", "article", "canonical-key", workspaceID, articleKey), workspaceID, func() (*model.PublicArticleResponse, error) {
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
	s.setHelpcenterCachedJSON(ctx, key, publicLocalizedCollectionCacheEntry{Collection: collection, Articles: articles}, hcCacheTTL, HelpcenterCacheTagWorkspace(workspaceID))
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
	s.setHelpcenterCachedJSON(ctx, key, publicLocalizedCollectionCacheEntry{Collection: collection, Articles: articles}, hcCacheTTL, HelpcenterCacheTagWorkspace(workspaceID))
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
	s.setHelpcenterCachedJSON(ctx, key, publicCollectionCacheEntry{Collection: collection, Articles: articles, SpaceSlug: spaceSlug}, hcCacheTTL, HelpcenterCacheTagWorkspace(workspaceID))
	return collection, articles, spaceSlug, nil
}

// GetPublicCollectionAlternatePaths returns canonical localized paths for the
// published collection translations used to emit SSR hreflang links without
// loading every locale's full space navigation.
func (s *DocsHelpcenterService) GetPublicCollectionAlternatePaths(
	ctx context.Context,
	cfg *model.DocsHelpcenterConfig,
	collectionID string,
	publicID string,
) (map[string]string, error) {
	if cfg == nil || !docsHelpcenterMultilingualEnabled(cfg) {
		return map[string]string{}, nil
	}

	key := hcCacheKey("hc", "collection", "alternate-paths", cfg.WorkspaceID, collectionID)
	if s.hcCache != nil {
		var cached map[string]string
		if s.getHelpcenterCachedJSON(ctx, key, &cached) {
			return cached, nil
		}
	}

	translations, err := s.hcRepo.ListPublicCollectionTranslationsByCollection(ctx, collectionID)
	if err != nil {
		return nil, err
	}

	enabledLocales := make(map[string]struct{}, len(cfg.EnabledLocales))
	for _, locale := range cfg.EnabledLocales {
		normalized := strings.ToLower(strings.TrimSpace(locale))
		if normalized != "" {
			enabledLocales[normalized] = struct{}{}
		}
	}

	paths := make(map[string]string, len(translations))
	for _, translation := range translations {
		locale := strings.ToLower(strings.TrimSpace(translation.Locale))
		if _, enabled := enabledLocales[locale]; !enabled {
			continue
		}
		slug := stringValue(translation.Slug)
		if slug == "" {
			continue
		}
		paths[locale] = buildDocsHelpcenterCollectionCanonicalPath(cfg, locale, slug, publicID)
	}

	if s.hcCache != nil {
		s.setHelpcenterCachedJSON(
			ctx,
			key,
			paths,
			hcCacheTTL,
			HelpcenterCacheTagWorkspace(cfg.WorkspaceID),
		)
	}
	return paths, nil
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
	s.setHelpcenterCachedJSON(ctx, key, publicPathResolveCacheEntry{Target: target}, hcCacheTTL, HelpcenterCacheTagWorkspace(workspaceID))
	return target, nil
}

// InvalidateHelpcenterCacheForWorkspace drops every cached help-center entry
// associated with a workspace, then schedules the same invalidation again
// after hcReinvalidateDelay. Safe to call when the cache is nil.
func (s *DocsHelpcenterService) InvalidateHelpcenterCacheForWorkspace(ctx context.Context, workspaceID string) {
	if s.hcCache == nil || workspaceID == "" {
		return
	}

	tags := []string{HelpcenterCacheTagWorkspace(workspaceID)}
	cfg, err := s.hcRepo.GetConfig(ctx, workspaceID)
	if err != nil {
		slog.WarnContext(
			ctx,
			"hc cache: resolve identifiers for invalidation failed",
			"error", err,
			"workspace_id", workspaceID,
		)
	} else if cfg != nil {
		if cfg.Subdomain != "" {
			tags = append(tags, HelpcenterCacheTagIdentifier(cfg.Subdomain))
		}
		if cfg.CustomDomain != nil && strings.TrimSpace(*cfg.CustomDomain) != "" {
			tags = append(tags, HelpcenterCacheTagIdentifier(*cfg.CustomDomain))
		}
	}

	s.invalidateHelpcenterTags(ctx, workspaceID, tags)
	s.scheduleHelpcenterReinvalidation(workspaceID, tags)
}

func (s *DocsHelpcenterService) invalidateHelpcenterTags(ctx context.Context, workspaceID string, tags []string) {
	if err := s.hcCache.InvalidateTags(ctx, tags...); err != nil {
		slog.WarnContext(ctx, "hc cache: invalidate failed", "error", err, "workspace_id", workspaceID)
	}
}

// scheduleHelpcenterReinvalidation runs the invalidation once more after a
// delay. A read that loaded data before a write and stored it after the
// first invalidation would otherwise stay cached for the full TTL. Each
// write restarts the workspace's timer, so a burst of writes (an import)
// ends with one extra invalidation.
// shortcut: the timer lives in this process, so a pod that exits inside the
// delay skips it; move to a durable timer if that ever shows up as staleness.
func (s *DocsHelpcenterService) scheduleHelpcenterReinvalidation(workspaceID string, tags []string) {
	delay := s.hcReinvalidateDelay
	if delay <= 0 {
		delay = hcReinvalidateDelay
	}

	s.hcReinvalidateMu.Lock()
	defer s.hcReinvalidateMu.Unlock()
	if s.hcReinvalidateTimers == nil {
		s.hcReinvalidateTimers = make(map[string]*time.Timer)
	}
	if existing := s.hcReinvalidateTimers[workspaceID]; existing != nil {
		existing.Stop()
	}

	var timer *time.Timer
	timer = time.AfterFunc(delay, func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("hc cache: delayed invalidation panicked", "panic", r, "workspace_id", workspaceID)
			}
		}()
		s.hcReinvalidateMu.Lock()
		if s.hcReinvalidateTimers[workspaceID] == timer {
			delete(s.hcReinvalidateTimers, workspaceID)
		}
		s.hcReinvalidateMu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.invalidateHelpcenterTags(ctx, workspaceID, tags)
	})
	s.hcReinvalidateTimers[workspaceID] = timer
}

func (s *DocsHelpcenterService) getCachedPublicArticle(ctx context.Context, key, workspaceID string, fetch func() (*model.PublicArticleResponse, error)) (*model.PublicArticleResponse, error) {
	if s.hcCache == nil {
		return fetch()
	}

	var out model.PublicArticleResponse
	if s.getHelpcenterCachedJSON(ctx, key, &out) {
		return &out, nil
	}

	article, err := fetch()
	if err != nil || article == nil {
		return article, err
	}
	s.setHelpcenterCachedJSON(ctx, key, article, hcCacheTTL, HelpcenterCacheTagWorkspace(workspaceID))
	return article, nil
}

// RecordPublicArticleView counts one view of a published article. Article
// reads no longer count views because rendered pages are served from cache;
// the public page reports each view through this instead. A localized view
// with a resolved locale counts against that translation.
func (s *DocsHelpcenterService) RecordPublicArticleView(ctx context.Context, workspaceID, requestedLocale, articleKey string) error {
	var (
		article *model.PublicArticleResponse
		err     error
	)
	if requestedLocale != "" {
		article, err = s.GetPublicArticleByLocalizedCanonicalKey(ctx, workspaceID, requestedLocale, articleKey)
	} else {
		article, err = s.GetPublicArticleByCanonicalKey(ctx, workspaceID, articleKey)
	}
	if err != nil {
		return err
	}
	if article == nil || article.ID == "" {
		return ErrPublicArticleNotFound
	}
	if requestedLocale != "" && article.Locale != "" {
		return s.hcRepo.IncrementTranslatedViewCount(ctx, article.ID, article.Locale)
	}
	return s.hcRepo.IncrementViewCount(ctx, article.ID)
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
	encoded = append(encoded, base64.RawURLEncoding.EncodeToString([]byte(hcCacheNamespace())))
	for _, part := range parts {
		encoded = append(encoded, base64.RawURLEncoding.EncodeToString([]byte(part)))
	}
	return strings.Join(encoded, ":")
}

// HelpcenterCacheTagIdentifier returns the cache tag for a hosted subdomain or
// custom-domain identifier used by the public renderer.
func HelpcenterCacheTagIdentifier(identifier string) string {
	return "hc:host:" + strings.ToLower(strings.TrimSpace(identifier))
}
