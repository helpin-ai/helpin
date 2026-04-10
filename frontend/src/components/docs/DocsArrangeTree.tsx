import { useMemo, useState } from 'react'
import {
  DndContext,
  closestCenter,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import {
  SortableContext,
  verticalListSortingStrategy,
  useSortable,
  arrayMove,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { ArrowRight01Icon, File01Icon, Folder01Icon, DragDropVerticalIcon } from '@/lib/icons'
import { timeAgo } from '@/lib/utils'
import { DOC_STATUS_LABELS } from '@/lib/docsTypes'
import { ICON_MAP, StoredIcon } from '@/components/ui/icon-picker'
import { Collapsible } from 'radix-ui'
import {
  useDocsCollections,
  useDocsDocuments,
  useReorderDocsSpaces,
  useReorderDocsCollections,
  useReorderDocsDocuments,
} from '@/hooks/queries'
import { toast } from 'sonner'
import type { DocsSpace, DocsDocument, SpaceType } from '@/lib/docsTypes'
import { buildCollectionTree, type CollectionTreeNode } from './docsCollectionTree'

// ── Sortable item wrapper ───────────────────────────────────────────────────

function SortableItem({ id, children }: { id: string; children: React.ReactNode }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id })
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
    zIndex: isDragging ? 50 : undefined,
  }
  return (
    <div ref={setNodeRef} style={style} {...attributes} className="flex items-center group/sortable">
      <button
        type="button"
        {...listeners}
        className="flex h-8 w-6 shrink-0 items-center justify-center cursor-grab rounded text-muted-foreground/40 hover:text-foreground hover:bg-muted/60 active:cursor-grabbing"
      >
        <DragDropVerticalIcon className="h-4 w-4" />
      </button>
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  )
}

// ── Document row ────────────────────────────────────────────────────────────

function statusColor(status: string): string {
  switch (status) {
    case 'published': return 'text-emerald-600 dark:text-emerald-400'
    case 'archived': return 'text-muted-foreground/60'
    default: return 'text-amber-600 dark:text-amber-400'
  }
}

function ArrangeDocRow({ doc }: { doc: DocsDocument }) {
  return (
    <div className="flex items-center gap-2.5 rounded-md px-3 py-2 text-sm">
      <File01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
      <span className="min-w-0 flex-1 truncate font-medium">{doc.title}</span>
      <span className={`shrink-0 text-xs font-medium ${statusColor(doc.status)}`}>
        {DOC_STATUS_LABELS[doc.status] ?? doc.status}
      </span>
      <span className="shrink-0 text-[11px] text-muted-foreground">
        Updated: {timeAgo(doc.updated_at)}
      </span>
    </div>
  )
}

// ── Reorderable child collection group ──────────────────────────────────────
//
// Renders one sibling bucket of collection tree nodes. A parent passes its
// direct children here; they become a dnd-kit SortableContext scoped to
// this specific (space_id, parent_collection_id) bucket. Reordering inside
// one bucket never touches siblings in a different bucket because the
// backend's ReorderSiblings is bucket-scoped by design.
function ArrangeCollectionChildren({
  parentCollectionId,
  children,
  spaceId,
  wsId,
}: {
  parentCollectionId: string | null
  children: CollectionTreeNode[]
  spaceId: string
  wsId: string
}) {
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }))
  const reorderColls = useReorderDocsCollections(wsId)
  const [localOrder, setLocalOrder] = useState<CollectionTreeNode[] | null>(null)
  const ordered = localOrder ?? children

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event
    if (!over || active.id === over.id) return
    const ids = ordered.map((node) => node.collection.id)
    const oldIdx = ids.indexOf(active.id as string)
    const newIdx = ids.indexOf(over.id as string)
    if (oldIdx === -1 || newIdx === -1) return
    const reorderedIds = arrayMove(ids, oldIdx, newIdx)
    const reorderedNodes = reorderedIds
      .map((id) => ordered.find((node) => node.collection.id === id)!)
      .filter(Boolean)
    setLocalOrder(reorderedNodes)
    reorderColls.mutate(
      {
        spaceId,
        data: {
          collection_ids: reorderedIds,
          parent_collection_id: parentCollectionId ?? '',
        },
      },
      {
        onSuccess: () => toast.success('Collection order updated'),
        onError: () => {
          toast.error('Failed to reorder')
          setLocalOrder(null)
        },
      },
    )
  }

  if (ordered.length === 0) return null

  return (
    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
      <SortableContext items={ordered.map((n) => n.collection.id)} strategy={verticalListSortingStrategy}>
        {ordered.map((node) => (
          <SortableItem key={node.collection.id} id={node.collection.id}>
            <ArrangeCollectionNode node={node} spaceId={spaceId} wsId={wsId} />
          </SortableItem>
        ))}
      </SortableContext>
    </DndContext>
  )
}

// ── Recursive collection node ───────────────────────────────────────────────
//
// Renders one collection with its direct articles and nested children.
// Child collections live in their own reorderable bucket and articles
// live in their own reorderable bucket — this matches the backend
// contract where (space_id, parent_collection_id) and (space_id,
// collection_id) are independent ordering keys.
function ArrangeCollectionNode({
  node,
  spaceId,
  wsId,
}: {
  node: CollectionTreeNode
  spaceId: string
  wsId: string
}) {
  const [open, setOpen] = useState(true)
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }))
  const reorderDocs = useReorderDocsDocuments(wsId)
  const [localDocs, setLocalDocs] = useState<DocsDocument[] | null>(null)
  const displayDocs = localDocs ?? node.documents

  const handleDocDragEnd = (event: DragEndEvent) => {
    const { active, over } = event
    if (!over || active.id === over.id) return
    const ids = displayDocs.map((d) => d.id)
    const oldIdx = ids.indexOf(active.id as string)
    const newIdx = ids.indexOf(over.id as string)
    if (oldIdx === -1 || newIdx === -1) return
    const reorderedIds = arrayMove(ids, oldIdx, newIdx)
    const reorderedDocs = reorderedIds.map((id) => displayDocs.find((d) => d.id === id)!).filter(Boolean)
    setLocalDocs(reorderedDocs)
    reorderDocs.mutate(
      {
        spaceId,
        data: {
          collection_id: node.collection.id,
          document_ids: reorderedIds,
        },
      },
      {
        onSuccess: () => toast.success('Document order updated'),
        onError: () => {
          toast.error('Failed to reorder')
          setLocalDocs(null)
        },
      },
    )
  }

  const CollIcon = node.collection.icon ? (ICON_MAP[node.collection.icon] ?? Folder01Icon) : Folder01Icon
  const totalChildren = node.children.length
  const hasContent = totalChildren > 0 || displayDocs.length > 0

  return (
    <Collapsible.Root open={open} onOpenChange={setOpen}>
      <Collapsible.Trigger asChild>
        <button
          type="button"
          className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm font-medium text-foreground/70 hover:bg-muted/40"
        >
          <ArrowRight01Icon className={`h-3.5 w-3.5 shrink-0 transition-transform ${open ? 'rotate-90' : ''}`} />
          <CollIcon className="h-3.5 w-3.5 shrink-0" />
          <span className="truncate">{node.collection.name}</span>
          {displayDocs.length > 0 && (
            <span className="rounded-full bg-muted px-1.5 text-[10px] tabular-nums text-muted-foreground">
              {displayDocs.length} {displayDocs.length === 1 ? 'doc' : 'docs'}
            </span>
          )}
          {totalChildren > 0 && (
            <span className="rounded-full bg-muted px-1.5 text-[10px] tabular-nums text-muted-foreground">
              {totalChildren} sub
            </span>
          )}
        </button>
      </Collapsible.Trigger>
      <Collapsible.Content>
        <div className="ml-4 border-l border-border/50 pl-1">
          {/* Nested child collections first — they act as container nodes. */}
          {totalChildren > 0 && (
            <ArrangeCollectionChildren
              parentCollectionId={node.collection.id}
              children={node.children}
              spaceId={spaceId}
              wsId={wsId}
            />
          )}
          {/* Then direct articles of this collection. */}
          {displayDocs.length > 0 ? (
            <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDocDragEnd}>
              <SortableContext items={displayDocs.map((d) => d.id)} strategy={verticalListSortingStrategy}>
                {displayDocs.map((doc) => (
                  <SortableItem key={doc.id} id={doc.id}>
                    <ArrangeDocRow doc={doc} />
                  </SortableItem>
                ))}
              </SortableContext>
            </DndContext>
          ) : !hasContent ? (
            <p className="px-2 py-1.5 text-[11px] text-muted-foreground/60">No documents</p>
          ) : null}
        </div>
      </Collapsible.Content>
    </Collapsible.Root>
  )
}

// ── Uncategorized articles bucket ───────────────────────────────────────────

function ArrangeUncategorizedBucket({
  documents,
  spaceId,
  wsId,
}: {
  documents: DocsDocument[]
  spaceId: string
  wsId: string
}) {
  const [open, setOpen] = useState(true)
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }))
  const reorderDocs = useReorderDocsDocuments(wsId)
  const [localDocs, setLocalDocs] = useState<DocsDocument[] | null>(null)
  const displayDocs = localDocs ?? documents

  if (displayDocs.length === 0) return null

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event
    if (!over || active.id === over.id) return
    const ids = displayDocs.map((d) => d.id)
    const oldIdx = ids.indexOf(active.id as string)
    const newIdx = ids.indexOf(over.id as string)
    if (oldIdx === -1 || newIdx === -1) return
    const reorderedIds = arrayMove(ids, oldIdx, newIdx)
    const reorderedDocs = reorderedIds.map((id) => displayDocs.find((d) => d.id === id)!).filter(Boolean)
    setLocalDocs(reorderedDocs)
    reorderDocs.mutate(
      { spaceId, data: { collection_id: undefined, document_ids: reorderedIds } },
      {
        onSuccess: () => toast.success('Document order updated'),
        onError: () => {
          toast.error('Failed to reorder')
          setLocalDocs(null)
        },
      },
    )
  }

  return (
    <Collapsible.Root open={open} onOpenChange={setOpen}>
      <Collapsible.Trigger asChild>
        <button
          type="button"
          className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm font-medium text-foreground/70 hover:bg-muted/40"
        >
          <ArrowRight01Icon className={`h-3.5 w-3.5 shrink-0 transition-transform ${open ? 'rotate-90' : ''}`} />
          <Folder01Icon className="h-3.5 w-3.5 shrink-0" />
          <span className="truncate">Uncategorized</span>
          <span className="rounded-full bg-muted px-1.5 text-[10px] tabular-nums text-muted-foreground">
            {displayDocs.length} {displayDocs.length === 1 ? 'doc' : 'docs'}
          </span>
        </button>
      </Collapsible.Trigger>
      <Collapsible.Content>
        <div className="ml-4 border-l border-border/50 pl-1">
          <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
            <SortableContext items={displayDocs.map((d) => d.id)} strategy={verticalListSortingStrategy}>
              {displayDocs.map((doc) => (
                <SortableItem key={doc.id} id={doc.id}>
                  <ArrangeDocRow doc={doc} />
                </SortableItem>
              ))}
            </SortableContext>
          </DndContext>
        </div>
      </Collapsible.Content>
    </Collapsible.Root>
  )
}

// ── Space node (fetches its own collections + documents) ────────────────────

function ArrangeSpace({
  space,
  wsId,
}: {
  space: DocsSpace
  wsId: string
}) {
  const [expanded, setExpanded] = useState(false)
  const { data: collections } = useDocsCollections(wsId, space.id)
  const { data: documents } = useDocsDocuments(wsId, { space_id: space.id })

  // Fold the flat (collections, documents) response into a tree rooted
  // at this space. Memoised on the two query results so rerenders from
  // unrelated state don't rebuild the tree on every pass.
  const tree = useMemo(
    () => buildCollectionTree(space.id, collections ?? [], documents ?? []),
    [space.id, collections, documents],
  )

  const collectionCount = collections?.length ?? 0
  const documentCount = documents?.length ?? 0

  return (
    <Collapsible.Root open={expanded} onOpenChange={setExpanded}>
      <Collapsible.Trigger asChild>
        <button
          type="button"
          className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2.5 text-left transition-colors hover:bg-muted/60"
        >
          <ArrowRight01Icon className={`h-4 w-4 shrink-0 text-muted-foreground transition-transform ${expanded ? 'rotate-90' : ''}`} />
          <StoredIcon
            name={space.icon}
            className="h-4 w-4 shrink-0 text-muted-foreground"
            textClassName="text-base"
            fallback={<Folder01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />}
          />
          <div className="min-w-0 flex-1 flex items-center gap-2">
            <span className="truncate text-sm font-semibold">{space.name}</span>
          </div>
          {!expanded && (
            <div className="flex items-center gap-5 shrink-0">
              {collectionCount > 0 && (
                <span className="text-[11px] text-muted-foreground">
                  {collectionCount} {collectionCount === 1 ? 'collection' : 'collections'}
                </span>
              )}
              {documentCount > 0 && (
                <span className="text-[11px] text-muted-foreground">
                  {documentCount} {documentCount === 1 ? 'doc' : 'docs'}
                </span>
              )}
            </div>
          )}
        </button>
      </Collapsible.Trigger>
      <Collapsible.Content>
        <div className="ml-5 border-l border-border/50 pb-2 pl-2">
          {/* Top-level collections form the first sibling bucket. Each
              recursive ArrangeCollectionNode owns its own children bucket
              and doc bucket so drag-drop stays scoped to one (space,
              parent) pair at a time. */}
          {tree.topLevel.length > 0 && (
            <ArrangeCollectionChildren
              parentCollectionId={null}
              children={tree.topLevel}
              spaceId={space.id}
              wsId={wsId}
            />
          )}
          {/* Uncategorized articles bucket sits alongside top-level
              collections and is always top-level in the space. */}
          <ArrangeUncategorizedBucket
            documents={tree.uncategorizedDocuments}
            spaceId={space.id}
            wsId={wsId}
          />
        </div>
      </Collapsible.Content>
    </Collapsible.Root>
  )
}

// ── Section (internal / external) ───────────────────────────────────────────

function ArrangeSection({
  label,
  sectionType,
  spaces,
  wsId,
}: {
  label: string
  sectionType: SpaceType
  spaces: DocsSpace[]
  wsId: string
}) {
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }))
  const reorderSpaces = useReorderDocsSpaces(wsId)
  const [localSpaces, setLocalSpaces] = useState<DocsSpace[] | null>(null)
  const displaySpaces = localSpaces ?? spaces

  if (displaySpaces.length === 0) return null

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event
    if (!over || active.id === over.id) return
    const ids = displaySpaces.map((s) => s.id)
    const oldIdx = ids.indexOf(active.id as string)
    const newIdx = ids.indexOf(over.id as string)
    if (oldIdx === -1 || newIdx === -1) return
    const reorderedIds = arrayMove(ids, oldIdx, newIdx)
    const reordered = reorderedIds.map((id) => displaySpaces.find((s) => s.id === id)!).filter(Boolean)
    setLocalSpaces(reordered)
    reorderSpaces.mutate(
      { section: sectionType, space_ids: reorderedIds },
      {
        onSuccess: () => toast.success('Space order updated'),
        onError: () => { toast.error('Failed to reorder'); setLocalSpaces(null) },
      },
    )
  }

  return (
    <div>
      <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        {label}
      </h3>
      <div className="divide-y divide-border/50 rounded-lg border border-border/60 bg-card">
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
          <SortableContext items={displaySpaces.map((s) => s.id)} strategy={verticalListSortingStrategy}>
            {displaySpaces.map((space) => (
              <SortableItem key={space.id} id={space.id}>
                <ArrangeSpace space={space} wsId={wsId} />
              </SortableItem>
            ))}
          </SortableContext>
        </DndContext>
      </div>
    </div>
  )
}

// ── Main export ─────────────────────────────────────────────────────────────

export function DocsArrangeTree({
  spaces,
  wsId,
}: {
  spaces: DocsSpace[]
  wsId: string
}) {
  const internal = spaces.filter((s) => s.type === 'internal').sort((a, b) => a.position - b.position)
  const external = spaces.filter((s) => s.type === 'external_capable').sort((a, b) => a.position - b.position)

  return (
    <div className="space-y-6">
      <ArrangeSection label="Team Spaces" sectionType="internal" spaces={internal} wsId={wsId} />
      <ArrangeSection label="External Spaces" sectionType="external_capable" spaces={external} wsId={wsId} />
    </div>
  )
}
