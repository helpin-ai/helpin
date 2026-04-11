import type { DocsDocument, DocStatus } from '@/lib/docsTypes'
import {
  collectionAncestorChain,
  findCollectionNode,
  type CollectionTree,
  type CollectionTreeNode,
} from '@/components/docs/docsCollectionTree'

/**
 * NodeView is the discriminated state that drives what the space
 * detail page renders. The sidebar is still the canonical tree
 * navigator; this type only describes what the main area shows.
 *
 * - loading: collections / documents not yet resolved
 * - space_root: no `?collection=` param; show the whole space
 * - collection: drilled into a specific collection node
 * - uncategorized: `?collection=__uncollected__`; dedicated view
 *
 * Every helper in this module takes a NodeView so the render site
 * never has to branch on URL strings.
 */
export type NodeView =
  | { kind: 'loading' }
  | { kind: 'space_root' }
  | { kind: 'collection'; node: CollectionTreeNode; ancestors: CollectionTreeNode[] }
  | { kind: 'uncategorized' }

/**
 * StatusFilter mirrors the table's filter state. `null` means "no
 * filter" — every status (including archived) is visible. The
 * space-detail page fetches with include_archived=true so archived
 * docs are always in scope until the user filters them out.
 */
export interface StatusFilter {
  status: DocStatus | null
}

/**
 * resolveView turns the `?collection=` URL param plus the loaded
 * tree into a NodeView.
 *
 * Fallback rules:
 * - Missing / empty param → space_root (or loading when tree is null)
 * - '__uncollected__' → uncategorized (or loading when tree is null)
 * - Unknown UUID → space_root (caller is expected to clean up the URL
 *   via a toast + replace navigation; see DocsSpaceDetail §4.10)
 */
export function resolveView(
  activeCollection: string | null | undefined,
  tree: CollectionTree | null,
): NodeView {
  if (!activeCollection) {
    return tree ? { kind: 'space_root' } : { kind: 'loading' }
  }
  if (activeCollection === '__uncollected__') {
    return tree ? { kind: 'uncategorized' } : { kind: 'loading' }
  }
  if (!tree) return { kind: 'loading' }
  const node = findCollectionNode(tree.topLevel, activeCollection)
  if (!node) return { kind: 'space_root' }
  const ancestors = collectionAncestorChain(tree.topLevel, activeCollection)
  return { kind: 'collection', node, ancestors }
}

/**
 * directChildrenOfView returns the child collection nodes that
 * should be rendered as cards at the current view. Never re-sorts:
 * the order already matches the sidebar because buildCollectionTree
 * is the single source of ordering.
 */
export function directChildrenOfView(
  view: NodeView,
  tree: CollectionTree | null,
): CollectionTreeNode[] {
  if (!tree) return []
  if (view.kind === 'space_root') return tree.topLevel
  if (view.kind === 'collection') return view.node.children
  return []
}

function passesFilter(doc: DocsDocument, filter: StatusFilter): boolean {
  if (!filter.status) return true
  return doc.status === filter.status
}

/**
 * scopedDocuments returns the documents the table should render for
 * the current view, applying the shared status filter.
 *
 * Reads directly from the pre-partitioned tree so the page and the
 * sidebar always agree on which docs belong to which collection —
 * there is no parallel partitioning here.
 */
export function scopedDocuments(
  view: NodeView,
  tree: CollectionTree | null,
  filter: StatusFilter,
): DocsDocument[] {
  if (!tree) return []
  if (view.kind === 'space_root') {
    const out: DocsDocument[] = []
    const walk = (nodes: CollectionTreeNode[]) => {
      for (const node of nodes) {
        for (const doc of node.documents) {
          if (passesFilter(doc, filter)) out.push(doc)
        }
        if (node.children.length > 0) walk(node.children)
      }
    }
    walk(tree.topLevel)
    for (const doc of tree.uncategorizedDocuments) {
      if (passesFilter(doc, filter)) out.push(doc)
    }
    return out
  }
  if (view.kind === 'collection') {
    return view.node.documents.filter((d) => passesFilter(d, filter))
  }
  if (view.kind === 'uncategorized') {
    return tree.uncategorizedDocuments.filter((d) => passesFilter(d, filter))
  }
  return []
}

/**
 * countDocsInSubtree returns the recursive total of documents under
 * a collection node (including the node's own direct docs and every
 * descendant's docs). Used for collection card previews so users can
 * see at a glance how big a subtree is. Applies the same filter as
 * scopedDocuments so header/card counts match the table.
 */
export function countDocsInSubtree(
  node: CollectionTreeNode,
  filter: StatusFilter,
): number {
  let count = 0
  for (const doc of node.documents) {
    if (passesFilter(doc, filter)) count += 1
  }
  for (const child of node.children) {
    count += countDocsInSubtree(child, filter)
  }
  return count
}

/**
 * countDirectDocs returns the number of documents directly attached
 * to a given node, OR to the uncategorized bucket when nodeOrNull is
 * null. Tree null → 0 (loading state).
 */
export function countDirectDocs(
  nodeOrNull: CollectionTreeNode | null,
  tree: CollectionTree | null,
  filter: StatusFilter,
): number {
  if (!tree) return 0
  const docs = nodeOrNull ? nodeOrNull.documents : tree.uncategorizedDocuments
  let count = 0
  for (const doc of docs) {
    if (passesFilter(doc, filter)) count += 1
  }
  return count
}

/**
 * countDirectChildren returns the number of immediate child
 * collections of a node, OR of the space root when nodeOrNull is
 * null. Tree null → 0 (loading state).
 */
export function countDirectChildren(
  nodeOrNull: CollectionTreeNode | null,
  tree: CollectionTree | null,
): number {
  if (!tree) return 0
  return nodeOrNull ? nodeOrNull.children.length : tree.topLevel.length
}
