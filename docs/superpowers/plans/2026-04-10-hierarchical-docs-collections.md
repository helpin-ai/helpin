# Hierarchical Docs Collections Implementation Plan

**Goal:** Add multi-level docs grouping by turning collections into a bounded tree shared by internal docs, public help center, widget help browsing, imports, translations, redirects, and search, while keeping canonical article URLs in the current `/:collection-slug/:article-slug` format.

**Architecture:** Keep `space` as the top-level boundary and evolve `collection` from a flat list into a tree using `parent_collection_id` and `depth`. Use the collection tree as the single source of truth for internal docs navigation, public help center navigation, and widget help browsing. Preserve current collection/article public path semantics, and extend the existing help center redirect flow so article moves and collection slug changes emit redirects without reinventing a second redirect system.

**Tech Stack:** Go 1.24, GORM/PostgreSQL, React 19, TanStack Router, TanStack Query, dnd-kit, widget-core (Preact), help-center SPA.

---

## Scope and product rules

- `space` remains the top-level container.
- `collection` becomes hierarchical with a bounded `depth` of `0`, `1`, or `2`.
- Articles belong to exactly one collection node.
- No separate public-sidebar JSON navigation system in v1.
- Internal docs, public help center, widget help browsing, and imports all use the same collection tree.
- Canonical public URLs stay:
  - Collection page: `/:collection-slug`
  - Article page: `/:collection-slug/:article-slug`
- Non-deleted collection slugs must be unique across the workspace in v1, which is intentionally stricter than public-only uniqueness so URL resolution and redirect writes are race-safe.
- Breadcrumbs reflect real collection ancestry.
- Moving an article to another collection changes its canonical URL and creates redirects from the old URL.
- Changing a published collection slug creates redirects for the old collection URL and descendant article URLs.
- Deleting a collection never deletes articles automatically; content is reparented safely.

## File map

### Backend schema and models
- Modify: `server/internal/model/docs.go`
- Modify: `server/internal/model/docs_helpcenter_multilingual.go`
- Modify: `server/internal/model/docs_helpcenter_publication.go`
- Modify: `server/internal/model/docs_redirect.go`
- Create: `server/internal/dbmigrate/sql/202604100001_docs_collection_tree.sql`

### Backend repositories and services
- Modify: `server/internal/repository/docs_collection.go`
- Modify: `server/internal/repository/docs_document.go`
- Modify: `server/internal/repository/docs_helpcenter.go`
- Modify: `server/internal/repository/docs_helpcenter_publication.go`
- Modify: `server/internal/repository/docs_search.go`
- Modify: `server/internal/repository/docs_redirect.go`
- Modify: `server/internal/service/docs_collection.go`
- Modify: `server/internal/service/docs_document.go`
- Modify: `server/internal/service/docs_helpcenter.go`
- Modify: `server/internal/service/docs_helpcenter_translation.go`
- Modify: `server/internal/service/docs_search.go`
- Modify: `server/internal/service/docs_import.go`
- Modify: `server/internal/service/support_inbox_widget.go`
- Modify: `server/internal/handler/docs.go`
- Modify: `server/internal/handler/support_inbox_widget.go`

### Frontend docs authoring
- Modify: `frontend/src/lib/docsTypes.ts`
- Modify: `frontend/src/lib/services/docsService.ts`
- Modify: `frontend/src/hooks/queries/useDocs.ts`
- Modify: `frontend/src/components/docs/docsOrderingTree.ts`
- Modify: `frontend/src/components/docs/DocsArrangeTree.tsx`
- Modify: `frontend/src/pages/docs/DocsHome.tsx`
- Modify: `frontend/src/pages/docs/DocsDocumentDetail.tsx`
- Modify: `frontend/src/pages/docs/DocsSpaceDetail.tsx`
- Modify: `frontend/src/pages/docs/DocsDocumentList.tsx`
- Create: `frontend/src/components/docs/CollectionTreePicker.tsx`
- Create: `frontend/src/components/docs/docsCollectionTree.ts`

### Public help center
- Modify: `help-center/src/lib/types.ts`
- Modify: `help-center/src/hooks/queries/index.ts`
- Modify: `help-center/src/lib/routeData.ts`
- Modify: `help-center/src/lib/navigation.ts`
- Modify: `help-center/src/components/navigation/NavTree.tsx`
- Modify: `help-center/src/components/navigation/Breadcrumbs.tsx`
- Modify: `help-center/src/components/routes/CollectionRouteView.tsx`
- Modify: `help-center/src/components/layout/Sidebar.tsx`
- Modify: `help-center/src/components/navigation/MobileNav.tsx`

### Widget help browsing
- Modify: `packages/shared/src/types/widget-config.ts`
- Modify: `packages/widget-core/src/components/HelpView.tsx`
- Modify: `packages/widget-core/src/components/HelpSpaceView.tsx`
- Modify: `packages/widget-core/src/components/HelpCollectionView.tsx`
- Modify: `packages/widget-core/src/components/ChatWindow.tsx`

### Tests
- Modify/Create:
  - `server/internal/service/docs_ordering_test.go`
  - `server/internal/service/docs_helpcenter_translation_test.go`
  - `server/internal/repository/docs_redirect_test.go`
  - `server/internal/handler/docs_helpcenter_public_locale_test.go`
  - `server/internal/service/support_inbox_widget_test.go`
  - `frontend/src/pages/docs/__tests__/DocsHome.test.tsx`
  - `help-center/src/components/navigation/__tests__/NavTree.test.tsx`
  - `help-center/src/components/routes/__tests__/CollectionRouteView.test.tsx`
  - `packages/widget-core/src/__tests__/ChatWindow.test.tsx`
  - `packages/widget-core/src/__tests__/HelpCollectionView.test.tsx`

## Task 1: Add collection tree schema

**Files:**
- Modify: `server/internal/model/docs.go`
- Modify: `server/internal/model/docs_redirect.go`
- Create: `server/internal/dbmigrate/sql/202604100001_docs_collection_tree.sql`
- Test: `server/internal/service/docs_ordering_test.go`

- [ ] **Step 1: Extend the collection model**

Add fields to `DocsCollection`:
- `ParentCollectionID *string`
- `Depth int`

Add request fields:
- `parent_collection_id` on create/update payloads

- [ ] **Step 2: Write the schema migration**

Add a migration that:
- adds `parent_collection_id uuid null`
- adds `depth integer not null default 0`
- preflights duplicate `(workspace_id, slug)` pairs across non-deleted collections
- resolves duplicates deterministically before adding the unique index:
  - keep the oldest collection in each duplicate group on the original slug
  - rename later duplicates to `{slug}-2`, `{slug}-3`, and so on until unique within the workspace
- adds a partial unique index on `(workspace_id, slug)` where `deleted_at is null`
- creates indexes on:
  - `(space_id, parent_collection_id, position)`
  - `parent_collection_id`
- drops or replaces the legacy flat ordering index `idx_docs_collection_space_pos`
- backfills all existing rows to `parent_collection_id = null`, `depth = 0`
- migrates any remaining `docs_slug_aliases` rows into `docs_redirects` if production data can still contain aliases that were never copied forward

- [ ] **Step 2a: Define duplicate-slug migration behavior**

Lock the migration semantics before writing SQL:
- duplicate collection slug groups are resolved during migration, not left for manual cleanup
- the oldest surviving collection keeps the original slug
- renamed duplicates do **not** get an automatic collection-level redirect from the old duplicate slug, because that source path remains canonical for the survivor and cannot also redirect
- if the migration logs renamed duplicates for operator visibility, write those rename mappings to the migration output or an audit note
- after dedupe completes, the partial unique index is created

- [ ] **Step 3: Add migration-safe constraints**

At minimum:
- prevent self-parenting at the service layer
- document same-space enforcement in service validation

If safe in SQL, add a foreign key from `parent_collection_id` to `docs_collections(id)` with `ON DELETE SET NULL`.

- [ ] **Step 4: Add failing tests for new fields**

Cover:
- existing collections still load
- new collections can have `parent_collection_id = nil`
- new collections can have `parent_collection_id != nil`

- [ ] **Step 5: Run targeted backend tests**

Run:
```bash
cd /root/teampulse/server && go test ./internal/service -run Test.*Docs.*Collection -v
```

- [ ] **Step 6: Commit**

```bash
git add server/internal/model/docs.go server/internal/model/docs_redirect.go server/internal/dbmigrate/sql/202604100001_docs_collection_tree.sql server/internal/service/docs_ordering_test.go
git commit -m "feat: add docs collection tree schema"
```

## Task 2: Make collection repository tree-aware

**Files:**
- Modify: `server/internal/repository/docs_collection.go`
- Test: `server/internal/service/docs_ordering_test.go`

- [ ] **Step 1: Add repository helpers for tree operations**

Implement:
- list collections by space preserving tree ordering inputs
- list children by parent
- get ancestors
- get descendants
- recalculate depth for a subtree
- reorder siblings for one parent bucket

Use iterative GORM lookups capped at 3 levels for ancestor/descendant traversal in v1. Do not introduce recursive CTEs or a materialized path column for this rollout.

- [ ] **Step 2: Add normalization by sibling bucket**

Replace flat `NormalizeSpace` assumptions with normalization scoped to:
- `space_id`
- `parent_collection_id`

- [ ] **Step 3: Add reparent support**

Add repository methods to:
- change parent
- move subtree
- recalculate descendant depths

- [ ] **Step 4: Write failing tests**

Cover:
- sibling reorder for top-level collections
- sibling reorder for nested collections
- retrieving descendants and ancestors
- depth recalculation after reparent

- [ ] **Step 5: Run targeted tests**

Run:
```bash
cd /root/teampulse/server && go test ./internal/service -run TestDocsOrdering -v
```

- [ ] **Step 6: Commit**

```bash
git add server/internal/repository/docs_collection.go server/internal/service/docs_ordering_test.go
git commit -m "feat: add tree-aware docs collection repository"
```

## Task 3: Add collection tree validation in service layer

**Files:**
- Modify: `server/internal/service/docs_collection.go`
- Test: `server/internal/service/docs_ordering_test.go`

- [ ] **Step 1: Add create/update validation**

Validate:
- parent exists
- parent belongs to same workspace and same space
- resulting `depth` is `0`, `1`, or `2`; reject any create/update that would produce `depth >= 3`
- no circular ancestry

- [ ] **Step 2: Add subtree reparent validation**

Prevent:
- moving a node under itself
- moving a node under its descendant
- moving a node so any descendant exceeds max depth

- [ ] **Step 3: Add service behavior for create/update**

When parent changes:
- recalculate current node depth
- recalculate descendant depths
- normalize old sibling bucket
- normalize new sibling bucket

- [ ] **Step 4: Write failing tests**

Cover:
- create nested collection
- reject cross-space parent
- reject self-parent
- reject cycle
- reject too-deep nesting
- allow valid reparent

- [ ] **Step 5: Run tests**

Run:
```bash
cd /root/teampulse/server && go test ./internal/service -run Test.*Collection.* -v
```

- [ ] **Step 6: Commit**

```bash
git add server/internal/service/docs_collection.go server/internal/service/docs_ordering_test.go
git commit -m "feat: validate hierarchical docs collections"
```

## Task 4: Update document move semantics for collection tree

**Files:**
- Modify: `server/internal/repository/docs_document.go`
- Modify: `server/internal/service/docs_document.go`
- Test: `server/internal/service/docs_ordering_test.go`

- [ ] **Step 1: Preserve article ownership as one collection**

Keep `collection_id` as the document’s owning collection.

- [ ] **Step 2: Update document move/reorder logic**

Ensure ordering works within:
- one collection bucket
- moved destination collection bucket

Keep any uncategorized/internal fallback logic only if still needed.

- [ ] **Step 3: Write failing tests**

Cover:
- moving a document between nested collections
- reordering within nested collection
- preserving contiguous positions after move

- [ ] **Step 4: Run tests**

Run:
```bash
cd /root/teampulse/server && go test ./internal/service -run Test.*Document.*Move -v
```

- [ ] **Step 5: Commit**

```bash
git add server/internal/repository/docs_document.go server/internal/service/docs_document.go server/internal/service/docs_ordering_test.go
git commit -m "feat: support document moves within collection tree"
```

## Task 5: Define safe delete and reparent rules

**Files:**
- Modify: `server/internal/repository/docs_collection.go`
- Modify: `server/internal/service/docs_collection.go`
- Test: `server/internal/service/docs_ordering_test.go`

- [ ] **Step 1: Implement delete behavior**

On collection delete:
- move child collections to the deleted node’s parent
- move direct articles to the deleted node’s parent level or configured fallback
- preserve sibling order as much as possible
- then soft-delete the collection

- [ ] **Step 2: Normalize affected buckets**

Normalize:
- old parent bucket
- new parent bucket
- moved document buckets

- [ ] **Step 3: Write failing tests**

Cover:
- delete top-level collection with children
- delete nested collection with articles
- no article loss
- no orphaned child collections

- [ ] **Step 4: Run tests**

Run:
```bash
cd /root/teampulse/server && go test ./internal/service -run Test.*Delete.*Collection -v
```

- [ ] **Step 5: Commit**

```bash
git add server/internal/repository/docs_collection.go server/internal/service/docs_collection.go server/internal/service/docs_ordering_test.go
git commit -m "feat: preserve subtree content when deleting collections"
```

## Task 6: Make help center translations and publications tree-aware

**Files:**
- Modify: `server/internal/model/docs_helpcenter_multilingual.go`
- Modify: `server/internal/model/docs_helpcenter_publication.go`
- Modify: `server/internal/service/docs_helpcenter_translation.go`
- Modify: `server/internal/repository/docs_helpcenter.go`
- Modify: `server/internal/repository/docs_helpcenter_publication.go`
- Test: `server/internal/service/docs_helpcenter_translation_test.go`
- Test: `server/internal/handler/docs_helpcenter_public_locale_test.go`

- [ ] **Step 1: Extend collection translation/publication reads**

Add ancestor/parent awareness to collection translation and publication queries.

- [ ] **Step 2: Keep article publication tied to owning collection**

Ensure article publications continue to store the owning `collection_id`, now potentially nested.

- [ ] **Step 3: Add breadcrumb/ancestor query helpers**

Repository helpers should fetch:
- collection ancestors
- direct child collections
- direct child articles

- [ ] **Step 4: Write failing tests**

Cover:
- nested collection translation lookup
- localized nested collection pages
- breadcrumbs include ancestors
- nested published collections order correctly

- [ ] **Step 5: Run tests**

Run:
```bash
cd /root/teampulse/server && go test ./internal/service -run TestDocsHelpcenterTranslation -v
cd /root/teampulse/server && go test ./internal/handler -run Test.*Helpcenter.*Locale -v
```

- [ ] **Step 6: Commit**

```bash
git add server/internal/model/docs_helpcenter_multilingual.go server/internal/model/docs_helpcenter_publication.go server/internal/service/docs_helpcenter_translation.go server/internal/repository/docs_helpcenter.go server/internal/repository/docs_helpcenter_publication.go server/internal/service/docs_helpcenter_translation_test.go server/internal/handler/docs_helpcenter_public_locale_test.go
git commit -m "feat: add nested collections to help center publication layer"
```

## Task 7: Keep current URL structure and add redirect behavior for moves

**Files:**
- Modify: `server/internal/service/docs_helpcenter.go`
- Modify: `server/internal/repository/docs_redirect.go`
- Test: `server/internal/repository/docs_redirect_test.go`

- [ ] **Step 1: Preserve canonical URL shape**

Continue canonical paths as:
- collection page: `/:collection-slug`
- article page: `/:collection-slug/:article-slug`

- [ ] **Step 2: Consolidate redirect source of truth**

Stop treating `DocsSlugAlias` and `docs_redirects` as parallel long-term mechanisms.

Implementation rules:
- `docs_redirects` is the canonical redirect table for this feature set
- remaining alias read/write callsites in `DocsHelpcenterService` and `DocsHelpcenterRepository` should be migrated or deleted as part of this task
- if legacy aliases remain in production, Task 1 migration or a one-time service backfill must copy them forward before tree rollout

- [ ] **Step 3: Extend existing article redirect logic for collection moves**

When article collection changes:
- extend `createSourceArticleRedirect()` / `UpdateArticleSlug()` behavior in `server/internal/service/docs_helpcenter.go`
- detect old collection slug path from the previous owning collection
- compute new canonical path from the new owning collection
- create or update redirect from old path to new path without creating loops

- [ ] **Step 4: Handle published collection slug changes**

When collection slug changes:
- redirect old collection URL
- redirect old article URLs under that collection to new article URLs

- [ ] **Step 5: Enforce unique collection slugs**

Add:
- service-level uniqueness checks before create/update
- database-backed uniqueness via `(workspace_id, slug) where deleted_at is null`
- new redirect type values in `model/docs_redirect.go` so auto-generated redirects are distinguishable from manual and import redirects:
  - `auto_article_move`
  - `auto_collection_rename`

- [ ] **Step 6: Write failing tests**

Cover:
- move article between collections creates redirect
- collection slug change creates descendant redirects
- duplicate published collection slug rejected
- A -> B -> A article move updates redirects without cycles
- new canonical path cannot remain as another redirect source without reconciliation

- [ ] **Step 7: Run tests**

Run:
```bash
cd /root/teampulse/server && go test ./internal/repository -run TestDocsRedirect -v
```

- [ ] **Step 8: Commit**

```bash
git add server/internal/service/docs_helpcenter.go server/internal/repository/docs_redirect.go server/internal/repository/docs_redirect_test.go
git commit -m "feat: redirect article paths when collections change"
```

## Task 8: Update internal docs types, services, and query hooks

**Files:**
- Modify: `frontend/src/lib/docsTypes.ts`
- Modify: `frontend/src/lib/services/docsService.ts`
- Modify: `frontend/src/hooks/queries/useDocs.ts`
- Test: existing hook tests under `frontend/src/hooks/__tests__/`

- [ ] **Step 1: Add collection tree fields to TS types**

Add:
- `parent_collection_id`
- `depth`
- any tree helper response types needed

- [ ] **Step 2: Update service payloads**

Support:
- create/update collection parent assignment
- tree-aware collection reorder/reparent endpoints if required

- [ ] **Step 3: Update query invalidation**

Ensure collection tree mutations invalidate:
- collection queries
- `allCollections`
- document list queries
- help center config/publication queries when relevant

- [ ] **Step 4: Write failing hook tests**

Cover:
- mutation payload includes parent collection
- invalidation includes affected space/document lists

- [ ] **Step 5: Run frontend tests**

Run:
```bash
cd /root/teampulse/frontend && npm exec vitest run src/hooks/__tests__/useDocsOrderingMutations.test.tsx
```

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/docsTypes.ts frontend/src/lib/services/docsService.ts frontend/src/hooks/queries/useDocs.ts frontend/src/hooks/__tests__/useDocsOrderingMutations.test.tsx
git commit -m "feat: add collection tree support to docs frontend types"
```

## Task 9: Replace flat Arrange tree with hierarchical collection tree

**Files:**
- Modify: `frontend/src/components/docs/docsOrderingTree.ts`
- Modify: `frontend/src/components/docs/DocsArrangeTree.tsx`
- Create: `frontend/src/components/docs/docsCollectionTree.ts`
- Test: `frontend/src/pages/docs/__tests__/DocsHome.test.tsx`

- [ ] **Step 1: Create a collection tree builder**

Add a helper that transforms:
- flat collections
- documents

into:
- nested collection nodes
- article lists per collection node

- [ ] **Step 2: Update Arrange tree rendering**

Render:
- top-level collections
- nested sub-collections
- article rows under each collection node
- collection expand/collapse

- [ ] **Step 3: Add drag/drop for sibling reorder and nesting**

Support:
- reorder within same parent
- move under another collection within allowed depth
- move article between collection nodes

- [ ] **Step 4: Add collection actions**

Support UI for:
- add sub-collection
- move collection
- delete collection

- [ ] **Step 5: Write failing UI tests**

Cover:
- nested collections render
- expand/collapse works
- top-level and nested ordering are stable

- [ ] **Step 6: Run tests**

Run:
```bash
cd /root/teampulse/frontend && npm exec vitest run src/pages/docs/__tests__/DocsHome.test.tsx
cd /root/teampulse/frontend && npm exec tsc --noEmit
```

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/docs/docsOrderingTree.ts frontend/src/components/docs/DocsArrangeTree.tsx frontend/src/components/docs/docsCollectionTree.ts frontend/src/pages/docs/__tests__/DocsHome.test.tsx
git commit -m "feat: add hierarchical docs arrange tree"
```

## Task 10: Add tree-aware collection pickers to docs create/edit flows

**Files:**
- Create: `frontend/src/components/docs/CollectionTreePicker.tsx`
- Modify: `frontend/src/pages/docs/DocsDocumentDetail.tsx`
- Modify: `frontend/src/pages/docs/DocsDocumentList.tsx`
- Modify: `frontend/src/pages/docs/DocsSpaceDetail.tsx`

- [ ] **Step 1: Build a reusable collection picker**

Picker should:
- show nested collections with indentation
- support search
- support “top-level collection” and “sub-collection” selection where needed

- [ ] **Step 2: Replace flat collection selects**

Use the tree picker in:
- article move flow
- article edit flow
- create collection flow

- [ ] **Step 3: Show full collection path where helpful**

In article detail pages, display:
- `Collection > Sub-collection`

- [ ] **Step 4: Write failing component tests**

Cover:
- nested options appear
- selection emits correct collection ID

- [ ] **Step 5: Run tests**

Run:
```bash
cd /root/teampulse/frontend && npm exec tsc --noEmit
```

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/docs/CollectionTreePicker.tsx frontend/src/pages/docs/DocsDocumentDetail.tsx frontend/src/pages/docs/DocsDocumentList.tsx frontend/src/pages/docs/DocsSpaceDetail.tsx
git commit -m "feat: use collection tree picker in docs flows"
```

## Task 11: Make help center navigation tree-aware

**Files:**
- Modify: `help-center/src/lib/types.ts`
- Modify: `help-center/src/hooks/queries/index.ts`
- Modify: `help-center/src/components/navigation/NavTree.tsx`
- Modify: `help-center/src/components/navigation/Breadcrumbs.tsx`
- Modify: `help-center/src/lib/navigation.ts`
- Test: `help-center/src/components/navigation/__tests__/NavTree.test.tsx`

- [ ] **Step 1: Extend nav types for nested collections**

Change public nav types to support:
- child collections
- direct articles
- breadcrumb ancestry

- [ ] **Step 2: Update sidebar tree rendering**

Render:
- expandable nested collections
- direct article links under the owning collection
- active ancestor expansion

- [ ] **Step 3: Update breadcrumbs and pager helpers**

Breadcrumbs should show full collection ancestry.
Pager should flatten visible published articles in tree order within the active space tree.

- [ ] **Step 4: Write failing tests**

Cover:
- nested sidebar rendering
- active article path expansion
- breadcrumb path correctness

- [ ] **Step 5: Run tests**

Run:
```bash
cd /root/teampulse/help-center && npm exec vitest run src/components/navigation/__tests__/NavTree.test.tsx
cd /root/teampulse/help-center && npm exec tsc --noEmit
```

- [ ] **Step 6: Commit**

```bash
git add help-center/src/lib/types.ts help-center/src/hooks/queries/index.ts help-center/src/components/navigation/NavTree.tsx help-center/src/components/navigation/Breadcrumbs.tsx help-center/src/lib/navigation.ts help-center/src/components/navigation/__tests__/NavTree.test.tsx
git commit -m "feat: add nested help center navigation"
```

## Task 12: Update collection pages and route data for nested collections

**Files:**
- Modify: `help-center/src/components/routes/CollectionRouteView.tsx`
- Modify: `help-center/src/lib/routeData.ts`
- Modify: route loaders under `help-center/src/routes/`
- Test: `help-center/src/components/routes/__tests__/CollectionRouteView.test.tsx`

- [ ] **Step 1: Update collection page behavior**

A collection page should show:
- collection title/description
- direct child collections
- direct child articles

- [ ] **Step 2: Remove flat-collection assumptions**

Replace any redirect-to-first-collection assumptions that break nested browsing.

- [ ] **Step 3: Update prefetching**

Prefetch the relevant nested collection navigation data, not just flat space navigation.

- [ ] **Step 4: Write failing tests**

Cover:
- collection page with child collections
- collection page with direct articles
- nested collection open behavior

- [ ] **Step 5: Run tests**

Run:
```bash
cd /root/teampulse/help-center && npm exec vitest run src/components/routes/__tests__/CollectionRouteView.test.tsx
cd /root/teampulse/help-center && npm exec tsc --noEmit
```

- [ ] **Step 6: Commit**

```bash
git add help-center/src/components/routes/CollectionRouteView.tsx help-center/src/lib/routeData.ts help-center/src/components/routes/__tests__/CollectionRouteView.test.tsx help-center/src/routes
git commit -m "feat: support nested collection pages in help center"
```

## Task 13: Make widget help browsing nested

**Files:**
- Modify: `packages/shared/src/types/widget-config.ts`
- Modify: `server/internal/service/support_inbox_widget.go`
- Modify: `server/internal/handler/support_inbox_widget.go`
- Modify: `packages/widget-core/src/components/HelpView.tsx`
- Modify: `packages/widget-core/src/components/HelpSpaceView.tsx`
- Modify: `packages/widget-core/src/components/HelpCollectionView.tsx`
- Modify: `packages/widget-core/src/components/ChatWindow.tsx`
- Test: `packages/widget-core/src/__tests__/HelpCollectionView.test.tsx`
- Test: `packages/widget-core/src/__tests__/ChatWindow.test.tsx`

- [ ] **Step 1: Extend widget help payload shape**

Return:
- nested collections
- direct articles
- breadcrumb/back context as needed

- [ ] **Step 2: Update widget API service behavior**

Widget endpoints should distinguish:
- top-level collections for a space
- nested collection contents

- [ ] **Step 3: Update widget UI flows**

Allow:
- opening top-level collection
- drilling into nested collection
- backing out one level at a time

- [ ] **Step 4: Write failing widget tests**

Cover:
- nested collection list rendering
- back navigation
- article opening from nested collection

- [ ] **Step 5: Run tests**

Run:
```bash
cd /root/teampulse/packages/widget-core && npm exec vitest run src/__tests__/ChatWindow.test.tsx src/__tests__/HelpCollectionView.test.tsx
```

- [ ] **Step 6: Commit**

```bash
git add packages/shared/src/types/widget-config.ts server/internal/service/support_inbox_widget.go server/internal/handler/support_inbox_widget.go packages/widget-core/src/components/HelpView.tsx packages/widget-core/src/components/HelpSpaceView.tsx packages/widget-core/src/components/HelpCollectionView.tsx packages/widget-core/src/components/ChatWindow.tsx packages/widget-core/src/__tests__/ChatWindow.test.tsx packages/widget-core/src/__tests__/HelpCollectionView.test.tsx
git commit -m "feat: support nested help center browsing in widget"
```

## Task 14: Update public search result context

**Files:**
- Modify: `server/internal/repository/docs_search.go`
- Modify: `server/internal/model/docs.go`
- Modify: help-center search result components if needed

- [ ] **Step 1: Enrich public search results**

Include:
- owning collection
- ancestor labels/path if useful

- [ ] **Step 2: Show nested context in result rows if needed**

Only if it improves comprehension without clutter.

- [ ] **Step 3: Write failing tests**

Cover:
- nested collection article still searchable
- result context references correct collection hierarchy

- [ ] **Step 4: Run tests**

Run:
```bash
cd /root/teampulse/server && go test ./internal/repository -run Test.*Search.* -v
```

- [ ] **Step 5: Commit**

```bash
git add server/internal/repository/docs_search.go server/internal/model/docs.go
git commit -m "feat: add nested collection context to public docs search"
```

## Task 15: Update importer mapping for nested source groups

**Files:**
- Modify: `server/internal/service/docs_import.go`
- Modify: `server/internal/docsimport/helpscout_normalize.go`
- Modify: any source-specific import mapping code as needed
- Test: `server/internal/service/docs_import_test.go`

- [ ] **Step 1: Add import mapping rules**

Map nested source groupings to nested collections up to supported depth.

- [ ] **Step 2: Add flattening behavior for over-depth structures**

If source depth exceeds supported depth:
- flatten into nearest allowed parent
- emit import warning

- [ ] **Step 3: Preserve stable content import semantics**

Do not let navigation-only constructs create fake content entities beyond supported collection tree depth.

- [ ] **Step 4: Write failing tests**

Cover:
- nested import produces nested collections
- unsupported depth produces warnings

- [ ] **Step 5: Run tests**

Run:
```bash
cd /root/teampulse/server && go test ./internal/service -run Test.*DocsImport.* -v
```

- [ ] **Step 6: Commit**

```bash
git add server/internal/service/docs_import.go server/internal/docsimport/helpscout_normalize.go server/internal/service/docs_import_test.go
git commit -m "feat: map imported docs groups into collection tree"
```

## Task 16: Update Help Center settings collection pickers and homepage card references

**Files:**
- Modify: `frontend/src/components/settings/HelpcenterTab.tsx`
- Modify: `frontend/src/components/settings/helpcenter/HelpcenterTranslationsTable.tsx`

- [ ] **Step 1: Make collection lookups tree-safe**

Any collection chooser/display in Helpcenter settings should understand nested collections and render full paths.

- [ ] **Step 2: Ensure featured card resolution still works**

Collection-linked homepage cards must resolve nested collections correctly.

- [ ] **Step 3: Run typecheck**

Run:
```bash
cd /root/teampulse/frontend && npm exec tsc --noEmit
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/settings/HelpcenterTab.tsx frontend/src/components/settings/helpcenter/HelpcenterTranslationsTable.tsx
git commit -m "feat: support nested collections in help center settings"
```

## Task 17: Add end-to-end regression coverage and manual QA checklist

**Files:**
- Modify: relevant tests added above
- Create: `docs/superpowers/plans/qa-hierarchical-docs-collections.md` (optional checklist if team wants a reusable QA doc)

- [ ] **Step 1: Run backend verification**

Run:
```bash
cd /root/teampulse/server && go test ./internal/service ./internal/repository ./internal/handler -run 'Docs|Helpcenter|SupportInboxWidget' -count=1
cd /root/teampulse/server && go build ./cmd/api
```

- [ ] **Step 2: Run frontend verification**

Run:
```bash
cd /root/teampulse/frontend && npm exec tsc --noEmit
cd /root/teampulse/help-center && npm exec tsc --noEmit
cd /root/teampulse/packages/widget-core && npm exec vitest run
```

- [ ] **Step 3: Manual QA**

Verify:
- create top-level collection
- create nested sub-collection
- move collection across parents
- reject 4th-level nesting
- move article between collections and confirm redirect
- browse nested tree internally
- browse nested tree publicly
- browse nested tree in widget
- search finds nested articles
- import nested structure from a test fixture

- [ ] **Step 4: Final integration commit**

```bash
git add server/internal/... frontend/src/... help-center/src/... packages/widget-core/src/... docs/superpowers/plans/qa-hierarchical-docs-collections.md
git commit -m "feat: add hierarchical docs collections across docs and help center"
```

## Risks and guardrails

- **URL churn risk:** article moves change URLs; redirect coverage must be complete before release.
- **Translation complexity:** nested collections affect localized breadcrumbs, localized collection pages, and publication queries together.
- **Arrange UX complexity:** dnd-kit nesting can become fragile; keep depth bounded and validate moves clearly.
- **Workspace size assumption:** v1 assumes fewer than roughly 500 collections per space. If real spaces exceed that, tree virtualization becomes a follow-up task.
- **Widget drift risk:** widget help browsing must use the same hierarchy APIs, not a separate flattened projection.
- **Import mismatch risk:** unsupported source depth must produce explicit warnings, not silent flattening.

## Release strategy

1. Ship schema and backend support to staging first with no separate feature-flag system.
2. Validate redirects, translations, realtime invalidation, and Arrange behavior in staging on real workspace data.
3. Enable hierarchical collections in internal docs Arrange.
4. Turn on public help center nested rendering.
5. Turn on widget nested browsing.
6. Enable import mapping for supported sources after the runtime behavior is stable.

## Decision lock-ins

These decisions are intentionally fixed before implementation so the rollout does not drift mid-stream.

### Collection depth rules

- A space can contain:
  - top-level collections (`depth = 0`)
  - child collections (`depth = 1`)
  - grandchild collections (`depth = 2`)
- Inserts or updates that would produce `depth >= 3` are rejected.
- Articles may be attached to a collection at any allowed depth.
- A collection may contain both:
  - direct child articles
  - child collections

### Slug and URL rules

- Collection slugs are unique per workspace for all non-deleted collections in v1, not merely within one parent collection.
- Migration rule for existing duplicate collection slugs:
  - keep the oldest collection on the original slug
  - suffix later duplicates to unique slugs during migration before the unique index is created
  - do not create automatic collection-level redirects from the old duplicate slug, because that path remains canonical for the survivor collection
- Article slugs continue to behave as they do today within the existing publication layer.
- Canonical collection URL remains:
  - `/:collection-slug`
- Canonical article URL remains:
  - `/:collection-slug/:article-slug`
- Collection ancestry is reflected in breadcrumbs and sidebar tree only, not in canonical URL depth.

### Article move rules

- Moving an article to a different collection changes its canonical public URL because the collection slug is part of the path.
- On article move:
  - compute old canonical path from previous published collection slug + current article slug
  - compute new canonical path from new published collection slug + current article slug
  - create redirect from old path to new path by extending the existing help center redirect helpers, not by adding a separate redirect path
- If the destination collection is unpublished or not public:
  - article remains internally moved
  - public publication state is recalculated based on existing publish rules
  - no public redirect should be emitted until a valid public path exists
- Redirect lifecycle rules:
  - auto-generated move redirects use type `auto_article_move`
  - auto-generated rename redirects use type `auto_collection_rename`
  - an A -> B -> A move must reconcile previous auto redirects so the final graph contains no cycles
  - if a new canonical path already exists as another redirect source, update or delete the stale auto redirect before writing the new one
  - manual redirects are never silently overwritten by auto-generated redirects

### Collection slug change rules

- If a published collection slug changes:
  - old collection URL redirects to the new collection URL
  - all descendant article URLs using the old collection slug redirect to their new canonical URLs
- Descendant collection URLs do **not** change automatically from ancestry changes alone because ancestry is not in the canonical path.
- Redirect generation must be idempotent; re-running slug updates must not create duplicate conflicting redirects.

### Collection move rules

- Moving a collection under a different parent does **not** change its canonical URL if its own slug stays the same.
- Descendant article URLs do **not** change when only ancestry changes.
- Breadcrumbs, internal sidebar placement, public sidebar placement, and widget browsing context **do** change immediately.

### Collection delete rules

- Deleting a collection never deletes descendant collections or articles.
- On delete:
  - child collections are reparented to the deleted collection’s parent
  - direct child articles are moved to the deleted collection’s parent level
  - if the deleted collection is top-level, direct child collections and articles become top-level within the space
- Positions must be normalized in all affected sibling buckets after delete.
- Redirects are only created if public canonical paths changed as a result of collection slug/path changes; simple reparenting alone should not emit redirects.

### Import mapping rules

- Imports should target the real collection tree, not a nav-only abstraction.
- Mapping policy:
  - source top-level doc grouping -> top-level collection
  - source second-level grouping -> child collection
  - source third-level grouping -> grandchild collection
  - deeper levels -> flatten into the nearest supported parent and emit a warning
- Imported documents are attached to the deepest supported mapped collection.
- Unsupported source constructs like Mintlify tabs, products, anchors, or version groups are not converted into fake collections.
  - Instead:
    - flatten where reasonable
    - emit explicit import warnings

### Widget browsing rules

- Widget help browsing mirrors the same collection tree used by the public help center.
- Widget collection view shows:
  - direct child collections first
  - direct child articles second
- Widget back navigation climbs one collection level at a time.

### Pager rules

- Public help center article pager flattens visible published articles in tree order within the active space tree.
- Pager order follows:
  - top-level collection order by `position`
  - nested collection order by sibling `position`
  - article order by article `position` within each owning collection

### Real-time invalidation rules

- Docs collection writes continue to use workspace websocket events through `publishWorkspaceEventWithParent(...)`.
- Collection create/update/delete/reorder/reparent events use:
  - `entity = "docs_collection"`
  - `action = "created" | "updated" | "deleted" | "reordered"`
  - `parent_type = "docs_space"`
  - `parent_id = space_id`
- Reparent and delete flows must invalidate both the old and new space/collection query buckets on the frontend.
- `frontend/src/hooks/useRealtimeSync.ts` must invalidate at least:
  - `queryKeys.docs.collections(workspaceId, spaceId)`
  - `queryKeys.docs.allCollections(workspaceId)`
  - `queryKeys.docs.documents(workspaceId)`
  - affected `queryKeys.docs.document(workspaceId, docId)` when document ownership changes

### Public collection page rules

- A public collection page may show both:
  - child collections
  - direct child articles
- Ordering on a collection page:
  - child collections ordered by sibling `position`
  - direct child articles ordered by article `position`
- Empty collections with no published direct children and no published descendant content should not be shown in public navigation.

### Internal docs sidebar rules

- Internal docs sidebar mirrors the collection tree for the active space.
- Active article open behavior:
  - all ancestor collections auto-expand
  - active article is highlighted
  - full breadcrumb path is shown in content view
- Articles should appear only under their owning collection node and should not be duplicated in ancestor nodes.
