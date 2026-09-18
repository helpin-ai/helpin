# Show documentation collections as pages

This historical frontend plan explains the April collection-page refactor for contributors tracing documentation navigation. The current implementation has since changed document scoping, depth controls, and translation entry points. Use the source review before interpreting the original design or checklist.

## Source review — 2026-09-18

- [DocsSpaceDetail](../../frontend/src/pages/docs/DocsSpaceDetail.tsx) uses typed route search, `buildCollectionTree`, extracted layout components, and a missing-collection toast with replacement navigation. [The shared helpers](../../frontend/src/pages/docs/spaceDetail/nodeSelection.ts) resolve root, collection, uncategorized, and loading views.
- A collection view now recursively includes documents from its descendants. The original direct-only table/header contract and its test expectations below are superseded. Root view includes all tree documents and uncategorized documents; cards count recursively, while child-collection counts remain direct.
- Status selection is sent in the document query as well as applied by the helper; `include_archived` remains true. Without a status filter, archived documents are included. The table is rendered only when scoped documents are nonempty, so the proposed always-available table filter empty state is not guaranteed by this layout.
- Create controls currently hide child-collection creation at `depth >= 1`, rather than the proposed depth-2 threshold. This describes frontend affordances, not an independent backend depth guarantee.
- `initialParentCollectionId` is implemented in the global create store and forwarded to the collection dialog. The [card](../../frontend/src/pages/docs/spaceDetail/CollectionCard.tsx) uses a wrapper with sibling navigation and overflow buttons, avoiding the original nested-button sketch.
- There is no separate `SpaceBreadcrumb.tsx`; the header uses `QuietBreadcrumbs`. Translation controls were moved out of this page to settings. The current page also includes API reference content for external-capable spaces, beyond this plan's initial scope.
- Tests and the manual matrix below are historical verification instructions. No browser, accessibility, realtime, or typecheck result is claimed by this source review. Original screenshots, commit references, and line numbers provide historical context only.

## Original plan

**Date:** 2026-04-11
**Owner:** frontend / docs module
**Status:** Final — ready for implementation
**Scope:** `frontend/src/pages/docs/DocsSpaceDetail.tsx`, the route file, `globalCreateStore`, and colocated new components
**Out of scope:** Sidebar, Arrange mode, widget / public help-center surfaces, backend

## Changelog

### v3 (2026-04-11, post-Codex review round 2)
- **§4.9 Route typing** — switched from `Route.useSearch()` to `useSearch({ from: '/_authenticated/w/$slug/docs/spaces/$spaceId' })` to avoid an import cycle between the route file and the page (the route file imports `DocsSpaceDetail`, so the page cannot import `Route` back).
- **§4.2 nodeSelection** — fixed `buildCollectionTree` signature: the real helper takes `(spaceId, collections, documents)` and returns `{ topLevel, uncategorizedDocuments }`, not a raw node array. Helper API rewritten to operate on `CollectionTree` / `CollectionTreeNode[]` end-to-end. `scopedDocuments` now reads docs directly from `tree.topLevel` / `node.documents` / `tree.uncategorizedDocuments` — no parallel partitioning.
- **§4.3 Controller** — render-tree pseudocode updated to the correct types. The memoised value is now `const tree = useMemo(() => buildCollectionTree(spaceId, collections, documents), [spaceId, collections, documents])` with `null` as the unloaded sentinel. Every helper gets `tree`, not raw `DocsCollection[]`.
- **§4.2 resolveView** — now takes `(activeCollection, tree)` and the `collection` variant carries `ancestors: CollectionTreeNode[]` computed via `collectionAncestorChain`. The controller no longer touches raw `DocsCollection[]`.
- **§4.8 DocumentsTable** — `showCollectionColumn` is `true` only for `space_root`. Previous wording ("space root / uncategorized") was wrong — every uncategorized row has `collection_id == null`, so the column would be all `—`.
- **§6.1 Unit tests** — removed stale `includeArchived` tests; replaced with status filter tests for `null`, `draft`, `published`, `archived`.

### v2 (2026-04-11, post-Codex review round 1)
- **§4.2 Controller & §5.3 Filter scope** — clarified archive rule: preserve current behavior (fetch with `include_archived=true`, hide archived client-side when a non-archived status is chosen, show them when the archived filter is chosen or no filter is set). Header counts mirror the same predicate so the numbers match the table for every filter state.
- **§4.3 Controller** — added `parentCollectionId` flow through `globalCreateStore` so "Add sub-collection" creates under the right parent. Alternative noted: render `CreateCollectionDialog` directly from `DocsSpaceDetail` with local state. Recommendation: extend the store (used in two more places than the dialog).
- **§4.2 & §4.3 nodeSelection** — `directChildrenOfView` must delegate to the existing `buildCollectionTree` helper in `frontend/src/components/docs/docsCollectionTree.ts` so sidebar and main-area order stay identical (same `(position, created_at)` tiebreak, same orphan handling).
- **§4.9 Route & URL typing** — added `validateSearch` to the `$spaceId.tsx` route file with a Zod or hand-rolled schema for `{ collection?: string | '__uncollected__' }`. Controller reads via `useSearch({ from: '/_authenticated/w/$slug/docs/spaces/$spaceId' })`, not `location.search`.
- **§4.8 DocumentsTable props** — expanded prop contract to include `members`, `wsSlug`, `duplicatingDocId`, `canEditDocs`, and explicitly names the status/label/icon/color helpers the table owns internally.

---

## 1. Why we are doing this

### 1.1 Background

Docs collections recently gained hierarchy — collections can now nest up to 3 levels deep (`docs/plans/2026-04-10-hierarchical-docs-collections.md`). The backend, sidebar, arrange tree, and widget were all updated for nesting. The admin **space detail page** (`DocsSpaceDetail.tsx`) was updated in two incremental passes:

1. First pass: add depth-based left indent and a muted "↳" glyph to the existing pill-strip tab row.
2. Second pass (commit `1778f780`): replace the flat strip with a **drill-down pill navigator** — a breadcrumb row plus a pill row that shows the current level's children, with a `← Back` pill to climb back up.

Both passes keep the filter-chip metaphor. The pill row is still trying to act as the primary navigator for the collection tree.

### 1.2 The problem with the current design

After live testing (screenshots at `waqar-images/sub-col1.png` and `waqar-images/sub-col2.png`) three concrete issues show up:

**Issue A — Three navigation layers stack on top of each other.**
When drilled into a sub-collection, the main area shows simultaneously:

- Space header: `Help Center · All teams · 2 documents`
- Breadcrumb strip: `All Collections / Getting Started`
- Pill row: `← Back | test col (0) | + Sub-collection`

Three rows, three mechanisms, same purpose. Users have to parse all three to figure out where they are.

**Issue B — The pill metaphor is the wrong shape for tree data.**
Pills are a flat-list control. Collections are a 3-level tree. We can only squeeze a tree into a flat strip by:
- **(a)** Indenting and truncating — doesn't scale past ~5 items.
- **(b)** Hiding levels behind drill-down — creates the three-layer stack in Issue A.

Every variant of "strip of chips" hits this wall. No amount of polish changes that pills model a flat filter taxonomy, not a hierarchy.

**Issue C — The main area duplicates the sidebar.**
The sidebar already renders the full collection tree with icons, indentation, and selection state. The pill row re-implements that navigator in the main area. Two parallel navigators compete for the same job, and each one is worse than a purpose-built single navigator.

**Issue D — Counts, empty states, and creation are hacked into the filter row.**
- `test col (0)` is a dead-end click: users tap it and land on a blank table with no explanation.
- `+ Sub-collection` / `+ Collection` live inside the pill row, mixing creation actions with navigation filters.
- Header count (`2 documents`) does not update when drilled into a child collection — the subtitle stays on the parent space count, leaving users unsure what they are looking at.
- "All Collections" is ambiguous: users wonder whether it filters collections or documents (it filters documents).

### 1.3 The insight

The sidebar is already the canonical tree navigator. The main area's job should be **content, not navigation.**

This is the Notion / Confluence / GitBook pattern: every node (space, collection, sub-collection) is a page. When a node is selected in the sidebar, the main area shows that page's content — header, child previews, documents — not a second navigator.

### 1.4 What this refactor does

Replace the pill-based navigator in `DocsSpaceDetail.tsx` with a **collection-as-page** layout:

- **Sidebar** (unchanged) remains the tree navigator.
- **Main area** renders the current node's content: breadcrumb, node header, sub-collection cards, documents table.
- Pills, drill-down strip, and all associated state are deleted.
- Every level of the tree renders with the same layout — one pattern that scales to any depth.
- Hierarchy lives in the sidebar. Content lives in the main area. No duplication.

### 1.5 Non-goals

- No changes to the sidebar (`DocsSpacesNav.tsx`) — already good after the tree-order + indent refactor.
- No changes to Arrange mode — separate route, separate component.
- No changes to the widget or public help-center view — separate surfaces.
- No backend changes — all data already available via existing hooks.
- No new create/edit collection flows — existing dialogs are reused unchanged.
- No changes to realtime invalidation — `useRealtimeSync` already handles tree updates.

---

## 2. Success criteria

1. Clicking a space in the sidebar renders a **space root page**: header, top-level collection cards, documents table, uncategorized section.
2. Clicking a collection in the sidebar or on a card renders a **collection page**: breadcrumb, header, sub-collection cards (if any), documents table (direct-only).
3. Clicking "Uncategorized" in the sidebar renders a **dedicated uncategorized view** with a matching header and a flat list of uncategorized documents.
4. A leaf collection with zero content renders a clear empty state with contextual CTAs.
5. Pills, drill-down strip, `← Back` pill, and breadcrumb-plus-pill duplication are gone.
6. URL contract is unchanged: `/w/{slug}/docs/spaces/{spaceId}?collection={id | __uncollected__ | omit}`.
7. Browser back/forward works at every level — the URL drives view state, not local `useState`.
8. All existing row actions (archive, unarchive, delete, duplicate, move, publish) and dialogs still work.
9. All existing help-center translation surfaces still work for external-capable spaces.
10. Permissions are respected: no create/edit/delete affordances for users without `canEditDocs`.
11. Deep-linking to a deleted collection gracefully falls back to the space root with a toast and URL cleanup.
12. No regressions in realtime updates, loading states, or keyboard/screen-reader accessibility.

---

## 3. UI / UX specification

### 3.1 Space root view

Triggered by `?collection` absent or the sidebar "space name" link.

```
┌──────────────────────────────────────────────────────────┐
│  [icon] Help Center                          [+] [⋯]     │
│  Short description                                       │
│  24 documents · 5 collections                            │
├──────────────────────────────────────────────────────────┤
│  Collections                                             │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐             │
│  │📁      │ │📁      │ │📁      │ │📁      │             │
│  │Getting │ │Billing │ │API     │ │Account │             │
│  │Started │ │        │ │Docs    │ │        │             │
│  │8 docs  │ │12 docs │ │20 docs │ │2 docs  │             │
│  │3 subs  │ │        │ │4 subs  │ │        │             │
│  └────────┘ └────────┘ └────────┘ └────────┘             │
│                                                          │
│  Documents                                               │
│  Title            Owner   Collection    Status  Updated  │
│  ─ First step     Waqar   Getting/…     Pub     Mar 30   │
│  ─ …                                                     │
│                                                          │
│  Uncategorized (3)                              [View →] │
└──────────────────────────────────────────────────────────┘
```

Rules:
- Header title = space name. Icon = space icon. Count line shows **recursive** totals (all docs in space, all top-level collections).
- Cards render every top-level collection (`parent_collection_id = null`) sorted by `position`.
- Each card shows: icon, name, truncated description, **recursive doc count**, **direct sub-collection count**.
- Documents section below cards shows **all docs in the space** (same data that currently populates the flat view). Respects filter/sort state.
- "Uncategorized (N)" section only renders when at least one doc has `collection_id = null`. Clicking `View →` navigates to `?collection=__uncollected__`.
- `[+]` button = "Add document" / "Add top-level collection" split (or dropdown menu). `[⋯]` = edit space / translations / delete space.
- For external-capable spaces, header also shows the existing translation buttons as first-class actions (not hidden in `[⋯]`).

### 3.2 Collection view

Triggered by `?collection={uuid}`.

```
┌──────────────────────────────────────────────────────────┐
│  Help Center ▸ Getting Started                           │
│  [icon] Getting Started                      [+] [⋯]     │
│  Optional description                                    │
│  2 documents · 3 sub-collections                         │
├──────────────────────────────────────────────────────────┤
│  Sub-collections                                         │
│  ┌────────┐ ┌────────┐ ┌────────┐                        │
│  │📁 Setup│ │📁 First│ │📁 Tips │                        │
│  │4 docs  │ │2 docs  │ │0 docs  │                        │
│  └────────┘ └────────┘ └────────┘                        │
│                                                          │
│  Documents                                               │
│  Title            Owner   Status    Updated              │
│  ─ First step     Waqar   Pub       Mar 30               │
│  ─ Second step    Waqar   Pub       Mar 27               │
└──────────────────────────────────────────────────────────┘
```

Rules:
- Breadcrumb shows full ancestor chain, each segment clickable. Middle segments truncate with tooltip when the row overflows. First and last segments stay full width when possible.
- Header title = collection name. Icon = collection icon. Description is rendered under the title when present.
- Count line shows **direct** doc count and **direct** sub-collection count. (Direct matches the table below; card counts below are recursive for scanning.)
- Sub-collections section only renders when the collection has at least one direct child.
- Documents section shows **direct docs only** (`collection_id === collectionId`). Collection column is **hidden** in the table (redundant with the breadcrumb + header).
- `[+]` button = "Add document here" / "Add sub-collection here". Sub-collection button disabled at depth 2.
- `[⋯]` = edit collection / delete collection / (optionally) translation actions for external-capable spaces.

### 3.3 Uncategorized view

Triggered by `?collection=__uncollected__`.

```
┌──────────────────────────────────────────────────────────┐
│  Help Center ▸ Uncategorized                             │
│  [inbox icon] Uncategorized                    [+]       │
│  Documents not assigned to any collection                │
│  3 documents                                             │
├──────────────────────────────────────────────────────────┤
│  Documents                                               │
│  Title            Owner   Status    Updated              │
│  ─ …                                                     │
└──────────────────────────────────────────────────────────┘
```

Rules:
- No sub-collections section (uncategorized has no children).
- No "Add sub-collection" action — nonsense for this view.
- `[+]` is limited to "Add document" (lands in uncategorized).
- Breadcrumb is `Space ▸ Uncategorized`.

### 3.4 Empty states

**Leaf collection with zero docs and zero sub-collections:**

```
┌──────────────────────────────────────────────────────────┐
│  Help Center ▸ Getting Started ▸ test col                │
│  [icon] test col                              [+] [⋯]    │
│  0 documents                                             │
├──────────────────────────────────────────────────────────┤
│                                                          │
│             Nothing here yet                             │
│                                                          │
│  [ + Add document ]   [ + Add sub-collection ]           │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

- "Add sub-collection" is hidden when the current collection is at depth 2 (max depth reached).
- Buttons route through the existing `CreateCollectionDialog` / global create flow with the correct `defaultParentCollectionId` / `collectionId`.
- For users without `canEditDocs`, the empty state reads "Nothing here yet" with no CTAs.

**Empty space (no collections, no docs):**
Same component, but with "Create your first collection" and "Create your first document" wording.

### 3.5 What gets deleted from the page

- Pill strip JSX (current lines covering `currentLevel`, `levelChildren`, `breadcrumbChain`, `handleDrillBack`, and the rendered pill row).
- `CollectionTabIcon` helper (replaced by `SidebarCollectionIcon` or a shared component).
- Inline table JSX (moved into `DocumentsTable.tsx`).
- Inline header JSX (moved into `SpaceNodeHeader.tsx`).
- The separate "uncategorized" pill.
- Any `setActiveCollection` local-state writes that are not replaced by `navigate()` calls.

---

## 4. Component plan

### 4.1 File layout

```
frontend/src/pages/docs/
├── DocsSpaceDetail.tsx                 # Thin controller — state, hooks, routing
└── spaceDetail/                        # New folder, colocated with parent
    ├── nodeSelection.ts                # Pure helpers: resolveView, scoping, counts
    ├── nodeSelection.test.ts           # Unit tests for helpers
    ├── SpaceNodeHeader.tsx             # Breadcrumb + title + counts + [+] / [⋯] actions
    ├── SpaceBreadcrumb.tsx             # Breadcrumb with middle-ellipsis overflow
    ├── CollectionCardGrid.tsx          # Responsive grid wrapper
    ├── CollectionCard.tsx              # Single card (body button + overflow menu)
    ├── DocumentsTable.tsx              # Extracted from current inline table
    ├── UncategorizedSection.tsx        # Root-only "View uncategorized" block
    └── EmptyNodeState.tsx              # Shared empty-state renderer
```

### 4.2 View resolution (`nodeSelection.ts`)

**Critical constraint:** this module must **not** reinvent tree-building logic. `frontend/src/components/docs/docsCollectionTree.ts` already exposes `buildCollectionTree`, `flattenCollectionTree`, `findCollectionNode`, and `collectionAncestorChain`, and is already used by the sidebar (`DocsSpacesNav.tsx`) and the Arrange tree. Reusing it guarantees that:

- Sibling order matches the sidebar exactly (same `(position, created_at)` tiebreak).
- Orphan / deleted / circular references are handled the same way.
- Document partitioning (per-collection `node.documents`, `tree.uncategorizedDocuments`) matches the sidebar so numbers never diverge.

**Real helper signatures** (verified against `docsCollectionTree.ts`):

```ts
buildCollectionTree(
  spaceId: string,
  collections: DocsCollection[],
  documents: DocsDocument[],
): CollectionTree

interface CollectionTree {
  topLevel: CollectionTreeNode[]
  uncategorizedDocuments: DocsDocument[]
}

interface CollectionTreeNode {
  collection: DocsCollection
  children: CollectionTreeNode[]
  documents: DocsDocument[]   // direct docs only, already position-sorted
}

findCollectionNode(tree: CollectionTreeNode[], collectionId: string): CollectionTreeNode | null
collectionAncestorChain(tree: CollectionTreeNode[], collectionId: string): CollectionTreeNode[]
```

`nodeSelection.ts` is a **thin adapter** over these helpers — it does not recompute ordering or repartition documents.

```ts
import type { DocsCollection, DocsDocument, DocStatus } from '@/lib/docsTypes'
import {
  findCollectionNode,
  collectionAncestorChain,
  flattenCollectionTree,
  type CollectionTree,
  type CollectionTreeNode,
} from '@/components/docs/docsCollectionTree'

export type NodeView =
  | { kind: 'loading' }
  | { kind: 'space_root' }
  | { kind: 'collection'; node: CollectionTreeNode; ancestors: CollectionTreeNode[] }
  | { kind: 'uncategorized' }

export interface StatusFilter {
  status: DocStatus | null
}

export function resolveView(
  activeCollection: string | null | undefined,
  tree: CollectionTree | null,   // null ⇒ not yet loaded
): NodeView {
  if (!activeCollection) return tree ? { kind: 'space_root' } : { kind: 'loading' }
  if (activeCollection === '__uncollected__') {
    return tree ? { kind: 'uncategorized' } : { kind: 'loading' }
  }
  if (!tree) return { kind: 'loading' }
  const node = findCollectionNode(tree.topLevel, activeCollection)
  if (!node) return { kind: 'space_root' } // deleted-id fallback (caller cleans URL)
  const ancestors = collectionAncestorChain(tree.topLevel, activeCollection)
  return { kind: 'collection', node, ancestors }
}

// Direct children of the current view, as tree nodes (already sorted).
// space_root → tree.topLevel
// collection → view.node.children
// uncategorized / loading → []
export function directChildrenOfView(
  view: NodeView,
  tree: CollectionTree | null,
): CollectionTreeNode[]

// Documents to render in the table at the current view — read directly
// from the pre-partitioned tree, then apply the status filter.
export function scopedDocuments(
  view: NodeView,
  tree: CollectionTree | null,
  filter: StatusFilter,
): DocsDocument[]
// Implementation:
// space_root   → flatten tree.topLevel's .documents + tree.uncategorizedDocuments
// collection   → view.node.documents
// uncategorized→ tree.uncategorizedDocuments
// then filter by status (see §5.3 predicate)

// Recursive doc count in a collection's subtree. Used for card previews.
// Walks node.children depth-first summing node.documents that pass filter.
export function countDocsInSubtree(
  node: CollectionTreeNode,
  filter: StatusFilter,
): number

// Direct doc count. id === null ⇒ uncategorized.
export function countDirectDocs(
  nodeOrNull: CollectionTreeNode | null,
  tree: CollectionTree | null,
  filter: StatusFilter,
): number

// Direct child collection count. id === null ⇒ top-level.
export function countDirectChildren(
  nodeOrNull: CollectionTreeNode | null,
  tree: CollectionTree | null,
): number
```

Rules:
- `resolveView(null, tree)` → `space_root` when tree is loaded, else `loading`.
- `resolveView(uuid, null)` → `loading`.
- `resolveView(uuid, tree)` where uuid is missing → `space_root` (deleted-id fallback; caller shows toast and cleans URL via §4.10).
- `resolveView('__uncollected__', tree)` → `uncategorized`.
- `directChildrenOfView` never re-sorts — it returns `tree.topLevel` or `node.children` as-is.
- `scopedDocuments` never re-partitions — it reads from `node.documents` / `tree.uncategorizedDocuments`.
- `countDocsInSubtree` memoises via `useMemo` at the card grid level so a 40-card render doesn't walk the tree 40 times.

**Why `CollectionTree | null` instead of `CollectionTree | undefined`:** the controller normalises with `tree ?? null` so all helpers see the same sentinel. `undefined` would leak the upstream `useDocsDocuments` / `useDocsCollections` pending state into pure logic.

### 4.3 Controller (`DocsSpaceDetail.tsx`)

Stays thin. Owns:
- All existing hooks: `useDocsSpace`, `useDocsCollections`, `useDocsDocuments`, translation hooks, mutation hooks.
- Dialog open/close state.
- Filter / sort state (`filterStatus`, `sortField`, `sortDir`).
- Deep-link fallback handling (see §4.10).
- Memoised `tree = buildCollectionTree(spaceId, collections, documents)` once both queries have loaded, then passing that `CollectionTree | null` into every helper.

Does **not** own:
- `activeCollection` as a local `useState` — it reads from URL via `useSearch({ from: '/_authenticated/w/$slug/docs/spaces/$spaceId' })` (see §4.9). The existing URL → state sync `useEffect` is removed; the URL becomes the single source of truth.
- Any navigation logic — all drill/breadcrumb clicks call `navigate({ to, search })` helpers passed into subcomponents.

**Creating a sub-collection:** `globalCreateStore` currently has `initialSpaceId` and `initialCollectionId` but **not** `initialParentCollectionId`. The store must be extended with a `parentCollectionId` option so `GlobalCreateModals.tsx` can forward it to `CreateCollectionDialog.defaultParentCollectionId`. Without this change, `openCreate('docs_collection', { spaceId, parentCollectionId })` silently creates a top-level collection — which is how the current "Add sub-collection" CTA would silently regress. See §4.11 for the full delta.

Render tree:

```tsx
// Inside DocsSpaceDetail()
const { collection: activeCollectionParam } = useSearch({
  from: '/_authenticated/w/$slug/docs/spaces/$spaceId',
})

const tree = useMemo<CollectionTree | null>(() => {
  if (!collections || !documents) return null
  return buildCollectionTree(spaceId, collections, documents)
}, [spaceId, collections, documents])

const view = useMemo(
  () => resolveView(activeCollectionParam ?? null, tree),
  [activeCollectionParam, tree],
)

const filter = useMemo<StatusFilter>(() => ({ status: filterStatus }), [filterStatus])

const childNodes = useMemo(
  () => directChildrenOfView(view, tree),
  [view, tree],
)

const scopedDocs = useMemo(
  () => scopedDocuments(view, tree, filter),
  [view, tree, filter],
)

const uncategorizedCount = useMemo(
  () => countDirectDocs(null, tree, filter),
  [tree, filter],
)
```

```tsx
<div className="flex h-full flex-col">
  <SpaceNodeHeader
    space={space}
    view={view}
    docCount={scopedDocs.length}
    subCount={childNodes.length}
    canEdit={canEditDocs}
    onNavigate={(target) => navigate({
      to: '/w/$slug/docs/spaces/$spaceId',
      params: { slug: wsSlug, spaceId },
      search: target === null ? {} : { collection: target },
    })}
    // ... dialog + create handlers
  />

  <div className="flex-1 overflow-auto">
    {view.kind === 'loading' && <NodeSkeleton />}

    {view.kind !== 'loading' && (
      <>
        {childNodes.length > 0 && (
          <CollectionCardGrid
            title={view.kind === 'space_root' ? 'Collections' : 'Sub-collections'}
            nodes={childNodes}                          // CollectionTreeNode[]
            filter={filter}                             // passed so cards can count
            canEdit={canEditDocs}
            onOpen={(node) => navigate({
              to: '/w/$slug/docs/spaces/$spaceId',
              params: { slug: wsSlug, spaceId },
              search: { collection: node.collection.id },
            })}
            onEdit={(node) => setEditingCollection(node.collection)}
            onDelete={(node) => setConfirmDelete({
              type: 'collection',
              id: node.collection.id,
              name: node.collection.name,
            })}
          />
        )}

        {scopedDocs.length > 0 ? (
          <DocumentsTable
            documents={scopedDocs}
            members={members}
            collectionNames={collectionNames}
            collectionAncestorPath={(id) => buildAncestorPath(tree, id)}
            showCollectionColumn={view.kind === 'space_root'}   // §4.8 — only at root
            wsSlug={wsSlug}
            filterStatus={filterStatus}
            onFilterStatus={setFilterStatus}
            sortField={sortField}
            sortDir={sortDir}
            onSort={(f, d) => { setSortField(f); setSortDir(d) }}
            canEdit={canEditDocs}
            duplicatingDocId={duplicatingDocId}
            onArchive={/* existing handler */}
            onUnarchive={/* existing handler */}
            onDelete={setDeleteConfirmDoc}
            onDuplicate={handleDuplicateDoc}
            onMove={setMovingDoc}
            onPublish={/* existing handler */}
          />
        ) : childNodes.length === 0 ? (
          <EmptyNodeState
            view={view}
            canEdit={canEditDocs}
            onCreateDocument={/* contextual handler */}
            onCreateChildCollection={/* contextual handler, null at depth 2 */}
          />
        ) : null}

        {view.kind === 'space_root' && uncategorizedCount > 0 && (
          <UncategorizedSection
            count={uncategorizedCount}
            onOpen={() => navigate({
              to: '/w/$slug/docs/spaces/$spaceId',
              params: { slug: wsSlug, spaceId },
              search: { collection: '__uncollected__' },
            })}
          />
        )}
      </>
    )}
  </div>

  {/* Existing dialogs stay mounted; only the action wiring changes */}
</div>
```

`buildAncestorPath(tree, id)` is a tiny local helper that calls `collectionAncestorChain(tree.topLevel, id)` plus the target node and joins their names with `" / "`. Kept local to the file — not worth a shared util.

### 4.4 `SpaceNodeHeader.tsx`

Handles all three view modes in a single component. Props:

```ts
interface SpaceNodeHeaderProps {
  space: DocsSpace | null
  view: NodeView
  docCount: number          // scope-appropriate count
  subCount: number          // direct sub-collection count
  canEdit: boolean
  hasTranslations: boolean  // space.type === 'external_capable'
  onNavigate: (params: { collectionId: string | null | '__uncollected__' }) => void
  onCreateDocument: () => void
  onCreateChildCollection: (() => void) | null // null when max depth reached or view is uncategorized
  onEditSpace: () => void
  onEditCollection: (() => void) | null
  onDeleteSpace: () => void
  onDeleteCollection: (() => void) | null
  // Translation actions preserved as-is for external-capable spaces
  onOpenTranslations: () => void
  onEditSpaceTranslation: (locale: string) => void
}
```

Structure:
- **Row 1** — `<nav aria-label="Breadcrumb">` rendering ancestors for `collection` / `uncategorized` views. Hidden for `space_root`. Uses `SpaceBreadcrumb` for truncation.
- **Row 2** — `<h1>` with icon + title. For `space_root`: space icon + space name. For `collection`: collection icon + collection name. For `uncategorized`: inbox icon + "Uncategorized".
- **Row 3** — description (optional).
- **Row 4** — count line: `N documents · M sub-collections` (or just `N documents` when no children).
- **Trailing** — `[+]` create menu (perm-gated) and `[⋯]` overflow menu. For external-capable spaces, translation buttons render alongside `[+]`.

### 4.5 `SpaceBreadcrumb.tsx`

Small breadcrumb component that takes a list of `{ label, onClick }` items and renders them with `▸` separators. Middle-ellipsis when overflow: shows `First ▸ … ▸ Second-to-last ▸ Last` with the hidden middle expanded into a tooltip. First and last items never truncate.

### 4.6 `CollectionCard.tsx`

```tsx
interface CollectionCardProps {
  collection: DocsCollection
  docCount: number      // recursive
  subCount: number      // direct
  canEdit: boolean
  onOpen: () => void
  onEdit: () => void
  onDelete: () => void
}
```

Structure:
- Outer `<button>` (body click → `onOpen`).
- Inside: `SidebarCollectionIcon` + name + description (line-clamped) + count line.
- Nested `[⋯]` overflow button absolutely positioned top-right, appears on hover/focus, `stopPropagation` on click. Hidden when `!canEdit`.
- `aria-label="Open {name}, {N} documents, {M} sub-collections"`.
- Keyboard: Enter/Space → `onOpen`. Menu key → opens overflow.

### 4.7 `CollectionCardGrid.tsx`

Wraps cards in a responsive grid: `grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3` with an `<h2>` section header above.

### 4.8 `DocumentsTable.tsx`

Pure extraction of the current inline table. **Zero behavior changes** vs today — same row actions, same filter, same sort, same owner rendering, same duplicate pending state. Only new additions are `showCollectionColumn` and the `collectionAncestorPath` helper.

The table **owns internally** (as module-level helpers, not passed as props):
- `statusColor(status)` — copied from `DocsSpaceDetail.tsx` lines 92–101
- `DOC_STATUS_LABELS` — already imported from `@/lib/docsTypes`
- `timeAgo` — already imported from `@/lib/utils`
- `File01Icon`, `MoreHorizontalIcon`, `ArchiveIcon`, `ArchiveRestoreIcon`, `Copy01Icon`, `Delete01Icon`, `FolderInputIcon`, `SentIcon`, `ArrowUp02Icon`, `ArrowDown02Icon`, `Tick01Icon`, `FilterHorizontalIcon` — from `@/lib/icons`
- `UserAvatar`, `formatAssignableMemberName(owner)` — already imported
- `DropdownMenu` primitives — already imported

Props (full contract so the extraction can't silently drop anything):

```ts
interface DocumentsTableProps {
  // Data
  documents: DocsDocument[]                     // already scoped by parent via scopedDocuments()
  members: AssignableMember[]                   // for owner lookup + avatar rendering
  collectionNames: Map<string, string>          // id → name, for the Collection column
  collectionAncestorPath: (id: string) => string // id → "Getting Started / test col" (new, space-root only)

  // View mode
  showCollectionColumn: boolean                 // true ONLY at space_root. Inside a collection every row
                                                // repeats the same collection name, and in uncategorized
                                                // every row is blank — both are redundant.
  wsSlug: string                                // needed for doc-row navigation target

  // Filter / sort (lifted state owned by DocsSpaceDetail)
  filterStatus: DocStatus | null
  onFilterStatus: (s: DocStatus | null) => void
  sortField: 'updated_at' | 'title' | 'status'
  sortDir: 'asc' | 'desc'
  onSort: (f: 'updated_at' | 'title' | 'status', d: 'asc' | 'desc') => void

  // Permissions
  canEdit: boolean

  // Pending state for the duplicate row action
  duplicatingDocId: string | null

  // Row actions — all thin delegates back to the controller's mutations / dialog setters
  onArchive: (doc: DocsDocument) => void
  onUnarchive: (doc: DocsDocument) => void
  onDelete: (doc: DocsDocument) => void
  onDuplicate: (doc: DocsDocument) => void
  onMove: (doc: DocsDocument) => void
  onPublish: (doc: DocsDocument) => void
}
```

Internal helpers the table handles itself (no need to bubble up):
- Navigating to a doc on title click — uses injected `wsSlug` and `useNavigate()` locally.
- Sorting the `documents` array by the current `sortField`/`sortDir` before rendering.
- Rendering `filterStatus` dropdown with the existing icon + label set.
- Showing "No documents match this filter" empty state when the sorted+filtered result is empty.

Empty state when `documents` is empty before filtering: the parent renders `EmptyNodeState` instead — the table is only rendered when `scopedDocs.length > 0`.

### 4.9 Typed route search

`frontend/src/routes/_authenticated/w/$slug/docs/spaces/$spaceId.tsx` currently has no `validateSearch`. That's fine today because the page casts `location.search` manually, but once URL search becomes the **single source of truth** for view state, it must be typed end-to-end.

**Import-cycle trap to avoid:** the route file imports `DocsSpaceDetail`. If `DocsSpaceDetail.tsx` then imports `Route` from the route file to call `Route.useSearch()`, we get a route → page → route cycle, which at runtime produces a `Cannot access 'Route' before initialization` crash. Instead, the page calls the route-agnostic `useSearch` hook with a `from` string — no import of the route file required. TanStack Router matches by path string at runtime.

Updated route file:

```tsx
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { DocsSpaceDetail } from '@/pages/docs/DocsSpaceDetail'

const spaceDetailSearchSchema = z.object({
  // `__uncollected__` is a sentinel for the dedicated uncategorized view.
  // Any other string is treated as a collection UUID; unknown/missing
  // UUIDs fall back to space root via resolveView + the deep-link guard.
  collection: z.string().optional(),
})

export const Route = createFileRoute('/_authenticated/w/$slug/docs/spaces/$spaceId')({
  validateSearch: spaceDetailSearchSchema,
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <DocsSpaceDetail />
    </div>
  ),
})
```

Controller usage — **`useSearch({ from })`, not `Route.useSearch()`**:

```tsx
import { useSearch } from '@tanstack/react-router'
// NOTE: do NOT import Route from the route file — that creates a cycle.

const { collection: activeCollectionParam } = useSearch({
  from: '/_authenticated/w/$slug/docs/spaces/$spaceId',
})
```

The `from` string is statically checked against the generated route tree, so typos are caught at compile time and the returned search object is typed as `{ collection?: string }` via the `validateSearch` schema. If a future refactor needs to move `Route.useSearch()` into the page, it can be done by extracting the route component into a separate file that imports both `Route` and `DocsSpaceDetail` — but for now `useSearch({ from })` is the idiomatic cycle-free fix.

All navigation calls in subcomponents use the typed search shape:

```ts
navigate({
  to: '/w/$slug/docs/spaces/$spaceId',
  params: { slug: wsSlug, spaceId },
  search: { collection: targetId },        // typed — compile-time checked
})
```

### 4.10 Deep-link fallback

In `DocsSpaceDetail.tsx`:

```ts
useEffect(() => {
  if (!collections || !activeCollectionParam || activeCollectionParam === '__uncollected__') return
  const exists = collections.some((c) => c.id === activeCollectionParam)
  if (!exists) {
    toast.error('That collection is no longer available.')
    navigate({
      to: '/w/$slug/docs/spaces/$spaceId',
      params: { slug: wsSlug, spaceId },
      search: {},
      replace: true,
    })
  }
}, [collections, activeCollectionParam, wsSlug, spaceId, navigate])
```

Only runs when `collections` has loaded. No flicker because `resolveView` returns `loading` while collections are pending.

### 4.11 `globalCreateStore` extension

**Problem:** `globalCreateStore` has `initialSpaceId` (for the "which space should this collection live in" field) and `initialCollectionId` (for the "which collection should this document go into" field) but has no field for **"which parent collection should this new collection nest under."** `GlobalCreateModals.tsx` currently passes only `spaceId` into `CreateCollectionDialog`, even though the dialog already accepts `defaultParentCollectionId`.

Result: if we call `openCreate('docs_collection', { spaceId, parentCollectionId })` from a "+ Sub-collection" CTA, the store silently drops `parentCollectionId`, the dialog receives `undefined`, and a top-level collection gets created instead. That's a silent regression of the current `DocsArrangeTree` "+ sub-collection" button, which reaches the dialog via a different path.

**Fix:** extend the store with one new field.

```ts
// frontend/src/stores/globalCreateStore.ts
interface GlobalCreateState {
  // ... existing fields
  initialParentCollectionId: string | undefined
  openCreate: (
    modal: Exclude<CreateModal, null>,
    options?: {
      teamId?: string
      ownerMemberId?: string
      sprintId?: string
      spaceId?: string
      collectionId?: string
      parentCollectionId?: string   // new
    },
  ) => void
  closeCreate: () => void
}
```

And forward it in `GlobalCreateModals.tsx`:

```tsx
<CreateCollectionDialog
  wsId={wsId}
  spaceId={initialSpaceId}
  defaultParentCollectionId={initialParentCollectionId}
  open={activeModal === 'docs_collection'}
  onOpenChange={(open) => { if (!open) closeCreate() }}
/>
```

**Alternative considered:** render `CreateCollectionDialog` directly from `DocsSpaceDetail` with local `createCollectionParent` state, bypassing the store. Rejected because (a) the global create flow is reached from the sidebar and the global "+" button, not just this page, and adding another dialog instance risks double-mount bugs, and (b) extending the store is a two-line change, while the local-dialog approach duplicates state that already lives in the store.

**Scope of the fix:**
- `frontend/src/stores/globalCreateStore.ts` — add field, destructure in `openCreate`, reset in `closeCreate`.
- `frontend/src/components/pm/GlobalCreateModals.tsx` — read `initialParentCollectionId` from the store and forward to `CreateCollectionDialog`.
- `frontend/src/pages/docs/DocsSpaceDetail.tsx` — call `openCreate('docs_collection', { spaceId, parentCollectionId: currentCollectionId })` from the header `[+]` and empty-state CTAs when the current view is `collection`.
- No changes needed in `CreateCollectionDialog.tsx` — it already accepts `defaultParentCollectionId`.
- No changes needed in `DocsArrangeTree.tsx` — its "+ sub-collection" button renders `CreateCollectionDialog` directly.

---

## 5. Data rules (counts, filters, permissions)

### 5.1 Doc counts

| Where | Count |
|---|---|
| Space root header | Recursive docs in space (respects archive filter) |
| Space root card | Recursive docs in that subtree |
| Collection header | Direct docs in that collection |
| Collection card (sub-collection) | Recursive docs in that subtree |
| Uncategorized header | Count of `collection_id == null` docs |

### 5.2 Sub-collection counts

| Where | Count |
|---|---|
| Space root header | Direct top-level collections |
| Space root card | Direct children of that collection |
| Collection header | Direct children of that collection |

Direct only everywhere — "3 subs" on a card means 3 immediate children, not 12 descendants.

### 5.3 Filter scope and the archive rule (unchanged from today)

**Current behavior (must be preserved):**
- `useDocsDocuments` is called with `include_archived: 'true'` so archived docs are always fetched.
- When `filterStatus === null`, the table shows every doc (including archived).
- When `filterStatus === 'draft'`, only drafts show.
- When `filterStatus === 'archived'`, only archived show.
- When `filterStatus === 'published'`, only published show.

**Plan rule:** `scopedDocuments(view, tree, filter)` is the only place this predicate lives. Both the table and the count helpers call it. The helper reads from the pre-partitioned `CollectionTree`, not from raw document arrays. Signature:

```ts
function scopedDocuments(
  view: NodeView,
  tree: CollectionTree | null,
  filter: { status: DocStatus | null },
): DocsDocument[] {
  if (!tree) return []
  const inScope =
    view.kind === 'space_root'
      ? [...flattenCollectionTree(tree.topLevel).flatMap((node) => node.documents), ...tree.uncategorizedDocuments]
      : view.kind === 'collection'
        ? view.node.documents
        : view.kind === 'uncategorized'
          ? tree.uncategorizedDocuments
          : []
  if (!filter.status) return inScope
  return inScope.filter(d => d.status === filter.status)
}
```

`countDocsInSubtree` and `countDirectDocs` take the same `filter` object and apply the exact same status predicate to the docs they read from the tree. `countDirectChildren` is unaffected by status filtering because child collection counts do not depend on document status. The result is that header counts and visible rows always agree — including the archive-default case.

**Consequence:** the header's count line always matches what the user sees. If the filter is off and there are 2 published + 1 archived doc, the header shows "3 documents"; if the user filters to "published", it shows "2 documents". This is an intentional improvement — today the header doesn't refresh at all when you filter.

`sortField`/`sortDir` are applied inside `DocumentsTable.tsx` after `scopedDocuments()`.

### 5.4 Permissions

- `canEditDocs = false` → no `[+]` button, no `[⋯]` menu items for create/edit/delete, no card overflow menus, no empty-state CTAs. All read actions (open doc, drill, filter) remain.
- Owner-only actions (delete space) gate via existing permission checks inside the overflow menu.

---

## 6. Testing

### 6.1 Unit tests (`nodeSelection.test.ts`)

All tests build their fixtures by calling `buildCollectionTree` on known collection/document arrays so the tests exercise the real tree shape, not a fake one.

**`resolveView`**
- `resolveView(null, null)` → `loading` (tree not yet built).
- `resolveView(null, tree)` → `space_root`.
- `resolveView('abc', null)` → `loading`.
- `resolveView('abc', tree)` where `abc` exists → `{ kind: 'collection', node, ancestors }` with ancestors ordered root-first.
- `resolveView('abc', tree)` where `abc` is a depth-2 leaf → `ancestors.length === 2`.
- `resolveView('missing', tree)` → `space_root` (deleted-id fallback).
- `resolveView('__uncollected__', null)` → `loading`.
- `resolveView('__uncollected__', tree)` → `uncategorized`.

**`directChildrenOfView`**
- `space_root` view → returns `tree.topLevel` in the exact order `buildCollectionTree` produced (same order as sidebar).
- `collection` view → returns `view.node.children` in the same position order.
- `uncategorized` view → returns `[]`.
- `loading` view → returns `[]`.

**`scopedDocuments` — reads from the pre-partitioned tree**
- `space_root` with no filter → returns every doc in the space (including archived).
- `collection` view → returns `view.node.documents` (direct docs only — sub-collection docs are not pulled up).
- `uncategorized` view → returns `tree.uncategorizedDocuments`.
- Empty tree → returns `[]` for every view kind.

**Status filter (the §5.3 predicate)**
- `filter.status === null` → every doc in scope is included (archived included — matches current `include_archived=true` fetch behavior).
- `filter.status === 'draft'` → only drafts.
- `filter.status === 'published'` → only published.
- `filter.status === 'archived'` → only archived.
- Predicate applied identically in `scopedDocuments`, `countDocsInSubtree`, `countDirectDocs`.

**`countDocsInSubtree`**
- Leaf node with N direct docs → returns N.
- Parent with M direct docs + child having K docs → returns M + K.
- Depth-2 tree → returns the full recursive sum.
- Same inputs with `filter.status === 'published'` → returns only published docs in the subtree.

**`countDirectDocs`**
- `(node, tree, filter)` → `node.documents` length after filter.
- `(null, tree, filter)` → `tree.uncategorizedDocuments` length after filter.
- `(null, null, filter)` → `0` (loading state).

**`countDirectChildren`**
- `(node, tree)` → `node.children.length`.
- `(null, tree)` → `tree.topLevel.length`.
- `(null, null)` → `0`.

### 6.2 Manual test matrix

1. Click space name in sidebar → lands on space root with cards + docs table.
2. Click a card → drills to collection view, URL updates, sidebar highlights.
3. Click a breadcrumb segment → returns to that level, URL updates.
4. Browser back button → retraces every navigation step.
5. Direct deep-link to `?collection=<uuid>` → lands on collection view.
6. Direct deep-link to `?collection=<deleted-uuid>` → toast + redirect to space root.
7. Leaf collection with zero docs → empty state + CTAs.
8. Click "+ Add document" CTA → opens global create modal with correct `collectionId`.
9. Click "+ Add sub-collection" CTA → opens `CreateCollectionDialog` with correct `defaultParentCollectionId`.
10. At depth 2 collection → "Add sub-collection" is hidden (max depth).
11. Card `[⋯]` → edit collection dialog opens pre-filled.
12. Card `[⋯]` → delete collection with confirm flow.
13. External-capable space root → translation buttons render, dialogs open.
14. View-only user → no `[+]`, no `[⋯]` items for create/edit/delete, no card overflow menus.
15. Uncategorized section at space root → clicking `View →` drills to uncategorized view.
16. Uncategorized view → no sub-collection section, no "Add sub-collection" button.
17. Filter status / sort headers in the table work at every view mode.
18. Archive / unarchive / delete / duplicate / move / publish row actions work.
19. Realtime update (edit another doc elsewhere) → table refreshes without reload.
20. Long collection name in breadcrumb (4+ levels at max) → middle ellipsis + tooltip.
21. Keyboard: Tab through breadcrumb → cards → card overflow button → table. Enter triggers correct action.
22. Screen reader announces h1 + section h2 + card aria-labels + breadcrumb landmark.

### 6.3 Regression watchlist

- Arrange mode (separate route) continues to work.
- Sidebar tree rendering, indent, tooltip, truncation detection.
- Widget / public help-center routes untouched.
- `useRealtimeSync` invalidation still fires for collection CRUD.
- Typecheck passes on the 4 pre-existing broken files unaffected (we are not touching them).

---

## 7. Execution plan (commit-by-commit)

Each commit is independently reviewable and revertable.

### Commit 1 — Enabling changes: route typing + store extension + helpers
- Extend `globalCreateStore.ts` with `initialParentCollectionId` (§4.11).
- Update `GlobalCreateModals.tsx` to forward it to `CreateCollectionDialog.defaultParentCollectionId`.
- Add `validateSearch` to `$spaceId.tsx` route file (§4.9).
- Add `frontend/src/pages/docs/spaceDetail/nodeSelection.ts` with `NodeView`, `resolveView`, `directChildrenOfView`, `scopedDocuments`, `countDocsInSubtree`, `countDirectDocs`, `countDirectChildren`. **All helpers delegate to `buildCollectionTree` from `docsCollectionTree.ts`** — no parallel tree logic.
- Add `nodeSelection.test.ts` covering the cases listed in 6.1, including the archive-filter behavior.
- No consumers in `DocsSpaceDetail.tsx` yet. Existing sidebar / Arrange use of `buildCollectionTree` is unaffected. Pure additive — zero runtime impact on the page.

### Commit 2 — Extract `DocumentsTable`
- Move current inline table (filter row, sort headers, rows, row actions, status pills, empty-state) into `spaceDetail/DocumentsTable.tsx`.
- `DocsSpaceDetail.tsx` imports it and passes current state through props.
- Pills, breadcrumb strip, and drill-down logic still present — this commit changes no UX.
- Verify: click every row action before and after. Filter + sort still behave identically.

### Commit 3 — Add new layout components (not yet wired)
- Add `SpaceBreadcrumb.tsx`, `SpaceNodeHeader.tsx`, `CollectionCard.tsx`, `CollectionCardGrid.tsx`, `UncategorizedSection.tsx`, `EmptyNodeState.tsx`, `NodeSkeleton.tsx`.
- Not yet referenced from `DocsSpaceDetail.tsx`.
- Storybook / screenshot testing if available, else manual import in a scratch route.

### Commit 4 — Wire the new layout
- Rewrite `DocsSpaceDetail.tsx` render tree to the new layout.
- Delete pill strip, drill-down logic, inline header, `setActiveCollection` local setter, `CollectionTabIcon`.
- URL becomes single source of truth (remove `useState(collectionParam)`).
- Add deep-link fallback `useEffect`.
- Wire all existing dialogs to the new header's action handlers.
- Manual-test the full matrix from 6.2.

### Commit 5 — Polish
- Dark mode pass (card borders, empty state).
- Responsive grid breakpoints verified at `sm`, `md`, `lg`, `xl`.
- Accessibility pass: heading levels, breadcrumb landmark, `aria-label` audits, keyboard focus order.
- Empty-state wording review.
- Any final visual tweaks.

---

## 8. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Collection view with many docs loses vertical space to cards | Sub-collection section collapses when empty → leaf views are card-free and use full height for the table. Branches need the extra context anyway. |
| Doc count mismatch between card (recursive) and header (direct) confuses users | Header count line includes both numbers when relevant (`2 documents · 3 sub-collections`); card counts are labelled "N docs" with the implicit "subtree" scope. Document in help if it comes up. |
| Deep-link flash if `resolveView('uuid', undefined)` fell through to `space_root` | `resolveView` explicitly returns `loading` when collections are unresolved; render a skeleton during that state. |
| Breaking realtime or filter-state behaviour during table extraction | Commit 2 is a pure extraction with no UX change, manually verified before proceeding to commit 3. |
| External-space translation UI regresses | Commit 4 explicitly wires translation buttons into `SpaceNodeHeader`; manual-test matrix item 13 covers this. |
| Browser back/forward breaks because of leftover local `activeCollection` setState | URL is the single source of truth from commit 4 onward; all drill actions call `navigate()`. Removing `useState` eliminates the drift class of bugs. |
| Commit 2 touches a large inline block and introduces a subtle regression | Extract with zero behavior change first. Rely on manual test matrix and the fact that the extraction is structural, not logical. |

---

## 9. Open questions for review

1. Should the space root view's Documents section be **all docs in space** (current proposal) or **only uncollected + top-level docs**? The current proposal matches today's "flat view of everything" behaviour, which users already know. Alternative: show only top-level direct + uncollected to avoid duplication with the cards. Recommendation: keep all docs in space — users use the table for "find a doc I know exists" regardless of where it lives.
2. Should card doc counts be **recursive** (proposed) or **direct**? Recursive is more useful for scanning ("which area is big?"). Direct is simpler and matches the header rule. Recommendation: recursive on cards, direct in headers — document the split in help tooltips if users ask.
3. Should the collection view include a "Show all documents in subtree" toggle? Out of scope for v1. Can add later if users ask.
4. Should Uncategorized be a section at the bottom of the space root view (proposed) or a pinned card at the top of the Collections grid? Section keeps Collections grid semantically clean. Pinned card makes uncategorized feel more "first-class". Recommendation: section at bottom — aligns with the "edge case" nature of uncategorized docs.
5. Should the card overflow menu include "Move collection to another parent"? Existing edit dialog already supports this via the parent picker. Recommendation: skip — keep card menu minimal (Edit, Delete), use dialog for moves.

---

## 10. Files touched summary

**New:**
- `frontend/src/pages/docs/spaceDetail/nodeSelection.ts`
- `frontend/src/pages/docs/spaceDetail/nodeSelection.test.ts`
- `frontend/src/pages/docs/spaceDetail/SpaceNodeHeader.tsx`
- `frontend/src/pages/docs/spaceDetail/SpaceBreadcrumb.tsx`
- `frontend/src/pages/docs/spaceDetail/CollectionCardGrid.tsx`
- `frontend/src/pages/docs/spaceDetail/CollectionCard.tsx`
- `frontend/src/pages/docs/spaceDetail/DocumentsTable.tsx`
- `frontend/src/pages/docs/spaceDetail/UncategorizedSection.tsx`
- `frontend/src/pages/docs/spaceDetail/EmptyNodeState.tsx`
- `frontend/src/pages/docs/spaceDetail/NodeSkeleton.tsx`

**Modified:**
- `frontend/src/pages/docs/DocsSpaceDetail.tsx` — rewritten to consume new layout, URL becomes source of truth
- `frontend/src/routes/_authenticated/w/$slug/docs/spaces/$spaceId.tsx` — add `validateSearch` (§4.9)
- `frontend/src/stores/globalCreateStore.ts` — add `initialParentCollectionId` (§4.11)
- `frontend/src/components/pm/GlobalCreateModals.tsx` — forward `initialParentCollectionId` to `CreateCollectionDialog` (§4.11)

**Unchanged:**
- `frontend/src/components/layout/sidebar/DocsSpacesNav.tsx` (sidebar)
- `frontend/src/components/docs/DocsArrangeTree.tsx` (arrange mode)
- `frontend/src/components/docs/CreateCollectionDialog.tsx` (dialog — already accepts `defaultParentCollectionId`)
- `frontend/src/components/docs/docsCollectionTree.ts` (shared tree helpers — **reused**, not reimplemented)
- All widget / public help-center components
- All backend code
