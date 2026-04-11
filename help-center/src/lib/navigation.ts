import type { NavItem, NavTreeNode } from './types'

export interface ArticlePagerLink {
  title: string
  slug: string
  collectionSlug: string
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
  currentArticleSlug: string,
): { prev?: ArticlePagerLink; next?: ArticlePagerLink } {
  const tree = buildNavTree(navigation)
  const flat: ArticlePagerLink[] = []
  const walk = (nodes: NavTreeNode[]) => {
    for (const node of nodes) {
      for (const article of node.item.articles) {
        flat.push({
          title: article.title,
          slug: article.slug,
          collectionSlug: node.item.slug,
          collectionName: node.item.name,
        })
      }
      if (node.children.length > 0) walk(node.children)
    }
  }
  walk(tree)

  const idx = flat.findIndex((a) => a.slug === currentArticleSlug)
  if (idx === -1) return {}

  return {
    prev: idx > 0 ? flat[idx - 1] : undefined,
    next: idx < flat.length - 1 ? flat[idx + 1] : undefined,
  }
}
