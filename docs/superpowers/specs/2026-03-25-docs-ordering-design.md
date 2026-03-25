# Docs Ordering Design

## Summary

Helpin needs a single canonical ordering model for docs spaces, collections, and articles. That order must be shared by:

- internal docs views
- the docs sidebar
- the public help center

The correct UX is not a settings-only feature. Ordering belongs to the Docs module because it is content structure, not public-site configuration.

This design makes the `All Docs` page the single hierarchy-management surface by adding an explicit `Arrange` mode with drag-and-drop and auto-save.

## Goals

- Let users reorder spaces, collections, and articles from one place.
- Make internal docs and the public help center show the same order.
- Keep the default docs browsing experience clean and fast.
- Use explicit, canonical `position` fields instead of separate public-only ordering config.

## Non-Goals

- Creating a separate public-only information architecture editor in Settings.
- Supporting cross-space or cross-collection drag moves in v1.
- Replacing the existing document move flow with drag-and-drop in this first iteration.
- Keeping `HelpcenterSpaceNavConfig.order` as the primary ordering source.

## Product Decision

### Ownership

The Docs module owns ordering.

Settings should remain responsible for:

- branding
- domain/subdomain
- homepage cards
- public visibility
- other help-center-specific configuration

If Settings needs to reference structure later, it should deep-link back to Docs rather than duplicating structure management there.

### Primary surface

Ordering is managed from:

- [`frontend/src/pages/docs/DocsHome.tsx`](../../../../frontend/src/pages/docs/DocsHome.tsx)

This page already acts as the highest-level hierarchy view. It shows all spaces and can be expanded to show collections and documents. That makes it the right place to manage information architecture across the whole docs system.

## UX Design

### Default mode

The `All Docs` page remains browse-first by default:

- spaces remain collapsible
- rows remain clickable
- users can open spaces and documents normally
- no drag handles are visible

This preserves the current lightweight behavior and avoids clutter.

### Arrange mode

Add a page-level toggle in the header:

- `Arrange`
- when active: `Done arranging`

When arrange mode is enabled:

- drag handles appear on reorderable rows
- row clicks stop navigating
- chevrons still expand and collapse spaces
- the page becomes a hierarchy editor rather than a browser

Show a compact helper line near the toggle:

- `Drag spaces, collections, and articles to reorder`

### Structure shown in arrange mode

The hierarchy should be displayed inline on the `All Docs` page:

- Internal spaces section
- External spaces section
- each space row can expand
- collections appear under the space
- articles appear under their collection
- an `Uncategorized` bucket appears for documents with no collection

The user should be able to understand and edit structure without leaving the page.

### Drag behavior

V1 drag rules should be intentionally narrow:

- spaces can reorder within their current section
  - internal within internal
  - external within external
- collections can reorder within their space
- articles can reorder within their current bucket
  - within a collection
  - within the space’s uncategorized bucket

V1 should not support:

- moving spaces between internal and external
- dragging collections between spaces
- dragging articles between collections
- dragging articles between spaces

Those are move operations, not reorder operations, and they already have separate UX concepts.

### Save behavior

Reordering should auto-save on each drop.

Behavior:

- optimistic UI update immediately after drop
- backend request persists the new order
- lightweight success toast such as `Order updated`
- revert the optimistic change if the request fails

`Done arranging` only exits arrange mode. It is not a save button.

This keeps the interaction lightweight and consistent with drag-and-drop ordering in modern content products.

## Data Model

### Spaces

Spaces already have a canonical order field:

- `DocsSpace.position`

This should remain the source of truth for:

- docs home ordering
- sidebar ordering
- public help center space ordering

### Collections

Collections already have a canonical order field:

- `DocsCollection.position`

This should remain the source of truth for:

- collection ordering in docs views
- collection ordering in the public help center

### Articles

Articles do not currently have a canonical order field. Add:

- `DocsDocument.position`

This should represent order within a single document bucket:

- a collection bucket when `collection_id` is set
- an uncategorized bucket when `collection_id` is null, scoped to a space

`DocsDocument.position` becomes the canonical article order for:

- docs home expanded hierarchy
- docs sidebar if articles are shown
- public help center collection pages
- public help center space navigation

### Help center config

`HelpcenterSpaceNavConfig.order` should no longer be the primary ordering mechanism.

Options:

- retain it only for hide/show state
- deprecate its `order` field later once consumers are migrated

The canonical order should come from content records, not from a separate help-center config document.

## Backend Design

### Schema changes

Add `position` to `docs_documents`.

Recommended index:

- `(space_id, collection_id, position)`

This supports ordered reads for both collection and uncategorized buckets.

### Create behavior

When a document is created, assign the next append position in its bucket:

- if `collection_id` exists, append within that collection
- otherwise append within uncategorized documents for that space

This avoids relying on timestamps for default order.

### Move behavior

The existing document move flow should be extended so that moving a document:

- removes it from its source bucket
- appends it to the end of the target bucket by default

This keeps ordering contiguous even when the document is moved through the existing move dialog.

### Reorder endpoints

Add dedicated reorder endpoints.

Recommended endpoint shape:

1. spaces reorder
- reorder all spaces within a section

2. collections reorder
- reorder collections within a space

3. documents reorder
- reorder documents within a single bucket

Bulk reorder payloads are preferable to one-row `position` patch calls because they:

- make drag-drop persistence simpler
- reduce normalization edge cases
- keep the backend in control of contiguous positions

### Normalization rules

After every reorder or move, positions should be contiguous starting from zero inside the relevant bucket.

This prevents duplicate or sparse positions and keeps internal/public ordering deterministic.

## Read Model Changes

### Internal docs views

Update internal docs queries so ordering is based on canonical position:

- spaces ordered by `DocsSpace.position`
- collections ordered by `DocsCollection.position`
- documents ordered by `DocsDocument.position`

Current internal document list queries that default to `is_pinned DESC, updated_at DESC` should not remain the canonical ordering for hierarchy views.

### Public help center

Update help center queries so public navigation and collection pages also use canonical position:

- spaces by `DocsSpace.position`
- collections by `DocsCollection.position`
- articles by `DocsDocument.position`

Pinned articles should not override canonical order in the public hierarchy if the product requirement is “same order everywhere.”

## Frontend Design

### Page behavior

The `All Docs` page gets an `Arrange` mode state.

In arrange mode:

- reuse `@dnd-kit`
- render sortable rows with drag handles
- disable navigation actions on sortable rows
- keep expand/collapse available for spaces

### Row model

The frontend should construct a tree view model:

- section
- space
- collection
- article
- uncategorized bucket

This keeps rendering and drag rules explicit and testable.

### Scope boundaries

`DocsSpaceDetail` should remain a browse/manage page, not the primary reorder surface.

It may later reflect canonical order, but v1 ordering management belongs only to `All Docs`.

## Error Handling

- If a reorder save fails, the UI should revert to the previous order.
- If data refreshes while arrange mode is active, prefer the server as source of truth after a successful mutation.
- If an item disappears because it was deleted in another session, show an error toast and refresh the affected subtree.

## Testing

### Backend

Add tests for:

- document create appends to the end of the correct bucket
- document move normalizes source and target bucket positions
- space reorder persists contiguous positions
- collection reorder persists contiguous positions
- document reorder persists contiguous positions
- public space, collection, and article queries return canonical order

### Frontend

Add tests for:

- arrange toggle enters and exits hierarchy-edit mode
- drag handles appear only in arrange mode
- dropping a space reorders optimistically and persists
- dropping a collection reorders within its space
- dropping an article reorders within its bucket
- navigation is disabled for sortable rows during arrange mode
- failed reorder reverts the optimistic state

## Rollout

### V1

- `All Docs` owns ordering
- drag-and-drop reorder for spaces, collections, and articles
- auto-save on each drop
- canonical shared order across internal docs and public help center

### Follow-ups

Potential future enhancements:

- cross-collection drag move
- cross-space drag move
- reorder support from `DocsSpaceDetail`
- hide/show controls for public help center structure
- undo action in the reorder toast
