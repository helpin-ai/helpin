# Multilingual Help Center Implementation Plan

> Source review, 2026-09-17

Historical multilingual design. Locale routing and translation services now exist
in [locale helpers](../../help-center/src/lib/locale.ts) and
[translation service](../../server/internal/service/docs_helpcenter_translation.go).
The public application now uses [TanStack Start SSR](../../help-center/vite.config.ts),
not the earlier SPA-only delivery described by these plans. PublicID-based routes
and publication snapshots further evolved the original model. This source review
confirms those implementation boundaries, not every proposed acceptance item or
live locale configuration.

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship complete multilingual support for the public Help Center with locale-aware content, URLs, switching, redirects, SEO, search, admin tooling, widget parity, and migration of all existing public docs.

**Architecture:** Keep the existing canonical docs tree (`docs_spaces`, `docs_collections`, `docs_documents`, `docs_contents`) for workspace-internal docs management, then add a full public-localization layer for Help Center content. Public reads move entirely to locale-specific localization rows. The default locale is backfilled from current public docs and kept in sync from the internal docs editor. Public page delivery becomes locale-aware and SEO-safe, with explicit per-locale routing, redirects, metadata, search, and alternate-language discovery.

**Tech Stack:** Go 1.24, PostgreSQL + GORM, existing docs/helpcenter repositories and services, React 19 + TanStack Router in `help-center`, existing docs admin/editor UI in `frontend`, existing TipTap JSON storage and renderer.

---

## Implementation principles

- We are not building an MVP. Every read path, write path, redirect, search path, SEO surface, and public rendering path must be locale-aware before this is considered done.
- Public Help Center reads must come from one consistent localization contract. Avoid split-brain behavior where some endpoints read canonical docs tables and others read translation tables.
- The public site must not expose untranslated locale URLs as if they were real translated pages.
- Old public URLs must keep working via deterministic redirects.
- Locale-aware search must ship with the feature. English-only search is not acceptable once public multilingual docs exist.
- Widget/help surfaces that expose Help Center articles must share the same locale contract.

---

## Final product decisions locked for implementation

- [ ] Use a **single Help Center per workspace/domain** with **supported locales**.
- [ ] Keep **one canonical space/collection/document graph**.
- [ ] Add **localized public variants** for spaces, collections, documents, and help-center copy.
- [ ] Use **locale-prefixed public URLs**: `/{locale}`, `/{locale}/{spaceSlug}`, `/{locale}/{spaceSlug}/{articleSlug}`.
- [ ] Keep **space slug stable** in v1 of this feature. Localize space names, not space slugs.
- [ ] Localize **collection slugs** and **article slugs**.
- [ ] If a locale-specific article URL is requested and that translation is not published, **302 redirect** to the default-locale equivalent URL.
- [ ] Search only published localized content for the requested locale.
- [ ] Add **manual language switcher** and **browser-language resolution**.
- [ ] Public SEO must include **canonical**, **alternate hreflang**, **x-default**, and **locale-aware sitemap**.
- [ ] View/helpful/not-helpful counts are per localization, not globally shared.
- [ ] Support widget Help Center browsing and article display must be locale-aware.

---

### Task 1: Create the multilingual data model

**Files:**
- Create: `server/migrations/052_docs_helpcenter_locales.sql`
- Create: `server/migrations/053_docs_helpcenter_localizations.sql`
- Create: `server/migrations/054_docs_helpcenter_localization_backfill.sql`
- Create: `server/internal/model/docs_localization.go`
- Modify: `server/internal/model/docs.go`
- Test: `server/internal/repository/docs_localization_test.go`

- [ ] **Step 1: Add locale registry schema**

Create `docs_helpcenter_locales` with:
- `id uuid primary key`
- `workspace_id uuid not null`
- `locale_code text not null`
- `is_default boolean not null default false`
- `is_enabled boolean not null default true`
- `is_public_visible boolean not null default false`
- `position integer not null default 0`
- `autodetect_enabled boolean not null default true`
- `created_at timestamptz not null default now()`
- `updated_at timestamptz not null default now()`

Add unique constraints:
- `(workspace_id, locale_code)` unique
- partial unique index for one default locale per workspace

- [ ] **Step 2: Add localized help-center config schema**

Create `docs_helpcenter_config_localizations` with:
- `id uuid primary key`
- `helpcenter_config_id uuid not null`
- `locale_code text not null`
- `seo_title text`
- `seo_description text`
- `search_placeholder text`
- `homepage_config jsonb not null default '{}'`
- `header_links jsonb not null default '[]'`
- `footer_config jsonb not null default '{}'`
- `created_at timestamptz not null default now()`
- `updated_at timestamptz not null default now()`

Unique:
- `(helpcenter_config_id, locale_code)` unique

- [ ] **Step 3: Add localized space/collection/document schema**

Create `docs_space_localizations` with:
- `space_id uuid not null`
- `locale_code text not null`
- `name text not null`
- `created_at`, `updated_at`

Unique:
- `(space_id, locale_code)` unique

Create `docs_collection_localizations` with:
- `collection_id uuid not null`
- `locale_code text not null`
- `name text not null`
- `description text`
- `slug text not null`
- `created_at`, `updated_at`

Unique:
- `(collection_id, locale_code)` unique
- `(locale_code, slug, collection workspace)` uniqueness via indexed join-safe fields or denormalized `workspace_id`

Create `docs_document_localizations` with:
- `id uuid primary key`
- `document_id uuid not null`
- `workspace_id uuid not null`
- `locale_code text not null`
- `title text not null`
- `excerpt text`
- `icon text`
- `content jsonb`
- `content_html text`
- `content_text text`
- `word_count integer not null default 0`
- `slug text not null`
- `seo_title text`
- `seo_description text`
- `status text not null default 'draft'`
- `translation_status text not null default 'missing'`
- `source_locale_code text`
- `source_version_id uuid`
- `last_synced_from_source_at timestamptz`
- `public_published_at timestamptz`
- `view_count integer not null default 0`
- `helpful_count integer not null default 0`
- `not_helpful_count integer not null default 0`
- `created_at`, `updated_at`

Unique:
- `(document_id, locale_code)` unique
- `(workspace_id, locale_code, slug)` unique where `public_published_at is not null`

- [ ] **Step 4: Add localization version history**

Create `docs_document_localization_versions` with:
- `id uuid primary key`
- `document_localization_id uuid not null`
- `document_id uuid not null`
- `locale_code text not null`
- `content jsonb`
- `content_text text`
- `word_count integer not null`
- `title text not null`
- `excerpt text`
- `seo_title text`
- `seo_description text`
- `slug text not null`
- `version_type text not null`
- `snapshot_label text`
- `created_by uuid not null`
- `created_at timestamptz not null default now()`

Add index:
- `(document_localization_id, created_at desc)`

- [ ] **Step 5: Add locale-aware redirects support**

Modify redirects schema by migration to add:
- `source_locale_code text`
- `target_locale_code text`

Backfill existing redirects to default locale later in Task 4.

- [ ] **Step 6: Add Go model types**

In `server/internal/model/docs_localization.go`, define:
- `DocsHelpcenterLocale`
- `DocsHelpcenterConfigLocalization`
- `DocsSpaceLocalization`
- `DocsCollectionLocalization`
- `DocsDocumentLocalization`
- `DocsDocumentLocalizationVersion`

Add DTOs:
- `CreateDocsLocaleRequest`
- `UpdateDocsLocaleRequest`
- `SaveDocsDocumentLocalizationRequest`
- `PublishDocsDocumentLocalizationRequest`
- `UpsertDocsCollectionLocalizationRequest`
- `UpsertDocsSpaceLocalizationRequest`

- [ ] **Step 7: Add repository tests for constraints and defaults**

Run: `cd server && env GOCACHE=/root/teampulse/.gocache go test ./internal/repository -run 'TestDocsLocalization' -v`
Expected: PASS with coverage for default locale uniqueness, localized slug uniqueness, and translation status defaults.

- [ ] **Step 8: Commit schema foundation**

Run:
```bash
git add server/migrations/052_docs_helpcenter_locales.sql server/migrations/053_docs_helpcenter_localizations.sql server/migrations/054_docs_helpcenter_localization_backfill.sql server/internal/model/docs.go server/internal/model/docs_localization.go server/internal/repository/docs_localization_test.go
git commit -m "feat: add multilingual help center schema"
```

---

### Task 2: Implement localization repositories and services

**Files:**
- Create: `server/internal/repository/docs_localization.go`
- Create: `server/internal/service/docs_localization.go`
- Modify: `server/internal/repository/docs_content.go`
- Modify: `server/internal/repository/docs_search.go`
- Modify: `server/internal/service/docs_content.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/temporal-worker/main.go`
- Test: `server/internal/service/docs_localization_test.go`

- [ ] **Step 1: Build repository methods**

Implement repository methods for:
- locale registry CRUD
- config localization CRUD
- space localization read/write
- collection localization read/write
- document localization get/list/save/publish/unpublish
- localization version create/list/get
- localized slug lookup
- equivalent-translation lookup by `document_id + locale_code`

- [ ] **Step 2: Build service-layer rules**

Implement service rules for:
- default locale required before public localization can be created
- public-visible locales must be enabled
- translation status transitions:
  - `missing -> draft`
  - `draft -> published`
  - source update marks sibling localizations `outdated`
- slug uniqueness per locale
- locale save writes version snapshot

- [ ] **Step 3: Keep canonical docs content as internal source, but make public reads localization-first**

Rule:
- `docs_contents` remains the internal docs content source
- `docs_document_localizations` becomes the public Help Center source
- saving default-locale public content mirrors the current canonical doc title/excerpt/content into the default localization row

Implement helper methods to:
- upsert default locale localization from canonical doc/content
- mark sibling localizations outdated when canonical/default changes

- [ ] **Step 4: Extend plain-text extraction**

Update localized save path so `content_text` and `word_count` are computed for localization rows using the same TipTap/plain-text logic currently in `docs_content.go`.

- [ ] **Step 5: Wire DI**

Register new repository/service constructors in:
- `server/cmd/api/main.go`
- `server/cmd/temporal-worker/main.go`

- [ ] **Step 6: Test service rules**

Run: `cd server && env GOCACHE=/root/teampulse/.gocache go test ./internal/service -run 'TestDocsLocalizationService' -v`
Expected: PASS for default locale sync, sibling outdated tracking, and publish/unpublish rules.

- [ ] **Step 7: Commit localization service layer**

Run:
```bash
git add server/internal/repository/docs_localization.go server/internal/service/docs_localization.go server/internal/repository/docs_content.go server/internal/repository/docs_search.go server/internal/service/docs_content.go server/cmd/api/main.go server/cmd/temporal-worker/main.go server/internal/service/docs_localization_test.go
git commit -m "feat: add docs localization repositories and services"
```

---

### Task 3: Replace public Help Center read paths with locale-aware reads

**Files:**
- Modify: `server/internal/handler/docs.go`
- Modify: `server/internal/service/docs_helpcenter.go`
- Modify: `server/internal/repository/docs_helpcenter.go`
- Modify: `server/internal/model/docs.go`
- Test: `server/internal/handler/docs_public_multilingual_test.go`
- Test: `server/internal/service/docs_helpcenter_multilingual_test.go`

- [ ] **Step 1: Add locale to public endpoint contract**

Update public handlers to accept locale explicitly:
- `GET /hc/{subdomain}/config?locale=...`
- `GET /hc/{subdomain}/spaces?locale=...`
- `GET /hc/{subdomain}/spaces/{spaceSlug}/navigation?locale=...`
- `GET /hc/{subdomain}/spaces/{spaceSlug}/articles/{articleSlug}?locale=...`
- `GET /hc/{subdomain}/collections/{collectionSlug}/articles/{articleSlug}?locale=...`
- `GET /hc/{subdomain}/search?q=...&locale=...`

Add a lightweight locale resolution endpoint:
- `GET /hc/{subdomain}/locale/resolve`

Response includes:
- `resolved_locale`
- `default_locale`
- `supported_locales`

- [ ] **Step 2: Make config public response locale-aware**

Public config response must include:
- default locale
- public-visible locales
- localized chrome for requested locale
- fallback locale if requested locale copy is missing

- [ ] **Step 3: Make nav/article/collection reads localization-first**

Replace current reads from `docs_helpcenter_articles` with localized reads from:
- `docs_space_localizations`
- `docs_collection_localizations`
- `docs_document_localizations`

Public article reads must return:
- localized title
- localized slug
- localized excerpt
- localized SEO
- localized content HTML
- localized counts
- locale metadata
- alternate locale mappings

- [ ] **Step 4: Implement missing translation redirect rule**

When locale-specific article/collection content is not published:
- resolve the default locale equivalent
- return redirect metadata or direct redirect response

Do not return default-language content as if it were localized content.

- [ ] **Step 5: Keep old non-locale public API behavior temporarily only as redirect/resolver**

Non-locale public routes must:
- resolve old path
- redirect to default-locale canonical path
- never remain the long-term article content source

- [ ] **Step 6: Add tests**

Run:
```bash
cd server && env GOCACHE=/root/teampulse/.gocache go test ./internal/handler -run 'TestDocsPublic.*Locale' -v
cd server && env GOCACHE=/root/teampulse/.gocache go test ./internal/service -run 'TestDocsHelpcenter.*Locale' -v
```
Expected: PASS for locale resolution, localized reads, missing translation redirects, and alternate locale mapping.

- [ ] **Step 7: Commit locale-aware public API**

Run:
```bash
git add server/internal/handler/docs.go server/internal/service/docs_helpcenter.go server/internal/repository/docs_helpcenter.go server/internal/model/docs.go server/internal/handler/docs_public_multilingual_test.go server/internal/service/docs_helpcenter_multilingual_test.go
git commit -m "feat: add locale-aware public help center APIs"
```

---

### Task 4: Migrate and backfill existing public content

**Files:**
- Modify: `server/migrations/054_docs_helpcenter_localization_backfill.sql`
- Create: `server/internal/service/docs_localization_backfill.go`
- Create: `server/internal/service/docs_localization_backfill_test.go`
- Modify: `server/internal/service/docs_import.go`
- Modify: `server/internal/model/docs_import.go`

- [ ] **Step 1: Backfill default locale rows from existing public docs**

Migration/backfill must copy:
- `docs_documents.title` -> default locale title
- `docs_documents.excerpt` -> default locale excerpt
- `docs_documents.icon` -> default locale icon
- `docs_contents.content` -> default locale content
- `docs_contents.content_text` -> default locale content_text
- `docs_contents.word_count` -> default locale word_count
- `docs_helpcenter_articles.slug` -> default locale slug
- `docs_helpcenter_articles.seo_title` -> default locale SEO title
- `docs_helpcenter_articles.seo_description` -> default locale SEO description
- `docs_helpcenter_articles.public_published_at` -> default locale publication
- feedback/view counts -> default locale metrics

- [ ] **Step 2: Create one default locale row per workspace**

Backfill default locale as:
- workspace-configured default if present in future
- otherwise `en`

Every workspace with a Help Center config must end with exactly one default locale row.

- [ ] **Step 3: Backfill old URLs into locale-prefixed redirects**

Create redirects from:
- `/{spaceSlug}/{articleSlug}` -> `/{defaultLocale}/{spaceSlug}/{articleSlug}`
- `/{collectionSlug}/{articleSlug}` -> `/{defaultLocale}/{collectionSlug}/{articleSlug}`
- imported legacy redirects -> locale-prefixed target paths

- [ ] **Step 4: Make import jobs locale-aware**

Extend docs import jobs and service flow to accept:
- source locale for imported docs
- default locale target for imported content

Imported docs should create default locale localization rows directly rather than relying on post-import backfill.

- [ ] **Step 5: Add verification command**

Run:
```bash
cd server && env GOCACHE=/root/teampulse/.gocache go test ./internal/service -run 'TestDocsLocalizationBackfill' -v
```
Expected: PASS verifying exact copy of current public metadata into default locale rows and old URL redirect coverage.

- [ ] **Step 6: Commit backfill layer**

Run:
```bash
git add server/migrations/054_docs_helpcenter_localization_backfill.sql server/internal/service/docs_localization_backfill.go server/internal/service/docs_localization_backfill_test.go server/internal/service/docs_import.go server/internal/model/docs_import.go
git commit -m "feat: backfill default locale help center content"
```

---

### Task 5: Implement locale-aware public rendering and SEO shell

**Files:**
- Create: `server/internal/service/helpcenter_html.go`
- Create: `server/internal/handler/helpcenter_pages.go`
- Modify: `server/internal/router/router.go`
- Modify: `help-center/src/main.tsx`
- Modify: `help-center/src/routes/__root.tsx`
- Modify: `help-center/src/routes/index.tsx`
- Modify: `help-center/src/routes/$spaceSlug.tsx`
- Modify: `help-center/src/routes/$spaceSlug/$articleSlug.tsx`
- Modify: `help-center/src/lib/types.ts`
- Modify: `help-center/src/lib/services.ts`
- Modify: `help-center/src/lib/utils.ts`
- Modify: `help-center/src/components/article/ArticleShell.tsx`
- Modify: `help-center/src/components/layout/TopBar.tsx`
- Test: `server/internal/handler/helpcenter_pages_test.go`
- Test: `help-center/src/routes/__tests__/locale-routing.test.tsx`

- [ ] **Step 1: Add locale-aware public page shell**

Implement backend-rendered HTML shell for public page requests that injects:
- locale-specific `<title>`
- locale-specific meta description
- canonical URL
- alternate `hreflang` links
- `x-default`
- locale preload payload

This shell should mount the existing `help-center` SPA bundle, not replace it with a new frontend stack.

- [ ] **Step 2: Move public route contract to locale paths**

Add public routes:
- `/`
- `/{locale}`
- `/{locale}/{spaceSlug}`
- `/{locale}/{spaceSlug}/{articleSlug}`
- `/{locale}/{collectionSlug}/{articleSlug}`

Rules:
- `/` resolves locale and redirects
- localized article routes are canonical
- non-localized legacy routes redirect

- [ ] **Step 3: Add root locale resolution**

Resolution order:
1. user-selected locale cookie
2. supported locale match from `Accept-Language`
3. default locale

Response:
- 302 redirect to localized path

- [ ] **Step 4: Update help-center SPA context**

Add locale to docs context and all queries.

Every data fetch in the public SPA must pass locale.

- [ ] **Step 5: Add public language switcher**

Add a header switcher that:
- lists public-visible locales
- navigates to equivalent localized article when available
- falls back to default-locale equivalent otherwise
- stores explicit choice in cookie/localStorage

- [ ] **Step 6: Add alternate locale payload**

Article responses must include alternate language routes so the switcher and head tags do not need client-side guesswork.

- [ ] **Step 7: Add tests**

Run:
```bash
cd server && env GOCACHE=/root/teampulse/.gocache go test ./internal/handler -run 'TestHelpcenterPages' -v
cd help-center && npm test -- --runInBand locale-routing
```
Expected: PASS for locale redirect, canonical localized route rendering, and switcher route resolution.

- [ ] **Step 8: Commit rendering layer**

Run:
```bash
git add server/internal/service/helpcenter_html.go server/internal/handler/helpcenter_pages.go server/internal/router/router.go help-center/src/main.tsx help-center/src/routes/__root.tsx help-center/src/routes/index.tsx help-center/src/routes/$spaceSlug.tsx help-center/src/routes/$spaceSlug/$articleSlug.tsx help-center/src/lib/types.ts help-center/src/lib/services.ts help-center/src/lib/utils.ts help-center/src/components/article/ArticleShell.tsx help-center/src/components/layout/TopBar.tsx server/internal/handler/helpcenter_pages_test.go help-center/src/routes/__tests__/locale-routing.test.tsx
git commit -m "feat: add locale-aware help center routing and rendering"
```

---

### Task 6: Implement locale-aware search, sitemap, and SEO metadata

**Files:**
- Modify: `server/internal/repository/docs_search.go`
- Modify: `server/internal/service/docs_search.go`
- Modify: `server/internal/handler/docs.go`
- Create: `server/internal/service/helpcenter_sitemap.go`
- Create: `server/internal/handler/helpcenter_sitemap.go`
- Create: `server/internal/service/helpcenter_sitemap_test.go`
- Modify: `help-center/src/hooks/useDocumentTitle.ts`
- Modify: `help-center/src/components/article/ArticleShell.tsx`

- [ ] **Step 1: Move public search to localization rows**

Public search must query `docs_document_localizations` instead of canonical docs content.

Filter by:
- workspace
- locale
- published status
- optional space

- [ ] **Step 2: Add locale-specific text-search config**

Map locale codes to Postgres configs:
- `en`, `en-US`, `en-GB` -> `english`
- `fr` -> `french`
- `de` -> `german`
- `es` -> `spanish`
- `pt`, `pt-BR` -> `portuguese`
- default fallback -> `simple`

- [ ] **Step 3: Add localized sitemap**

Expose sitemap output that includes:
- localized URLs
- canonical URL per locale
- alternate language references

- [ ] **Step 4: Add per-locale metadata response contract**

Article/collection/home responses must expose the exact metadata needed for:
- document title
- social title/description
- canonical
- alternate links

- [ ] **Step 5: Add tests**

Run:
```bash
cd server && env GOCACHE=/root/teampulse/.gocache go test ./internal/repository -run 'TestDocsPublicSearch.*Locale' -v
cd server && env GOCACHE=/root/teampulse/.gocache go test ./internal/service -run 'TestHelpcenterSitemap' -v
```
Expected: PASS for locale-specific search matches and sitemap alternate generation.

- [ ] **Step 6: Commit search/SEO**

Run:
```bash
git add server/internal/repository/docs_search.go server/internal/service/docs_search.go server/internal/handler/docs.go server/internal/service/helpcenter_sitemap.go server/internal/handler/helpcenter_sitemap.go server/internal/service/helpcenter_sitemap_test.go help-center/src/hooks/useDocumentTitle.ts help-center/src/components/article/ArticleShell.tsx
git commit -m "feat: add multilingual help center search and seo"
```

---

### Task 7: Build admin locale settings and translation workflow

**Files:**
- Modify: `frontend/src/lib/docsTypes.ts`
- Modify: `frontend/src/lib/services/docsService.ts`
- Modify: `frontend/src/hooks/queries/useDocs.ts`
- Modify: `frontend/src/lib/queryKeys.ts`
- Modify: `frontend/src/pages/docs/DocsDocumentDetail.tsx`
- Modify: `frontend/src/components/docs/ExternalPublishPanel.tsx`
- Modify: `frontend/src/pages/Settings.tsx`
- Create: `frontend/src/components/settings/HelpCenterLocalesTab.tsx`
- Create: `frontend/src/components/docs/DocsTranslationTabs.tsx`
- Create: `frontend/src/components/docs/DocsTranslationStatusBadge.tsx`
- Create: `frontend/src/components/docs/DocsLocalePublishPanel.tsx`
- Test: `frontend/src/components/docs/__tests__/DocsTranslationTabs.test.tsx`
- Test: `frontend/src/components/settings/__tests__/HelpCenterLocalesTab.test.tsx`

- [ ] **Step 1: Add admin locale settings UI**

Settings must allow:
- add/remove supported locales
- mark default locale
- reorder locales
- toggle public visibility
- toggle browser autodetect eligibility

- [ ] **Step 2: Add per-document translation workflow**

Document detail must support:
- locale tabs or locale dropdown
- save localized title/excerpt/content/SEO
- publish/unpublish per locale
- visible translation status badge
- outdated translation indicator

- [ ] **Step 3: Replace single external publish panel**

`ExternalPublishPanel` should become a locale matrix, not one global slug/publish toggle.

It must show for each locale:
- slug
- translation status
- publish state
- last updated
- source locale reference

- [ ] **Step 4: Keep internal docs editing stable**

Internal docs behavior must still work for non-external spaces.

Rules:
- no locale UI for purely internal spaces unless later expanded intentionally
- external-capable spaces expose translation UI

- [ ] **Step 5: Mark sibling locales outdated on default-locale changes**

The admin UI must show which translations are stale immediately after a source-locale edit.

- [ ] **Step 6: Add tests**

Run:
```bash
cd frontend && npx vitest run src/components/docs/__tests__/DocsTranslationTabs.test.tsx src/components/settings/__tests__/HelpCenterLocalesTab.test.tsx
cd frontend && ./node_modules/.bin/tsc -p tsconfig.json --noEmit --incremental false
```
Expected: PASS for translation switching, locale settings updates, and typed API coverage.

- [ ] **Step 7: Commit admin/editor UX**

Run:
```bash
git add frontend/src/lib/docsTypes.ts frontend/src/lib/services/docsService.ts frontend/src/hooks/queries/useDocs.ts frontend/src/lib/queryKeys.ts frontend/src/pages/docs/DocsDocumentDetail.tsx frontend/src/components/docs/ExternalPublishPanel.tsx frontend/src/pages/Settings.tsx frontend/src/components/settings/HelpCenterLocalesTab.tsx frontend/src/components/docs/DocsTranslationTabs.tsx frontend/src/components/docs/DocsTranslationStatusBadge.tsx frontend/src/components/docs/DocsLocalePublishPanel.tsx frontend/src/components/docs/__tests__/DocsTranslationTabs.test.tsx frontend/src/components/settings/__tests__/HelpCenterLocalesTab.test.tsx
git commit -m "feat: add multilingual help center admin workflow"
```

---

### Task 8: Add widget/help-surface locale parity

**Files:**
- Modify: `packages/widget-core/src/components/helpApi.ts`
- Modify: `packages/widget-core/src/components/HelpArticleView.tsx`
- Modify: `packages/widget-core/src/components/HelpCollectionView.tsx`
- Modify: `packages/widget-core/src/types.ts`
- Modify: `server/internal/service/support_inbox_widget.go`
- Modify: `server/internal/repository/docs_helpcenter.go`
- Test: `packages/widget-core/src/__tests__/helpApi.test.ts`
- Test: `server/internal/service/support_inbox_widget_test.go`

- [ ] **Step 1: Add locale parameter to widget help APIs**

Widget help article/collection reads must accept locale and use the same public localization rules.

- [ ] **Step 2: Choose widget locale resolution**

Resolution order:
1. widget-provided locale if present
2. session locale if present
3. browser locale
4. help-center default locale

- [ ] **Step 3: Keep widget article slugs locale-consistent**

If widget links out to the public Help Center, those links must use the locale-aware canonical URL.

- [ ] **Step 4: Add tests**

Run:
```bash
cd packages/widget-core && npm test -- --runInBand helpApi
cd server && env GOCACHE=/root/teampulse/.gocache go test ./internal/service -run 'TestSupportInbox.*Help.*Locale' -v
```
Expected: PASS for locale selection and localized widget article fetching.

- [ ] **Step 5: Commit widget parity**

Run:
```bash
git add packages/widget-core/src/components/helpApi.ts packages/widget-core/src/components/HelpArticleView.tsx packages/widget-core/src/components/HelpCollectionView.tsx packages/widget-core/src/types.ts server/internal/service/support_inbox_widget.go server/internal/repository/docs_helpcenter.go packages/widget-core/src/__tests__/helpApi.test.ts server/internal/service/support_inbox_widget_test.go
git commit -m "feat: add locale-aware help widget docs"
```

---

### Task 9: Add end-to-end redirect, fallback, and publishing safety

**Files:**
- Modify: `server/internal/repository/docs_redirect.go`
- Modify: `server/internal/service/docs_helpcenter.go`
- Modify: `server/internal/handler/docs.go`
- Create: `server/internal/service/docs_redirect_multilingual_test.go`
- Create: `help-center/src/components/layout/LocaleSwitcher.tsx`
- Test: `help-center/src/components/__tests__/LocaleSwitcher.test.tsx`

- [ ] **Step 1: Guarantee legacy URL preservation**

Support:
- old non-locale public URLs
- imported alias paths
- localized slug changes
- language switching between equivalents

- [ ] **Step 2: Add explicit locale-aware slug change redirects**

If article slug changes in `fr`, only `fr` old paths should redirect to new `fr` path.

Default locale slug changes must preserve old default-locale route and old non-locale route.

- [ ] **Step 3: Add locale switcher no-dead-end rule**

When equivalent translation does not exist:
- switcher sends user to default-locale equivalent
- show a subtle notice if desired in UI, but never 404

- [ ] **Step 4: Add tests**

Run:
```bash
cd server && env GOCACHE=/root/teampulse/.gocache go test ./internal/service -run 'TestDocsRedirect.*Locale' -v
cd help-center && npm test -- --runInBand LocaleSwitcher
```
Expected: PASS for locale-aware redirect chains and language-switch fallback behavior.

- [ ] **Step 5: Commit redirect hardening**

Run:
```bash
git add server/internal/repository/docs_redirect.go server/internal/service/docs_helpcenter.go server/internal/handler/docs.go server/internal/service/docs_redirect_multilingual_test.go help-center/src/components/layout/LocaleSwitcher.tsx help-center/src/components/__tests__/LocaleSwitcher.test.tsx
git commit -m "feat: harden multilingual help center redirects"
```

---

### Task 10: Full verification, migration QA, and release checklist

**Files:**
- Create: `docs/superpowers/checklists/help-center-multilingual-qa.md`
- Create: `server/internal/handler/docs_public_multilingual_e2e_test.go`
- Create: `help-center/src/routes/__tests__/multilingual-helpcenter.e2e.test.tsx`

- [ ] **Step 1: Run full backend test suite**

Run:
```bash
cd server && env GOCACHE=/root/teampulse/.gocache go test ./...
```
Expected: PASS

- [ ] **Step 2: Run full frontend docs admin suite**

Run:
```bash
cd frontend && ./node_modules/.bin/tsc -p tsconfig.json --noEmit --incremental false
cd frontend && NODE_OPTIONS=--max-old-space-size=4096 npx vite build
```
Expected: PASS

- [ ] **Step 3: Run full public help-center suite**

Run:
```bash
cd help-center && ./node_modules/.bin/tsc -p tsconfig.app.json --noEmit
cd help-center && NODE_OPTIONS=--max-old-space-size=4096 npx vite build
```
Expected: PASS

- [ ] **Step 4: Run multilingual scenario tests**

Must cover:
- default locale page load
- browser locale redirect
- explicit locale switch
- untranslated locale redirect to default equivalent
- localized article search
- locale-specific slug changes
- old non-locale URL redirect
- alternate locale tags
- widget localized article load
- feedback counts per locale

- [ ] **Step 5: Run migration verification against a copy of production-like data**

Verify:
- every workspace with public docs gets one default locale
- every existing public article has one default localization row
- old public URLs redirect correctly
- no slug collisions in localized tables

- [ ] **Step 6: Manual QA matrix**

Write and execute checklist for:
- desktop/mobile
- custom domain and helpin subdomain
- English + one Latin-script locale + one non-Latin locale if supported
- collection page, article page, homepage, search, widget article view

- [ ] **Step 7: Final release commit**

Run:
```bash
git add docs/superpowers/checklists/help-center-multilingual-qa.md server/internal/handler/docs_public_multilingual_e2e_test.go help-center/src/routes/__tests__/multilingual-helpcenter.e2e.test.tsx
git commit -m "test: verify multilingual help center end to end"
```

---

## Acceptance criteria

- [ ] One Help Center supports multiple locales without duplicating spaces/collections/documents.
- [ ] Every public page has a locale-aware canonical URL.
- [ ] Every localized article can expose all alternate published locales.
- [ ] Old non-locale URLs redirect safely to default-locale equivalents.
- [ ] Search is locale-aware and does not leak unrelated locales by default.
- [ ] Untranslated locale URLs do not render wrong-language content at that locale URL.
- [ ] The public language switcher always lands on a valid page.
- [ ] Admins can manage locales, translation state, per-locale slugs, per-locale publish state, and outdated translations.
- [ ] Default locale content is fully backfilled from existing public docs.
- [ ] Widget/help surfaces respect locale too.
- [ ] Public article metadata is SEO-safe with canonical, alternate `hreflang`, and sitemap support.
- [ ] Full backend, frontend, and help-center builds/tests pass.

---

## Risks that must be actively tested

- Slug collisions after localized backfill
- Redirect loops between non-locale and locale-aware routes
- Search indexing wrong locale content
- Default locale sync drifting from canonical docs content
- Missing alternate locale mappings in switcher or head tags
- Widget locale mismatches
- Partial translations showing in public nav/search before publish
- Locale-specific counts being accidentally shared globally
- Locale-specific page metadata not present in initial HTML shell

---

## Execution note

This plan intentionally front-loads schema and public-read contract work before admin UX. That order is mandatory. If the data model and public route contract are not correct first, the editor/admin UI will lock in the wrong assumptions and create rework.

Plan complete and saved to `docs/plans/2026-03-23-help-center-multilingual-implementation.md`. Ready to execute?
