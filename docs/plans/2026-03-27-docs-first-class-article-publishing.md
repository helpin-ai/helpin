# Docs First-Class Article Publishing Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a real draft-vs-live publishing system for docs articles so `Publish` and `Update` are meaningful, public help center reads are stable, and slug changes only go live when the user explicitly publishes or updates.

**Architecture:** Keep the existing docs editor and translation editor as draft-authoring surfaces, but introduce a single published snapshot store for public article content. Source documents and localized article translations continue autosaving drafts into their current tables; first publish and later update copy the current draft into a published snapshot row, and public read/search/listing paths read that snapshot instead of the draft tables.

**Tech Stack:** Go 1.24, GORM, PostgreSQL migrations, React 19, TanStack Query, TipTap JSON content, existing docs/help-center services and handlers.

---

## Scope

This plan intentionally scopes the first-class publishing system to **articles only**:
- source/default-locale article publishing
- localized article publishing
- public article detail pages
- public collection/article nav listings
- public search/article result payloads
- docs editor publish/update button state
- source and locale article slug edits

This plan does **not** introduce snapshot publishing for spaces or collections yet. Their current publish model remains unchanged. That keeps the change set focused enough to ship safely while still making article publish/update behavior real and consistent.

## Current State Summary

- Source article drafts live in:
  - `docs_documents`
  - `docs_contents`
  - `docs_helpcenter_articles`
- Localized article drafts live in:
  - `docs_helpcenter_article_translations`
- Public help center currently reads current draft-backed rows once the article is marked published:
  - source reads current `docs_documents` + `docs_contents` + `docs_helpcenter_articles`
  - locale reads current `docs_helpcenter_article_translations`
- Result: once an article is published, later autosaved edits are already live. The editor has no real `Update` step.

## File Map

### Backend schema and model
- Create: `server/migrations/062_docs_helpcenter_article_publications.sql`
- Create: `server/internal/model/docs_helpcenter_publication.go`
- Modify: `server/internal/model/docs.go`
- Modify: `server/internal/model/docs_helpcenter_multilingual.go`

### Backend repository
- Create: `server/internal/repository/docs_helpcenter_publication.go`
- Create: `server/internal/repository/docs_helpcenter_publication_test.go`
- Modify: `server/internal/repository/docs_helpcenter.go`
- Modify: `server/internal/repository/docs_helpcenter_translation.go`

### Backend services and handlers
- Modify: `server/internal/service/docs_helpcenter.go`
- Modify: `server/internal/service/docs_helpcenter_translation.go`
- Modify: `server/internal/service/docs_content.go`
- Modify: `server/internal/service/docs_document.go`
- Modify: `server/internal/handler/docs.go`
- Modify: `server/internal/handler/docs_helpcenter_translation.go`
- Modify: `server/internal/router/router.go` only if response or slug-edit routes need adjustment
- Create: `server/internal/service/docs_helpcenter_publication_test.go`
- Modify: existing docs help-center service/handler tests as needed

### Frontend types, data hooks, UI
- Modify: `frontend/src/lib/docsTypes.ts`
- Modify: `frontend/src/lib/services/docsService.ts`
- Modify: `frontend/src/hooks/queries/useDocs.ts`
- Modify: `frontend/src/pages/docs/DocsDocumentDetail.tsx`
- Modify: `frontend/src/components/docs/SlugDisplay.tsx`
- Modify: `frontend/src/components/docs/helpcenter/PublishSlugDialog.tsx`
- Create: `frontend/src/components/docs/__tests__/DocsDocumentDetail.publishState.test.tsx`
- Modify: `frontend/src/components/docs/__tests__/SlugDisplay.test.tsx`
- Modify: `frontend/src/lib/__tests__/docsService.test.ts`

### Help-center read regression coverage
- Modify: `help-center` tests only if client payload assumptions change

## Data Model Decision

Use **one published snapshot table for article content across source and locales**.

Recommended table:
- `docs_helpcenter_article_publications`

Recommended columns:
- `id`
- `document_id`
- `workspace_id`
- `space_id`
- `collection_id`
- `locale`
- `title`
- `slug`
- `excerpt`
- `content`
- `content_text`
- `seo_title`
- `seo_description`
- `published_at`
- `created_at`
- `updated_at`

Recommended constraints:
- unique `(document_id, locale)`
- unique `(space_id, locale, slug)` for public route safety
- nullable `collection_id`

Notes:
- Source/default locale uses its actual locale in this table, not a special sentinel.
- This stores only the **latest live snapshot**, not a full publication history.
- Existing draft tables remain the source of truth for editor work.

## API / UX Behavior Decision

### Publish button behavior
- Unpublished draft: `Publish (EN)`
- Published with no draft/live difference: muted `Published (EN)`
- Published with unpublished changes: active `Update (EN)`

### Dirty state definition
An article has `has_unpublished_changes = true` when the current draft fields differ from the published snapshot fields for the active locale.

Fields that count:
- title
- draft slug
- excerpt / meta description
- TipTap content JSON
- derived SEO fields written from title/excerpt

### Slug editing behavior
- Before first publish:
  - slug edits only affect the draft
  - publish/update dialog confirms the current draft slug
- After publish:
  - slug edits still change the draft only
  - public route remains on the live slug until `Update`
  - on update, create redirect from previous live slug to new live slug

This applies to:
- source article slug edits
- localized article slug edits

## Task 1: Add published article snapshot persistence

**Files:**
- Create: `server/migrations/062_docs_helpcenter_article_publications.sql`
- Create: `server/internal/model/docs_helpcenter_publication.go`
- Modify: `server/internal/model/docs.go`
- Modify: `server/internal/model/docs_helpcenter_multilingual.go`
- Test: `server/internal/repository/docs_helpcenter_publication_test.go`

- [ ] **Step 1: Write the failing repository tests for publication snapshot CRUD**

Cover:
- create first source snapshot
- create first locale snapshot
- overwrite existing snapshot on update
- unique slug enforcement per `(space_id, locale)`
- fetch by document+locale
- list snapshots by collection for public listing

- [ ] **Step 2: Run the new repository test file and verify it fails**

Run: `cd server && env GOCACHE=/tmp/go-build-docs-publish-plan go test ./internal/repository -run 'TestDocsHelpcenterArticlePublicationRepository' -count=1 -v`

Expected: FAIL because the publication model/repository does not exist yet.

- [ ] **Step 3: Add the SQL migration**

Create `server/migrations/062_docs_helpcenter_article_publications.sql` with:
- table creation
- indexes
- unique constraints
- nullable `collection_id`
- no data backfill yet

Note: migration numbering already has duplicate `061_*` files in this repo. Before committing, keep the new migration filename unique and aligned with the repo’s existing startup behavior.

- [ ] **Step 4: Add the publication model**

Create `server/internal/model/docs_helpcenter_publication.go` with:
- `DocsHelpcenterArticlePublication`
- `TableName()`

Also add transient response fields to existing models:
- `DocsDocument.HasUnpublishedChanges bool`
- `DocsDocument.LivePublishedAt *time.Time`
- `DocsDocument.LiveSlug *string`
- `DocsHelpcenterArticleTranslation.HasUnpublishedChanges bool`
- `DocsHelpcenterArticleTranslation.LivePublishedAt *time.Time`
- `DocsHelpcenterArticleTranslation.LiveSlug *string`

- [ ] **Step 5: Implement the repository**

Create `server/internal/repository/docs_helpcenter_publication.go` with focused methods:
- `GetArticlePublication(ctx, documentID, locale)`
- `UpsertArticlePublication(ctx, publication)`
- `ListArticlePublicationsByCollection(ctx, collectionID, locale)`
- `GetArticlePublicationByCollectionSlug(ctx, collectionID, locale, articleSlug)`
- `ListArticlePublicationsBySpace(ctx, spaceID, locale)` for uncategorized/general listing if needed

Keep repository methods publication-specific. Do not stuff them into an existing giant helper method.

- [ ] **Step 6: Run repository tests and make them pass**

Run: `cd server && env GOCACHE=/tmp/go-build-docs-publish-plan go test ./internal/repository -run 'TestDocsHelpcenterArticlePublicationRepository' -count=1 -v`

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add server/migrations/062_docs_helpcenter_article_publications.sql server/internal/model/docs_helpcenter_publication.go server/internal/model/docs.go server/internal/model/docs_helpcenter_multilingual.go server/internal/repository/docs_helpcenter_publication.go server/internal/repository/docs_helpcenter_publication_test.go
git commit -m "feat(docs): add article publication snapshot storage"
```

## Task 2: Implement source article publish/update against snapshots

**Files:**
- Modify: `server/internal/service/docs_helpcenter.go`
- Modify: `server/internal/service/docs_document.go`
- Modify: `server/internal/handler/docs.go`
- Modify: `server/internal/repository/docs_helpcenter.go`
- Test: `server/internal/service/docs_helpcenter_publication_test.go`

- [ ] **Step 1: Write failing service tests for source publish and update**

Cover:
- first publish creates a source-locale snapshot
- later draft edits do not change the snapshot immediately
- dirty detection becomes true after a draft edit
- second publish updates snapshot and clears dirty state
- slug-change update creates redirect from old live slug to new live slug on publish/update

- [ ] **Step 2: Run the source publish service tests and verify failure**

Run: `cd server && env GOCACHE=/tmp/go-build-docs-publish-plan go test ./internal/service -run 'TestDocsHelpcenterSourcePublishing' -count=1 -v`

Expected: FAIL because source publish still reads/writes live draft rows only.

- [ ] **Step 3: Add a focused source publication builder**

In `server/internal/service/docs_helpcenter.go`, add a helper that builds a publication snapshot from current source draft state:
- document title
- `docs_helpcenter_articles.slug`
- excerpt
- current TipTap content
- derived SEO title/description
- current locale from help-center config default locale

- [ ] **Step 4: Update source publish flow to upsert the snapshot**

Change source public publish/update behavior so publish:
- ensures the doc is internally published
- ensures public slug exists or confirms requested slug
- copies current draft into `docs_helpcenter_article_publications`
- updates `public_published_at`
- creates redirect if live slug changed

Do not change editor autosave behavior in this task.

- [ ] **Step 5: Add dirty-state enrichment for source docs**

When returning the doc detail used by the editor, enrich the document with:
- `live_published_at`
- `live_slug`
- `has_unpublished_changes`

Use a direct field comparison against the stored publication snapshot. Do not introduce hash-based caching yet.

- [ ] **Step 6: Ensure public source reads use snapshot content**

Change source public article/detail/listing read paths in `server/internal/repository/docs_helpcenter.go` and `server/internal/service/docs_helpcenter.go` to read source article title/slug/content/SEO from the source-locale publication snapshot instead of current draft tables.

- [ ] **Step 7: Run service tests and make them pass**

Run: `cd server && env GOCACHE=/tmp/go-build-docs-publish-plan go test ./internal/service -run 'TestDocsHelpcenterSourcePublishing' -count=1 -v`

Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add server/internal/service/docs_helpcenter.go server/internal/service/docs_document.go server/internal/handler/docs.go server/internal/repository/docs_helpcenter.go server/internal/service/docs_helpcenter_publication_test.go
git commit -m "feat(docs): publish source articles from snapshots"
```

## Task 3: Implement locale article publish/update against snapshots

**Files:**
- Modify: `server/internal/service/docs_helpcenter_translation.go`
- Modify: `server/internal/handler/docs_helpcenter_translation.go`
- Modify: `server/internal/repository/docs_helpcenter_translation.go`
- Modify: `server/internal/repository/docs_helpcenter.go`
- Test: `server/internal/service/docs_helpcenter_translation_test.go`
- Test: `server/internal/handler/docs_helpcenter_translation_test.go`

- [ ] **Step 1: Write failing locale publication tests**

Cover:
- first locale publish creates locale snapshot
- later draft translation edits do not change live public article until update
- `has_unpublished_changes` toggles true after draft edit
- publish/update copies title, excerpt, content, slug, SEO into snapshot
- update creates redirect from previous live locale slug to new live locale slug

- [ ] **Step 2: Run locale publication tests and verify failure**

Run: `cd server && env GOCACHE=/tmp/go-build-docs-publish-plan go test ./internal/service -run 'TestDocsHelpcenterTranslationPublishing' -count=1 -v`

Expected: FAIL because locale publish still serves current translation rows live.

- [ ] **Step 3: Add a locale publication builder**

In `server/internal/service/docs_helpcenter_translation.go`, add a helper that builds a publication snapshot from the current translation draft row.

Use:
- locale title
- locale draft slug
- excerpt
- TipTap content
- derived SEO title/description

- [ ] **Step 4: Change locale publish/update to upsert snapshots**

Update `PublishArticleTranslation(...)` so it:
- validates parent space/collection publish state as today
- derives or accepts slug as today
- upserts the locale publication snapshot
- marks translation status published
- creates redirect if the live slug changes

- [ ] **Step 5: Change locale slug editing to draft-only**

Update `UpdateArticleTranslationSlug(...)` so it:
- changes the translation draft slug only
- does **not** create a redirect immediately
- leaves redirect creation to the next publish/update when the live slug actually changes

- [ ] **Step 6: Enrich translation responses with live/dirty state**

For article translation list/detail responses, populate:
- `live_published_at`
- `live_slug`
- `has_unpublished_changes`

- [ ] **Step 7: Make locale public reads use snapshot rows**

Update localized public article and collection listing reads so article titles/slugs/content come from locale publication snapshots, not current translation drafts.

- [ ] **Step 8: Run locale tests and make them pass**

Run:
- `cd server && env GOCACHE=/tmp/go-build-docs-publish-plan go test ./internal/service -run 'TestDocsHelpcenterTranslationPublishing' -count=1 -v`
- `cd server && env GOCACHE=/tmp/go-build-docs-publish-plan go test ./internal/handler -run 'TestDocsHelpcenterTranslationHandler' -count=1 -v`

Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add server/internal/service/docs_helpcenter_translation.go server/internal/handler/docs_helpcenter_translation.go server/internal/repository/docs_helpcenter_translation.go server/internal/repository/docs_helpcenter.go server/internal/service/docs_helpcenter_translation_test.go server/internal/handler/docs_helpcenter_translation_test.go
git commit -m "feat(docs): publish locale articles from snapshots"
```

## Task 4: Move public listings and search to live snapshots

**Files:**
- Modify: `server/internal/repository/docs_helpcenter.go`
- Modify: `server/internal/service/docs_helpcenter.go`
- Modify: search-related docs service/repository files if article public search still reads draft-backed rows
- Test: add/extend repository and service tests for collection page, nav, and search

- [ ] **Step 1: Write failing tests for public listings/search using stale live content**

Cover:
- published article appears on collection page with old live title after draft edit
- after update, collection page reflects new title
- localized collection page behaves the same
- public search does not surface draft-only title/body edits before update

- [ ] **Step 2: Run the tests and verify failure**

Run: `cd server && env GOCACHE=/tmp/go-build-docs-publish-plan go test ./internal/service -run 'TestDocsHelpcenterPublicArticleListings' -count=1 -v`

Expected: FAIL because collection pages and/or search still read draft tables.

- [ ] **Step 3: Refactor public article queries to read publication snapshots**

Change:
- collection article listing
- navigation article listing
- article detail fetch
- public search article payloads

Rules:
- title/slug/excerpt/content/SEO come from publication snapshots
- collection/space metadata can continue coming from current published collection/space rows in this phase
- a missing snapshot means the article is not publicly readable even if the draft row exists

- [ ] **Step 4: Run the public read tests and make them pass**

Run: `cd server && env GOCACHE=/tmp/go-build-docs-publish-plan go test ./internal/service -run 'TestDocsHelpcenterPublicArticleListings|TestDocsHelpcenterPublicSearch' -count=1 -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/internal/repository/docs_helpcenter.go server/internal/service/docs_helpcenter.go
git commit -m "feat(docs): serve public article content from live snapshots"
```

## Task 5: Expose publish-state and dirty-state to the editor

**Files:**
- Modify: `frontend/src/lib/docsTypes.ts`
- Modify: `frontend/src/lib/services/docsService.ts`
- Modify: `frontend/src/hooks/queries/useDocs.ts`
- Modify: backend handlers/services touched by doc and translation GET responses if needed
- Test: `frontend/src/lib/__tests__/docsService.test.ts`

- [ ] **Step 1: Write the failing frontend contract tests**

Cover:
- doc detail can carry `has_unpublished_changes`, `live_published_at`, `live_slug`
- article translation payload can carry `has_unpublished_changes`, `live_published_at`, `live_slug`
- source slug edit route no longer implies immediate live publish

- [ ] **Step 2: Run the contract tests and verify failure**

Run: `cd frontend && npm exec vitest run src/lib/__tests__/docsService.test.ts`

Expected: FAIL because the current types and service assumptions do not include the new fields/semantics.

- [ ] **Step 3: Update TS types and service adapters**

Add the new state fields to:
- `DocsDocument`
- `DocsHelpcenterArticleTranslation`

Keep existing API endpoints unless a handler contract must change.

- [ ] **Step 4: Update query hooks and invalidation assumptions**

Ensure source and locale publish/update mutations invalidate:
- doc detail
- content if needed
- article translation list
- public-link-related metadata

- [ ] **Step 5: Run frontend contract tests and typecheck**

Run:
- `cd frontend && npm exec vitest run src/lib/__tests__/docsService.test.ts`
- `cd frontend && npm exec tsc --noEmit`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/docsTypes.ts frontend/src/lib/services/docsService.ts frontend/src/hooks/queries/useDocs.ts frontend/src/lib/__tests__/docsService.test.ts
git commit -m "feat(docs): expose live publish state to editor clients"
```

## Task 6: Redesign docs editor publish/update UX

**Files:**
- Modify: `frontend/src/pages/docs/DocsDocumentDetail.tsx`
- Modify: `frontend/src/components/docs/SlugDisplay.tsx`
- Modify: `frontend/src/components/docs/helpcenter/PublishSlugDialog.tsx`
- Create: `frontend/src/components/docs/__tests__/DocsDocumentDetail.publishState.test.tsx`
- Modify: `frontend/src/components/docs/__tests__/SlugDisplay.test.tsx`

- [ ] **Step 1: Write failing UI tests for publish/update states**

Cover:
- draft source article shows `Publish (EN)`
- published clean source article shows muted `Published (EN)`
- published dirty source article shows `Update (EN)`
- locale article follows the same three states
- changing slug after publish marks unpublished changes instead of updating live immediately
- update action publishes the draft and returns to `Published (LOCALE)`

- [ ] **Step 2: Run the UI tests and verify failure**

Run: `cd frontend && npm exec vitest run src/components/docs/__tests__/DocsDocumentDetail.publishState.test.tsx src/components/docs/__tests__/SlugDisplay.test.tsx`

Expected: FAIL because the current header only distinguishes `Publish` vs `Update` by published status.

- [ ] **Step 3: Update button-state logic in `DocsDocumentDetail.tsx`**

Implement:
- `Publish (LOCALE)` when not yet live
- muted `Published (LOCALE)` when live and clean
- `Update (LOCALE)` when live and dirty

Also add a subtle text treatment near the status area:
- `Unpublished changes` only when dirty

Do not add a second save system. Keep editor autosave as draft-save only.

- [ ] **Step 4: Change slug editing UX to draft/live semantics**

In `SlugDisplay.tsx` and surrounding handlers:
- source and locale slug edits update draft slug only
- if the article is already live, show subtle `Takes effect when you update`
- keep existing save/cancel affordances and tooltips

- [ ] **Step 5: Keep publish-time slug confirmation only for first publish**

`PublishSlugDialog` should appear when:
- first source publish has no slug
- first locale publish has no slug

Do not show it for routine updates if a draft slug already exists.

- [ ] **Step 6: Run UI tests, eslint, and typecheck**

Run:
- `cd frontend && npm exec vitest run src/components/docs/__tests__/DocsDocumentDetail.publishState.test.tsx src/components/docs/__tests__/SlugDisplay.test.tsx`
- `cd frontend && npm exec eslint src/pages/docs/DocsDocumentDetail.tsx src/components/docs/SlugDisplay.tsx src/components/docs/helpcenter/PublishSlugDialog.tsx`
- `cd frontend && npm exec tsc --noEmit`

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add frontend/src/pages/docs/DocsDocumentDetail.tsx frontend/src/components/docs/SlugDisplay.tsx frontend/src/components/docs/helpcenter/PublishSlugDialog.tsx frontend/src/components/docs/__tests__/DocsDocumentDetail.publishState.test.tsx frontend/src/components/docs/__tests__/SlugDisplay.test.tsx
git commit -m "feat(docs): add first-class publish and update states"
```

## Task 7: Regression hardening and end-to-end verification

**Files:**
- Modify: any touched tests that still assume live-autosave publishing
- Optionally add help-center route tests if article titles/slugs from snapshots alter assumptions

- [ ] **Step 1: Run the focused backend suites**

Run:
- `cd server && env GOCACHE=/tmp/go-build-docs-publish-plan go test ./internal/repository -run 'TestDocsHelpcenterArticlePublicationRepository|TestDocsHelpcenterTranslationRepository' -count=1 -v`
- `cd server && env GOCACHE=/tmp/go-build-docs-publish-plan go test ./internal/service -run 'TestDocsHelpcenterSourcePublishing|TestDocsHelpcenterTranslationPublishing|TestDocsHelpcenterPublicArticleListings|TestDocsHelpcenterPublicSearch' -count=1 -v`
- `cd server && env GOCACHE=/tmp/go-build-docs-publish-plan go test ./internal/handler -run 'TestDocsHelpcenterTranslationHandler|TestDocsHandler' -count=1 -v`
- `cd server && env GOCACHE=/tmp/go-build-docs-publish-plan go build ./cmd/api`

- [ ] **Step 2: Run the focused frontend suites**

Run:
- `cd frontend && npm exec vitest run src/lib/__tests__/docsService.test.ts src/components/docs/__tests__/DocsDocumentDetail.publishState.test.tsx src/components/docs/__tests__/SlugDisplay.test.tsx`
- `cd frontend && npm exec tsc --noEmit`
- `cd frontend && npm exec eslint src/pages/docs/DocsDocumentDetail.tsx src/components/docs/SlugDisplay.tsx src/lib/services/docsService.ts src/hooks/queries/useDocs.ts`

- [ ] **Step 3: Run help-center verification if response shapes changed**

Run:
- `cd help-center && npm test`
- `cd help-center && npm run build`

- [ ] **Step 4: Perform manual smoke verification**

Manual checklist:
1. Source article draft -> first publish -> public page appears.
2. Edit published source title/body -> public page stays unchanged -> button becomes `Update`.
3. Click `Update` -> public page now shows new content.
4. Edit published source slug -> old URL still works -> button shows `Update`.
5. Click `Update` -> new URL goes live and old URL redirects.
6. Repeat the same flow for one non-default locale article.
7. Confirm collection page listings and public search keep showing the live snapshot until update.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "test(docs): harden first-class article publishing flow"
```

## Notes For The Implementer

- Do **not** add a full publishing history model in this plan. One latest live snapshot is enough.
- Do **not** change the existing editor autosave cadence. The point is to make autosave draft-only, not to redesign saving.
- Do **not** broaden this into space/collection snapshot publishing in the same branch unless article scope is proven insufficient by failing tests.
- Redirect creation should happen when the live slug changes on publish/update, not when the draft slug input changes.
- Public read paths must be audited carefully. Missing just one listing/search path will make the system feel inconsistent.

## Expected Outcome

After this plan is implemented:
- publishing is a real editorial action
- update is a real live-content promotion step
- public docs stop leaking draft changes
- slug changes become predictable and safe
- the docs editor button finally matches the actual system behavior
