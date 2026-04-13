# Collection PublicID Implementation Plan

**Date**: 2026-04-13
**Status**: Approved, ready for implementation
**Priority**: High — production change

## Overview

Add an 8-char hex PublicID to collections, mirroring the existing article PublicID pattern. Collection URLs change from `/c/{slug}` to `/c/{slug}-{publicID}`. Lookup uses PublicID (slug is cosmetic). Old slug-only URLs still resolve via fallback.

## Reference: Article PublicID Pattern

- Model: `DocsHelpcenterArticle.PublicID` (8-char hex, globally unique index)
- Generation: `crypto/rand` -> 4 random bytes -> hex encode -> retry up to 16 times on collision
- Key building: `buildDocsHelpcenterArticleKey(slug, publicID)` in `server/internal/service/docs_public_paths.go`
- Key parsing: `parseDocsHelpcenterArticleKey(key)` — finds last dash, validates 8-char hex suffix
- Frontend: `buildArticleKey(slug, publicId)` in `help-center/src/lib/articleKey.ts`
- Nav response: `PublicNavArticle` includes `PublicID` field

## Implementation Steps

### Step 1: Backend Model & Migration

**Files:**
- `server/internal/model/docs.go` — add `PublicID` to `DocsCollection`
- New migration file

**Changes:**
- Add `PublicID string` field to `DocsCollection` struct with `gorm:"uniqueIndex"`
- Migration:
  - `ALTER TABLE docs_collections ADD COLUMN IF NOT EXISTS public_id TEXT NOT NULL DEFAULT ''`
  - Backfill: generate random 8-char hex for all existing collections (use PL/pgSQL loop)
  - `CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_collections_public_id ON docs_collections (public_id) WHERE public_id != ''`
- Must be idempotent (`IF NOT EXISTS`, `WHERE public_id = ''` for backfill)

### Step 2: Backend Repository

**Files:**
- `server/internal/repository/docs_collection.go`
- `server/internal/repository/docs_helpcenter.go`

**Changes:**
- Add `GetByPublicID(ctx, publicID) (*model.DocsCollection, error)` to `DocsCollectionRepository`
- Add `CollectionPublicIDExists(ctx, publicID, excludeID string) (bool, error)` for collision check
- Update `ListSpaceNavigation` query to SELECT `c.public_id` and populate `PublicNavCollection.PublicID`
- Update localized navigation builder to include `public_id`
- Update `ListWidgetCollections` to include `public_id` in `WidgetHelpCollection`

### Step 3: Backend Service — Key Building/Parsing & Generation

**Files:**
- `server/internal/service/docs_public_paths.go`
- `server/internal/service/docs_collection.go`

**Changes:**
- Add `buildDocsHelpcenterCollectionKey(slug, publicID string) string` — same pattern as article
- Add `parseDocsHelpcenterCollectionKey(key string) (slug, publicID string, ok bool)` — same last-dash extraction
- Update `buildDocsHelpcenterCollectionCanonicalPath(cfg, locale, collectionSlug, publicID)` — add publicID param, build key
- Add `ensureUniqueCollectionPublicID(ctx, collectionID)` to DocsCollectionService or DocsHelpcenterService
- Generate PublicID in `DocsCollectionService.Create()` before persisting
- **All 6+ call sites** of `buildDocsHelpcenterCollectionCanonicalPath` must be updated

### Step 4: Backend Service — Public Lookup (Non-Localized)

**Files:**
- `server/internal/service/docs_helpcenter.go`

**Changes:**
- Update `GetPublicCollection(workspaceID, collectionKey)`:
  1. Parse `collectionKey` with `parseDocsHelpcenterCollectionKey`
  2. If publicID found: lookup collection by publicID
  3. If parse fails (old slug-only URL): fall back to `GetBySlug`

### Step 4a: Backend Service — Public Lookup (Localized) — CRITICAL

**Files:**
- `server/internal/service/docs_helpcenter.go`

**Changes:**
- `resolvePublicCollectionTranslationByCanonicalSlug(ctx, cfg, workspaceID, requestedLocale, collectionKey)`:
  1. Parse `collectionKey` to extract publicID
  2. If publicID found: lookup base collection by publicID -> get collectionID -> lookup translation for collectionID + locale
  3. If parse fails: fall back to current slug-only translation lookup
- Same pattern for `resolvePublicCollectionTranslationBySlug` (space-scoped variant)
- Affects:
  - `GetPublicLocalizedCollection` (line ~1507)
  - `GetPublicLocalizedCollectionByCanonicalPath` (line ~1593)
  - `GetPublicArticleByLocalizedCanonicalPath` (line ~1687 — uses collection slug to resolve context)
  - `resolveDynamicPublicPath` (redirect resolver, case "c" blocks at lines ~2392, ~2443)

### Step 5: Backend Handler

**Files:**
- `server/internal/handler/docs.go`

**Changes:**
- `PublicGetCollectionPage`: `collectionSlug` URL param now contains a collectionKey. Pass to service which parses it.
- No route changes needed — `/c/{collectionSlug}` stays the same
- Verify all three code paths in the handler (space-scoped, localized, non-localized) pass the full key

### Step 6: Navigation Response Models

**Files:**
- `server/internal/model/docs.go`
- `server/internal/model/support_inbox.go`

**Changes:**
- Add `PublicID string json:"public_id"` to `PublicNavCollection`
- Add `PublicID string json:"public_id"` to `PublicNavBreadcrumbEntry`
- Add `PublicID string json:"public_id"` to `WidgetHelpCollection`
- Verify `PublicSearchResultResponse.CollectionSlug` includes publicID (or add separate field)

### Step 7: Redirect Resolver

**Files:**
- `server/internal/service/docs_helpcenter.go` — `resolveDynamicPublicPath`

**Changes:**
- `case "c"` (line ~2392): parse `collectionSlug` as key, extract publicID, include in resolved canonical path
- Fallback `len(segments) == 1` (line ~2443): same treatment
- All `buildDocsHelpcenterCollectionCanonicalPath` calls in resolver must pass publicID

### Step 8: Widget

**Files:**
- `server/internal/service/support_inbox_widget.go`

**Changes:**
- `ListWidgetHelpArticles`: parse incoming `collectionSlug` with `parseDocsHelpcenterCollectionKey`
  - If publicID found: lookup by publicID
  - If parse fails: fall back to `GetBySlug`
- Widget collections response includes `public_id`

### Step 9: Help Center Frontend — Key Building

**Files:**
- `help-center/src/lib/collectionKey.ts` (NEW file)
- `help-center/src/lib/locale.ts`

**Changes:**
- Create `buildCollectionKey(slug, publicId)` and `parseCollectionKey(key)` (mirrors `articleKey.ts`)
- Update `buildLocaleCollectionPath(locale, collectionSlug, publicId)` — build key from slug+publicId
- Update `buildCanonicalCollectionPath(multilingualEnabled, locale, collectionSlug, publicId)` — same
- Update ALL call sites (~8+):
  - `NavTree.tsx` (lines ~80, 217, 371)
  - `Breadcrumbs.tsx` (line ~61)
  - `CollectionRouteView.tsx` (lines ~364, 471)
  - `LocalizedHomePage.tsx` (line ~192)
  - `locale.ts` lines 56, 87-94

### Step 10: Frontend Types & API

**Files:**
- `help-center/src/lib/types.ts`
- `help-center/src/hooks/queries/index.ts`

**Changes:**
- Add `public_id: string` to `NavItem` type (collection variant)
- API calls already use `collectionSlug` param — value just changes to include publicID
- `ArticlePagerLink` needs collection `publicId` for pager link construction

### Step 11: Frontend Route Components

**Files:**
- `help-center/src/routes/c/$collectionSlug.tsx`
- `help-center/src/routes/$locale/c/$collectionSlug.tsx`

**Changes:**
- No route file changes needed — `$collectionSlug` captures the full key including publicID
- The redirect in non-multilingual -> multilingual mode must pass the full key

### Step 12: Import Service

**Files:**
- `server/internal/service/docs_import.go`

**Changes:**
- Collections created via `DocsCollectionService.Create()` which will auto-generate PublicIDs after Step 3
- Existing redirect `TargetCollectionSlug` stores raw slugs — backward compat via slug fallback
- No additional import changes beyond Step 3

### Step 13: Tests

**Files:**
- `server/internal/service/docs_public_paths_test.go` (or new)
- `help-center/src/lib/__tests__/locale.test.ts`

**Changes:**
- Unit tests for `buildDocsHelpcenterCollectionKey` / `parseDocsHelpcenterCollectionKey`
- Unit tests for `ensureUniqueCollectionPublicID`
- Update frontend locale tests for new collection path format
- Manual test: collection page loads, nav links work, old URLs resolve

## Backward Compatibility

- Old `/c/{slug-only}` URLs: parse fails (no valid 8-char hex suffix) -> fall back to slug-only lookup
- Every service function that receives a collection slug MUST implement parse-then-fallback
- Functions needing fallback: `GetPublicCollection`, `GetPublicLocalizedCollection`, `GetPublicLocalizedCollectionByCanonicalPath`, `resolvePublicCollectionTranslationByCanonicalSlug`, `resolvePublicCollectionTranslationBySlug`, `resolveDynamicPublicPath`, `ListWidgetHelpArticles`
- Redirect resolver handles old imported paths
- Existing redirect records with slug-only `TargetCollectionSlug` work via fallback

## Unique Index Scope

PublicID is **globally unique** (like articles) — not workspace or space scoped. This makes lookup clean: extract publicID from any URL and find the collection without needing additional context.

## Risk Assessment

- **High impact**: changes public URLs for all help centers
- **Mitigated by**: backward compat fallback for all old URLs
- **Migration risk**: low — backfill is additive (new column, no data loss)
- **Collision risk**: negligible — 4 billion possible values, 16 retries
