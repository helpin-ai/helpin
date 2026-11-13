import type { DocsCollection, DocsDocument, ReorderDocsChildItem } from '@/lib/docsTypes'
import { sortTreeItems, type CollectionTreeNode } from '@/components/docs/docsCollectionTree'

/**
 * DocsOrderGroup is one block of rows in the collection view's saved
 * order ("Help center order" / "Sidebar order"): the open collection's
 * own documents, then each sub-collection's documents in tree order.
 * Rows can be dragged within a group (reorder) or into another group
 * (move to that collection).
 */
export interface DocsOrderGroup {
  /** The collection the group's documents belong to. */
  collection: DocsCollection
  /**
   * Path relative to the open collection, e.g. "Routing" or
   * "Routing › Rules". null for the open collection itself.
   */
  label: string | null
  /** Every document in the collection, all statuses, in saved order. */
  documents: DocsDocument[]
  /**
   * Direct child collections. They share one ordered sibling list with
   * the documents, so a reorder must send them back in their slots.
   */
  childCollections: DocsCollection[]
}

/**
 * buildOrderGroups flattens a collection node into order groups: the
 * node first, then every descendant collection depth-first, matching
 * the sidebar's tree order.
 */
export function buildOrderGroups(node: CollectionTreeNode): DocsOrderGroup[] {
  const groups: DocsOrderGroup[] = []
  const walk = (current: CollectionTreeNode, path: string[]) => {
    groups.push({
      collection: current.collection,
      label: path.length > 0 ? path.join(' › ') : null,
      documents: current.documents,
      childCollections: current.children.map((child) => child.collection),
    })
    for (const child of current.children) walk(child, [...path, child.collection.name])
  }
  walk(node, [])
  return groups
}

/**
 * placeVisibleOrder merges a new order of the visible documents back
 * into the full list of a group's documents (which also holds rows the
 * status filter hides, like archived docs). Each hidden document stays
 * right after the visible document it followed before; hidden docs at
 * the top stay at the top. `allIds` must not contain ids that are new
 * to the group (a doc dragged in from another group); those only
 * appear in `visibleOrder`.
 */
export function placeVisibleOrder(allIds: string[], visibleOrder: string[]): string[] {
  const visible = new Set(visibleOrder)
  const leading: string[] = []
  const following = new Map<string, string[]>()
  let anchor: string | null = null
  for (const id of allIds) {
    if (visible.has(id)) {
      anchor = id
    } else if (anchor === null) {
      leading.push(id)
    } else {
      following.set(anchor, [...(following.get(anchor) ?? []), id])
    }
  }
  return [...leading, ...visibleOrder.flatMap((id) => [id, ...(following.get(id) ?? [])])]
}

/**
 * buildReorderItems returns the reorder-children payload for a group:
 * its child collections keep their slots in the shared sibling order,
 * and the document slots are filled with `docIds` in the new order.
 * Documents beyond the existing slots (one dragged in from another
 * group) go after the last slot.
 */
export function buildReorderItems(group: DocsOrderGroup, docIds: string[]): ReorderDocsChildItem[] {
  const siblings = sortTreeItems([
    ...group.childCollections.map((c) => ({ ...orderFields(c), kind: 'collection' as const })),
    ...group.documents.map((d) => ({ ...orderFields(d), kind: 'article' as const })),
  ])
  const queue = [...docIds]
  const items: ReorderDocsChildItem[] = []
  for (const sibling of siblings) {
    if (sibling.kind === 'collection') {
      items.push({ kind: 'collection', id: sibling.id })
    } else if (queue.length > 0) {
      items.push({ kind: 'article', id: queue.shift()! })
    }
  }
  for (const id of queue) items.push({ kind: 'article', id })
  return items
}

function orderFields(item: { id: string; sort_key?: string; position: number; created_at: string }) {
  return { id: item.id, sort_key: item.sort_key, position: item.position, created_at: item.created_at }
}
