import { describe, it, expect } from 'vitest'
import {
  buildNavTree,
  flattenNavTree,
  navAncestorChain,
  getArticlePager,
} from '../navigation'
import type { NavItem } from '../types'

function makeNavItem(
  id: string,
  parent: string | null,
  depth: number,
  name = id,
  slug = id,
  articles: { id: string; title: string; slug: string; public_id: string }[] = [],
  position = 0,
): NavItem {
  return {
    id,
    name,
    slug,
    public_id: id.slice(0, 8).padEnd(8, '0'),
    icon: null,
    parent_collection_id: parent,
    depth,
    position,
    articles: articles.map((a, index) => ({ ...a, position: index, published_at: null })),
  }
}

describe('buildNavTree', () => {
  it('folds a flat NavItem list into a nested tree', () => {
    const items = [
      makeNavItem('root', null, 0),
      makeNavItem('child-a', 'root', 1),
      makeNavItem('child-b', 'root', 1),
      makeNavItem('grand', 'child-a', 2),
    ]
    const tree = buildNavTree(items)
    expect(tree).toHaveLength(1)
    const root = tree[0]!
    expect(root.item.id).toBe('root')
    expect(root.children.map((n) => n.item.id)).toEqual(['child-a', 'child-b'])
    expect(root.children[0]!.children.map((n) => n.item.id)).toEqual(['grand'])
  })

  it('promotes orphaned nodes to top-level when their parent is missing', () => {
    const items = [
      makeNavItem('orphan', 'missing', 1),
    ]
    const tree = buildNavTree(items)
    expect(tree).toHaveLength(1)
    expect(tree[0]!.item.id).toBe('orphan')
  })
})

describe('flattenNavTree', () => {
  it('returns nodes depth-first in tree order', () => {
    const items = [
      makeNavItem('root', null, 0),
      makeNavItem('a', 'root', 1),
      makeNavItem('b', 'root', 1),
      makeNavItem('a1', 'a', 2),
    ]
    const flat = flattenNavTree(buildNavTree(items))
    expect(flat.map((n) => n.item.id)).toEqual(['root', 'a', 'a1', 'b'])
  })
})

describe('navAncestorChain', () => {
  const items = [
    makeNavItem('root', null, 0),
    makeNavItem('mid', 'root', 1),
    makeNavItem('leaf', 'mid', 2),
  ]
  const tree = buildNavTree(items)

  it('returns ancestors top-down for a leaf node', () => {
    expect(navAncestorChain(tree, 'leaf').map((n) => n.item.id)).toEqual(['root', 'mid'])
  })

  it('returns empty for a top-level node', () => {
    expect(navAncestorChain(tree, 'root')).toEqual([])
  })

  it('returns empty for a missing node', () => {
    expect(navAncestorChain(tree, 'ghost')).toEqual([])
  })
})

describe('getArticlePager', () => {
  const items = [
    makeNavItem('root', null, 0, 'Root', 'root', [
      { id: 'r1', title: 'R1', slug: 'r1', public_id: 'aaa111aa' },
      { id: 'r2', title: 'R2', slug: 'r2', public_id: 'bbb222bb' },
    ]),
    makeNavItem('child', 'root', 1, 'Child', 'child', [
      { id: 'c1', title: 'C1', slug: 'c1', public_id: 'ccc333cc' },
    ], 2),
  ]

  it('advances from a parent-level article into the first child-level article', () => {
    const pager = getArticlePager(items, 'bbb222bb')
    expect(pager.next?.slug).toBe('c1')
    expect(pager.prev?.slug).toBe('r1')
  })

  it('returns undefined ends at the first and last article', () => {
    const first = getArticlePager(items, 'aaa111aa')
    expect(first.prev).toBeUndefined()
    expect(first.next?.slug).toBe('r2')

    const last = getArticlePager(items, 'ccc333cc')
    expect(last.prev?.slug).toBe('r2')
    expect(last.next).toBeUndefined()
  })

  it('returns empty object for an unknown slug', () => {
    expect(getArticlePager(items, 'ghost')).toEqual({})
  })
})
