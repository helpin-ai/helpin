import { describe, expect, it } from 'vitest'
import type { DocsCollection, DocsDocument } from '@/lib/docsTypes'
import { buildCollectionTree } from '@/components/docs/docsCollectionTree'
import { buildOrderGroups, buildReorderItems, placeVisibleOrder } from './docsOrder'

function makeCollection(overrides: Partial<DocsCollection> & { id: string }): DocsCollection {
  return {
    workspace_id: 'ws',
    space_id: 'space',
    name: overrides.id,
    slug: overrides.id,
    depth: 0,
    position: 0,
    sort_key: '',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  } as DocsCollection
}

function makeDoc(overrides: Partial<DocsDocument> & { id: string }): DocsDocument {
  return {
    workspace_id: 'ws',
    space_id: 'space',
    title: overrides.id,
    status: 'published',
    visibility: 'workspace',
    tags: [],
    position: 0,
    sort_key: '',
    is_pinned: false,
    is_publicly_shared: false,
    is_locked: false,
    created_by: 'user',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  } as DocsDocument
}

describe('buildOrderGroups', () => {
  it('lists the open collection first, then descendants depth-first with relative paths', () => {
    const collections = [
      makeCollection({ id: 'inbox', name: 'Inbox' }),
      makeCollection({ id: 'routing', name: 'Routing', parent_collection_id: 'inbox', depth: 1, position: 1 }),
      makeCollection({ id: 'rules', name: 'Rules', parent_collection_id: 'routing', depth: 2 }),
      makeCollection({ id: 'tags', name: 'Tags', parent_collection_id: 'inbox', depth: 1, position: 2 }),
    ]
    const docs = [
      makeDoc({ id: 'a', collection_id: 'inbox', position: 1 }),
      makeDoc({ id: 'b', collection_id: 'inbox', position: 0 }),
      makeDoc({ id: 'r1', collection_id: 'routing' }),
      makeDoc({ id: 'x1', collection_id: 'rules' }),
    ]
    const tree = buildCollectionTree('space', collections, docs)
    const groups = buildOrderGroups(tree.topLevel[0])

    expect(groups.map((g) => [g.collection.id, g.label])).toEqual([
      ['inbox', null],
      ['routing', 'Routing'],
      ['rules', 'Routing › Rules'],
      ['tags', 'Tags'],
    ])
    expect(groups[0].documents.map((d) => d.id)).toEqual(['b', 'a'])
    expect(groups[0].childCollections.map((c) => c.id)).toEqual(['routing', 'tags'])
  })
})

describe('placeVisibleOrder', () => {
  it('keeps hidden docs after the visible doc they followed', () => {
    // a, [x hidden], b, c  → move c to the top
    expect(placeVisibleOrder(['a', 'x', 'b'], ['c', 'a', 'b'])).toEqual(['c', 'a', 'x', 'b'])
  })

  it('keeps leading hidden docs at the top', () => {
    expect(placeVisibleOrder(['x', 'a', 'b'], ['b', 'a'])).toEqual(['x', 'b', 'a'])
  })

  it('reattaches hidden docs to the previous visible doc when their anchor moved away', () => {
    // a, b, [x hidden] with b dragged above a: allIds excludes the dragged doc
    expect(placeVisibleOrder(['a', 'x'], ['b', 'a'])).toEqual(['b', 'a', 'x'])
  })

  it('places a doc dragged in from another group where it was dropped', () => {
    expect(placeVisibleOrder(['a', 'b'], ['a', 'new', 'b'])).toEqual(['a', 'new', 'b'])
  })
})

describe('buildReorderItems', () => {
  it('keeps child collections in their sibling slots and fills doc slots in the new order', () => {
    const collections = [
      makeCollection({ id: 'inbox' }),
      makeCollection({ id: 'routing', parent_collection_id: 'inbox', depth: 1, position: 1 }),
    ]
    const docs = [
      makeDoc({ id: 'a', collection_id: 'inbox', position: 0 }),
      makeDoc({ id: 'b', collection_id: 'inbox', position: 2 }),
    ]
    const group = buildOrderGroups(buildCollectionTree('space', collections, docs).topLevel[0])[0]

    expect(buildReorderItems(group, ['b', 'a'])).toEqual([
      { kind: 'article', id: 'b' },
      { kind: 'collection', id: 'routing' },
      { kind: 'article', id: 'a' },
    ])
  })

  it('appends a doc moved in from another group after the last slot', () => {
    const collections = [makeCollection({ id: 'inbox' })]
    const docs = [makeDoc({ id: 'a', collection_id: 'inbox' })]
    const group = buildOrderGroups(buildCollectionTree('space', collections, docs).topLevel[0])[0]

    expect(buildReorderItems(group, ['moved', 'a'])).toEqual([
      { kind: 'article', id: 'moved' },
      { kind: 'article', id: 'a' },
    ])
  })
})
