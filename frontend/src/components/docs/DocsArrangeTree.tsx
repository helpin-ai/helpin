import { useMemo, useState } from 'react'
import {
  DndContext,
  DragOverlay,
  closestCenter,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core'
import {
  SortableContext,
  verticalListSortingStrategy,
  useSortable,
  arrayMove,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { ArrowRight01Icon, File01Icon, Folder01Icon, DragDropVerticalIcon, PlusSignIcon } from '@/lib/icons'
import { timeAgo } from '@/lib/utils'
import { DOC_STATUS_LABELS } from '@/lib/docsTypes'
import { ICON_MAP, StoredIcon } from '@/components/ui/icon-picker'
import { Collapsible } from 'radix-ui'
import {
  useDocsCollections,
  useDocsDocuments,
  useReorderDocsSpaces,
  useReorderDocsChildren,
} from '@/hooks/queries'
import { toast } from 'sonner'
import { COLLECTION_ROW_CLASS, ARTICLE_ROW_CLASS, DOC_ICON_CLASS, COLLECTION_ICON_CLASS, STATUS_BADGE_CLASS, UPDATED_TEXT_CLASS, COUNT_BADGE_CLASS, statusColor } from '@/pages/docs/docsTreeStyles'
import type { DocsSpace, DocsDocument, SpaceType } from '@/lib/docsTypes'
import { buildCollectionTree, type CollectionTreeNode } from './docsCollectionTree'
import { CreateCollectionDialog } from './CreateCollectionDialog'

// Must match maxCollectionDepth in server/internal/service/docs_collection.go.
// Collections at this depth cannot host any more children.
const MAX_COLLECTION_DEPTH = 1

// ── Sortable item wrapper ───────────────────────────────────────────────────

function SortableItem({ id, children }: { id: string; children: React.ReactNode }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id })
  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition: transition ?? 'transform 200ms ease',
  }
  return (
    <div
      ref={setNodeRef}
      style={style}
      {...attributes}
      className={`flex items-center group/sortable ${isDragging ? 'opacity-30' : ''}`}
    >
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

/**
 * DragPreview is the compact floating card rendered inside every
 * DragOverlay. It shows just the icon + name of the dragged item
 * so the cursor follows a clean, fixed-size preview regardless
 * of how large the source item's subtree is.
 */
function DragPreview({
  icon,
  label,
  kind,
  docCount,
  subCount,
}: {
  icon?: string | null
  label: string
  kind: 'space' | 'collection' | 'document'
  /** Direct documents inside (collections/spaces only). */
  docCount?: number
  /** Direct sub-collections inside (collections/spaces only). */
  subCount?: number
}) {
  const meta: string[] = [kind === 'document' ? 'Document' : kind === 'space' ? 'Space' : 'Collection']
  if (kind !== 'document') {
    if ((docCount ?? 0) > 0) meta.push(`${docCount} ${docCount === 1 ? 'document' : 'documents'}`)
    if ((subCount ?? 0) > 0) meta.push(`${subCount} ${subCount === 1 ? (kind === 'space' ? 'collection' : 'sub-collection') : (kind === 'space' ? 'collections' : 'sub-collections')}`)
  }

  return (
    <div className="flex items-center gap-2.5 rounded-md border border-border/60 bg-card px-3 py-2 shadow-lg">
      {kind === 'document' ? (
        <File01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
      ) : (
        <StoredIcon
          name={icon}
          className="h-4 w-4 shrink-0 text-muted-foreground"
          textClassName=""
          fallback={<Folder01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />}
        />
      )}
      <div className="min-w-0">
        <div className="max-w-[220px] truncate text-sm font-medium">{label}</div>
        <div className="text-[11px] text-muted-foreground">{meta.join(' · ')}</div>
      </div>
    </div>
  )
}

/**
 * SpaceDragPreview wraps DragPreview with hooks that read the
 * space's collection/document counts from the TanStack Query cache.
 * ArrangeSpace already fetched this data, so the hooks resolve
 * instantly from cache with no new network requests.
 */
function SpaceDragPreview({ space, wsId }: { space: DocsSpace; wsId: string }) {
  const { data: collections } = useDocsCollections(wsId, space.id)
  const { data: documents } = useDocsDocuments(wsId, { space_id: space.id })
  return (
    <DragPreview
      icon={space.icon}
      label={space.name}
      kind="space"
      docCount={documents?.length}
      subCount={collections?.length}
    />
  )
}

// ── Document row ────────────────────────────────────────────────────────────

function ArrangeDocRow({ doc }: { doc: DocsDocument }) {
  return (
    <div className={`flex items-center gap-2.5 rounded-md px-3 py-2 ${ARTICLE_ROW_CLASS}`}>
      <File01Icon className={DOC_ICON_CLASS} />
      <span className="min-w-0 flex-1 truncate">{doc.title}</span>
      <span className={`${STATUS_BADGE_CLASS} ${statusColor(doc.status)}`}>
        {DOC_STATUS_LABELS[doc.status] ?? doc.status}
      </span>
      <span className={UPDATED_TEXT_CLASS}>
        Updated: {timeAgo(doc.updated_at)}
      </span>
    </div>
  )
}

// ── Unified bucket items (Model B: mixed collections + docs) ──────────────
//
// Replaces the old two-zone approach (separate DndContexts for collections
// and docs) with a single drag zone per bucket. Collections and docs share
// a sort_key space, so the user can drag any item to any position.
type BucketItem =
  | { type: 'collection'; id: string; sortKey: string; node: CollectionTreeNode }
  | { type: 'doc'; id: string; sortKey: string; doc: DocsDocument }

function ArrangeBucketItems({
  parentCollectionId,
  collections,
  documents,
  spaceId,
  wsId,
  onAddSubCollection,
}: {
  parentCollectionId: string | null
  collections: CollectionTreeNode[]
  documents: DocsDocument[]
  spaceId: string
  wsId: string
  onAddSubCollection: (parentId: string) => void
}) {
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }))
  const reorderChildren = useReorderDocsChildren(wsId)

  const merged = useMemo<BucketItem[]>(() => {
    const items: BucketItem[] = [
      ...collections.map((n) => ({
        type: 'collection' as const,
        id: n.collection.id,
        sortKey: n.collection.sort_key ?? '',
        node: n,
      })),
      ...documents.map((d) => ({
        type: 'doc' as const,
        id: d.id,
        sortKey: d.sort_key ?? '',
        doc: d,
      })),
    ]
    items.sort((a, b) => a.sortKey.localeCompare(b.sortKey) || a.id.localeCompare(b.id))
    return items
  }, [collections, documents])

  const [localOrder, setLocalOrder] = useState<BucketItem[] | null>(null)
  const [dragActiveId, setDragActiveId] = useState<string | null>(null)
  const ordered = localOrder ?? merged

  // Reset local state when upstream data changes (e.g., after mutation settles).
  useMemo(() => setLocalOrder(null), [merged])

  const handleDragEnd = (event: DragEndEvent) => {
    setDragActiveId(null)
    const { active, over } = event
    if (!over || active.id === over.id) return
    const ids = ordered.map((item) => `${item.type}:${item.id}`)
    const oldIdx = ids.indexOf(active.id as string)
    const newIdx = ids.indexOf(over.id as string)
    if (oldIdx === -1 || newIdx === -1) return
    const reordered = arrayMove([...ordered], oldIdx, newIdx)
    setLocalOrder(reordered)
    reorderChildren.mutate(
      {
        spaceId,
        data: {
          parent_collection_id: parentCollectionId ?? '',
          items: reordered.map((item) => ({
            kind: item.type === 'collection' ? 'collection' : 'article',
            id: item.id,
          })),
        },
      },
      {
        onSuccess: () => toast.success('Order updated'),
        onError: () => {
          toast.error('Failed to reorder')
          setLocalOrder(null)
        },
      },
    )
  }

  if (ordered.length === 0) return null

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCenter}
      onDragStart={(event: DragStartEvent) => setDragActiveId(event.active.id as string)}
      onDragEnd={handleDragEnd}
      onDragCancel={() => setDragActiveId(null)}
    >
      <SortableContext items={ordered.map((item) => `${item.type}:${item.id}`)} strategy={verticalListSortingStrategy}>
        {ordered.map((item) => {
          const prefixedId = `${item.type}:${item.id}`
          if (item.type === 'collection') {
            return (
              <SortableItem key={prefixedId} id={prefixedId}>
                <ArrangeCollectionNode
                  node={item.node}
                  spaceId={spaceId}
                  wsId={wsId}
                  onAddSubCollection={onAddSubCollection}
                  forceCollapsed={dragActiveId === prefixedId}
                />
              </SortableItem>
            )
          }
          return (
            <SortableItem key={prefixedId} id={prefixedId}>
              <ArrangeDocRow doc={item.doc} />
            </SortableItem>
          )
        })}
      </SortableContext>
      <DragOverlay dropAnimation={null}>
        {dragActiveId
          ? (() => {
              const item = ordered.find((i) => `${i.type}:${i.id}` === dragActiveId)
              if (!item) return null
              if (item.type === 'collection') {
                return (
                  <DragPreview
                    icon={item.node.collection.icon}
                    label={item.node.collection.name}
                    kind="collection"
                    docCount={item.node.documents.length}
                    subCount={item.node.children.length}
                  />
                )
              }
              return <DragPreview label={item.doc.title} kind="document" />
            })()
          : null}
      </DragOverlay>
    </DndContext>
  )
}

// ── Recursive collection node ───────────────────────────────────────────────
//
// Renders one collection header with its content (sub-collections + docs)
// inside a collapsible panel. The content is handled by ArrangeBucketItems
// which provides a single unified drag zone.
function ArrangeCollectionNode({
  node,
  spaceId,
  wsId,
  onAddSubCollection,
  forceCollapsed = false,
}: {
  node: CollectionTreeNode
  spaceId: string
  wsId: string
  onAddSubCollection: (parentId: string) => void
  /** When true, the node collapses to a single row during drag so
   *  the user sees a compact preview instead of the full subtree. */
  forceCollapsed?: boolean
}) {
  const [open, setOpen] = useState(false)
  const effectiveOpen = forceCollapsed ? false : open
  const canHostChildren = node.collection.depth < MAX_COLLECTION_DEPTH

  const CollIcon = node.collection.icon ? (ICON_MAP[node.collection.icon] ?? Folder01Icon) : Folder01Icon
  const totalChildren = node.children.length
  const hasContent = totalChildren > 0 || node.documents.length > 0

  return (
    <Collapsible.Root open={effectiveOpen} onOpenChange={setOpen}>
      <div className="group/arrange-node flex w-full items-center gap-1">
        <Collapsible.Trigger asChild>
          <button
            type="button"
            className={`flex min-w-0 flex-1 items-center gap-2 rounded-md px-3 py-2 ${COLLECTION_ROW_CLASS} hover:bg-muted/40`}
          >
            <ArrowRight01Icon className={`h-3.5 w-3.5 shrink-0 transition-transform ${effectiveOpen ? 'rotate-90' : ''}`} />
            <CollIcon className={COLLECTION_ICON_CLASS} />
            <span className="truncate">{node.collection.name}</span>
            {node.documents.length > 0 && (
              <span className={COUNT_BADGE_CLASS}>
                {node.documents.length} {node.documents.length === 1 ? 'doc' : 'docs'}
              </span>
            )}
            {totalChildren > 0 && (
              <span className={COUNT_BADGE_CLASS}>
                {totalChildren} sub
              </span>
            )}
          </button>
        </Collapsible.Trigger>
        {canHostChildren && (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation()
              onAddSubCollection(node.collection.id)
            }}
            className="shrink-0 rounded-md p-1.5 text-muted-foreground/70 opacity-0 transition-opacity hover:bg-muted/60 hover:text-foreground group-hover/arrange-node:opacity-100 focus:opacity-100"
            aria-label={`Add sub-collection under ${node.collection.name}`}
            title="Add sub-collection"
          >
            <PlusSignIcon className="h-3.5 w-3.5" />
          </button>
        )}
      </div>
      <Collapsible.Content>
        <div className="ml-4 border-l border-border/50 pl-1">
          {hasContent ? (
            <ArrangeBucketItems
              parentCollectionId={node.collection.id}
              collections={node.children}
              documents={node.documents}
              spaceId={spaceId}
              wsId={wsId}
              onAddSubCollection={onAddSubCollection}
            />
          ) : (
            <p className="px-2 py-1.5 text-[11px] text-muted-foreground/60">No documents</p>
          )}
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
  // Dialog state for the "Add sub-collection" action. When a user
  // clicks the + button on an ArrangeCollectionNode, we stash the
  // parent id here and open the create dialog. A null parent means
  // the dialog is closed.
  const [addSubParentId, setAddSubParentId] = useState<string | null>(null)

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
          {/* Unified bucket: top-level collections + uncategorized docs
              in a single drag zone. Users can drag any item to any
              position — the sort_key model makes this safe. */}
          <ArrangeBucketItems
            parentCollectionId={null}
            collections={tree.topLevel}
            documents={tree.uncategorizedDocuments}
            spaceId={space.id}
            wsId={wsId}
            onAddSubCollection={setAddSubParentId}
          />
        </div>
      </Collapsible.Content>
      <CreateCollectionDialog
        wsId={wsId}
        spaceId={space.id}
        open={addSubParentId !== null}
        onOpenChange={(open) => {
          if (!open) setAddSubParentId(null)
        }}
        defaultParentCollectionId={addSubParentId}
      />
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
  const [spaceDragActiveId, setSpaceDragActiveId] = useState<string | null>(null)
  const displaySpaces = localSpaces ?? spaces

  if (displaySpaces.length === 0) return null

  const handleDragEnd = (event: DragEndEvent) => {
    setSpaceDragActiveId(null)
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
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragStart={(event: DragStartEvent) => setSpaceDragActiveId(event.active.id as string)}
          onDragEnd={handleDragEnd}
          onDragCancel={() => setSpaceDragActiveId(null)}
        >
          <SortableContext items={displaySpaces.map((s) => s.id)} strategy={verticalListSortingStrategy}>
            {displaySpaces.map((space) => (
              <SortableItem key={space.id} id={space.id}>
                <ArrangeSpace space={space} wsId={wsId} />
              </SortableItem>
            ))}
          </SortableContext>
          <DragOverlay dropAnimation={null}>
            {spaceDragActiveId ? (() => {
              const space = displaySpaces.find((s) => s.id === spaceDragActiveId)
              return space ? <SpaceDragPreview space={space} wsId={wsId} /> : null
            })() : null}
          </DragOverlay>
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
