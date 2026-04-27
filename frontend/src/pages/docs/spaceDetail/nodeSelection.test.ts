import { describe, expect, it } from 'vitest'
import { buildCollectionTree } from '@/components/docs/docsCollectionTree'
import type { DocsCollection, DocsDocument, DocStatus } from '@/lib/docsTypes'
import {
  countDirectChildren,
  countDirectDocs,
  countDocsInSubtree,
  directChildrenOfView,
  resolveView,
  scopedDocuments,
  type NodeView,
} from './nodeSelection'

const SPACE_ID = 'space-1'

// ── fixture builders ────────────────────────────────────────────────

function makeCollection(overrides: Partial<DocsCollection> & Pick<DocsCollection, 'id' | 'name' | 'position'>): DocsCollection {
  return {
    space_id: SPACE_ID,
    workspace_id: 'ws-1',
    parent_collection_id: null,
    depth: 0,
    slug: overrides.id,
    sort_key: 'an',
    created_by: 'user-1',
    created_at: '2026-04-01T00:00:00.000Z',
    updated_at: '2026-04-01T00:00:00.000Z',
    ...overrides,
  }
}

function makeDoc(overrides: Partial<DocsDocument> & Pick<DocsDocument, 'id' | 'title'>): DocsDocument {
  return {
    workspace_id: 'ws-1',
    space_id: SPACE_ID,
    status: 'published' as DocStatus,
    visibility: 'workspace_wide',
    tags: [],
    position: 0,
    sort_key: 'an',
    is_pinned: false,
    is_publicly_shared: false,
    is_locked: false,
    created_by: 'user-1',
    created_at: '2026-04-01T00:00:00.000Z',
    updated_at: '2026-04-01T00:00:00.000Z',
    ...overrides,
  }
}

// Standard fixture used by most tests.
//
//   Getting Started (gs, pos 0)
//   ├── Setup (setup, pos 0)     — 1 direct doc (d-setup-1)
//   │   └── Deep (deep, pos 0)   — 1 direct doc (d-deep-1, archived)
//   └── First Steps (first, pos 1) — 0 direct docs
//   Billing (billing, pos 1)     — 1 direct doc (d-billing-1)
//   (uncategorized)              — 1 doc (d-uncat-1, draft)
//
// Getting Started itself has 1 direct doc (d-gs-1).
function buildStandardFixture() {
  const collections: DocsCollection[] = [
    makeCollection({ id: 'gs', name: 'Getting Started', position: 0, depth: 0, sort_key: 'an' }),
    makeCollection({ id: 'setup', name: 'Setup', position: 0, depth: 1, parent_collection_id: 'gs', sort_key: 'an' }),
    makeCollection({ id: 'deep', name: 'Deep', position: 0, depth: 2, parent_collection_id: 'setup', sort_key: 'an' }),
    makeCollection({ id: 'first', name: 'First Steps', position: 1, depth: 1, parent_collection_id: 'gs', sort_key: 'ao' }),
    makeCollection({ id: 'billing', name: 'Billing', position: 1, depth: 0, sort_key: 'ao' }),
  ]
  const documents: DocsDocument[] = [
    makeDoc({ id: 'd-gs-1', title: 'GS direct doc', collection_id: 'gs' }),
    makeDoc({ id: 'd-setup-1', title: 'Setup doc', collection_id: 'setup' }),
    makeDoc({ id: 'd-deep-1', title: 'Deep archived', collection_id: 'deep', status: 'archived' }),
    makeDoc({ id: 'd-billing-1', title: 'Billing doc', collection_id: 'billing' }),
    makeDoc({ id: 'd-uncat-1', title: 'Uncat draft', status: 'draft' }),
  ]
  return { collections, documents, tree: buildCollectionTree(SPACE_ID, collections, documents) }
}

const NO_FILTER = { status: null }
const DRAFT = { status: 'draft' as DocStatus }
const PUBLISHED = { status: 'published' as DocStatus }
const ARCHIVED = { status: 'archived' as DocStatus }

// ── resolveView ──────────────────────────────────────────────────────

describe('resolveView', () => {
  it('returns loading when tree is null and param is missing', () => {
    expect(resolveView(null, null)).toEqual({ kind: 'loading' })
    expect(resolveView(undefined, null)).toEqual({ kind: 'loading' })
  })

  it('returns space_root when param is missing and tree is loaded', () => {
    const { tree } = buildStandardFixture()
    expect(resolveView(null, tree)).toEqual({ kind: 'space_root' })
    expect(resolveView(undefined, tree)).toEqual({ kind: 'space_root' })
  })

  it('returns loading when a collection id is given but tree is null', () => {
    expect(resolveView('gs', null)).toEqual({ kind: 'loading' })
  })

  it('returns collection view with ancestors for a known id', () => {
    const { tree } = buildStandardFixture()
    const view = resolveView('deep', tree)
    expect(view.kind).toBe('collection')
    if (view.kind !== 'collection') throw new Error('expected collection view')
    expect(view.node.collection.id).toBe('deep')
    // Ancestors are ordered root-first, immediate-parent last, and
    // do not include the target itself.
    expect(view.ancestors.map((a) => a.collection.id)).toEqual(['gs', 'setup'])
  })

  it('returns empty ancestors for a top-level collection', () => {
    const { tree } = buildStandardFixture()
    const view = resolveView('gs', tree)
    expect(view.kind).toBe('collection')
    if (view.kind !== 'collection') throw new Error('expected collection view')
    expect(view.ancestors).toEqual([])
  })

  it('falls back to space_root for an unknown collection id', () => {
    const { tree } = buildStandardFixture()
    expect(resolveView('ghost', tree)).toEqual({ kind: 'space_root' })
  })

  it('returns uncategorized when the sentinel is passed', () => {
    const { tree } = buildStandardFixture()
    expect(resolveView('__uncollected__', tree)).toEqual({ kind: 'uncategorized' })
  })

  it('returns loading when uncategorized is requested before the tree resolves', () => {
    expect(resolveView('__uncollected__', null)).toEqual({ kind: 'loading' })
  })
})

// ── directChildrenOfView ─────────────────────────────────────────────

describe('directChildrenOfView', () => {
  it('returns tree.topLevel for space_root view', () => {
    const { tree } = buildStandardFixture()
    const children = directChildrenOfView({ kind: 'space_root' }, tree)
    // Order must match buildCollectionTree: gs (position 0) then billing (position 1).
    expect(children.map((n) => n.collection.id)).toEqual(['gs', 'billing'])
  })

  it('returns view.node.children for collection view', () => {
    const { tree } = buildStandardFixture()
    const view = resolveView('gs', tree)
    const children = directChildrenOfView(view, tree)
    expect(children.map((n) => n.collection.id)).toEqual(['setup', 'first'])
  })

  it('returns [] for uncategorized view', () => {
    const { tree } = buildStandardFixture()
    expect(directChildrenOfView({ kind: 'uncategorized' }, tree)).toEqual([])
  })

  it('returns [] for loading view', () => {
    const { tree } = buildStandardFixture()
    expect(directChildrenOfView({ kind: 'loading' }, tree)).toEqual([])
  })

  it('returns [] when the tree is null', () => {
    expect(directChildrenOfView({ kind: 'space_root' }, null)).toEqual([])
  })
})

// ── scopedDocuments ──────────────────────────────────────────────────

describe('scopedDocuments', () => {
  it('returns every doc in space for space_root with no filter (archived included)', () => {
    const { tree } = buildStandardFixture()
    const docs = scopedDocuments({ kind: 'space_root' }, tree, NO_FILTER)
    // 1 gs + 1 setup + 1 deep (archived) + 1 billing + 1 uncat (draft)
    expect(docs.map((d) => d.id).sort()).toEqual(
      ['d-billing-1', 'd-deep-1', 'd-gs-1', 'd-setup-1', 'd-uncat-1'].sort(),
    )
  })

  it('returns direct + sub-collection docs recursively for a collection view', () => {
    const { tree } = buildStandardFixture()
    const view = resolveView('gs', tree)
    const docs = scopedDocuments(view, tree, NO_FILTER)
    // Getting Started has 1 direct doc + setup has 1 + deep has 1 (archived).
    // All three should appear (recursive walk).
    expect(docs.map((d) => d.id)).toEqual(['d-gs-1', 'd-setup-1', 'd-deep-1'])
  })

  it('returns tree.uncategorizedDocuments for uncategorized view', () => {
    const { tree } = buildStandardFixture()
    const docs = scopedDocuments({ kind: 'uncategorized' }, tree, NO_FILTER)
    expect(docs.map((d) => d.id)).toEqual(['d-uncat-1'])
  })

  it('returns [] for loading view', () => {
    const { tree } = buildStandardFixture()
    expect(scopedDocuments({ kind: 'loading' }, tree, NO_FILTER)).toEqual([])
  })

  it('returns [] when the tree is null', () => {
    expect(scopedDocuments({ kind: 'space_root' }, null, NO_FILTER)).toEqual([])
  })

  it('filters to drafts only when filter.status === draft', () => {
    const { tree } = buildStandardFixture()
    const docs = scopedDocuments({ kind: 'space_root' }, tree, DRAFT)
    expect(docs.map((d) => d.id)).toEqual(['d-uncat-1'])
  })

  it('filters to published only when filter.status === published', () => {
    const { tree } = buildStandardFixture()
    const docs = scopedDocuments({ kind: 'space_root' }, tree, PUBLISHED)
    expect(docs.map((d) => d.id).sort()).toEqual(['d-billing-1', 'd-gs-1', 'd-setup-1'].sort())
  })

  it('filters to archived only when filter.status === archived', () => {
    const { tree } = buildStandardFixture()
    const docs = scopedDocuments({ kind: 'space_root' }, tree, ARCHIVED)
    expect(docs.map((d) => d.id)).toEqual(['d-deep-1'])
  })
})

// ── countDocsInSubtree ───────────────────────────────────────────────

describe('countDocsInSubtree', () => {
  it('counts a leaf with only direct docs', () => {
    const { tree } = buildStandardFixture()
    const view = resolveView('billing', tree) as Extract<NodeView, { kind: 'collection' }>
    expect(countDocsInSubtree(view.node, NO_FILTER)).toBe(1)
  })

  it('counts a parent plus all descendants', () => {
    const { tree } = buildStandardFixture()
    const view = resolveView('gs', tree) as Extract<NodeView, { kind: 'collection' }>
    // gs direct (1) + setup direct (1) + deep direct (1, archived) + first direct (0)
    expect(countDocsInSubtree(view.node, NO_FILTER)).toBe(3)
  })

  it('counts a depth-1 parent plus its depth-2 child', () => {
    const { tree } = buildStandardFixture()
    const view = resolveView('setup', tree) as Extract<NodeView, { kind: 'collection' }>
    // setup direct (1) + deep direct (1)
    expect(countDocsInSubtree(view.node, NO_FILTER)).toBe(2)
  })

  it('respects the status filter', () => {
    const { tree } = buildStandardFixture()
    const view = resolveView('gs', tree) as Extract<NodeView, { kind: 'collection' }>
    // gs subtree published: d-gs-1 + d-setup-1 = 2 (d-deep-1 is archived)
    expect(countDocsInSubtree(view.node, PUBLISHED)).toBe(2)
    // Only the archived one
    expect(countDocsInSubtree(view.node, ARCHIVED)).toBe(1)
    // No drafts in this subtree
    expect(countDocsInSubtree(view.node, DRAFT)).toBe(0)
  })
})

// ── countDirectDocs ──────────────────────────────────────────────────

describe('countDirectDocs', () => {
  it('counts direct docs on a node (not recursive)', () => {
    const { tree } = buildStandardFixture()
    const view = resolveView('gs', tree) as Extract<NodeView, { kind: 'collection' }>
    expect(countDirectDocs(view.node, tree, NO_FILTER)).toBe(1)
  })

  it('counts uncategorized docs when node is null', () => {
    const { tree } = buildStandardFixture()
    expect(countDirectDocs(null, tree, NO_FILTER)).toBe(1)
  })

  it('returns 0 when the tree is null', () => {
    expect(countDirectDocs(null, null, NO_FILTER)).toBe(0)
  })

  it('respects the status filter', () => {
    const { tree } = buildStandardFixture()
    // uncat doc is a draft → 1 when filtering to draft, 0 otherwise
    expect(countDirectDocs(null, tree, DRAFT)).toBe(1)
    expect(countDirectDocs(null, tree, PUBLISHED)).toBe(0)
    expect(countDirectDocs(null, tree, ARCHIVED)).toBe(0)
  })
})

// ── countDirectChildren ──────────────────────────────────────────────

describe('countDirectChildren', () => {
  it('counts top-level collections when node is null', () => {
    const { tree } = buildStandardFixture()
    expect(countDirectChildren(null, tree)).toBe(2) // gs, billing
  })

  it('counts immediate children of a collection node', () => {
    const { tree } = buildStandardFixture()
    const view = resolveView('gs', tree) as Extract<NodeView, { kind: 'collection' }>
    expect(countDirectChildren(view.node, tree)).toBe(2) // setup, first
  })

  it('returns 0 for a leaf collection', () => {
    const { tree } = buildStandardFixture()
    const view = resolveView('deep', tree) as Extract<NodeView, { kind: 'collection' }>
    expect(countDirectChildren(view.node, tree)).toBe(0)
  })

  it('returns 0 when the tree is null', () => {
    expect(countDirectChildren(null, null)).toBe(0)
  })
})
