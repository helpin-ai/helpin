# Featured cards with subcollections

> Historical design, source-compared on 2026-09-17. This page explains the
> original nested-collection picker for contributors. Subcollection selection is
> implemented, with ordering and breadcrumb differences from the proposal below.

## Current implementation

[HelpcenterTab](../../frontend/src/components/settings/HelpcenterTab.tsx) contains
the inline searchable `CollectionTreeMultiSelect`. It passes all loaded space
collections through [tree options](../../frontend/src/components/docs/CollectionTreePicker.tsx),
uses 12px depth indentation, toggles each collection independently, and keeps the
popover open. Only selected collections render as sortable featured rows.

The zero-selection trigger reads **Select collections**, not **Add collections**.
An empty space displays “No collections in this space.” outside the picker.
Although the sortable row accepts an optional `pathLabel`, the current call site
does not pass it; ancestor breadcrumbs are therefore not shown above selected
row names. Tree paths are still used for picker search.

Newly selected cards are inserted according to the space's natural collection
order, rather than always appended. Existing featured cards are still ordered
through `orderCollectionsForFeaturedCards`, and drag reordering remains available.
The original append-only and selected-row breadcrumb requirements are not accurate
descriptions of this implementation.

The [backend resolver](../../server/internal/service/docs_helpcenter.go) resolves
collection cards by ID or by space plus ID/slug, without excluding subcollections.
The [tree migration](../../server/internal/dbmigrate/sql/202604100004_docs_collection_tree.sql)
defines the non-deleted workspace/slug unique index; its presence does not prove
it has been applied to a particular deployment. No browser or deployment check
was performed for this source comparison.

## Original design record

**Date:** 2026-04-29
**Status:** Approved

## Problem

In Settings > Help Center > Home page > Featured cards, when a space is selected, only top-level collections are shown. Sub-collections are excluded by a filter (`!c.parent_collection_id`). However, the public help center can display sub-collections on the homepage. Admins have no way to feature a sub-collection.

## Solution

Replace the current "all collections with inline checkboxes" UI with a two-part layout:

1. **Multi-select tree dropdown** — shows the full collection hierarchy for the selected space, with checkboxes for toggling collections on/off as featured cards
2. **Selected collection rows** — only featured collections appear below the dropdown as `SortableFeaturedCollectionRow` items (drag handle, icon picker, description field, reorderable)

## Scope

**Frontend only** — no backend or API changes required.

The backend already:
- Loads all collections (including sub-collections) via `ListBySpace()`
- Resolves sub-collections in `resolveFeaturedCardCollection()`
- Has no `parent_collection_id` filter in the enrichment service

## Design

### UX Flow

1. Admin selects a space from the existing homepage space dropdown
2. Below the space selector, a "Add collections" dropdown shows the full collection tree for that space
3. Collections are displayed with depth indentation (12px per level) and folder icons, matching the existing `CollectionTreePicker` visual style
4. Each item has a checkbox — checked items are featured cards
5. Already-featured collections appear checked in the dropdown
6. Below the dropdown, only selected/featured collections render as `SortableFeaturedCollectionRow` rows with:
   - Drag handle for reordering
   - Icon picker
   - Collection name (with breadcrumb path label for sub-collections, e.g. "Getting Started / Setup")
   - Description input field
7. Toggling off in the dropdown removes the row; toggling on adds it

### Component Changes

#### `HelpcenterTab.tsx`

1. **Remove** the `topLevelCollections` filter at line 928-929:
   ```typescript
   // REMOVE:
   const topLevelCollections = spaceCollections.filter(
     (c) => !c.parent_collection_id,
   );
   ```
   Use all `spaceCollections` instead.

2. **Replace** the current collection list (which shows ALL collections with inline checkboxes) with:
   - A `CollectionTreeMultiSelect` dropdown at the top
   - Only featured/selected collections rendered as `SortableFeaturedCollectionRow` below

3. **Add `pathLabel`** to `SortableFeaturedCollectionRow` for sub-collections — show ancestor breadcrumbs (e.g. "Parent / Middle") above the collection name so admins know where the sub-collection lives in the hierarchy.

#### New: `CollectionTreeMultiSelect` component

A multi-select variant of the existing `CollectionTreePicker`. Can be built inline in `HelpcenterTab.tsx` or extracted to a shared component.

**Reuses:**
- `buildCollectionTreeOptions()` from `CollectionTreePicker.tsx` for tree data
- `Command`/`CommandInput`/`CommandList` from cmdk for searchable dropdown
- Same depth-indentation and folder icon styling

**Differs from `CollectionTreePicker`:**
- Multi-select (checkboxes) instead of single-select (radio)
- Popover stays open after selection (user can toggle multiple)
- Trigger shows count of selected items (e.g. "3 collections selected") instead of a single name

**Props:**
```typescript
interface CollectionTreeMultiSelectProps {
  collections: DocsCollection[]
  spaceId: string
  selectedIds: string[]
  onToggle: (collectionId: string) => void
}
```

### Data Flow

```
spaceCollections (all, including sub-collections)
  → buildCollectionTreeOptions(spaceId, spaceCollections)
  → CollectionTreeMultiSelect dropdown (tree with checkboxes)
  → user toggles → onToggle adds/removes featured card
  → selectedIds → filter to featured collections
  → SortableFeaturedCollectionRow[] (drag, icon, description)
  → save → same HomepageFeaturedCard[] payload as today
```

### Selected Rows Ordering

Selected/featured collections below the dropdown are ordered by their position in the saved `homepage_featured_cards` array (i.e. drag order). `orderCollectionsForFeaturedCards()` is still used to derive this order from the saved cards. Newly added collections append to the end.

### `pathLabel` for Sub-Collections

The existing `pathBySourceId` map (built via `buildCollectionTreeOptions`) already computes ancestor breadcrumbs for all collections including sub-collections. Since we now pass all `spaceCollections` (not just top-level), `pathBySourceId` will correctly populate for sub-collections. The `pathLabel` is shown above the collection name in `SortableFeaturedCollectionRow` (e.g. "Getting Started" for a sub-collection under "Getting Started").

### Edge Cases

- **Parent/child selection is independent** — selecting a parent does not auto-select its children, and vice versa. Each collection is toggled independently.
- **Empty space** — if the space has zero collections, the dropdown renders with a "No collections." empty state (same as `CollectionTreePicker`).
- **Trigger label** — singular: "1 collection selected", plural: "3 collections selected", zero: "Add collections".
- **Slug uniqueness** — collection slugs have a partial unique index on `(workspace_id, slug)` for non-deleted rows. Within a single space, slug collisions cannot occur, so `findFeaturedCardForCollection` matching by slug remains safe.
- **Scrollable list** — the dropdown uses `CommandList` which has a default max-height with scroll, same as `CollectionTreePicker`.

### What Stays the Same

- `HomepageFeaturedCard` type — no changes
- Backend API contract — no changes
- `orderCollectionsForFeaturedCards()` — still used for ordering selected rows by saved card position
- `findFeaturedCardForCollection()` — works with any collection ID
- `SortableFeaturedCollectionRow` — same component, just receives `pathLabel` for sub-collections
- Public help center rendering — already handles any `link_type='collection'` card
- Backend enrichment — already resolves sub-collections

## Files Modified

| File | Change |
|------|--------|
| `frontend/src/components/settings/HelpcenterTab.tsx` | Remove top-level filter, add `CollectionTreeMultiSelect`, pass `pathLabel` to rows, only render selected rows |

## Files Created

None — `CollectionTreeMultiSelect` will be built inline in `HelpcenterTab.tsx` (single use case). Can be extracted later if needed elsewhere.
