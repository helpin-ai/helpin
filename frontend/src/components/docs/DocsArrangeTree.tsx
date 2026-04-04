import { useState } from 'react'
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
import { ICON_MAP } from '@/components/ui/icon-picker'
import { Collapsible } from 'radix-ui'
import {
  useDocsCollections,
  useDocsDocuments,
  useReorderDocsSpaces,
  useReorderDocsCollections,
  useReorderDocsDocuments,
} from '@/hooks/queries'
import { toast } from 'sonner'
import type { DocsSpace, DocsDocument, DocsCollection, SpaceType } from '@/lib/docsTypes'

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

// ── Document bucket (collection or uncategorized) ───────────────────────────

function ArrangeBucket({
  label,
  icon,
  documents,
  spaceId,
  collectionId,
  wsId,
}: {
  label: string
  icon?: string
  documents: DocsDocument[]
  spaceId: string
  collectionId: string | null
  wsId: string
}) {
  const [open, setOpen] = useState(true)
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }))
  const reorderDocs = useReorderDocsDocuments(wsId)
  const [localDocs, setLocalDocs] = useState<DocsDocument[] | null>(null)
  const displayDocs = localDocs ?? documents

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
      { spaceId, data: { collection_id: collectionId ?? undefined, document_ids: reorderedIds } },
      {
        onSuccess: () => toast.success('Document order updated'),
        onError: () => { toast.error('Failed to reorder'); setLocalDocs(null) },
      },
    )
  }

  const CollIcon = icon ? (ICON_MAP[icon] ?? Folder01Icon) : Folder01Icon

  return (
    <Collapsible.Root open={open} onOpenChange={setOpen}>
      <Collapsible.Trigger asChild>
        <button
          type="button"
          className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm font-medium text-foreground/70 hover:bg-muted/40"
        >
          <ArrowRight01Icon className={`h-3.5 w-3.5 shrink-0 transition-transform ${open ? 'rotate-90' : ''}`} />
          <CollIcon className="h-3.5 w-3.5 shrink-0" />
          <span className="truncate">{label}</span>
          <span className="rounded-full bg-muted px-1.5 text-[10px] tabular-nums text-muted-foreground">{displayDocs.length} {displayDocs.length === 1 ? 'doc' : 'docs'}</span>
        </button>
      </Collapsible.Trigger>
      <Collapsible.Content>
        <div className="ml-4 border-l border-border/50 pl-1">
          {displayDocs.length > 0 ? (
            <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
              <SortableContext items={displayDocs.map((d) => d.id)} strategy={verticalListSortingStrategy}>
                {displayDocs.map((doc) => (
                  <SortableItem key={doc.id} id={doc.id}>
                    <ArrangeDocRow doc={doc} />
                  </SortableItem>
                ))}
              </SortableContext>
            </DndContext>
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
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }))
  const reorderColls = useReorderDocsCollections(wsId)
  const [localColls, setLocalColls] = useState<DocsCollection[] | null>(null)

  const sortedColls = localColls ?? (collections ?? []).slice().sort((a, b) => a.position - b.position)

  const collectionMap = new Map<string, DocsDocument[]>()
  const uncollected: DocsDocument[] = []
  if (documents) {
    for (const doc of documents) {
      if (doc.collection_id) {
        const list = collectionMap.get(doc.collection_id) ?? []
        list.push(doc)
        collectionMap.set(doc.collection_id, list)
      } else {
        uncollected.push(doc)
      }
    }
  }

  const handleCollDragEnd = (event: DragEndEvent) => {
    const { active, over } = event
    if (!over || active.id === over.id) return
    const ids = sortedColls.map((c) => c.id)
    const oldIdx = ids.indexOf(active.id as string)
    const newIdx = ids.indexOf(over.id as string)
    if (oldIdx === -1 || newIdx === -1) return
    const reorderedIds = arrayMove(ids, oldIdx, newIdx)
    const reordered = reorderedIds.map((id) => sortedColls.find((c) => c.id === id)!).filter(Boolean)
    setLocalColls(reordered)
    reorderColls.mutate(
      { spaceId: space.id, data: { collection_ids: reorderedIds } },
      {
        onSuccess: () => toast.success('Collection order updated'),
        onError: () => { toast.error('Failed to reorder'); setLocalColls(null) },
      },
    )
  }

  return (
    <Collapsible.Root open={expanded} onOpenChange={setExpanded}>
      <Collapsible.Trigger asChild>
        <button
          type="button"
          className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2.5 text-left transition-colors hover:bg-muted/60"
        >
          <ArrowRight01Icon className={`h-4 w-4 shrink-0 text-muted-foreground transition-transform ${expanded ? 'rotate-90' : ''}`} />
          {space.icon ? (
            <span className="text-base">{space.icon}</span>
          ) : (
            <Folder01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
          )}
          <div className="min-w-0 flex-1 flex items-center gap-2">
            <span className="truncate text-sm font-semibold">{space.name}</span>
          </div>
          {!expanded && (
            <div className="flex items-center gap-5 shrink-0">
              {sortedColls.length > 0 && (
                <span className="text-[11px] text-muted-foreground">{sortedColls.length} {sortedColls.length === 1 ? 'collection' : 'collections'}</span>
              )}
              {(documents?.length ?? 0) > 0 && (
                <span className="text-[11px] text-muted-foreground">{documents?.length} {documents?.length === 1 ? 'doc' : 'docs'}</span>
              )}
            </div>
          )}
        </button>
      </Collapsible.Trigger>
      <Collapsible.Content>
        <div className="ml-5 border-l border-border/50 pb-2 pl-2">
          {/* Sortable collections */}
          {sortedColls.length > 0 && (
            <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleCollDragEnd}>
              <SortableContext items={sortedColls.map((c) => c.id)} strategy={verticalListSortingStrategy}>
                {sortedColls.map((coll) => (
                  <SortableItem key={coll.id} id={coll.id}>
                    <ArrangeBucket
                      label={coll.name}
                      icon={coll.icon}
                      documents={collectionMap.get(coll.id) ?? []}
                      spaceId={space.id}
                      collectionId={coll.id}
                      wsId={wsId}
                    />
                  </SortableItem>
                ))}
              </SortableContext>
            </DndContext>
          )}

          {/* Uncategorized bucket */}
          {uncollected.length > 0 && (
            <ArrangeBucket
              label="Uncategorized"
              documents={uncollected}
              spaceId={space.id}
              collectionId={null}
              wsId={wsId}
            />
          )}
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
