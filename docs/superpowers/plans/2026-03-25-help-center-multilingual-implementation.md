# Help Center Multilingual Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship public-only multilingual Help Center support with one canonical internal docs tree, locale-specific public translations for spaces/collections/articles, per-locale publish state, locale-aware public URLs, locale-scoped search, and translation-management UI for admins.

**Architecture:** Keep `docs_spaces`, `docs_collections`, `docs_documents`, and canonical `docs_contents` as the internal source of truth. Add a public translation overlay owned by the help-center domain: locale settings on `DocsHelpcenterConfig`, translation tables for public spaces/collections/articles, and sync hooks that keep the default locale mirrored from the source while marking non-default locales as `needs_review` when source content changes. Public backend reads and the `help-center` SPA must become locale-aware end to end.

**Tech Stack:** Go 1.24, Chi, GORM/PostgreSQL, SQL migrations, React 19, TypeScript 5.9, TanStack Query, TanStack Router, TipTap JSON, Vite 7, Vitest.

---

## Inputs And Guardrails

- Spec: `docs/superpowers/specs/2026-03-25-helpcenter-multilingual-design.md`
- This plan supersedes older drafts:
  - `docs/superpowers/plans/2026-03-23-help-center-multilingual-plan.md`
  - `docs/superpowers/plans/2026-03-23-help-center-multilingual-implementation.md`
- V1 is **public Help Center only**. Do not make the internal docs editor generally multilingual.
- Default-locale public translations are **system-managed mirrors**, not a separate authoring branch.
- Enforce parent-before-child publish:
  - translated space required before translated collection publish
  - translated collection required before translated article publish
- Use safe public URLs in v1:
  - `/:locale`
  - `/:locale/:spaceSlug`
  - `/:locale/:spaceSlug/:collectionSlug`
  - `/:locale/:spaceSlug/:collectionSlug/:articleSlug`
- Fallback is a read-time behavior. Missing translations must still be obvious in admin.
- Keep `help-center/src/routeTree.gen.ts` generated. Do not edit it manually.

## File Map

### Backend

- Create: `server/migrations/059_helpcenter_multilingual.sql`
  - Add locale config fields to `docs_helpcenter_configs`
  - Create translation tables and indexes
  - Backfill default-locale translation rows from existing public content
- Create: `server/internal/model/docs_helpcenter_multilingual.go`
  - Translation model structs and DTOs
- Create: `server/internal/repository/docs_helpcenter_translation.go`
  - Locale registry and translation CRUD
  - Default-locale mirror upsert helpers
- Create: `server/internal/repository/docs_helpcenter_translation_test.go`
  - Repository tests for uniqueness, state transitions, and backfill-safe behavior
- Create: `server/internal/service/docs_helpcenter_translation.go`
  - Translation business rules, parent validation, source freshness, and publish workflows
- Create: `server/internal/service/docs_helpcenter_translation_test.go`
  - Service tests for default mirror sync, `needs_review`, publish validation, and fallback
- Create: `server/internal/handler/docs_helpcenter_translation.go`
  - Admin endpoints for locales and translations
- Create: `server/internal/handler/docs_helpcenter_public_locale_test.go`
  - Handler tests for locale-aware public reads and redirects
- Modify: `server/internal/model/docs.go`
  - Extend help-center config and public response DTOs where needed
- Modify: `server/internal/repository/docs_helpcenter.go`
  - Make public reads locale-aware
- Modify: `server/internal/repository/docs_search.go`
  - Add locale-scoped public search
- Modify: `server/internal/service/docs_helpcenter.go`
  - Route public read logic through locale-aware translation queries
- Modify: `server/internal/service/docs_content.go`
  - Trigger default-locale mirror refresh and mark non-default locales `needs_review`
- Modify: `server/internal/service/docs_document.go`
  - Trigger source sync when public-facing article metadata changes
- Modify: `server/internal/service/docs_space.go`
  - Trigger source sync for external-capable space changes
- Modify: `server/internal/service/docs_collection.go`
  - Trigger source sync for external collection changes
- Modify: `server/internal/router/router.go`
  - Register admin translation endpoints and locale-aware public endpoints
- Modify: `server/cmd/api/main.go`
  - Wire new repositories/services/handlers

### Frontend Admin App

- Create: `frontend/src/components/settings/helpcenter/HelpcenterLocalesCard.tsx`
  - Help-center locale settings editor
- Create: `frontend/src/components/settings/helpcenter/__tests__/HelpcenterLocalesCard.test.tsx`
  - Locale settings regression tests
- Create: `frontend/src/components/docs/helpcenter/TranslationsPanel.tsx`
  - Shared translations panel with locale rows and actions
- Create: `frontend/src/components/docs/helpcenter/TranslationStatusBadge.tsx`
  - Shared locale status UI
- Create: `frontend/src/components/docs/helpcenter/EditSpaceTranslationDialog.tsx`
  - Space translation editor
- Create: `frontend/src/components/docs/helpcenter/EditCollectionTranslationDialog.tsx`
  - Collection translation editor
- Create: `frontend/src/components/docs/helpcenter/EditArticleTranslationDialog.tsx`
  - Article translation editor
- Create: `frontend/src/components/docs/helpcenter/__tests__/TranslationsPanel.test.tsx`
  - Shared panel tests for missing/draft/published/needs_review rows
- Modify: `frontend/src/lib/docsTypes.ts`
  - Locale settings types, translation types, DTOs
- Modify: `frontend/src/lib/services/docsService.ts`
  - Locale settings and translation API adapters
- Modify: `frontend/src/hooks/queries/useDocs.ts`
  - Query/mutation hooks and invalidation for locales/translations
- Modify: `frontend/src/components/settings/HelpcenterTab.tsx`
  - Mount locale settings card
- Modify: `frontend/src/pages/docs/DocsSpaceDetail.tsx`
  - Space/collection translations UI
- Modify: `frontend/src/pages/docs/DocsDocumentDetail.tsx`
  - Article translations panel and editor entry points

### Public Help Center App

- Modify: `help-center/package.json`
  - Add test dependencies and `test` script if missing
- Create: `help-center/vitest.config.ts`
  - Help-center test config
- Create: `help-center/src/lib/locale.ts`
  - Locale path building, fallback helpers, switcher targets
- Create: `help-center/src/lib/__tests__/locale.test.ts`
  - Locale helper tests
- Create: `help-center/src/components/layout/LocaleSwitcher.tsx`
  - Public language switcher
- Create: `help-center/src/components/layout/__tests__/LocaleSwitcher.test.tsx`
  - Switcher behavior tests
- Create: `help-center/src/routes/$locale.tsx`
  - Locale layout/guard route
- Create: `help-center/src/routes/$locale/index.tsx`
  - Locale-prefixed homepage
- Create: `help-center/src/routes/$locale/$spaceSlug.tsx`
  - Locale-prefixed space layout
- Create: `help-center/src/routes/$locale/$spaceSlug/index.tsx`
  - Locale-prefixed space landing redirect
- Create: `help-center/src/routes/$locale/$spaceSlug/$collectionSlug/index.tsx`
  - Locale-prefixed collection page
- Create: `help-center/src/routes/$locale/$spaceSlug/$collectionSlug/$articleSlug.tsx`
  - Locale-prefixed article page
- Create: `help-center/src/routes/$locale/search.tsx`
  - Locale-prefixed search page
- Modify: `help-center/src/lib/types.ts`
  - Locale-aware config, nav, article, collection, and fallback metadata
- Modify: `help-center/src/lib/services.ts`
  - Locale-aware API calls
- Modify: `help-center/src/lib/queryKeys.ts`
  - Include locale in query keys
- Modify: `help-center/src/hooks/queries/index.ts`
  - Include locale in queries
- Modify: `help-center/src/contexts/DocsContext.tsx`
  - Expose active locale, enabled locales, and default locale
- Modify: `help-center/src/contexts/SpaceContext.tsx`
  - Include locale-aware navigation/pager behavior
- Modify: `help-center/src/components/layout/TopBar.tsx`
  - Render language switcher
- Modify: `help-center/src/components/layout/Sidebar.tsx`
  - Build locale-aware links
- Modify: `help-center/src/components/navigation/*`
  - Carry locale through nav and breadcrumbs
- Modify: `help-center/src/routes/index.tsx`
  - Redirect bare root to default locale
- Modify: `help-center/src/routes/search.tsx`
  - Redirect or retire legacy non-locale search route

## Data Model Decisions To Preserve

- Internal docs remain the canonical source tree and canonical content.
- Default-locale public translations are derived from source docs, not independently authored.
- Non-default locales own their public-facing:
  - title
  - description/excerpt
  - slug
  - TipTap JSON content
  - SEO title/description
  - publish state
- `missing` is derived by absence of a translation row.
- Public metrics are per locale translation row.
- Public fallback never makes untranslated locale URLs look like true translated pages.

## API Shapes

Use explicit admin DTOs and locale-aware public routes.

Suggested admin DTOs:

```go
type UpdateDocsHelpcenterLocalesRequest struct {
	DefaultLocale         string   `json:"default_locale"`
	EnabledLocales        []string `json:"enabled_locales"`
	ShowLanguageSwitcher  bool     `json:"show_language_switcher"`
	FallbackToDefault     bool     `json:"fallback_to_default_locale"`
}

type UpsertDocsHelpcenterSpaceTranslationRequest struct {
	Locale string  `json:"locale"`
	Name   string  `json:"name"`
	Slug   string  `json:"slug"`
	Status string  `json:"status"`
}

type UpsertDocsHelpcenterCollectionTranslationRequest struct {
	Locale      string  `json:"locale"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Slug        string  `json:"slug"`
	Status      string  `json:"status"`
}

type UpsertDocsHelpcenterArticleTranslationRequest struct {
	Locale         string          `json:"locale"`
	Title          string          `json:"title"`
	Slug           string          `json:"slug"`
	Excerpt        *string         `json:"excerpt"`
	Content        json.RawMessage `json:"content"`
	SEOtitle       *string         `json:"seo_title"`
	SEOdescription *string         `json:"seo_description"`
	Status         string          `json:"status"`
}
```

Suggested public route shape:

- `GET /hc/{subdomain}/config`
- `GET /hc/{subdomain}/{locale}/spaces`
- `GET /hc/{subdomain}/{locale}/spaces/{spaceSlug}/navigation`
- `GET /hc/{subdomain}/{locale}/spaces/{spaceSlug}/collections/{collectionSlug}`
- `GET /hc/{subdomain}/{locale}/spaces/{spaceSlug}/collections/{collectionSlug}/articles/{articleSlug}`
- `GET /hc/{subdomain}/{locale}/search`

Suggested admin route shape:

- `GET /api/docs/helpcenter/locales`
- `PUT /api/docs/helpcenter/locales`
- `GET /api/docs/helpcenter/spaces/{spaceId}/translations`
- `PUT /api/docs/helpcenter/spaces/{spaceId}/translations/{locale}`
- `POST /api/docs/helpcenter/spaces/{spaceId}/translations/{locale}/publish`
- `POST /api/docs/helpcenter/spaces/{spaceId}/translations/{locale}/unpublish`
- matching collection and document routes

### Task 1: Schema And Model Foundation

**Files:**
- Create: `server/migrations/059_helpcenter_multilingual.sql`
- Create: `server/internal/model/docs_helpcenter_multilingual.go`
- Modify: `server/internal/model/docs.go`
- Test: `server/internal/repository/docs_helpcenter_translation_test.go`

- [ ] **Step 1: Write the failing repository tests**

Add focused tests like:

```go
func TestDocsHelpcenterTranslationRepository_DefaultLocaleUniqueness(t *testing.T) {}
func TestDocsHelpcenterTranslationRepository_SpaceSlugUniquePerWorkspaceLocale(t *testing.T) {}
func TestDocsHelpcenterTranslationRepository_CollectionSlugUniquePerSpaceLocale(t *testing.T) {}
func TestDocsHelpcenterTranslationRepository_DefaultLocaleBackfillCreatesMirrorRows(t *testing.T) {}
```

- [ ] **Step 2: Run the repo test to verify the red state**

Run:

```bash
env GOCACHE=/tmp/go-build-hc-i18n go test ./internal/repository -run 'TestDocsHelpcenterTranslationRepository' -v
```

Expected:
- FAIL because translation tables/models do not exist yet

- [ ] **Step 3: Add the migration**

In `server/migrations/059_helpcenter_multilingual.sql`, add:

```sql
ALTER TABLE docs_helpcenter_configs
  ADD COLUMN IF NOT EXISTS default_locale text NOT NULL DEFAULT 'en',
  ADD COLUMN IF NOT EXISTS enabled_locales jsonb NOT NULL DEFAULT '["en"]'::jsonb,
  ADD COLUMN IF NOT EXISTS show_language_switcher boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS fallback_to_default_locale boolean NOT NULL DEFAULT true;
```

Create:

- `docs_helpcenter_space_translations`
- `docs_helpcenter_collection_translations`
- `docs_helpcenter_article_translations`

Backfill default-locale rows from:

- external-capable spaces
- collections in external-capable spaces
- externally published help-center articles

- [ ] **Step 4: Add model structs and DTOs**

Create `server/internal/model/docs_helpcenter_multilingual.go` with:

```go
type DocsHelpcenterSpaceTranslation struct {}
type DocsHelpcenterCollectionTranslation struct {}
type DocsHelpcenterArticleTranslation struct {}
type UpdateDocsHelpcenterLocalesRequest struct {}
type UpsertDocsHelpcenterArticleTranslationRequest struct {}
```

Keep new multilingual structs out of the already-large `docs.go` file when possible.

- [ ] **Step 5: Run the repo test to verify the schema/model foundation**

Run:

```bash
env GOCACHE=/tmp/go-build-hc-i18n go test ./internal/repository -run 'TestDocsHelpcenterTranslationRepository' -v
```

Expected:
- PASS with backfill and uniqueness coverage

- [ ] **Step 6: Commit**

```bash
git add server/migrations/059_helpcenter_multilingual.sql server/internal/model/docs.go server/internal/model/docs_helpcenter_multilingual.go server/internal/repository/docs_helpcenter_translation_test.go
git commit -m "feat(helpcenter): add multilingual translation schema"
```

### Task 2: Repository And Service Rules For Translations

**Files:**
- Create: `server/internal/repository/docs_helpcenter_translation.go`
- Create: `server/internal/service/docs_helpcenter_translation.go`
- Create: `server/internal/service/docs_helpcenter_translation_test.go`
- Modify: `server/cmd/api/main.go`

- [ ] **Step 1: Write the failing service tests**

Add tests like:

```go
func TestDocsHelpcenterTranslationService_UpsertDefaultLocaleMirror(t *testing.T) {}
func TestDocsHelpcenterTranslationService_SourceChangeMarksNonDefaultNeedsReview(t *testing.T) {}
func TestDocsHelpcenterTranslationService_PublishArticleRequiresTranslatedParents(t *testing.T) {}
func TestDocsHelpcenterTranslationService_FallbackResolvesDefaultLocaleWhenTranslationMissing(t *testing.T) {}
```

- [ ] **Step 2: Run the service test to verify the red state**

Run:

```bash
env GOCACHE=/tmp/go-build-hc-i18n go test ./internal/service -run 'TestDocsHelpcenterTranslationService' -v
```

Expected:
- FAIL because repository/service methods are missing

- [ ] **Step 3: Implement repository methods**

Implement focused methods for:

- locale config read/write
- list/get/upsert translation rows
- publish/unpublish locale rows
- resolve locale-specific slugs
- refresh default-locale mirror rows
- mark sibling locales `needs_review`

Keep read/write helpers small and translation-specific; do not keep adding more logic into `docs_helpcenter.go`.

- [ ] **Step 4: Implement service rules**

Implement rules for:

- default locale must be in enabled locales
- locale status transitions
- parent translation must be published before child publish
- default-locale mirrors are not manually edited
- fallback chooses default locale only when allowed

- [ ] **Step 5: Wire DI**

Register the new repository/service in `server/cmd/api/main.go` and pass it to docs/help-center handlers/services that need it.

- [ ] **Step 6: Run the service test to verify it passes**

Run:

```bash
env GOCACHE=/tmp/go-build-hc-i18n go test ./internal/service -run 'TestDocsHelpcenterTranslationService' -v
```

Expected:
- PASS with mirror sync and parent validation coverage

- [ ] **Step 7: Commit**

```bash
git add server/internal/repository/docs_helpcenter_translation.go server/internal/service/docs_helpcenter_translation.go server/internal/service/docs_helpcenter_translation_test.go server/cmd/api/main.go
git commit -m "feat(helpcenter): add translation service rules"
```

### Task 3: Admin APIs And Source Sync Hooks

**Files:**
- Create: `server/internal/handler/docs_helpcenter_translation.go`
- Modify: `server/internal/router/router.go`
- Modify: `server/internal/service/docs_helpcenter.go`
- Modify: `server/internal/service/docs_content.go`
- Modify: `server/internal/service/docs_document.go`
- Modify: `server/internal/service/docs_space.go`
- Modify: `server/internal/service/docs_collection.go`
- Test: `server/internal/handler/docs_helpcenter_public_locale_test.go`

- [ ] **Step 1: Write the failing handler tests**

Add tests for:

```go
func TestDocsHelpcenterTranslationHandler_UpdateLocales(t *testing.T) {}
func TestDocsHelpcenterTranslationHandler_UpsertArticleTranslation(t *testing.T) {}
func TestDocsHelpcenterTranslationHandler_PublishArticleTranslationRejectsMissingParents(t *testing.T) {}
```

- [ ] **Step 2: Run the handler test to verify the red state**

Run:

```bash
env GOCACHE=/tmp/go-build-hc-i18n go test ./internal/handler -run 'TestDocsHelpcenterTranslationHandler' -v
```

Expected:
- FAIL because routes and handlers are not registered

- [ ] **Step 3: Add admin translation handlers and routes**

Register locale and translation routes under `/api/docs/helpcenter/...`.

Split the new handler code into `docs_helpcenter_translation.go` instead of adding more complexity to the existing large `docs.go`.

- [ ] **Step 4: Add source-sync hooks**

When source content changes:

- `docs_content.go` refreshes the default-locale article translation mirror
- `docs_document.go` refreshes title/excerpt/public slug source fields and marks siblings `needs_review`
- `docs_space.go` and `docs_collection.go` refresh default-locale public name/slug mirrors when public-facing parents change

Do not trigger on purely internal-only fields that do not affect public content.

- [ ] **Step 5: Run targeted tests**

Run:

```bash
env GOCACHE=/tmp/go-build-hc-i18n go test ./internal/handler -run 'TestDocsHelpcenterTranslationHandler' -v
env GOCACHE=/tmp/go-build-hc-i18n go test ./internal/service -run 'TestDocsHelpcenterTranslationService' -v
```

Expected:
- PASS

- [ ] **Step 6: Commit**

```bash
git add server/internal/handler/docs_helpcenter_translation.go server/internal/router/router.go server/internal/service/docs_helpcenter.go server/internal/service/docs_content.go server/internal/service/docs_document.go server/internal/service/docs_space.go server/internal/service/docs_collection.go server/internal/handler/docs_helpcenter_public_locale_test.go
git commit -m "feat(helpcenter): add translation admin APIs and source sync"
```

### Task 4: Frontend Admin Locale Settings And Translation Panels

**Files:**
- Create: `frontend/src/components/settings/helpcenter/HelpcenterLocalesCard.tsx`
- Create: `frontend/src/components/settings/helpcenter/__tests__/HelpcenterLocalesCard.test.tsx`
- Create: `frontend/src/components/docs/helpcenter/TranslationsPanel.tsx`
- Create: `frontend/src/components/docs/helpcenter/TranslationStatusBadge.tsx`
- Create: `frontend/src/components/docs/helpcenter/EditSpaceTranslationDialog.tsx`
- Create: `frontend/src/components/docs/helpcenter/EditCollectionTranslationDialog.tsx`
- Create: `frontend/src/components/docs/helpcenter/EditArticleTranslationDialog.tsx`
- Create: `frontend/src/components/docs/helpcenter/__tests__/TranslationsPanel.test.tsx`
- Modify: `frontend/src/lib/docsTypes.ts`
- Modify: `frontend/src/lib/services/docsService.ts`
- Modify: `frontend/src/hooks/queries/useDocs.ts`
- Modify: `frontend/src/components/settings/HelpcenterTab.tsx`
- Modify: `frontend/src/pages/docs/DocsSpaceDetail.tsx`
- Modify: `frontend/src/pages/docs/DocsDocumentDetail.tsx`

- [ ] **Step 1: Write the failing frontend tests**

Add tests for:

```tsx
it('saves default locale and enabled locale settings')
it('renders locale rows with missing draft published and needs_review states')
it('disables publish when translated parent rows are not ready')
```

- [ ] **Step 2: Run the frontend tests to capture red**

Run:

```bash
cd frontend && npm exec vitest run \
  src/components/settings/helpcenter/__tests__/HelpcenterLocalesCard.test.tsx \
  src/components/docs/helpcenter/__tests__/TranslationsPanel.test.tsx
```

Expected:
- FAIL because locale settings and translations UI do not exist

- [ ] **Step 3: Add types, services, and query hooks**

Add typed APIs for:

- locale settings get/update
- list/get/upsert translations
- publish/unpublish translation
- mark reviewed

Keep invalidation targeted by workspace, space, collection, document, and locale.

- [ ] **Step 4: Add locale settings UI to Help Center settings**

Mount `HelpcenterLocalesCard` inside `frontend/src/components/settings/HelpcenterTab.tsx` near other help-center policy controls.

Required controls:

- default locale
- enabled locales
- switcher toggle
- fallback toggle

- [ ] **Step 5: Add translations panel to docs surfaces**

Use:

- `DocsSpaceDetail.tsx` for space and collection translation management
- `DocsDocumentDetail.tsx` for article translation management

Rows must show:

- locale
- status
- freshness
- actions: add/edit/publish/unpublish/mark reviewed

- [ ] **Step 6: Run the frontend tests to verify green**

Run:

```bash
cd frontend && npm exec vitest run \
  src/components/settings/helpcenter/__tests__/HelpcenterLocalesCard.test.tsx \
  src/components/docs/helpcenter/__tests__/TranslationsPanel.test.tsx
cd frontend && npm exec tsc --noEmit
```

Expected:
- PASS
- TypeScript clean

- [ ] **Step 7: Commit**

```bash
git add frontend/src/lib/docsTypes.ts frontend/src/lib/services/docsService.ts frontend/src/hooks/queries/useDocs.ts frontend/src/components/settings/helpcenter/HelpcenterLocalesCard.tsx frontend/src/components/settings/helpcenter/__tests__/HelpcenterLocalesCard.test.tsx frontend/src/components/docs/helpcenter frontend/src/components/settings/HelpcenterTab.tsx frontend/src/pages/docs/DocsSpaceDetail.tsx frontend/src/pages/docs/DocsDocumentDetail.tsx
git commit -m "feat(helpcenter): add admin locale and translation workflow"
```

### Task 5: Locale-Aware Public Backend Reads, Search, And Redirects

**Files:**
- Modify: `server/internal/repository/docs_helpcenter.go`
- Modify: `server/internal/repository/docs_search.go`
- Modify: `server/internal/service/docs_helpcenter.go`
- Modify: `server/internal/handler/docs.go`
- Modify: `server/internal/router/router.go`
- Test: `server/internal/service/docs_helpcenter_translation_test.go`
- Test: `server/internal/handler/docs_helpcenter_public_locale_test.go`

- [ ] **Step 1: Write the failing public locale tests**

Add tests for:

```go
func TestDocsHelpcenterPublicLocale_ListSpacesUsesLocaleTranslations(t *testing.T) {}
func TestDocsHelpcenterPublicLocale_GetArticleFallsBackToDefaultLocale(t *testing.T) {}
func TestDocsHelpcenterPublicLocale_DisabledLocaleRedirectsToDefault(t *testing.T) {}
func TestDocsHelpcenterPublicLocale_SearchOnlyReturnsRequestedLocale(t *testing.T) {}
```

- [ ] **Step 2: Run the backend tests to capture red**

Run:

```bash
env GOCACHE=/tmp/go-build-hc-i18n go test ./internal/service -run 'TestDocsHelpcenterPublicLocale' -v
env GOCACHE=/tmp/go-build-hc-i18n go test ./internal/handler -run 'TestDocsHelpcenterPublicLocale' -v
```

Expected:
- FAIL because public reads are still single-locale

- [ ] **Step 3: Refactor public reads to translation-first**

Change `server/internal/repository/docs_helpcenter.go` so:

- space lists come from `docs_helpcenter_space_translations`
- nav comes from translated collections/articles
- collection pages come from translated collections + translated articles
- article pages come from `docs_helpcenter_article_translations`
- locale fallback returns the default locale variant only through explicit service logic

- [ ] **Step 4: Add locale-scoped public search**

Update `server/internal/repository/docs_search.go` so public search filters by:

- locale
- published translation status
- only translation content, not canonical content

- [ ] **Step 5: Register locale-aware public routes**

Add backend routes for locale-prefixed public endpoints and legacy redirect helpers.

- [ ] **Step 6: Run the backend tests to verify green**

Run:

```bash
env GOCACHE=/tmp/go-build-hc-i18n go test ./internal/service -run 'TestDocsHelpcenterPublicLocale|TestDocsHelpcenterTranslationService' -v
env GOCACHE=/tmp/go-build-hc-i18n go test ./internal/handler -run 'TestDocsHelpcenterPublicLocale' -v
env GOCACHE=/tmp/go-build-hc-i18n go build ./cmd/api
```

Expected:
- PASS
- API builds cleanly

- [ ] **Step 7: Commit**

```bash
git add server/internal/repository/docs_helpcenter.go server/internal/repository/docs_search.go server/internal/service/docs_helpcenter.go server/internal/handler/docs.go server/internal/router/router.go server/internal/handler/docs_helpcenter_public_locale_test.go
git commit -m "feat(helpcenter): make public reads locale-aware"
```

### Task 6: Locale-Aware Public SPA Routing And Switcher

**Files:**
- Modify: `help-center/package.json`
- Create: `help-center/vitest.config.ts`
- Create: `help-center/src/lib/locale.ts`
- Create: `help-center/src/lib/__tests__/locale.test.ts`
- Create: `help-center/src/components/layout/LocaleSwitcher.tsx`
- Create: `help-center/src/components/layout/__tests__/LocaleSwitcher.test.tsx`
- Create: `help-center/src/routes/$locale.tsx`
- Create: `help-center/src/routes/$locale/index.tsx`
- Create: `help-center/src/routes/$locale/$spaceSlug.tsx`
- Create: `help-center/src/routes/$locale/$spaceSlug/index.tsx`
- Create: `help-center/src/routes/$locale/$spaceSlug/$collectionSlug/index.tsx`
- Create: `help-center/src/routes/$locale/$spaceSlug/$collectionSlug/$articleSlug.tsx`
- Create: `help-center/src/routes/$locale/search.tsx`
- Modify: `help-center/src/lib/types.ts`
- Modify: `help-center/src/lib/services.ts`
- Modify: `help-center/src/lib/queryKeys.ts`
- Modify: `help-center/src/hooks/queries/index.ts`
- Modify: `help-center/src/contexts/DocsContext.tsx`
- Modify: `help-center/src/contexts/SpaceContext.tsx`
- Modify: `help-center/src/components/layout/TopBar.tsx`
- Modify: `help-center/src/components/layout/Sidebar.tsx`
- Modify: `help-center/src/components/navigation/Breadcrumbs.tsx`
- Modify: `help-center/src/components/navigation/MobileNav.tsx`
- Modify: `help-center/src/routes/index.tsx`
- Modify: `help-center/src/routes/search.tsx`

- [ ] **Step 1: Add failing help-center tests**

Add tests for:

```ts
it('builds locale-aware article and collection paths')
it('switches to the matching locale path when translation exists')
it('falls back to default locale path when translation is missing')
```

- [ ] **Step 2: Run the help-center tests to capture red**

Run:

```bash
cd help-center && npm exec vitest run \
  src/lib/__tests__/locale.test.ts \
  src/components/layout/__tests__/LocaleSwitcher.test.tsx
```

Expected:
- FAIL because locale helpers and switcher do not exist

- [ ] **Step 3: Add locale test infrastructure**

Update `help-center/package.json` with:

```json
{
  "scripts": {
    "test": "vitest run"
  }
}
```

Add dev dependencies:

- `vitest`
- `jsdom`
- `@testing-library/react`

- [ ] **Step 4: Implement locale helpers and switcher**

Create pure helpers for:

- route building
- default-locale redirects
- switcher target resolution
- fallback path behavior

Mount `LocaleSwitcher` in `TopBar`.

- [ ] **Step 5: Convert the app to locale-prefixed routes**

Create locale-prefixed routes and retire legacy non-locale pages behind redirect behavior.

Make sure every query and link includes `locale`.

- [ ] **Step 6: Regenerate the route tree and verify the app**

Run:

```bash
cd help-center && npm exec vitest run \
  src/lib/__tests__/locale.test.ts \
  src/components/layout/__tests__/LocaleSwitcher.test.tsx
cd help-center && npm run build
```

Expected:
- PASS
- Vite build regenerates `routeTree.gen.ts`

- [ ] **Step 7: Commit**

```bash
git add help-center/package.json help-center/vitest.config.ts help-center/src/lib/locale.ts help-center/src/lib/__tests__/locale.test.ts help-center/src/components/layout/LocaleSwitcher.tsx help-center/src/components/layout/__tests__/LocaleSwitcher.test.tsx help-center/src/routes help-center/src/lib/types.ts help-center/src/lib/services.ts help-center/src/lib/queryKeys.ts help-center/src/hooks/queries/index.ts help-center/src/contexts/DocsContext.tsx help-center/src/contexts/SpaceContext.tsx help-center/src/components/layout/TopBar.tsx help-center/src/components/layout/Sidebar.tsx help-center/src/components/navigation/Breadcrumbs.tsx help-center/src/components/navigation/MobileNav.tsx
git commit -m "feat(helpcenter): add locale-aware public app routes"
```

### Task 7: Final Migration Verification, Backfill Audit, And Rollout Checks

**Files:**
- Modify: `docs/superpowers/specs/2026-03-25-helpcenter-multilingual-design.md` only if implementation realities require follow-up notes
- Create: `server/internal/repository/docs_helpcenter_backfill_test.go`
- Create: `frontend/src/components/docs/helpcenter/__tests__/translationFallbackNotice.test.tsx` if a fallback note ships in v1

- [ ] **Step 1: Write backfill verification tests**

Add tests covering:

```go
func TestDocsHelpcenterMultilingualBackfill_CreatesDefaultLocaleRowsForExistingPublishedContent(t *testing.T) {}
func TestDocsHelpcenterMultilingualBackfill_DoesNotCreateRowsForNonPublicContent(t *testing.T) {}
```

- [ ] **Step 2: Run the backfill test**

Run:

```bash
env GOCACHE=/tmp/go-build-hc-i18n go test ./internal/repository -run 'TestDocsHelpcenterMultilingualBackfill' -v
```

Expected:
- PASS

- [ ] **Step 3: Run full targeted verification**

Run:

```bash
cd server && env GOCACHE=/tmp/go-build-hc-i18n go test ./internal/repository -run 'TestDocsHelpcenter(TranslationRepository|MultilingualBackfill)' -v
cd server && env GOCACHE=/tmp/go-build-hc-i18n go test ./internal/service -run 'TestDocsHelpcenter(TranslationService|PublicLocale)' -v
cd server && env GOCACHE=/tmp/go-build-hc-i18n go test ./internal/handler -run 'TestDocsHelpcenter(TranslationHandler|PublicLocale)' -v
cd server && env GOCACHE=/tmp/go-build-hc-i18n go build ./cmd/api
cd frontend && npm exec vitest run \
  src/components/settings/helpcenter/__tests__/HelpcenterLocalesCard.test.tsx \
  src/components/docs/helpcenter/__tests__/TranslationsPanel.test.tsx
cd frontend && npm exec tsc --noEmit
cd help-center && npm exec vitest run \
  src/lib/__tests__/locale.test.ts \
  src/components/layout/__tests__/LocaleSwitcher.test.tsx
cd help-center && npm run build
```

Expected:
- all targeted backend tests pass
- frontend typecheck passes
- help-center tests pass
- both apps build cleanly

- [ ] **Step 4: Smoke-check the end-to-end workflow manually**

Manual checks:

- enable `en` + `fr` in help-center settings
- create/publish default-locale mirrored public content
- add a French space translation, collection translation, and article translation
- confirm `fr` article publish is blocked until translated parents exist
- load `/fr/...` public route
- confirm switcher works
- confirm missing translation falls back to `/en/...`
- confirm `/fr/search?q=...` only returns French published results

- [ ] **Step 5: Commit the rollout-hardening work**

```bash
git add server/internal/repository/docs_helpcenter_backfill_test.go frontend/src/components/docs/helpcenter/__tests__/translationFallbackNotice.test.tsx
git commit -m "test(helpcenter): verify multilingual backfill and rollout"
```
