import type { CollectionTreeNode } from '@/components/docs/docsCollectionTree'
import { CollectionCard } from './CollectionCard'
import { countDocsInSubtree, type StatusFilter } from './nodeSelection'

export interface CollectionCardGridProps {
  /** Section heading, e.g. "Collections" or "Sub-collections". */
  title: string
  /** Nodes to render as cards, already sorted by the tree helper. */
  nodes: CollectionTreeNode[]
  /** Status filter applied to recursive doc counts so cards match the table. */
  filter: StatusFilter
  canEdit: boolean
  onOpen: (node: CollectionTreeNode) => void
  onEdit: (node: CollectionTreeNode) => void
  onDelete: (node: CollectionTreeNode) => void
}

/**
 * CollectionCardGrid renders a titled, responsive grid of
 * CollectionCard tiles. Hidden entirely when the node list is
 * empty — empty sub-collection sections should not render a
 * dangling header.
 */
export function CollectionCardGrid({
  title,
  nodes,
  filter,
  canEdit,
  onOpen,
  onEdit,
  onDelete,
}: CollectionCardGridProps) {
  if (nodes.length === 0) return null

  return (
    <section className="space-y-3">
      <h2 className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
        {title}
      </h2>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {nodes.map((node) => (
          <CollectionCard
            key={node.collection.id}
            node={node}
            docCount={countDocsInSubtree(node, filter)}
            subCount={node.children.length}
            canEdit={canEdit}
            onOpen={() => onOpen(node)}
            onEdit={() => onEdit(node)}
            onDelete={() => onDelete(node)}
          />
        ))}
      </div>
    </section>
  )
}
