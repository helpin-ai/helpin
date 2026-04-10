# Hierarchical Docs Collections — Manual QA Checklist

This is the reusable QA checklist for the hierarchical docs collections
rollout (plan `2026-04-10-hierarchical-docs-collections.md`). Run the
checklist on staging after every merge that touches docs, help center,
or widget help surfaces.

## Backend & data

- [ ] Applying the `202604100001_docs_collection_tree` migration on a
      staging snapshot succeeds when the snapshot contains duplicate
      `(workspace_id, slug)` pairs (they should be renamed `-2`, `-3`,
      … with no redirects emitted for the losers).
- [ ] After migration, `idx_docs_collections_ws_slug_alive` is the
      partial unique index and `idx_docs_collection_space_pos` is gone.
- [ ] `docs_collections.parent_collection_id` and `depth` columns exist
      and default to `NULL` and `0` respectively.
- [ ] Inserting a collection with `depth >= 3` via the service fails
      with `ErrDocsCollectionDepthExceeded` → HTTP 400.
- [ ] Renaming a collection to a slug already in use returns
      `ErrDocsCollectionSlugTaken` → HTTP 409.

## Internal docs (admin)

- [ ] Create a top-level collection — depth 0, slug unique.
- [ ] Create a sub-collection under the top-level — depth 1, parent
      breadcrumb visible.
- [ ] Create a grandchild (depth 2). Creating a fourth level must be
      rejected by the UI with a clear error toast.
- [ ] Move a collection to a new parent via the tree picker — the
      Arrange tree updates, descendants keep their relative depths,
      both old and new sibling buckets normalise to contiguous 0..N.
- [ ] Reorder siblings by drag/drop inside one bucket — the other
      buckets stay untouched.
- [ ] Delete a middle collection — its children reparent to the
      deleted node's parent, its direct articles move up to that same
      level, no content is lost.
- [ ] Delete a top-level collection — its children become top-level
      and its direct articles become uncategorised.
- [ ] Move an article from one nested collection to another — the
      source bucket compacts, the target bucket appends.

## Public help center

- [ ] Browse `/:space` — the landing page renders a card grid of
      top-level collections. No silent redirect to the first
      collection.
- [ ] Click into a nested collection — the page shows child
      collection tiles above the direct article list, and the
      breadcrumb shows the ancestor chain top-down.
- [ ] Navigate to `/:collection-slug` where the collection has both
      child collections and direct articles — both sections render.
- [ ] Sidebar (`NavTree`) renders nested collections with depth
      indentation and expands ancestors when the active article is
      deeply nested.
- [ ] Article prev/next pager walks tree order: after the last article
      of a parent collection, next goes to the first article of the
      first child collection.
- [ ] Legacy flat collection URLs still resolve — the canonical
      `/:collection-slug/:article-slug` shape is unchanged.

## Redirects

- [ ] Rename a collection — visiting the old collection URL
      (`/:old-slug`) returns a 301 to `/:new-slug`.
- [ ] Rename a collection — visiting `/:old-slug/:article-slug` for
      every directly published article returns 301 to
      `/:new-slug/:article-slug`.
- [ ] Move an article between two collections —
      `/:old-collection-slug/:article-slug` redirects to
      `/:new-collection-slug/:article-slug` with type
      `auto_article_move`.
- [ ] Move the same article back (A → B → A) — there is exactly
      one redirect row workspace-wide afterwards, and the old
      intermediate redirect is gone (no cycles).
- [ ] Manual-type redirects in `docs_redirects` are unaffected by the
      tree rollout.

## Widget

- [ ] Open the widget on a space — only top-level collections appear;
      each tile shows a combined article count and sub-collection
      count.
- [ ] Click a collection with children — the drilldown view shows
      child collection tiles followed by direct articles, and the
      header subtitle shows the ancestor chain ("Root › Middle").
- [ ] Back button goes to the parent collection (or space root when
      already at depth 0).
- [ ] Open an article from a nested collection — article renders and
      the back button still returns to the nested collection view.

## Search

- [ ] Search for an article that lives in a nested collection — the
      result row shows the full ancestor path ("Root / Middle /
      Current") in the muted caption.
- [ ] Search for an article in a top-level collection — the result
      shows just the collection name (no ancestor path).
- [ ] Search result click navigates to the canonical article URL.
- [ ] Search in French (or another configured locale) — each result's
      ancestor path uses the localised names where available, and
      falls back to source names where a locale translation is
      missing.

## Settings

- [ ] Help Center homepage featured cards: nested collections show
      a small breadcrumb line ("Parent / Middle") above the collection
      name in both the selected and unselected rows.
- [ ] Help Center translations table: collections appear depth-first
      under their owning space, indented by depth level.
- [ ] Toggling / reordering featured cards continues to store cards
      by collection slug so the public landing page reflects the
      change on next deploy.

## Realtime

- [ ] With two browser sessions viewing the same workspace, rename or
      move a collection in session A — session B's Arrange tree,
      docs list, and help center nav refresh within a few seconds.
- [ ] Deleting a collection in session A flattens descendants in
      session B without a manual refresh.

## Regression — nothing pre-existing should break

- [ ] Flat-collection workspaces (none of the collections have
      parents) behave identically to before: same URLs, same nav,
      same search, same widget.
- [ ] The HelpScout importer still runs end to end and creates
      collections at depth 0.
- [ ] The existing `docs_slug_aliases` based redirect path keeps
      resolving old article slugs that predate the `docs_redirects`
      consolidation.
