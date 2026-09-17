# Docs Ordering Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add canonical ordering for docs spaces, collections, and articles, managed from `All Docs`, with the same order reflected in internal docs hierarchy surfaces and the public help center.

**Architecture:** Reuse existing `position` fields for spaces and collections, add `DocsDocument.position`, and backfill all three levels so existing workspaces become deterministic immediately. Expose bulk reorder endpoints scoped to a single sibling group, then add an `Arrange` toggle on `DocsHome` that uses nested `@dnd-kit` sortable lists with optimistic local state, auto-save on drop, and revert-on-failure behavior.

**Tech Stack:** Go 1.24, Chi, GORM/PostgreSQL, SQL migrations, React 19, TypeScript 5.9, TanStack Query, `@dnd-kit`, Vitest, React Testing Library.

---

## Inputs And Guardrails

- Spec: `docs/specs/2026-03-25-docs-ordering-design.md`
- Primary UI surface: `frontend/src/pages/docs/DocsHome.tsx`
- Do not add ordering UI to Settings in v1.
- Do not support cross-section, cross-space, or cross-collection drag moves in v1.
- `HelpcenterSpaceNavConfig.order` is no longer a source of truth for ordering. Leave `hidden` behavior alone.
- Keep `DocsSpaceDetail` as a browse/manage page. It can reflect canonical order later, but v1 ordering management lives only on `All Docs`.

## File Map

### Backend

- Create: `server/migrations/058_docs_ordering.sql`
  - Add `docs_documents.position`
  - Add `(space_id, collection_id, position)` index
  - Backfill and normalize existing `docs_spaces`, `docs_collections`, and `docs_documents`
- Create: `server/internal/repository/docs_ordering_test.go`
  - Repository tests for space, collection, and document ordering helpers
  - Covers create, move, reorder, and collection-delete rehoming
- Create: `server/internal/repository/docs_helpcenter_ordering_test.go`
  - Regression tests for public help center and widget article ordering
- Create: `server/internal/service/docs_document_ordering_test.go`
  - Service tests for create/update/move paths that can change a document bucket
- Modify: `server/internal/model/docs.go`
  - Add `DocsDocument.Position`
  - Add reorder request DTOs
  - Remove raw collection `position` patching from update DTOs if it is no longer needed
- Modify: `server/internal/repository/docs_space.go`
  - Add next-position and reorder helpers scoped to `(workspace_id, type)`
- Modify: `server/internal/repository/docs_collection.go`
  - Add next-position and reorder helpers scoped to `space_id`
  - Change collection delete to rehome docs into uncategorized with contiguous positions
- Modify: `server/internal/repository/docs_document.go`
  - Add next-position, reorder, move-with-normalization, and list-order helpers scoped to `(space_id, collection_id)`
- Modify: `server/internal/repository/docs_helpcenter.go`
  - Switch public and widget reads from timestamp/pinned ordering to canonical `position`
- Modify: `server/internal/service/docs_space.go`
  - Append new spaces to the end of their section
  - Treat `type` changes as a move between sections with normalization
- Modify: `server/internal/service/docs_collection.go`
  - Append new collections to the end of their space
  - Route reorders through dedicated reorder helpers
- Modify: `server/internal/service/docs_document.go`
  - Append new docs to the end of the target bucket
  - Treat `collection_id` changes during update as a move, not a raw patch
  - Extend move behavior to normalize source and target buckets
  - Add document reorder service method
- Modify: `server/internal/handler/docs.go`
  - Add reorder handlers for spaces, collections, and documents
- Modify: `server/internal/router/router.go`
  - Register reorder routes under `/api/docs`

### Frontend

- Create: `frontend/src/components/docs/docsOrderingTree.ts`
  - Pure helpers to build the `All Docs` arrange tree and produce reorder payloads
- Create: `frontend/src/components/docs/__tests__/docsOrderingTree.test.ts`
  - Pure tests for tree building and reorder reducers
- Create: `frontend/src/components/docs/DocsArrangeTree.tsx`
  - Nested sortable tree UI used only in arrange mode
- Create: `frontend/src/pages/docs/__tests__/DocsHome.ordering.test.tsx`
  - Page-level tests for arrange toggle, drag/save, and revert-on-failure
- Modify: `frontend/src/lib/docsTypes.ts`
  - Add `DocsDocument.position`
  - Add reorder request DTOs
  - Remove collection raw `position` patch typing if the backend stops accepting it
- Modify: `frontend/src/lib/services/docsService.ts`
  - Add reorder API calls
- Modify: `frontend/src/hooks/queries/useDocs.ts`
  - Add reorder mutations and targeted invalidation
- Modify: `frontend/src/pages/docs/DocsHome.tsx`
  - Add `Arrange` / `Done arranging` toggle
  - Switch between browse mode and `DocsArrangeTree`
  - Keep optimistic arrange state local to the page

## Data Model Decisions To Preserve

- Spaces are ordered within their visible section, not globally.
  - Use `(workspace_id, type)` as the bucket for `DocsSpace.position`
  - Internal and external spaces can reuse the same numeric positions because they render in separate sections
- Collections are ordered within a single `space_id`
- Documents are ordered within a single `(space_id, collection_id)` bucket
  - `collection_id = NULL` is the uncategorized bucket for that space
- Existing “move-by-update” paths must be hardened:
  - `DocsDocumentDetail` currently changes `collection_id` through `PATCH /docs/documents/{id}`
  - `DocsSpace` already allows `type` changes through `PATCH /docs/spaces/{id}`
  - These paths must reuse ordering helpers so they cannot create duplicate or sparse positions

## API Shapes

Use explicit reorder routes instead of raw `position` patches:

```go
type ReorderDocsSpacesRequest struct {
	Section  string   `json:"section"`   // "internal" | "external_capable"
	SpaceIDs []string `json:"space_ids"` // full ordered sibling list
}

type ReorderDocsCollectionsRequest struct {
	CollectionIDs []string `json:"collection_ids"` // full ordered list for one space
}

type ReorderDocsDocumentsRequest struct {
	CollectionID *string  `json:"collection_id"` // nil => uncategorized bucket
	DocumentIDs  []string `json:"document_ids"`  // full ordered list for one bucket
}
```

Recommended routes:

- `PUT /api/docs/spaces/reorder`
- `PUT /api/docs/spaces/{spaceId}/collections/reorder`
- `PUT /api/docs/spaces/{spaceId}/documents/reorder`

Use nested sortable lists on the frontend instead of one global tree drag context. That keeps drag scope aligned with the allowed sibling-only rules and reduces accidental cross-parent drops.

### Task 1: Backend Foundation - Canonical Position Storage And Normalization

**Files:**
- Create: `server/migrations/058_docs_ordering.sql`
- Create: `server/internal/repository/docs_ordering_test.go`
- Modify: `server/internal/model/docs.go`
- Modify: `server/internal/repository/docs_space.go`
- Modify: `server/internal/repository/docs_collection.go`
- Modify: `server/internal/repository/docs_document.go`

- [ ] **Step 1: Write the failing repository tests**

Create `server/internal/repository/docs_ordering_test.go` with focused tests like:

```go
func TestDocsSpaceRepository_ReorderWithinTypeKeepsContiguousPositions(t *testing.T) {}
func TestDocsCollectionRepository_ReorderWithinSpaceKeepsContiguousPositions(t *testing.T) {}
func TestDocsDocumentRepository_ReorderWithinBucketKeepsContiguousPositions(t *testing.T) {}
func TestDocsCollectionRepository_DeleteRehomesDocsIntoUncategorizedOrder(t *testing.T) {}
```

Seed dirty data on purpose:
- duplicate `position = 0` rows
- sparse positions like `0, 3, 8`
- documents spread across collection and uncategorized buckets

- [ ] **Step 2: Run the repo tests to capture the red state**

Run:

```bash
env GOCACHE=/tmp/go-build-docs-ordering go test ./internal/repository -run 'TestDocs(SpaceRepository|CollectionRepository|DocumentRepository)_' -v
```

Expected:
- FAIL because `DocsDocument.position` and reorder helpers do not exist yet
- or FAIL because duplicate/sparse positions are not normalized

- [ ] **Step 3: Add the schema change and backfill existing data**

In `server/migrations/058_docs_ordering.sql`, add:

```sql
ALTER TABLE docs_documents
  ADD COLUMN IF NOT EXISTS position integer NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_docs_doc_space_collection_position
  ON docs_documents (space_id, collection_id, position);
```

Backfill all existing records with contiguous positions:

```sql
WITH ranked_spaces AS (
  SELECT id,
         row_number() OVER (
           PARTITION BY workspace_id, type
           ORDER BY position ASC, created_at ASC, id ASC
         ) - 1 AS normalized_position
  FROM docs_spaces
  WHERE deleted_at IS NULL
)
UPDATE docs_spaces s
SET position = r.normalized_position
FROM ranked_spaces r
WHERE r.id = s.id;
```

Repeat the same pattern for:
- `docs_collections` partitioned by `space_id`
- `docs_documents` partitioned by `(space_id, collection_id)`

- [ ] **Step 4: Add repository helpers that enforce contiguous positions**

Implement helpers such as:

```go
func (r *DocsSpaceRepository) NextPosition(ctx context.Context, workspaceID, section string) (int, error)
func (r *DocsSpaceRepository) Reorder(ctx context.Context, workspaceID, section string, orderedIDs []string) error

func (r *DocsCollectionRepository) NextPosition(ctx context.Context, spaceID string) (int, error)
func (r *DocsCollectionRepository) Reorder(ctx context.Context, spaceID string, orderedIDs []string) error

func (r *DocsDocumentRepository) NextPosition(ctx context.Context, spaceID string, collectionID *string) (int, error)
func (r *DocsDocumentRepository) Reorder(ctx context.Context, spaceID string, collectionID *string, orderedIDs []string) error
func (r *DocsDocumentRepository) Move(ctx context.Context, id, spaceID string, collectionID *string) error
```

Rules:
- every helper runs inside a transaction
- every affected bucket is normalized after the write
- collection delete must preserve moved docs order when rehoming them into uncategorized

- [ ] **Step 5: Add `DocsDocument.position` to the model**

In `server/internal/model/docs.go`, add:

```go
Position int `json:"position" gorm:"not null;default:0;index:idx_docs_doc_space_collection_position,priority:3"`
```

Keep `space_id` and `collection_id` in the same index definition.

- [ ] **Step 6: Re-run the repo tests**

Run:

```bash
env GOCACHE=/tmp/go-build-docs-ordering go test ./internal/repository -run 'TestDocs(SpaceRepository|CollectionRepository|DocumentRepository)_' -v
```

Expected:
- PASS for the new ordering tests

- [ ] **Step 7: Commit**

```bash
git add server/migrations/058_docs_ordering.sql server/internal/model/docs.go server/internal/repository/docs_space.go server/internal/repository/docs_collection.go server/internal/repository/docs_document.go server/internal/repository/docs_ordering_test.go
git commit -m "feat(docs): add canonical docs ordering storage"
```

---

### Task 2: Backend Integration - Safe Create, Update, Move, And Reorder APIs

**Files:**
- Create: `server/internal/service/docs_document_ordering_test.go`
- Modify: `server/internal/model/docs.go`
- Modify: `server/internal/service/docs_space.go`
- Modify: `server/internal/service/docs_collection.go`
- Modify: `server/internal/service/docs_document.go`
- Modify: `server/internal/handler/docs.go`
- Modify: `server/internal/router/router.go`

- [ ] **Step 1: Write the failing service tests for the existing escape hatches**

Create `server/internal/service/docs_document_ordering_test.go` with tests like:

```go
func TestDocsDocumentService_CreateAppendsToBucket(t *testing.T) {}
func TestDocsDocumentService_UpdateCollectionUsesMoveSemantics(t *testing.T) {}
func TestDocsDocumentService_MoveNormalizesSourceAndTargetBuckets(t *testing.T) {}
func TestDocsSpaceService_UpdateTypeMovesSpaceToTargetSection(t *testing.T) {}
```

These tests should cover:
- create in collection
- create in uncategorized
- changing `collection_id` through the generic update path
- changing a space from `internal` to `external_capable`

- [ ] **Step 2: Run the service tests to capture the red state**

Run:

```bash
env GOCACHE=/tmp/go-build-docs-ordering go test ./internal/service -run 'TestDocs(DocumentService|SpaceService)_' -v
```

Expected:
- FAIL because create/update/move/type-change paths still use raw writes

- [ ] **Step 3: Add reorder request DTOs and remove raw position patching**

In `server/internal/model/docs.go`:
- add `ReorderDocsSpacesRequest`
- add `ReorderDocsCollectionsRequest`
- add `ReorderDocsDocumentsRequest`
- remove `Position` from `UpdateDocsCollectionRequest` if nothing in the app still needs raw patch-based collection ordering

- [ ] **Step 4: Route every bucket-changing service path through the ordering helpers**

Implement the service rules:
- `DocsSpaceService.Create` appends within `(workspace_id, type)`
- `DocsSpaceService.Update` treats `type` changes as “move section + normalize both sections”
- `DocsCollectionService.Create` appends within `space_id`
- `DocsDocumentService.Create` appends within `(space_id, collection_id)`
- `DocsDocumentService.Update` must not raw-patch `collection_id`; if it changes, call the move helper within the same space
- `DocsDocumentService.Move` appends to the target bucket and normalizes the source bucket

- [ ] **Step 5: Add dedicated reorder service, handler, and router support**

Handler methods:

```go
func (h *DocsHandler) ReorderSpaces(w http.ResponseWriter, r *http.Request) {}
func (h *DocsHandler) ReorderCollections(w http.ResponseWriter, r *http.Request) {}
func (h *DocsHandler) ReorderDocuments(w http.ResponseWriter, r *http.Request) {}
```

Register routes:

```go
r.With(requirePerm(authorization.PermDocsEdit)).Put("/spaces/reorder", h.Docs.ReorderSpaces)
r.With(requirePerm(authorization.PermDocsEdit)).Put("/spaces/{spaceId}/collections/reorder", h.Docs.ReorderCollections)
r.With(requirePerm(authorization.PermDocsEdit)).Put("/spaces/{spaceId}/documents/reorder", h.Docs.ReorderDocuments)
```

Return a lightweight response such as:

```json
{ "message": "order updated" }
```

- [ ] **Step 6: Re-run the service tests**

Run:

```bash
env GOCACHE=/tmp/go-build-docs-ordering go test ./internal/service -run 'TestDocs(DocumentService|SpaceService)_' -v
```

Expected:
- PASS for the new service tests

- [ ] **Step 7: Build the backend once**

Run:

```bash
env GOCACHE=/tmp/go-build-docs-ordering go build ./cmd/api
```

Expected:
- clean build

- [ ] **Step 8: Commit**

```bash
git add server/internal/model/docs.go server/internal/service/docs_space.go server/internal/service/docs_collection.go server/internal/service/docs_document.go server/internal/service/docs_document_ordering_test.go server/internal/handler/docs.go server/internal/router/router.go
git commit -m "feat(docs): add safe docs reorder APIs"
```

---

### Task 3: Backend Read Paths - Use Canonical Order Everywhere It Matters

**Files:**
- Create: `server/internal/repository/docs_helpcenter_ordering_test.go`
- Modify: `server/internal/repository/docs_document.go`
- Modify: `server/internal/repository/docs_helpcenter.go`

- [ ] **Step 1: Write the failing public-order regression tests**

Create tests like:

```go
func TestDocsHelpcenterRepository_ListSpaceNavigationUsesCollectionAndDocumentPosition(t *testing.T) {}
func TestDocsHelpcenterRepository_GetPublicCollectionBySlugUsesDocumentPosition(t *testing.T) {}
func TestDocsHelpcenterRepository_WidgetArticleListsUseDocumentPosition(t *testing.T) {}
```

Also add one repository test for internal hierarchy reads:

```go
func TestDocsDocumentRepository_ListBySpaceUsesCanonicalBucketOrder(t *testing.T) {}
```

- [ ] **Step 2: Run the failing read-path tests**

Run:

```bash
env GOCACHE=/tmp/go-build-docs-ordering go test ./internal/repository -run 'TestDocs(HelpcenterRepository|DocumentRepository)_.*(Position|Order)' -v
```

Expected:
- FAIL because public queries still sort by `created_at` or `is_pinned`

- [ ] **Step 3: Change the internal hierarchy read ordering**

In `server/internal/repository/docs_document.go`, keep the flat list behavior for non-hierarchy pages, but switch hierarchy-scoped reads to canonical order.

Recommended rule:

```go
if spaceID != nil && *spaceID != "" {
    query = query.Order("collection_id ASC NULLS FIRST, position ASC, created_at ASC")
} else {
    query = query.Order("is_pinned DESC, updated_at DESC")
}
```

This preserves existing flat list behavior for pages like `DocsDocumentList`, while making `DocsHome`, `Sidebar`, and other space-scoped hierarchy views deterministic.

- [ ] **Step 4: Switch public and widget article queries to `position`**

Update these queries in `server/internal/repository/docs_helpcenter.go`:
- `ListSpaceNavigation`
- `ListPublicDocumentsBySpace`
- `GetPublicCollectionBySlug`
- `ListWidgetArticlesByCollectionID`
- `ListWidgetArticlesBySpaceUncategorized`

Replace timestamp and pinned-first ordering with:

```sql
ORDER BY d.position ASC, d.created_at ASC
```

Keep collection ordering as:

```sql
ORDER BY c.position ASC, c.created_at ASC
```

- [ ] **Step 5: Re-run the targeted repository tests**

Run:

```bash
env GOCACHE=/tmp/go-build-docs-ordering go test ./internal/repository -run 'TestDocs(HelpcenterRepository|DocumentRepository)_.*(Position|Order)' -v
```

Expected:
- PASS

- [ ] **Step 6: Commit**

```bash
git add server/internal/repository/docs_document.go server/internal/repository/docs_helpcenter.go server/internal/repository/docs_helpcenter_ordering_test.go
git commit -m "fix(docs): use canonical docs ordering in help center reads"
```

---

### Task 4: Frontend Contracts - Reorder DTOs, Mutations, And Pure Arrange Helpers

**Files:**
- Create: `frontend/src/components/docs/docsOrderingTree.ts`
- Create: `frontend/src/components/docs/__tests__/docsOrderingTree.test.ts`
- Modify: `frontend/src/lib/docsTypes.ts`
- Modify: `frontend/src/lib/services/docsService.ts`
- Modify: `frontend/src/hooks/queries/useDocs.ts`

- [ ] **Step 1: Write the failing pure helper tests**

Create `frontend/src/components/docs/__tests__/docsOrderingTree.test.ts` with cases like:

```ts
it('builds internal and external sections in position order', () => {})
it('reorders spaces only inside their section', () => {})
it('reorders collections only inside their space', () => {})
it('reorders documents only inside their current bucket', () => {})
it('returns the correct API payload for each drop target', () => {})
```

- [ ] **Step 2: Run the helper tests to capture the red state**

Run:

```bash
cd /root/teampulse/frontend && npm exec vitest run src/components/docs/__tests__/docsOrderingTree.test.ts
```

Expected:
- FAIL because the helper module and reorder DTOs do not exist yet

- [ ] **Step 3: Add the frontend ordering types and service methods**

In `frontend/src/lib/docsTypes.ts`:

```ts
export interface DocsDocument {
  // ...
  position: number;
}

export interface ReorderDocsSpacesRequest {
  section: SpaceType;
  space_ids: string[];
}

export interface ReorderDocsCollectionsRequest {
  collection_ids: string[];
}

export interface ReorderDocsDocumentsRequest {
  collection_id?: string;
  document_ids: string[];
}
```

In `frontend/src/lib/services/docsService.ts`, add:
- `reorderSpaces`
- `reorderCollections`
- `reorderDocuments`

- [ ] **Step 4: Add simple reorder mutations**

In `frontend/src/hooks/queries/useDocs.ts`, add:
- `useReorderDocsSpaces`
- `useReorderDocsCollections`
- `useReorderDocsDocuments`

Keep these hooks simple:
- call the reorder endpoint
- invalidate only the affected docs queries
- do not put the optimistic tree reducer inside React Query

The page will own optimistic arrange state locally because:
- queries are nested per space
- only `DocsHome` needs this interaction
- rollback is easier with a local snapshot

- [ ] **Step 5: Implement the pure arrange helpers**

In `frontend/src/components/docs/docsOrderingTree.ts`, add focused helpers such as:

```ts
export function buildDocsOrderingSections(/* fetched docs data */) {}
export function reorderSpaceSection(/* tree + ids */) {}
export function reorderCollectionsInSpace(/* tree + ids */) {}
export function reorderDocumentsInBucket(/* tree + ids */) {}
```

Keep this file free of React code so it is easy to test and reuse from `DocsHome`.

- [ ] **Step 6: Re-run the helper tests**

Run:

```bash
cd /root/teampulse/frontend && npm exec vitest run src/components/docs/__tests__/docsOrderingTree.test.ts
```

Expected:
- PASS

- [ ] **Step 7: Commit**

```bash
git add frontend/src/lib/docsTypes.ts frontend/src/lib/services/docsService.ts frontend/src/hooks/queries/useDocs.ts frontend/src/components/docs/docsOrderingTree.ts frontend/src/components/docs/__tests__/docsOrderingTree.test.ts
git commit -m "feat(docs): add docs ordering client contracts"
```

---

### Task 5: Frontend UI - `All Docs` Arrange Mode With Optimistic Auto-Save

**Files:**
- Create: `frontend/src/components/docs/DocsArrangeTree.tsx`
- Create: `frontend/src/pages/docs/__tests__/DocsHome.ordering.test.tsx`
- Modify: `frontend/src/pages/docs/DocsHome.tsx`

- [ ] **Step 1: Write the failing page-level interaction tests**

Create `frontend/src/pages/docs/__tests__/DocsHome.ordering.test.tsx` covering:

```ts
it('shows drag handles only in arrange mode', () => {})
it('disables row navigation while arranging', () => {})
it('optimistically reorders a space and saves it', () => {})
it('optimistically reorders a collection inside one space', () => {})
it('optimistically reorders a document inside one bucket', () => {})
it('reverts the local tree when a save fails', () => {})
```

- [ ] **Step 2: Run the page tests to capture the red state**

Run:

```bash
cd /root/teampulse/frontend && npm exec vitest run src/pages/docs/__tests__/DocsHome.ordering.test.tsx
```

Expected:
- FAIL because arrange mode and reorder wiring do not exist yet

- [ ] **Step 3: Build the arrange-only tree component**

In `frontend/src/components/docs/DocsArrangeTree.tsx`:
- use one sortable list per sibling group, not one global tree
- render drag handles only in arrange mode
- keep expand/collapse controls active
- prevent row click navigation while arranging
- expose callbacks like:

```ts
onReorderSpaces(section, orderedIds)
onReorderCollections(spaceId, orderedIds)
onReorderDocuments(spaceId, collectionId, orderedIds)
```

This component should render:
- internal spaces section
- external spaces section
- collections under each expanded space
- uncategorized bucket under each expanded space

- [ ] **Step 4: Wire `DocsHome` to toggle browse vs arrange mode**

In `frontend/src/pages/docs/DocsHome.tsx`:
- add `Arrange` / `Done arranging` header button
- keep the existing browse UI untouched when arrange mode is off
- when arrange mode is on:
  - build a local arrange tree from fetched spaces/collections/documents
  - apply the reorder reducer immediately on drop
  - call the matching reorder mutation
  - show `toast.success('Order updated')` on success
  - restore the previous tree snapshot on failure and show an error toast

Do not navigate from rows in arrange mode.

- [ ] **Step 5: Re-run the page tests**

Run:

```bash
cd /root/teampulse/frontend && npm exec vitest run src/components/docs/__tests__/docsOrderingTree.test.ts src/pages/docs/__tests__/DocsHome.ordering.test.tsx
```

Expected:
- PASS

- [ ] **Step 6: Run a focused lint pass on touched docs files**

Run:

```bash
cd /root/teampulse/frontend && npm exec eslint src/pages/docs/DocsHome.tsx src/components/docs/DocsArrangeTree.tsx src/components/docs/docsOrderingTree.ts src/hooks/queries/useDocs.ts src/lib/services/docsService.ts src/lib/docsTypes.ts
```

Expected:
- no new lint errors from the ordering work

- [ ] **Step 7: Commit**

```bash
git add frontend/src/pages/docs/DocsHome.tsx frontend/src/components/docs/DocsArrangeTree.tsx frontend/src/pages/docs/__tests__/DocsHome.ordering.test.tsx
git commit -m "feat(docs): add all docs arrange mode"
```

---

### Task 6: Final Verification And Manual QA

**Files:**
- Verify only

- [ ] **Step 1: Run the targeted backend test suite**

Run:

```bash
env GOCACHE=/tmp/go-build-docs-ordering go test ./internal/repository -run 'TestDocs' -v
env GOCACHE=/tmp/go-build-docs-ordering go test ./internal/service -run 'TestDocs' -v
```

Expected:
- PASS for all docs ordering tests added in this plan

- [ ] **Step 2: Run the targeted frontend test suite**

Run:

```bash
cd /root/teampulse/frontend && npm exec vitest run src/components/docs/__tests__/docsOrderingTree.test.ts src/pages/docs/__tests__/DocsHome.ordering.test.tsx
```

Expected:
- PASS

- [ ] **Step 3: Build both apps once**

Run:

```bash
env GOCACHE=/tmp/go-build-docs-ordering go build ./cmd/api
cd /root/teampulse/frontend && npm run build
```

Expected:
- both builds succeed

- [ ] **Step 4: Manual QA checklist**

Verify these flows in a real workspace:
- `All Docs` shows `Arrange` toggle only when the user can edit docs
- reordering internal spaces does not affect the external section order
- reordering external spaces updates public help center space order
- reordering collections in one space updates `All Docs`, sidebar, and public collection order
- reordering articles inside a collection updates `All Docs`, widget collection articles, and public collection page order
- reordering uncategorized articles updates the `General` / `Uncategorized` bucket order consistently
- changing a document’s collection from `DocsDocumentDetail` does not create duplicate positions
- deleting a collection preserves the moved docs order in uncategorized
- changing a space from internal to external appends it to the external section and removes it from internal without position corruption

- [ ] **Step 5: Clean working tree and final status check**

Run:

```bash
git status --short
```

Expected:
- only intended docs-ordering changes are present

