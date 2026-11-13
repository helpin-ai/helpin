import { useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { useNavigate } from '@tanstack/react-router'
import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import {
  SortableContext,
  arrayMove,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { DocsBulkToolbar, DocsSelectionCheckbox } from '@/components/docs/DocsBulkToolbar'
import { DocsExceptionalStatus } from '@/components/docs/DocsLibraryList'
import { useDocsSelection } from '@/components/docs/useDocsSelection'
import { PendingProposalBadge } from '@/components/docs/proposals/PendingProposalBadge'
import { QuietFilterDropdown, QuietSearchInput, QuietTextAction } from '@/components/design-system/quiet'
import { UserAvatar } from '@/components/pm/UserAvatar'
import { Checkbox } from '@/components/ui/checkbox'
import { QuickTooltip } from '@/components/ui/quick-tooltip'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  ArchiveIcon,
  ArchiveRestoreIcon,
  ArrowDown02Icon,
  ArrowUp02Icon,
  ArrowUpDownIcon,
  Copy01Icon,
  Delete01Icon,
  DragDropVerticalIcon,
  FolderInputIcon,
  LeftToRightListNumberIcon,
  MoreHorizontalIcon,
  SentIcon,
} from '@/lib/icons'
import { formatAssignableMemberName } from '@/lib/assignableMembers'
import { DOC_STATUS_LABELS } from '@/lib/docsTypes'
import type { DocsDocument, DocStatus } from '@/lib/docsTypes'
import type { AssignableMember } from '@/lib/types'
import { cn, timeAgo } from '@/lib/utils'
import { placeVisibleOrder, type DocsOrderGroup } from './docsOrder'

export type DocumentsTableSortField = 'title' | 'path' | 'owner' | 'updated_at'
export type DocumentsTableSortDir = 'asc' | 'desc'

const DEFAULT_SORT_DIR: Record<DocumentsTableSortField, DocumentsTableSortDir> = {
  title: 'asc',
  path: 'asc',
  owner: 'asc',
  updated_at: 'desc',
}

const STATUS_OPTIONS = (Object.entries(DOC_STATUS_LABELS) as [DocStatus, string][])
  .map(([value, label]) => ({ value, label }))

const ROW_MOVED_FLASH_MS = 1600

export interface DocumentsTablePathColumn {
  /** Column heading: "Collection" at the space root, "Sub-collection" inside one. */
  label: string
  /** Shown for documents without a path, e.g. "Uncategorized" or "—". */
  emptyLabel: string
  pathFor: (doc: DocsDocument) => string | null
}

/** One drop in the saved-order view. */
export interface DocsOrderChange {
  doc: DocsDocument
  from: DocsOrderGroup
  to: DocsOrderGroup
  /** Every document id in `to`, all statuses, in the new order (includes `doc`). */
  toOrder: string[]
  /** Every document id in `from`, all statuses, in the order before the drag. */
  fromOrderBefore: string[]
}

export interface DocumentsTableOrder {
  /** Toggle label, e.g. "Help center order" or "Sidebar order". */
  label: string
  /** Tooltip explaining where this order shows up. */
  description: string
  groups: DocsOrderGroup[]
  active: boolean
  onActiveChange: (active: boolean) => void
  /** Saves a drop. Resolves once the list can show the saved order (or the save failed). */
  onChange: (change: DocsOrderChange) => Promise<void>
}

export interface DocumentsTableProps {
  /** Every document in the current scope, all statuses, unsorted. */
  documents: DocsDocument[]
  members: AssignableMember[]
  /** Collection / sub-collection / location column; null hides it. */
  pathColumn: DocumentsTablePathColumn | null
  /** Hide the status filter where the list is one status (Drafts). */
  showStatusFilter?: boolean
  /** Accessible names for the list and its search field. */
  listLabel?: string
  searchLabel?: string
  /** Saved-order view (collection pages only); null hides the toggle. */
  order: DocumentsTableOrder | null
  wsSlug: string
  wsId: string
  selectionScope: string

  // Filter / sort state is lifted to the caller so it survives
  // navigation between scopes. An empty status list shows every status.
  statuses: DocStatus[]
  onStatusesChange: (next: DocStatus[]) => void
  sortField: DocumentsTableSortField
  sortDir: DocumentsTableSortDir
  onSort: (field: DocumentsTableSortField, dir: DocumentsTableSortDir) => void

  /** Permission gate for all write actions, selection, and dragging. */
  canEdit: boolean
  /** Pending state for the duplicate row action. */
  duplicatingDocId: string | null

  onArchive: (doc: DocsDocument) => void
  onUnarchive: (doc: DocsDocument) => void
  onDelete: (doc: DocsDocument) => void
  onDuplicate: (doc: DocsDocument) => void
  onMove: (doc: DocsDocument) => void
  onPublish: (doc: DocsDocument) => void
}

interface TableRow {
  doc: DocsDocument
  /** Set in the saved-order view. */
  group: DocsOrderGroup | null
  /** 1-based position among the group's visible rows, in the saved-order view. */
  position: number | null
}

interface OrderLayoutEntry {
  id: string
  groupId: string
}

/**
 * DocumentsTable renders the docs document lists (space pages, My
 * documents, Recent docs, Drafts): search,
 * status filter, sortable column headers (Title, Collection, Owner,
 * Created by, Updated), bulk selection, row actions, and, inside a
 * collection, the saved-order view where rows can be dragged to
 * reorder or into another sub-collection.
 *
 * The caller scopes `documents` (via scopedDocuments()) and owns
 * persistence; this component filters, sorts, and lays out rows.
 */
export function DocumentsTable({
  documents,
  members,
  pathColumn,
  showStatusFilter = true,
  listLabel = 'Documents in this space',
  searchLabel = 'Search documents in this space',
  order,
  wsSlug,
  wsId,
  selectionScope,
  statuses,
  onStatusesChange,
  sortField,
  sortDir,
  onSort,
  canEdit,
  duplicatingDocId,
  onArchive,
  onUnarchive,
  onDelete,
  onDuplicate,
  onMove,
  onPublish,
}: DocumentsTableProps) {
  const navigate = useNavigate()
  const [searchQuery, setSearchQuery] = useState('')
  // Optimistic row layout while a drop saves; cleared once the caller
  // has refetched (or the save failed and the server order returns).
  const [pendingLayout, setPendingLayout] = useState<OrderLayoutEntry[] | null>(null)
  const [movedDocId, setMovedDocId] = useState<string | null>(null)
  const flashTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => () => { if (flashTimer.current) clearTimeout(flashTimer.current) }, [])

  const query = searchQuery.trim().toLowerCase()
  const ordering = order?.active ? order : null

  const memberById = useMemo(() => {
    const map = new Map<string, AssignableMember>()
    for (const member of members) {
      map.set(member.id, member)
      if (member.user_id) map.set(member.user_id, member)
    }
    return map
  }, [members])

  const rows = useMemo<TableRow[]>(() => {
    const visible = (doc: DocsDocument) =>
      (statuses.length === 0 || statuses.includes(doc.status)) &&
      (!query || (doc.title ?? '').toLowerCase().includes(query))

    if (ordering) {
      const docById = new Map(documents.map((doc) => [doc.id, doc]))
      const groupById = new Map(ordering.groups.map((group) => [group.collection.id, group]))
      const layout = pendingLayout ?? ordering.groups.flatMap((group) =>
        group.documents.map((doc) => ({ id: doc.id, groupId: group.collection.id })))
      const counters = new Map<string, number>()
      const out: TableRow[] = []
      for (const entry of layout) {
        const doc = docById.get(entry.id)
        const group = groupById.get(entry.groupId)
        if (!doc || !group || !visible(doc)) continue
        const position = (counters.get(entry.groupId) ?? 0) + 1
        counters.set(entry.groupId, position)
        out.push({ doc, group, position })
      }
      return out
    }

    // owner_id holds a workspace member id; created_by holds a user id.
    // Both resolve through memberById.
    const ownerName = (doc: DocsDocument) => {
      const member = doc.owner_id ? memberById.get(doc.owner_id) : undefined
      return member ? formatAssignableMemberName(member) : ''
    }
    const textKey = (doc: DocsDocument) =>
      sortField === 'path' ? (pathColumn?.pathFor(doc) ?? '')
        : sortField === 'owner' ? ownerName(doc)
          : (doc.title ?? '')
    return documents
      .filter(visible)
      .sort((a, b) => {
        if (sortField === 'updated_at') {
          const cmp = new Date(a.updated_at).getTime() - new Date(b.updated_at).getTime()
          return sortDir === 'asc' ? cmp : -cmp
        }
        const av = textKey(a)
        const bv = textKey(b)
        // Empty collection / owner values sort last in both directions.
        if (!av !== !bv) return av ? -1 : 1
        const cmp = av.localeCompare(bv)
        return sortDir === 'asc' ? cmp : -cmp
      })
      .map((doc) => ({ doc, group: null, position: null }))
  }, [documents, statuses, query, ordering, pendingLayout, pathColumn, sortField, sortDir, memberById])

  const visibleDocs = useMemo(() => rows.map((row) => row.doc), [rows])
  const selection = useDocsSelection(
    `${wsId}:${selectionScope}:${statuses.join(',')}:${query}:${ordering ? 'order' : 'sort'}`,
    visibleDocs,
  )

  const hiddenArchivedCount = statuses.length > 0 && !statuses.includes('archived')
    ? documents.filter((doc) => doc.status === 'archived').length
    : 0

  const saving = pendingLayout !== null
  const canDrag = Boolean(ordering) && canEdit && !query && !saving
  const dragHint = query ? 'Clear the search to reorder' : 'Drag to reorder'

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  const flashRow = (docId: string) => {
    if (flashTimer.current) clearTimeout(flashTimer.current)
    setMovedDocId(docId)
    flashTimer.current = setTimeout(() => setMovedDocId(null), ROW_MOVED_FLASH_MS)
  }

  const handleDragEnd = ({ active, over }: DragEndEvent) => {
    if (!ordering || !over || active.id === over.id) return
    const fromIdx = rows.findIndex((row) => row.doc.id === active.id)
    const toIdx = rows.findIndex((row) => row.doc.id === over.id)
    if (fromIdx < 0 || toIdx < 0) return
    const moving = rows[fromIdx]
    const from = moving.group!
    const to = rows[toIdx].group!
    // The dropped row joins the group of the row it was dropped on.
    const nextRows = arrayMove(rows, fromIdx, toIdx)
      .map((row) => (row === moving ? { ...row, group: to } : row))
    const visibleOrder = nextRows
      .filter((row) => row.group!.collection.id === to.collection.id)
      .map((row) => row.doc.id)
    const toOrder = placeVisibleOrder(
      to.documents.map((doc) => doc.id).filter((id) => id !== moving.doc.id),
      visibleOrder,
    )
    setPendingLayout(ordering.groups.flatMap((group) => {
      const ids = group.collection.id === to.collection.id
        ? toOrder
        : group.documents.map((doc) => doc.id).filter((id) => id !== moving.doc.id)
      return ids.map((id) => ({ id, groupId: group.collection.id }))
    }))
    flashRow(moving.doc.id)
    void ordering.onChange({
      doc: moving.doc,
      from,
      to,
      toOrder,
      fromOrderBefore: from.documents.map((doc) => doc.id),
    }).finally(() => setPendingLayout(null))
  }

  const sortHeader = (field: DocumentsTableSortField, label: string, className?: string) => {
    const isActive = !ordering && sortField === field
    return (
      <button
        type="button"
        onClick={() => {
          if (ordering) order?.onActiveChange(false)
          onSort(field, isActive ? (sortDir === 'asc' ? 'desc' : 'asc') : DEFAULT_SORT_DIR[field])
        }}
        aria-label={`Sort by ${label}`}
        className={cn(
          'group/sort flex shrink-0 items-center gap-1 text-left transition-colors hover:text-quiet-text-primary focus-visible:text-quiet-text-primary focus-visible:outline-none',
          isActive && 'text-quiet-text-primary',
          className,
        )}
      >
        <span className="truncate">{label}</span>
        {isActive ? (
          sortDir === 'asc'
            ? <ArrowUp02Icon className="h-3 w-3 shrink-0" aria-hidden="true" />
            : <ArrowDown02Icon className="h-3 w-3 shrink-0" aria-hidden="true" />
        ) : (
          <ArrowUpDownIcon className="h-3 w-3 shrink-0 opacity-0 transition-opacity group-hover/sort:opacity-60 group-focus-visible/sort:opacity-60" aria-hidden="true" />
        )}
      </button>
    )
  }

  return (
    <div className="@container space-y-2">
      <div className="flex flex-col items-stretch gap-2 sm:flex-row sm:items-center sm:justify-between">
        <QuietSearchInput
          containerClassName="min-w-0 flex-1 sm:max-w-64"
          placeholder="Search..."
          aria-label={searchLabel}
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
        />
        <div className="flex shrink-0 items-center justify-end gap-4">
          {order && (
            <QuickTooltip label={order.description}>
              <QuietTextAction
                type="button"
                aria-pressed={order.active}
                onClick={() => order.onActiveChange(!order.active)}
                className={cn('gap-1.5', order.active && 'font-medium text-quiet-text-primary')}
              >
                <LeftToRightListNumberIcon className="h-3.5 w-3.5" />
                {order.label}
              </QuietTextAction>
            </QuickTooltip>
          )}
          {showStatusFilter && <QuietFilterDropdown
            label="Status"
            showLabel="inline"
            bordered
            multiple
            searchable={false}
            emptyLabel="All statuses"
            options={STATUS_OPTIONS}
            selected={statuses}
            onChange={(next) => onStatusesChange(next as DocStatus[])}
          />}
        </div>
      </div>

      {/* Stays mounted while rows exist so its dialogs survive the selection clearing. */}
      {canEdit && rows.length > 0 && (
        <DocsBulkToolbar key={selection.scope} wsId={wsId} selection={selection} showSelectAll={false} />
      )}

      <div>
        <div className="flex min-h-9 items-stretch border-b border-border text-xs font-medium text-quiet-text-tertiary">
          {canEdit && (
            <div className="flex shrink-0 items-center pl-3 pr-1">
              <Checkbox
                aria-label="Select all visible documents"
                checked={selection.allChecked ? true : selection.ids.length ? 'indeterminate' : false}
                disabled={selection.busy || !selection.hasSelectable}
                onCheckedChange={selection.toggleAll}
              />
            </div>
          )}
          {ordering && <div className="w-12 shrink-0" aria-hidden="true" />}
          <div className="flex min-w-0 flex-1 items-center gap-4 px-3">
            {sortHeader('title', 'Title', 'min-w-0 flex-1')}
            {pathColumn && sortHeader('path', pathColumn.label, 'hidden w-48 @3xl:flex')}
            {sortHeader('owner', 'Owner', 'hidden w-40 @4xl:flex')}
            <span className="hidden w-40 shrink-0 items-center @5xl:flex">Created by</span>
            {sortHeader('updated_at', 'Updated', 'hidden w-24 justify-end @3xl:flex')}
          </div>
          {canEdit && <div className="w-9 shrink-0" aria-hidden="true" />}
        </div>

        {rows.length === 0 ? (
          <p className="px-3 py-8 text-sm text-quiet-text-tertiary">No documents match.</p>
        ) : (
          <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
            <SortableContext items={rows.map((row) => row.doc.id)} strategy={verticalListSortingStrategy}>
              <div
                role="list"
                aria-label={listLabel}
                className="[&>[data-slot=docs-table-row]:last-child]:border-b-0"
              >
                {rows.map((row) => (
                  <DocumentsTableRow
                    key={row.doc.id}
                    row={row}
                    showOrder={Boolean(ordering)}
                    reorderable={canEdit}
                    draggable={canDrag}
                    dragHint={dragHint}
                    moved={movedDocId === row.doc.id}
                    path={pathColumn ? (pathColumn.pathFor(row.doc) ?? null) : undefined}
                    pathEmptyLabel={pathColumn?.emptyLabel}
                    owner={row.doc.owner_id ? memberById.get(row.doc.owner_id) : undefined}
                    createdBy={memberById.get(row.doc.created_by)}
                    selection={canEdit ? <DocsSelectionCheckbox doc={row.doc} selection={selection} /> : undefined}
                    onOpen={() => navigate({
                      to: '/w/$slug/docs/documents/$docId',
                      params: { slug: wsSlug, docId: row.doc.id },
                    })}
                    actions={canEdit ? (
                      <DocumentRowActions
                        doc={row.doc}
                        busy={selection.busy}
                        duplicating={duplicatingDocId === row.doc.id}
                        onArchive={onArchive}
                        onUnarchive={onUnarchive}
                        onDelete={onDelete}
                        onDuplicate={onDuplicate}
                        onMove={onMove}
                        onPublish={onPublish}
                      />
                    ) : undefined}
                  />
                ))}
              </div>
            </SortableContext>
          </DndContext>
        )}

        {hiddenArchivedCount > 0 && (
          <div className="flex items-center gap-1.5 px-3 pt-3 text-xs text-quiet-text-tertiary">
            <ArchiveIcon className="h-3.5 w-3.5" aria-hidden="true" />
            <span>
              {hiddenArchivedCount} archived {hiddenArchivedCount === 1 ? 'document' : 'documents'} hidden
            </span>
            <span aria-hidden="true">·</span>
            <QuietTextAction type="button" className="text-xs text-quiet-text-primary underline underline-offset-4" onClick={() => onStatusesChange([])}>
              Show
            </QuietTextAction>
          </div>
        )}
      </div>
    </div>
  )
}

function DocumentsTableRow({
  row,
  showOrder,
  reorderable,
  draggable,
  dragHint,
  moved,
  path,
  pathEmptyLabel,
  owner,
  createdBy,
  selection,
  onOpen,
  actions,
}: {
  row: TableRow
  showOrder: boolean
  /** Whether the viewer may reorder at all (shows the drag handle). */
  reorderable: boolean
  /** Whether dragging is possible right now (no search, nothing saving). */
  draggable: boolean
  dragHint: string
  moved: boolean
  /** undefined hides the column; null renders `pathEmptyLabel`. */
  path: string | null | undefined
  pathEmptyLabel?: string
  owner?: AssignableMember
  createdBy?: AssignableMember
  selection?: ReactNode
  onOpen: () => void
  actions?: ReactNode
}) {
  const { doc } = row
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: doc.id,
    disabled: !draggable,
  })
  const style: CSSProperties = {
    transform: CSS.Translate.toString(transform),
    transition,
  }
  const updatedLabel = timeAgo(doc.updated_at)
  const title = doc.title || 'Untitled'
  const compactMeta = [path === undefined ? null : (path ?? pathEmptyLabel), `Updated ${updatedLabel}`]
    .filter(Boolean)
    .join(' · ')

  return (
    <div
      ref={setNodeRef}
      style={style}
      role="listitem"
      data-slot="docs-table-row"
      className={cn(
        'group/doc-row flex min-h-11 items-stretch border-b border-border/45 transition-colors hover:bg-muted/35 focus-within:bg-muted/35',
        // Opaque only while lifted, so the row covers the rows it passes over.
        isDragging && 'relative z-10 bg-background shadow-md',
        moved && 'docs-row-moved',
      )}
    >
      {selection && <div className="flex shrink-0 items-center pl-3 pr-1">{selection}</div>}
      {showOrder && (
        <div className="flex w-12 shrink-0 items-center justify-end gap-1 pr-1">
          {reorderable && <button
            type="button"
            {...attributes}
            {...listeners}
            disabled={!draggable}
            aria-label={`Reorder ${title}`}
            title={dragHint}
            className="flex h-7 w-5 shrink-0 cursor-grab touch-none items-center justify-center rounded-[6px] text-quiet-muted transition-colors hover:bg-quiet-hover hover:text-quiet-text-primary focus-visible:bg-quiet-hover focus-visible:text-quiet-text-primary focus-visible:outline-none active:cursor-grabbing disabled:cursor-default disabled:opacity-40 disabled:hover:bg-transparent"
          >
            <DragDropVerticalIcon className="h-4 w-4" />
          </button>}
          <span className="w-5 text-right text-xs tabular-nums text-quiet-muted">{row.position}</span>
        </div>
      )}
      <button
        type="button"
        onClick={onOpen}
        className="flex min-w-0 flex-1 items-center gap-4 px-3 py-2 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
      >
        <span className="min-w-0 flex-1">
          <span className="flex min-w-0 items-center gap-2">
            <span className="truncate text-sm font-medium text-foreground">{title}</span>
            <DocsExceptionalStatus status={doc.status} />
            <PendingProposalBadge count={doc.pending_change_proposal_count ?? 0} />
          </span>
          <span className="mt-0.5 block truncate text-[11px] text-muted-foreground/75 @3xl:hidden">{compactMeta}</span>
        </span>
        {path !== undefined && (
          <span className={cn('hidden w-48 shrink-0 truncate text-xs @3xl:block', path ? 'text-quiet-text-secondary' : 'text-quiet-muted')}>
            {path ?? pathEmptyLabel}
          </span>
        )}
        <PersonCell member={owner} className="hidden w-40 @4xl:flex" />
        <PersonCell member={createdBy} className="hidden w-40 @5xl:flex" />
        <span className="hidden w-24 shrink-0 text-right text-xs text-muted-foreground @3xl:block" title={new Date(doc.updated_at).toLocaleString()}>
          {updatedLabel}
        </span>
      </button>
      {actions && (
        <div className="flex w-9 shrink-0 items-center justify-center opacity-100 transition-opacity md:opacity-0 md:group-hover/doc-row:opacity-100 md:group-focus-within/doc-row:opacity-100">
          {actions}
        </div>
      )}
    </div>
  )
}

function PersonCell({ member, className }: { member?: AssignableMember; className?: string }) {
  if (!member) {
    return <span className={cn('shrink-0 items-center text-xs text-quiet-muted', className)}>—</span>
  }
  const name = formatAssignableMemberName(member)
  return (
    <span className={cn('min-w-0 shrink-0 items-center gap-2 text-xs text-quiet-text-secondary', className)}>
      <UserAvatar
        name={member.display_name || member.email}
        avatarUrl={member.avatar_url}
        avatarStyle={member.avatar_style}
        avatarSeed={member.avatar_seed}
        avatarBackgroundMode={member.avatar_background_mode}
        avatarBackgroundColor={member.avatar_background_color}
        fallbackColorSeed={member.email}
        className="h-4 w-4 shrink-0"
        fallbackClassName="text-[7px]"
      />
      <span className="truncate">{name}</span>
    </span>
  )
}

function DocumentRowActions({
  doc,
  busy,
  duplicating,
  onArchive,
  onUnarchive,
  onDelete,
  onDuplicate,
  onMove,
  onPublish,
}: {
  doc: DocsDocument
  busy: boolean
  duplicating: boolean
} & Pick<DocumentsTableProps, 'onArchive' | 'onUnarchive' | 'onDelete' | 'onDuplicate' | 'onMove' | 'onPublish'>) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className="rounded p-1.5 text-foreground/50 transition-colors hover:bg-muted hover:text-foreground"
          disabled={busy}
          aria-label={`More options for ${doc.title}`}
        >
          <MoreHorizontalIcon className="h-4 w-4" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-40">
        <DropdownMenuItem disabled={duplicating} onClick={() => onDuplicate(doc)}>
          <Copy01Icon className="h-3.5 w-3.5" />
          {duplicating ? 'Duplicating...' : 'Duplicate'}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => onMove(doc)}>
          <FolderInputIcon className="h-3.5 w-3.5" />
          Move to...
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        {doc.status === 'draft' && (
          <DropdownMenuItem onClick={() => onPublish(doc)}>
            <SentIcon className="h-3.5 w-3.5" />
            Publish
          </DropdownMenuItem>
        )}
        {doc.status === 'archived' ? (
          <DropdownMenuItem onClick={() => onUnarchive(doc)}>
            <ArchiveRestoreIcon className="h-3.5 w-3.5" />
            Unarchive
          </DropdownMenuItem>
        ) : (
          <DropdownMenuItem onClick={() => onArchive(doc)}>
            <ArchiveIcon className="h-3.5 w-3.5" />
            Archive
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem
          className="text-destructive focus:text-destructive"
          onClick={() => onDelete(doc)}
        >
          <Delete01Icon className="h-3.5 w-3.5" />
          Delete
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
