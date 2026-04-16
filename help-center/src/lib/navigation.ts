import type { NavItem, NavTreeNode } from './types'

export interface ArticlePagerLink {
  title: string
  slug: string
  publicId: string
  collectionSlug: string
  collectionPublicId: string
  collectionName: string
}

/**
 * buildNavTree folds the flat NavItem[] returned by the backend into
 * a nested tree using parent_collection_id. Orphans whose parent is
 * missing from the response are defensively promoted to the top level
 * so a stale snapshot cannot hide content.
 *
 * The backend already orders the response by (depth, parent_collection_id,
 * position) but we re-sort each sibling bucket here anyway — it's cheap
 * and keeps this helper self-contained for tests.
 */
export function buildNavTree(items: NavItem[]): NavTreeNode[] {
  const byId = new Map<string, NavTreeNode>()
  for (const item of items) {
    byId.set(item.id, { item, children: [] })
  }
  const roots: NavTreeNode[] = []
  for (const item of items) {
    const node = byId.get(item.id)!
    if (item.parent_collection_id && byId.has(item.parent_collection_id)) {
      byId.get(item.parent_collection_id)!.children.push(node)
    } else {
      roots.push(node)
    }
  }
  return roots
}

/**
 * flattenNavTree walks a nav tree depth-first and returns every node
 * in tree order (ancestors before descendants, siblings preserved).
 */
export function flattenNavTree(tree: NavTreeNode[]): NavTreeNode[] {
  const out: NavTreeNode[] = []
  const walk = (nodes: NavTreeNode[]) => {
    for (const node of nodes) {
      out.push(node)
      if (node.children.length > 0) walk(node.children)
    }
  }
  walk(tree)
  return out
}

/**
 * navAncestorChain returns the ancestor chain of the given collection
 * ordered top-down (root first, immediate parent last). The target
 * itself is not included. Returns an empty array for top-level nodes.
 */
export function navAncestorChain(tree: NavTreeNode[], collectionId: string): NavTreeNode[] {
  const path: NavTreeNode[] = []
  const walk = (nodes: NavTreeNode[]): boolean => {
    for (const node of nodes) {
      if (node.item.id === collectionId) return true
      path.push(node)
      if (walk(node.children)) return true
      path.pop()
    }
    return false
  }
  walk(tree)
  return path
}

/**
 * getArticlePager returns the previous and next article in tree order
 * for a given article slug. Tree order matches the lock-in rule in the
 * hierarchical-docs plan: articles are listed by owning collection in
 * (depth, parent, position) order, with each collection's direct
 * articles listed before its children's articles.
 */
export function getArticlePager(
  navigation: NavItem[],
  currentArticleID: string,
): { prev?: ArticlePagerLink; next?: ArticlePagerLink } {
  const tree = buildNavTree(navigation)
  const flat: ArticlePagerLink[] = []
  // Walk in merged (article + sub-collection) position order so the
  // pager matches the visible sidebar order.
  type Entry =
    | { kind: 'article'; position: number; article: NavItem['articles'][number]; node: NavTreeNode }
    | { kind: 'collection'; position: number; child: NavTreeNode }
  const walk = (nodes: NavTreeNode[]) => {
    for (const node of nodes) {
      const entries: Entry[] = []
      for (const article of node.item.articles) {
        entries.push({ kind: 'article', position: article.position, article, node })
      }
      for (const child of node.children) {
        entries.push({ kind: 'collection', position: child.item.position, child })
      }
      entries.sort((a, b) => {
        if (a.position !== b.position) return a.position - b.position
        if (a.kind !== b.kind) return a.kind === 'collection' ? -1 : 1
        const aId = a.kind === 'article' ? a.article.id : a.child.item.id
        const bId = b.kind === 'article' ? b.article.id : b.child.item.id
        return aId.localeCompare(bId)
      })
      for (const e of entries) {
        if (e.kind === 'article') {
          flat.push({
            title: e.article.title,
            slug: e.article.slug,
            publicId: e.article.public_id,
            collectionSlug: e.node.item.slug,
            collectionPublicId: e.node.item.public_id,
            collectionName: e.node.item.name,
          })
        } else {
          walk([e.child])
        }
      }
    }
  }
  walk(tree)

  const idx = flat.findIndex((a) => a.publicId === currentArticleID)
  if (idx === -1) return {}

  return {
    prev: idx > 0 ? flat[idx - 1] : undefined,
    next: idx < flat.length - 1 ? flat[idx + 1] : undefined,
  }
}
