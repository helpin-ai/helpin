import type { DocsCollection, DocsDocument } from '@/lib/docsTypes'

/**
 * CollectionTreeNode is one collection plus its direct child collections
 * and its direct articles (documents whose collection_id points at this
 * collection). Descendant articles live under child nodes, not this one.
 */
export interface CollectionTreeNode {
  collection: DocsCollection
  children: CollectionTreeNode[]
  documents: DocsDocument[]
}

/**
 * CollectionTree is the full tree for a single space: zero or more
 * top-level collection nodes plus any uncategorized articles that don't
 * belong to any collection.
 */
export interface CollectionTree {
  topLevel: CollectionTreeNode[]
  uncategorizedDocuments: DocsDocument[]
}

/**
 * sortByKey sorts items by sort_key (fractional ordering) when
 * available, falling back to position + created_at for pre-backfill
 * rows (sort_key is empty or the sentinel '~').
 */
function hasSortKey(item: { sort_key?: string }): boolean {
  return !!item.sort_key && item.sort_key !== '~'
}

function sortItems<T extends { sort_key?: string; position: number; created_at: string; id: string }>(items: T[]): T[] {
  return items.slice().sort((a, b) => {
    if (hasSortKey(a) && hasSortKey(b)) {
      return a.sort_key!.localeCompare(b.sort_key!) || a.id.localeCompare(b.id)
    }
    if (a.position !== b.position) return a.position - b.position
    return a.created_at.localeCompare(b.created_at)
  })
}

/**
 * buildCollectionTree folds a flat list of collections and documents
 * into a nested tree. The input may contain collections that are not
 * in the active space — they are silently ignored. Ordering within
 * each sibling bucket is by sort_key (fractional ordering) when
 * available, falling back to (position, created_at) for pre-backfill
 * data. The backend enforces a depth cap of 2, but this function
 * handles arbitrary depth gracefully so a data anomaly does not
 * produce an infinite loop.
 */
export function buildCollectionTree(
  spaceId: string,
  collections: DocsCollection[],
  documents: DocsDocument[],
): CollectionTree {
  const inSpace = sortItems(
    collections.filter((c) => c.space_id === spaceId && !c.deleted_at),
  )

  const nodesById = new Map<string, CollectionTreeNode>()
  for (const c of inSpace) {
    nodesById.set(c.id, { collection: c, children: [], documents: [] })
  }

  // Partition documents by owning collection (or uncategorized) and
  // keep them sorted by sort_key (or position as fallback).
  const docsByCollection = new Map<string, DocsDocument[]>()
  const uncategorized: DocsDocument[] = []
  const docsInSpace = documents.filter((d) => d.space_id === spaceId)
  for (const d of docsInSpace) {
    if (d.collection_id) {
      const list = docsByCollection.get(d.collection_id) ?? []
      list.push(d)
      docsByCollection.set(d.collection_id, list)
    } else {
      uncategorized.push(d)
    }
  }
  for (const node of nodesById.values()) {
    node.documents = sortItems(docsByCollection.get(node.collection.id) ?? [])
  }

  // Link children to parents. Top-level collections are those whose
  // parent is null or whose referenced parent does not exist in the
  // map (defensive handling of orphaned rows).
  const topLevel: CollectionTreeNode[] = []
  for (const c of inSpace) {
    const node = nodesById.get(c.id)!
    if (c.parent_collection_id && nodesById.has(c.parent_collection_id)) {
      nodesById.get(c.parent_collection_id)!.children.push(node)
    } else {
      topLevel.push(node)
    }
  }

  return {
    topLevel,
    uncategorizedDocuments: sortItems(uncategorized),
  }
}

/**
 * flattenCollectionTree walks a collection tree depth-first and returns
 * every node in tree order (ancestors before descendants, siblings in
 * their sibling bucket order). Used by components that need a linear
 * iteration order, like article pagers.
 */
export function flattenCollectionTree(tree: CollectionTreeNode[]): CollectionTreeNode[] {
  const out: CollectionTreeNode[] = []
  const walk = (nodes: CollectionTreeNode[]) => {
    for (const node of nodes) {
      out.push(node)
      if (node.children.length > 0) walk(node.children)
    }
  }
  walk(tree)
  return out
}

/**
 * findCollectionNode returns the tree node for a given collection id or
 * null when the id is not present in the tree. The search is depth-first
 * so it is bounded by the tree's total node count.
 */
export function findCollectionNode(
  tree: CollectionTreeNode[],
  collectionId: string,
): CollectionTreeNode | null {
  for (const node of tree) {
    if (node.collection.id === collectionId) return node
    const hit = findCollectionNode(node.children, collectionId)
    if (hit) return hit
  }
  return null
}

/**
 * collectionAncestorChain returns the ancestor nodes of the given
 * collection ordered top-down (root first, immediate parent last). The
 * target collection itself is NOT included. Returns an empty slice when
 * the collection is top-level or not found.
 */
export function collectionAncestorChain(
  tree: CollectionTreeNode[],
  collectionId: string,
): CollectionTreeNode[] {
  const path: CollectionTreeNode[] = []
  const walk = (nodes: CollectionTreeNode[]): boolean => {
    for (const node of nodes) {
      if (node.collection.id === collectionId) return true
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
 * Builds compact breadcrumb labels for every collection in a flat result set.
 * Cycles and missing parents are handled defensively so list rendering remains
 * usable even when collection data is temporarily inconsistent.
 */
export function buildCollectionPathLabels(collections: DocsCollection[]): Map<string, string> {
  const byId = new Map(collections.map((collection) => [collection.id, collection]))
  const labels = new Map<string, string>()

  const labelFor = (collection: DocsCollection): string => {
    const cached = labels.get(collection.id)
    if (cached) return cached

    const names = [collection.name]
    const visited = new Set([collection.id])
    let parentId = collection.parent_collection_id
    while (parentId && !visited.has(parentId)) {
      visited.add(parentId)
      const parent = byId.get(parentId)
      if (!parent) break
      names.unshift(parent.name)
      parentId = parent.parent_collection_id
    }
    const label = names.join(' › ')
    labels.set(collection.id, label)
    return label
  }

  for (const collection of collections) labelFor(collection)
  return labels
}
