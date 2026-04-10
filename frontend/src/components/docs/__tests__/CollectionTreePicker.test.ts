import { describe, it, expect } from 'vitest'
import { buildCollectionTreeOptions } from '../CollectionTreePicker'
import type { DocsCollection } from '@/lib/docsTypes'

function makeCollection(
  id: string,
  parent: string | null,
  depth: number,
  position: number,
  name = id,
  spaceId = 'space-1',
): DocsCollection {
  return {
    id,
    space_id: spaceId,
    workspace_id: 'ws-1',
    parent_collection_id: parent,
    depth,
    name,
    slug: id,
    position,
    created_by: 'user-1',
    created_at: `2026-04-10T12:00:${position.toString().padStart(2, '0')}Z`,
    updated_at: `2026-04-10T12:00:${position.toString().padStart(2, '0')}Z`,
  }
}

describe('buildCollectionTreeOptions', () => {
  it('emits options depth-first with indented depth values', () => {
    const collections = [
      makeCollection('root-a', null, 0, 0, 'Root A'),
      makeCollection('child-a', 'root-a', 1, 0, 'Child A'),
      makeCollection('grand-a', 'child-a', 2, 0, 'Grand A'),
      makeCollection('root-b', null, 0, 1, 'Root B'),
    ]
    const options = buildCollectionTreeOptions('space-1', collections)
    expect(options.map((o) => o.id)).toEqual(['root-a', 'child-a', 'grand-a', 'root-b'])
    expect(options.map((o) => o.depth)).toEqual([0, 1, 2, 0])
  })

  it('builds a breadcrumb path that includes every ancestor name', () => {
    const collections = [
      makeCollection('root', null, 0, 0, 'Root'),
      makeCollection('mid', 'root', 1, 0, 'Middle'),
      makeCollection('leaf', 'mid', 2, 0, 'Leaf'),
    ]
    const options = buildCollectionTreeOptions('space-1', collections)
    const leaf = options.find((o) => o.id === 'leaf')
    expect(leaf?.path).toBe('Root / Middle / Leaf')
  })

  it('ignores collections from other spaces', () => {
    const collections = [
      makeCollection('same', null, 0, 0, 'Same', 'space-1'),
      makeCollection('other', null, 0, 0, 'Other', 'space-2'),
    ]
    const options = buildCollectionTreeOptions('space-1', collections)
    expect(options.map((o) => o.id)).toEqual(['same'])
  })
})
