import { describe, it, expect } from 'vitest'
import {
  buildCollectionTree,
  flattenCollectionTree,
  findCollectionNode,
  collectionAncestorChain,
  buildCollectionPathLabels,
} from '../docsCollectionTree'
import type { DocsCollection, DocsDocument } from '@/lib/docsTypes'

function makeCollection(
  id: string,
  parent: string | null,
  depth: number,
  position: number,
  spaceId = 'space-1',
): DocsCollection {
  return {
    id,
    space_id: spaceId,
    workspace_id: 'ws-1',
    parent_collection_id: parent,
    depth,
    name: id,
    slug: id,
    position,
    created_by: 'user-1',
    created_at: `2026-04-10T12:00:${position.toString().padStart(2, '0')}Z`,
    updated_at: `2026-04-10T12:00:${position.toString().padStart(2, '0')}Z`,
  }
}

function makeDoc(
  id: string,
  collectionId: string | null,
  position: number,
  spaceId = 'space-1',
): DocsDocument {
  return {
    id,
    workspace_id: 'ws-1',
    space_id: spaceId,
    collection_id: collectionId ?? undefined,
    title: id,
    status: 'draft',
    visibility: 'workspace_wide',
    position,
    is_pinned: false,
    is_publicly_shared: false,
    is_locked: false,
    created_by: 'user-1',
    created_at: `2026-04-10T12:00:${position.toString().padStart(2, '0')}Z`,
    updated_at: `2026-04-10T12:00:${position.toString().padStart(2, '0')}Z`,
  } as DocsDocument
}

describe('buildCollectionTree', () => {
  it('nests child collections under their parent', () => {
    const collections = [
      makeCollection('root', null, 0, 0),
      makeCollection('child-a', 'root', 1, 0),
      makeCollection('child-b', 'root', 1, 1),
      makeCollection('grand', 'child-a', 2, 0),
    ]
    const tree = buildCollectionTree('space-1', collections, [])

    expect(tree.topLevel).toHaveLength(1)
    const root = tree.topLevel[0]
    expect(root.collection.id).toBe('root')
    expect(root.children.map((n) => n.collection.id)).toEqual(['child-a', 'child-b'])
    expect(root.children[0].children.map((n) => n.collection.id)).toEqual(['grand'])
    expect(root.children[1].children).toHaveLength(0)
  })

  it('attaches documents to their owning collection', () => {
    const collections = [
      makeCollection('root', null, 0, 0),
      makeCollection('child', 'root', 1, 0),
    ]
    const docs = [
      makeDoc('doc-root-1', 'root', 0),
      makeDoc('doc-root-2', 'root', 1),
      makeDoc('doc-child', 'child', 0),
    ]
    const tree = buildCollectionTree('space-1', collections, docs)

    expect(tree.topLevel[0].documents.map((d) => d.id)).toEqual(['doc-root-1', 'doc-root-2'])
    expect(tree.topLevel[0].children[0].documents.map((d) => d.id)).toEqual(['doc-child'])
  })

  it('returns uncategorized documents separately from collection-owned ones', () => {
    const collections = [makeCollection('root', null, 0, 0)]
    const docs = [
      makeDoc('doc-root', 'root', 0),
      makeDoc('doc-orphan-1', null, 0),
      makeDoc('doc-orphan-2', null, 1),
    ]
    const tree = buildCollectionTree('space-1', collections, docs)

    expect(tree.topLevel[0].documents.map((d) => d.id)).toEqual(['doc-root'])
    expect(tree.uncategorizedDocuments.map((d) => d.id)).toEqual(['doc-orphan-1', 'doc-orphan-2'])
  })

  it('ignores collections from other spaces', () => {
    const collections = [
      makeCollection('same', null, 0, 0, 'space-1'),
      makeCollection('other', null, 0, 0, 'space-2'),
    ]
    const tree = buildCollectionTree('space-1', collections, [])
    expect(tree.topLevel.map((n) => n.collection.id)).toEqual(['same'])
  })

  it('promotes collections whose parent is missing to top-level', () => {
    const collections = [
      makeCollection('orphan', 'missing', 1, 0),
    ]
    const tree = buildCollectionTree('space-1', collections, [])
    expect(tree.topLevel).toHaveLength(1)
    expect(tree.topLevel[0].collection.id).toBe('orphan')
  })
})

describe('flattenCollectionTree', () => {
  it('returns nodes depth-first in tree order', () => {
    const collections = [
      makeCollection('root', null, 0, 0),
      makeCollection('a', 'root', 1, 0),
      makeCollection('b', 'root', 1, 1),
      makeCollection('a1', 'a', 2, 0),
      makeCollection('a2', 'a', 2, 1),
    ]
    const tree = buildCollectionTree('space-1', collections, [])
    const flat = flattenCollectionTree(tree.topLevel)
    expect(flat.map((n) => n.collection.id)).toEqual(['root', 'a', 'a1', 'a2', 'b'])
  })
})

describe('findCollectionNode and collectionAncestorChain', () => {
  const collections = [
    makeCollection('root', null, 0, 0),
    makeCollection('mid', 'root', 1, 0),
    makeCollection('leaf', 'mid', 2, 0),
  ]
  const tree = buildCollectionTree('space-1', collections, [])

  it('finds a nested node by id', () => {
    expect(findCollectionNode(tree.topLevel, 'leaf')?.collection.id).toBe('leaf')
  })

  it('returns null for unknown ids', () => {
    expect(findCollectionNode(tree.topLevel, 'ghost')).toBeNull()
  })

  it('returns ancestors top-down for a leaf node', () => {
    const chain = collectionAncestorChain(tree.topLevel, 'leaf')
    expect(chain.map((n) => n.collection.id)).toEqual(['root', 'mid'])
  })

  it('returns empty for a top-level node', () => {
    expect(collectionAncestorChain(tree.topLevel, 'root')).toEqual([])
  })
})

describe('buildCollectionPathLabels', () => {
  it('builds breadcrumb labels for nested collections', () => {
    const collections = [
      makeCollection('Guides', null, 0, 0),
      makeCollection('Setup', 'Guides', 1, 0),
      makeCollection('Web', 'Setup', 2, 0),
    ]

    expect(buildCollectionPathLabels(collections).get('Web')).toBe('Guides › Setup › Web')
  })

  it('keeps orphaned collections usable', () => {
    const orphan = makeCollection('Orphan', 'missing', 1, 0)
    expect(buildCollectionPathLabels([orphan]).get('Orphan')).toBe('Orphan')
  })
})
