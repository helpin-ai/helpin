# Remove Help Center Slug Uniqueness Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove collection and article slug uniqueness from the help center while preserving stable public URLs, redirects, in-app fetches, and widget embedded article behavior through PublicIDs.

**Architecture:** Treat `public_id` as the durable public identity for articles and collections. Slugs become display-only URL text. Slug-only legacy paths continue to work only when they resolve unambiguously or through explicit redirects; all canonical URLs, widget opens, embedded article APIs, and redirect targets use PublicID-backed references.

**Tech Stack:** Go 1.24, GORM, PostgreSQL dbmigrate SQL, Chi routes, React/Vite help-center app, packages/sdk-js, packages/widget-core, Vitest, Go tests.

---

## Product Direction

Help Scout's Beacon embedded articles model opens articles by article ID, not by slug. Their HTML attributes accept an `ARTICLE_ID`, and their JS API path is ID-oriented. Helpin should follow the same product contract:

- Public URLs stay SEO-friendly: `/articles/{slug}-{publicID}` and `/c/{slug}-{publicID}`.
- Embedded/in-app/widget article fetches should accept a bare article PublicID.
- Collection/article slugs may duplicate freely because they are no longer identity.
- Space slugs remain unique for now because spaces do not have PublicIDs yet and public navigation still resolves by space slug.

## File Structure

Primary backend changes:

- Modify `server/internal/service/docs_public_paths.go`: parse article and collection references as either `{slug}-{publicID}` or bare `{publicID}`.
- Modify `server/internal/service/docs_helpcenter.go`: resolve canonical articles/collections by PublicID first, make slug-only fallbacks ambiguity-safe, and stop uniquing source slugs.
- Modify `server/internal/service/docs_collection.go`: stop rejecting duplicate collection slugs.
- Modify `server/internal/service/docs_helpcenter_translation.go`: stop uniquing collection/article translation slugs; keep space translation slug uniqueness.
- Modify `server/internal/repository/docs_helpcenter.go`: add exact-one slug lookup helpers and PublicID-first lookup helpers for every public slug-only `First()` path, including `GetPublicArticleBySlug`, `GetPublicArticleByCollectionSlug`, and `GetPublicCollectionBySlug`.
- Modify `server/internal/repository/docs_collection.go`: keep `GetByPublicID`; remove or stop using `SlugTakenInWorkspace`; remove or replace `GetBySlug` with an ambiguity-safe lookup for legacy callers.
- Modify `server/internal/repository/docs_helpcenter_translation.go`: remove collection/article slug-exists checks; keep space slug-exists.
- Modify `server/internal/repository/docs_helpcenter_publication.go`: remove article publication slug uniqueness assumptions.
- Modify `server/internal/model/docs_helpcenter_multilingual.go`: remove slug `uniqueIndex` tags from collection/article translation slugs.
- Modify `server/internal/model/docs_helpcenter_publication.go`: change article publication slug unique index tag to non-unique lookup index.
- Modify `server/internal/model/docs_redirect.go`: persist canonical target paths or PublicID target fields.
- Modify `server/internal/service/docs_import.go`: update HelpScout import redirect target mapping so it does not rely on globally unique collection/article slugs.
- Create `server/internal/dbmigrate/sql/202604140716_remove_helpcenter_slug_uniqueness.sql`: drop unique slug indexes and add non-unique lookup indexes.

Primary frontend/package changes:

- Modify `help-center/src/lib/articleKey.ts` and tests: parse bare PublicID.
- Modify `help-center/src/lib/collectionKey.ts` and tests: parse bare PublicID.
- Modify `packages/widget-core/src/components/helpApi.ts`: rename arguments from slug to ref where applicable and allow bare PublicID.
- Modify `packages/widget-core/src/components/ChatWindow.tsx`: keep `openArticleRequest.articleKey` as the canonical field.
- Modify `packages/widget-core/src/components/helpTree.ts`: rename `findHelpCollectionBySlug` / `helpCollectionAncestorPath` inputs to collection refs and match bare PublicIDs.
- Modify `packages/sdk-js/src/core/widget.ts`, `packages/sdk-js/src/core/client.ts`, and `packages/sdk-js/src/core/types.ts`: document and type `openArticle(articleId)` as PublicID-first.
- Add SDK DOM attribute support in `packages/sdk-js/src/index.ts` or a focused helper file for Help Scout-style embedded article links.

---

### Task 1: Public Reference Parsing

**Files:**
- Modify: `server/internal/service/docs_public_paths.go`
- Test: `server/internal/service/docs_public_paths_test.go`
- Modify: `help-center/src/lib/articleKey.ts`
- Modify: `help-center/src/lib/collectionKey.ts`
- Test: `help-center/src/lib/__tests__/collectionKey.test.ts`
- Test: add or update `help-center/src/lib/__tests__/articleKey.test.ts`

- [ ] **Step 1: Write failing Go parser tests**

Add tests that prove:

```go
slug, publicID, ok := parseDocsHelpcenterArticleKey("abc123ef")
// want slug == "", publicID == "abc123ef", ok == true

slug, publicID, ok = parseDocsHelpcenterCollectionKey("def456ab")
// want slug == "", publicID == "def456ab", ok == true
```

Also keep existing `{slug}-{publicID}` expectations unchanged.

- [ ] **Step 2: Run Go parser tests and verify failure**

Run:

```bash
cd server
go test ./internal/service -run 'TestDocsHelpcenter.*Key' -count=1
```

Expected: tests for bare PublicID fail.

- [ ] **Step 3: Implement bare PublicID parsing**

In `docs_public_paths.go`, update both parse functions:

- Normalize the full key first.
- If the full key is exactly an 8-char hex PublicID, return `slug=""`, `publicID`, `ok=true`.
- Otherwise use the existing last-dash parsing.

Do not change `buildDocsHelpcenterArticleKey` or `buildDocsHelpcenterCollectionKey`; canonical URLs should still prefer slug plus PublicID.

- [ ] **Step 4: Write failing frontend parser tests**

Add equivalent TypeScript tests:

```ts
expect(parseArticleKey('abc123ef')).toEqual({ slug: '', publicId: 'abc123ef' })
expect(parseCollectionKey('def456ab')).toEqual({ slug: '', publicId: 'def456ab' })
```

- [ ] **Step 5: Implement frontend parser updates**

Update `articleKey.ts` and `collectionKey.ts` to accept bare PublicIDs while preserving current build behavior.

- [ ] **Step 6: Verify**

Run:

```bash
cd server
go test ./internal/service -run 'TestDocsHelpcenter.*Key|TestDocsPublicPaths' -count=1
cd ../help-center
npm test -- articleKey collectionKey
```

Expected: parser tests pass.

- [ ] **Step 7: Commit**

```bash
git add server/internal/service/docs_public_paths.go server/internal/service/docs_public_paths_test.go help-center/src/lib/articleKey.ts help-center/src/lib/collectionKey.ts help-center/src/lib/__tests__
git commit -m "feat: accept bare help center public ids"
```

---

### Task 2: PublicID-Backed Redirect Targets

**Files:**
- Modify: `server/internal/model/docs_redirect.go`
- Modify: `server/internal/service/docs_helpcenter.go`
- Modify: `server/internal/repository/docs_redirect.go`
- Create: `server/internal/dbmigrate/sql/202604140715_add_docs_redirect_target_paths.sql`
- Modify: `frontend/src/lib/services/docsRedirectService.ts`
- Modify: `frontend/src/lib/docsRedirectPaths.ts`
- Modify: `frontend/src/components/settings/RedirectsTab.tsx`
- Test: `server/internal/repository/docs_redirect_test.go`
- Test: `server/internal/service/docs_helpcenter_translation_test.go`

- [ ] **Step 1: Write failing redirect model/service tests**

Cover:

- Creating a manual redirect to `/c/getting-started-abc123ef` stores a canonical `target_path`.
- Resolving a redirect target uses `target_path` when present and never depends on collection slug uniqueness.
- Legacy rows with only `target_collection_slug` and `target_article_slug` still hydrate to a canonical PublicID path.

- [ ] **Step 2: Add migration for canonical redirect targets**

Create migration:

```sql
ALTER TABLE docs_redirects
  ADD COLUMN IF NOT EXISTS target_path TEXT;

CREATE INDEX IF NOT EXISTS idx_docs_redirects_ws_target_path
  ON docs_redirects (workspace_id, target_path);
```

Do not drop legacy target slug columns yet.

- [ ] **Step 3: Update model and repository**

Change `DocsRedirect.TargetPath` from computed-only to persisted:

```go
TargetPath *string `json:"target_path,omitempty"`
```

Keep `TargetCollectionSlug` and `TargetArticleSlug` for old rows and compatibility.

- [ ] **Step 4: Update service write paths**

For manual, import, collection rename, article move, and article slug redirects:

- Build canonical PublicID paths with `buildDocsHelpcenterCollectionCanonicalPath` and `buildDocsHelpcenterArticleCanonicalPath`.
- Store `target_path` whenever target collection/article rows are known.
- Continue filling legacy slug fields for older UI/API consumers.

- [ ] **Step 5: Update redirect resolution**

In `resolvePublicPathTarget`, if a redirect has `TargetPath`, return or recursively resolve that path first. Only fall back to legacy slug fields if `TargetPath` is empty.

- [ ] **Step 6: Update redirect UI**

Allow manual target entry to be a canonical path containing PublicIDs. Picker mode should submit the collection/article PublicID-backed `target_path`.

- [ ] **Step 7: Verify**

Run:

```bash
cd server
go test ./internal/repository -run DocsRedirect -count=1
go test ./internal/service -run 'Redirect|ResolvePublicPath|CollectionRedirect' -count=1
cd ../frontend
npm test -- RedirectsTab docsRedirectPaths
```

Expected: redirect tests pass and legacy redirect rows remain readable.

- [ ] **Step 8: Commit**

```bash
git add server/internal/model/docs_redirect.go server/internal/service/docs_helpcenter.go server/internal/repository/docs_redirect.go server/internal/dbmigrate/sql/202604140715_add_docs_redirect_target_paths.sql frontend/src/lib/services/docsRedirectService.ts frontend/src/lib/docsRedirectPaths.ts frontend/src/components/settings/RedirectsTab.tsx
git commit -m "feat: store public id backed redirect targets"
```

---

### Task 3: Help Scout-Style Embedded Article IDs

**Files:**
- Modify: `packages/sdk-js/src/core/widget.ts`
- Modify: `packages/sdk-js/src/core/client.ts`
- Modify: `packages/sdk-js/src/core/types.ts`
- Modify: `packages/sdk-js/src/index.ts`
- Modify: `packages/widget-core/src/components/helpApi.ts`
- Modify: `packages/widget-core/src/components/ChatWindow.tsx`
- Test: `packages/sdk-js/test/unit/core/widget.test.ts`
- Test: add `packages/sdk-js/test/unit/core/embedded-articles.test.ts`
- Test: `packages/widget-core/src/__tests__/HelpArticleView.test.tsx`

- [ ] **Step 1: Write failing SDK tests**

Cover:

```ts
helpin('openArticle', 'abc123ef')
```

opens the widget article view with `articleKey === 'abc123ef'`.

Also cover DOM attributes modeled after Help Scout:

```html
<a href="#" data-helpin-article="abc123ef">Open</a>
<a href="#" data-helpin-article-inline="abc123ef">Open inline</a>
<a href="#" data-helpin-article-sidebar="abc123ef">Open sidebar</a>
<a href="#" data-helpin-article-modal="abc123ef">Open modal</a>
```

For v1, all four attributes may open the standard widget article view. Keep the attribute names reserved for future display modes.

- [ ] **Step 2: Update SDK naming**

Rename internal request state from `articleSlug` to `articleKey` or `articleRef`. Preserve public method name:

```ts
openArticle(articleId: string, options?: ShowArticleOptions): void
```

The `articleId` is a Helpin article PublicID. Continue accepting full canonical keys for compatibility.

- [ ] **Step 3: Implement DOM attribute binding**

After boot, attach one delegated click listener to `document`.

Behavior:

- Find closest element with one of the `data-helpin-article*` attributes.
- Prevent default when found.
- Read the attribute value as `articleId`.
- Call `widgetManager.openArticle(articleId, { mode })`.

Do not create separate popover/sidebar/modal UI in this task unless the product explicitly asks for it later.

- [ ] **Step 4: Update widget-core API language**

Change `fetchHelpArticles(host, widgetKey, collectionSlug)` to accept `collectionRef` and `fetchHelpArticle(..., articleRef)` to accept bare PublicID or canonical key. Keep endpoint paths unchanged for now.

- [ ] **Step 5: Verify package tests**

Run:

```bash
npm test --workspace @helpin-ai/sdk-js -- widget embedded
npm test --workspace @helpin-ai/widget-core -- HelpArticleView
```

Expected: bare PublicID opens and fetches articles.

- [ ] **Step 6: Commit**

```bash
git add packages/sdk-js/src packages/sdk-js/test packages/widget-core/src
git commit -m "feat: open embedded help articles by public id"
```

---

### Task 4: Ambiguity-Safe Slug-Only Fallbacks

**Files:**
- Modify: `server/internal/repository/docs_helpcenter.go`
- Modify: `server/internal/repository/docs_collection.go`
- Modify: `server/internal/service/docs_helpcenter.go`
- Modify: `server/internal/service/support_inbox_widget.go`
- Test: `server/internal/service/docs_helpcenter_translation_test.go`
- Test: `server/internal/repository/docs_helpcenter_widget_test.go`

- [ ] **Step 1: Write failing ambiguity tests**

Create fixtures with two live collections in the same workspace using `slug='getting-started'` and distinct PublicIDs.

Assert:

- `/c/getting-started-abc123ef` resolves by PublicID.
- `/c/getting-started` returns not found or no dynamic redirect when ambiguous.
- An explicit `docs_redirects.source_path='/getting-started'` with `target_path='/c/getting-started-abc123ef'` still resolves.
- Widget `GET /help/collections/getting-started/articles` returns not found when ambiguous.
- Widget `GET /help/collections/getting-started-abc123ef/articles` succeeds.

- [ ] **Step 2: Add exact-one repository helpers**

Before adding helpers, grep for all remaining public slug-only lookup patterns:

```bash
cd server
rg -n "GetBySlug|GetPublic.*BySlug|ByCollectionSlug|slug = \\?|First\\(" internal/repository internal/service
```

Every public/runtime slug-only lookup that can encounter collection or article duplicates must be classified as one of:

- PublicID-backed canonical lookup: no slug uniqueness needed.
- Legacy fallback: must use exact-one lookup and return ambiguous/not found when there is more than one candidate.
- Space/workspace/internal slug lookup: still intentionally unique or out of scope.

For each slug-only fallback path, query at most two rows and return:

```go
type SlugLookupResult[T any] struct {
  Value *T
  Ambiguous bool
}
```

or local equivalents. Do not use `First()` for slug-only public fallback once duplicates are allowed.

The explicit functions to convert or replace are:

- `DocsCollectionRepository.GetBySlug`
- `DocsHelpcenterRepository.GetPublicArticleBySlug`
- `DocsHelpcenterRepository.GetPublicArticleByCollectionSlug`
- `DocsHelpcenterRepository.GetPublicCollectionBySlug`
- `DocsHelpcenterRepository.GetPublicArticleTranslationBySlug`
- `DocsHelpcenterRepository.GetPublicArticleTranslationByCollectionSlug`
- `DocsHelpcenterRepository.GetPublicCollectionTranslationBySlug`
- `DocsHelpcenterRepository.GetPublicCollectionTranslationByWorkspaceSlug`

- [ ] **Step 3: Update public collection fallback**

In `GetPublicCollection`, keep PublicID key resolution first. For bare slug fallback, use exact-one lookup. If ambiguous, return nil and log at debug or warn with `workspace_id` and `slug`.

- [ ] **Step 4: Update public article legacy path fallback**

In `GetPublicArticleByCanonicalPath` and localized canonical path fallback, require both collection and article slug to resolve exactly one candidate. If ambiguous, return nil. Canonical `/articles/{publicID}` remains unaffected.

- [ ] **Step 5: Update widget fallbacks**

`ListWidgetHelpArticles` should:

- Resolve collection by PublicID if `collectionRef` contains or is a PublicID.
- Use exact-one slug fallback only for legacy callers.
- Return `collection not found` on ambiguity.

`GetWidgetHelpArticle` should already use PublicID for canonical keys. After Task 1, bare PublicID should work.

- [ ] **Step 6: Verify no unsafe slug `First()` remains**

Run:

```bash
cd server
rg -n "GetBySlug|GetPublic.*BySlug|ByCollectionSlug|slug = \\?|First\\(" internal/repository internal/service
```

Expected: any remaining matches are either space/workspace slug lookups, internal non-public lookups, or exact-one/ambiguity-safe helpers. Add comments near any intentionally retained slug lookup explaining why duplicates cannot apply.

- [ ] **Step 7: Verify**

Run:

```bash
cd server
go test ./internal/service -run 'PublicCollection|CanonicalPath|ResolvePublicPath' -count=1
go test ./internal/repository -run 'Widget|Helpcenter' -count=1
```

Expected: ambiguous slug-only fallbacks do not choose an arbitrary row.

- [ ] **Step 8: Commit**

```bash
git add server/internal/repository/docs_helpcenter.go server/internal/repository/docs_collection.go server/internal/service/docs_helpcenter.go server/internal/service/support_inbox_widget.go server/internal/service/*test.go server/internal/repository/*test.go
git commit -m "fix: make legacy help center slug lookups ambiguity safe"
```

---

### Task 5: Remove Application-Level Slug Uniquing

**Files:**
- Modify: `server/internal/service/docs_collection.go`
- Modify: `server/internal/service/docs_helpcenter.go`
- Modify: `server/internal/service/docs_helpcenter_translation.go`
- Modify: `server/internal/repository/docs_collection.go`
- Modify: `server/internal/repository/docs_helpcenter_translation.go`
- Modify: `server/internal/service/docs_import.go`
- Modify: `server/internal/service/docs_import_nextra.go`
- Test: `server/internal/service/docs_ordering_test.go`
- Test: `server/internal/service/docs_helpcenter_translation_test.go`
- Test: `server/internal/service/docs_import_test.go`
- Test: `server/internal/docsimport/*_test.go`

- [ ] **Step 1: Write failing duplicate slug behavior tests**

Assert:

- Two collections with the same slug can be created in different spaces in one workspace.
- Two collections with the same slug can be created in the same space if names collide.
- Publishing two articles with the same slug in the same space succeeds when their PublicIDs differ.
- Publishing two localized article translations with the same slug in the same locale succeeds when PublicIDs differ.
- Publishing two localized collection translations with the same slug succeeds when collection PublicIDs differ.

- [ ] **Step 2: Remove collection uniqueness checks**

In `DocsCollectionService.Create`, delete the `SlugTakenInWorkspace` pre-check. On create errors, only map PublicID unique conflicts to retry or error; do not map every unique violation to `ErrDocsCollectionSlugTaken`.

In `DocsHelpcenterService.UpdateCollectionSlug`, remove `collectionSlugTaken` checks.

- [ ] **Step 3: Remove article slug suffixing**

In `PublishExternally` and `UpdateArticleSlug`, replace:

```go
slug, err = s.ensureUniqueSourcePublicationSlug(...)
```

with:

```go
slug = normalizedSlugOrFallback(...)
```

Keep empty slug fallback.

- [ ] **Step 4: Remove collection/article translation slug suffixing**

Keep `ensureUniqueSpaceTranslationSlug` because space slugs still identify spaces. Remove uniqueness suffixing for:

- collection translations
- article translations

Use normalized requested slug or name/title fallback directly.

- [ ] **Step 5: Remove importer duplicate-slug retry behavior**

In `docs_import_nextra.go`, stop retrying collection creation with `-2`, `-3` on `ErrDocsCollectionSlugTaken`. Duplicate imported headings should preserve their natural slug because PublicID makes the final URL unique.

- [ ] **Step 6: Update HelpScout importer redirect mapping**

In `docs_import.go`, audit `categoryToCollectionSlug`, article redirect creation, and redirect-map generation. Keep slug fields only as legacy compatibility data. Where the target collection/article row exists, write or backfill a PublicID-backed `target_path`.

Acceptance for this step:

- Importing two HelpScout categories/articles with duplicate slugs does not require suffixing.
- Redirect records created by the importer resolve through `target_path` to `/c/{slug}-{collectionPublicID}` or `/articles/{slug}-{articlePublicID}`.
- The importer report/redirect map shows canonical PublicID URLs, not slug-only URLs.

- [ ] **Step 7: Run a final slug-uniqueness grep**

Run:

```bash
cd server
rg -n "SlugTakenInWorkspace|ErrDocsCollectionSlugTaken|ensureUnique.*Slug|SlugExists.*slug|slug is already in use|slug conflict" internal
```

Expected: remaining slug uniqueness logic applies only to spaces/workspaces or has been intentionally deleted. Collection/article PublicID objects should not use suffixing for uniqueness.

- [ ] **Step 8: Verify service tests**

Run:

```bash
cd server
go test ./internal/service -run 'DocsCollection|Helpcenter|Translation|Ordering' -count=1
go test ./internal/docsimport -count=1
```

Expected: duplicate slugs are allowed where PublicID exists.

- [ ] **Step 9: Commit**

```bash
git add server/internal/service server/internal/repository server/internal/docsimport
git commit -m "feat: stop enforcing help center slug uniqueness"
```

---

### Task 6: Drop Database Slug Unique Indexes

**Files:**
- Create: `server/internal/dbmigrate/sql/202604140716_remove_helpcenter_slug_uniqueness.sql`
- Modify: `server/internal/model/docs.go`
- Modify: `server/internal/model/docs_helpcenter_multilingual.go`
- Modify: `server/internal/model/docs_helpcenter_publication.go`
- Test: `server/internal/dbmigrate` validation if present

- [ ] **Step 1: Write migration**

Use idempotent SQL:

```sql
-- Collections with PublicID no longer need slug uniqueness.
DROP INDEX IF EXISTS idx_docs_collection_ws_slug;
DROP INDEX IF EXISTS idx_docs_collections_ws_slug_alive;

CREATE INDEX IF NOT EXISTS idx_docs_collections_ws_slug_lookup
  ON docs_collections (workspace_id, slug)
  WHERE deleted_at IS NULL;

-- Collection translations route canonically by collection PublicID.
DROP INDEX IF EXISTS idx_docs_hc_collection_space_locale_slug;
CREATE INDEX IF NOT EXISTS idx_docs_hc_collection_space_locale_slug_lookup
  ON docs_helpcenter_collection_translations (space_id, locale, slug);

-- Article translations route canonically by article PublicID.
DROP INDEX IF EXISTS idx_docs_hc_article_space_locale_slug;
CREATE INDEX IF NOT EXISTS idx_docs_hc_article_space_locale_slug_lookup
  ON docs_helpcenter_article_translations (space_id, locale, slug);

-- Live article publications route canonically by article PublicID.
DROP INDEX IF EXISTS idx_docs_hc_article_pub_space_locale_slug;
CREATE INDEX IF NOT EXISTS idx_docs_hc_article_pub_space_locale_slug_lookup
  ON docs_helpcenter_article_publications (space_id, locale, slug);
```

Do not drop:

- `idx_docs_space_ws_slug`
- `idx_docs_hc_space_ws_locale_slug`
- workspace or organization slug uniqueness
- PublicID unique indexes
- `(document_id, locale)` uniqueness for translations/publications

- [ ] **Step 2: Update GORM tags**

Remove slug `uniqueIndex` tags for collection/article translation/publication slug lookup fields. Replace with normal `index` tags where useful.

Keep unique tags for:

- `DocsHelpcenterSpaceTranslation` `(workspace_id, locale, slug)`
- `DocsHelpcenterSpaceTranslation` `(space_id, locale)`
- `DocsHelpcenterCollectionTranslation` `(collection_id, locale)`
- `DocsHelpcenterArticleTranslation` `(document_id, locale)`
- `DocsHelpcenterArticlePublication` `(document_id, locale)`

- [ ] **Step 3: Update comments and error mapping**

Remove comments that say collection slug uniqueness is enforced by dbmigrate. Keep `ErrDocsCollectionSlugTaken` only if another module still needs it; otherwise delete it and update handler error mapping.

- [ ] **Step 4: Validate migrations**

Run:

```bash
cd server
go run ./cmd/migrate validate
go test ./internal/dbmigrate -count=1
```

Expected: migration validation passes.

- [ ] **Step 5: Verify full backend tests**

Run:

```bash
cd server
go test ./...
```

Expected: all backend tests pass.

- [ ] **Step 6: Commit**

```bash
git add server/internal/dbmigrate/sql/202604140716_remove_helpcenter_slug_uniqueness.sql server/internal/model
git commit -m "db: drop help center slug unique indexes"
```

---

### Task 7: Frontend and Widget Copy/Types

**Files:**
- Modify: `help-center/src/routes/c/$collectionSlug.tsx`
- Modify: `help-center/src/routes/$locale/c/$collectionSlug.tsx`
- Modify: `help-center/src/routes/articles/$articleKey.tsx`
- Modify: `help-center/src/routes/$locale/articles/$articleKey.tsx`
- Modify: `help-center/src/lib/types.ts`
- Modify: `help-center/src/components/navigation/NavTree.tsx`
- Modify: `help-center/src/components/routes/ArticleRouteView.tsx`
- Modify: `help-center/src/components/routes/CollectionRouteView.tsx`
- Modify: `packages/widget-core/src/components/helpApi.ts`
- Modify: `packages/widget-core/src/components/helpTree.ts`
- Modify: `packages/widget-core/src/components/HelpCollectionView.tsx`
- Modify: `packages/widget-core/src/components/HelpArticleView.tsx`
- Modify: `packages/sdk-js/README.md`
- Test: `help-center/src/lib/__tests__/locale.test.ts`
- Test: `packages/widget-core/src/__tests__/HelpCollectionView.test.tsx`

- [ ] **Step 1: Ensure all links use PublicID-backed builders**

Audit all help-center links and route loaders. Every collection link must call `buildCollectionKey(slug, public_id)`. Every article link must call `buildArticleKey(slug, public_id)`.

Explicitly check these route files:

- `help-center/src/routes/c/$collectionSlug.tsx`
- `help-center/src/routes/$locale/c/$collectionSlug.tsx`
- `help-center/src/routes/articles/$articleKey.tsx`
- `help-center/src/routes/$locale/articles/$articleKey.tsx`

These route files may still use `collectionSlug`/`articleKey` as route parameter names, but they must pass canonical keys or bare PublicIDs to APIs, not assume the slug portion is unique.

- [ ] **Step 2: Update widget help collection requests**

When opening a collection from widget navigation, pass `buildHelpCollectionKey(collection.slug, collection.public_id)`, not raw slug.

Also update `packages/widget-core/src/components/helpTree.ts`:

- Rename or adapt `findHelpCollectionBySlug` so it matches by PublicID when given either bare `public_id` or `{slug}-{public_id}`.
- Rename or adapt `helpCollectionAncestorPath` with the same collection-ref semantics.
- Keep raw slug fallback only for legacy state, and make duplicate slug fallback deterministic only for non-ambiguous local tree state.

- [ ] **Step 3: Update SDK README**

Document:

```js
helpin('openArticle', 'abc123ef')
```

and:

```html
<a href="#" data-helpin-article="abc123ef">Need help?</a>
```

State that the value is the article PublicID shown in Helpin's article editor or API response.

- [ ] **Step 4: Verify frontend/package tests**

Run:

```bash
cd help-center
npm test -- locale navigation articleKey collectionKey
cd ../packages/widget-core
npm test
cd ../sdk-js
npm test
```

Expected: all relevant frontend and package tests pass.

- [ ] **Step 5: Commit**

```bash
git add help-center/src packages/widget-core packages/sdk-js/README.md
git commit -m "docs: document public id article embeds"
```

---

### Task 8: Migration Backfill and Production Safety

**Files:**
- Modify: `server/internal/dbmigrate/sql/202604140715_add_docs_redirect_target_paths.sql`
- Add: `server/internal/service/docs_redirect_backfill_test.go` if service-level backfill is needed
- Modify: `docs/AGENTS_AND_AUTOMATION.md` only if public help center contract is documented there
- Add: `docs/helpcenter-public-id-contract.md` if no existing docs page fits

- [ ] **Step 1: Backfill redirect target paths before dropping uniqueness**

Because legacy redirect targets are slug-based, run the target-path backfill while slug uniqueness still exists. If SQL cannot reliably build target paths due service helpers, implement a Go migration/backfill command in `dbmigrate` or service startup with idempotent behavior.

- [ ] **Step 2: Add observability for ambiguous legacy slugs**

Add structured logs whenever a slug-only legacy path is ambiguous:

```go
slog.WarnContext(ctx, "ambiguous help center slug fallback", "workspace_id", workspaceID, "slug", slug, "path", path)
```

Do not log article content or customer data.

- [ ] **Step 3: Document rollback**

Rollback is not "recreate unique indexes" once duplicates exist. Document the rollback as:

- Re-enable app-level suffixing.
- Run a dedupe script for duplicates.
- Recreate unique indexes only after dedupe.

- [ ] **Step 4: Final verification**

Run:

```bash
cd server
go test ./...
cd ../frontend
npm test
npm run build
cd ../help-center
npm test
npm run build
cd ../packages/widget-core
npm test
cd ../sdk-js
npm test
```

Expected: all tests and builds pass.

- [ ] **Step 5: Commit docs/observability**

```bash
git add docs server
git commit -m "docs: define help center public id contract"
```

---

## Deployment Sequence

1. Deploy parser support and canonical redirect target support while slug uniqueness is still present.
2. Backfill `docs_redirects.target_path`.
3. Deploy ambiguity-safe slug fallback behavior.
4. Deploy application changes that stop suffixing or rejecting duplicate slugs.
5. Apply the dbmigrate migration that drops collection/article slug unique indexes.
6. Monitor ambiguous slug fallback logs and redirect 404 rates.

This order avoids a window where duplicate slugs can exist while old `First()`-style slug lookups still choose arbitrary rows.

## Acceptance Criteria

- Two collections in the same workspace can have identical `slug` values.
- Two articles in the same space can have identical source, translated, and live publication slugs.
- Canonical public URLs remain unique through PublicIDs.
- Widget and SDK can open an article by bare PublicID.
- Explicit redirects continue to work after duplicate slugs exist.
- Slug-only legacy paths never pick an arbitrary row when duplicates exist.
- Space slugs remain unique until spaces receive their own PublicID design.
