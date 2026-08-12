import { describe, expect, it } from 'vitest'
import type { DocsCollection, DocsDocument, DocsSpace } from '@/lib/docsTypes'
import {
  buildDocsSidebarTree,
  findCollectionAncestry,
  resolveDocsSidebarSpaceId,
} from '../docsSidebarState'

const spaces = [{ id: 'space-a' }, { id: 'space-b' }] as DocsSpace[]

const collections = [
  { id: 'parent', name: 'Parent', position: 0, parent_collection_id: null },
  { id: 'child', name: 'Child', position: 0, parent_collection_id: 'parent' },
  { id: 'second', name: 'Second', position: 1, parent_collection_id: null },
] as DocsCollection[]

const documents = [
  { id: 'doc-b', title: 'B', position: 1, collection_id: 'parent' },
  { id: 'doc-a', title: 'A', position: 0, collection_id: 'parent' },
  { id: 'doc-child', title: 'Child doc', position: 0, collection_id: 'child' },
] as DocsDocument[]

describe('docs sidebar state', () => {
  it('resolves route, document, selected, stored, then first-space precedence', () => {
    expect(resolveDocsSidebarSpaceId({ spaces, routeSpaceId: 'space-b' })).toBe('space-b')
    expect(resolveDocsSidebarSpaceId({ spaces, documentSpaceId: 'space-b' })).toBe('space-b')
    expect(resolveDocsSidebarSpaceId({ spaces, selectedSpaceId: 'space-b' })).toBe('space-b')
    expect(resolveDocsSidebarSpaceId({ spaces, storedSpaceId: 'space-b' })).toBe('space-b')
    expect(resolveDocsSidebarSpaceId({ spaces, storedSpaceId: 'deleted' })).toBe('space-a')
  })

  it('builds ordered nested collections with ordered direct documents', () => {
    const tree = buildDocsSidebarTree(collections, documents)
    expect(tree.map((node) => node.collection.id)).toEqual(['parent', 'second'])
    expect(tree[0].documents.map((document) => document.id)).toEqual(['doc-a', 'doc-b'])
    expect(tree[0].children[0].collection.id).toBe('child')
    expect(tree[0].children[0].documents[0].id).toBe('doc-child')
  })

  it('returns collection ancestry from root through the active collection', () => {
    expect(findCollectionAncestry(collections, 'child')).toEqual(['parent', 'child'])
    expect(findCollectionAncestry(collections, 'missing')).toEqual([])
  })
})
