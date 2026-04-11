import { useNavigate } from '@tanstack/react-router'
import {
  ArchiveIcon,
  ArchiveRestoreIcon,
  ArrowDown01Icon,
  ArrowDown02Icon,
  ArrowUp02Icon,
  Copy01Icon,
  Delete01Icon,
  File01Icon,
  FilterHorizontalIcon,
  FolderInputIcon,
  FolderOpenIcon,
  MoreHorizontalIcon,
  PlusSignIcon,
  SentIcon,
  Tick01Icon,
} from '@/lib/icons'
import { timeAgo } from '@/lib/utils'
import { DOC_STATUS_LABELS } from '@/lib/docsTypes'
import type { DocsDocument, DocStatus } from '@/lib/docsTypes'
import type { AssignableMember } from '@/lib/types'
import { UserAvatar } from '@/components/pm/UserAvatar'
import { formatAssignableMemberName } from '@/lib/assignableMembers'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

/**
 * Tailwind color class for a document status badge. Matches the
 * colors the inline table used before extraction.
 */
function statusColor(status: string): string {
  switch (status) {
    case 'published':
      return 'text-emerald-600 dark:text-emerald-400'
    case 'archived':
      return 'text-muted-foreground/60'
    default:
      return 'text-amber-600 dark:text-amber-400'
  }
}

export type DocumentsTableSortField = 'updated_at' | 'title' | 'status'
export type DocumentsTableSortDir = 'asc' | 'desc'

export interface DocumentsTableProps {
  /** Already-scoped documents (pre-sort, pre-filter). */
  documents: DocsDocument[]
  /** Assignable members for owner rendering. */
  members: AssignableMember[]
  /** collection id → display name, used for the Collection column. */
  collectionNames: Map<string, string>
  /**
   * Whether any collection exists in the current space. Controls
   * which empty-state variant renders when there are no documents.
   */
  hasCollections: boolean
  /** Workspace slug — needed for doc-row navigation. */
  wsSlug: string

  // Filter / sort state is lifted to the caller so it survives
  // navigation between scopes.
  filterStatus: DocStatus | null
  onFilterStatus: (next: DocStatus | null) => void
  sortField: DocumentsTableSortField
  sortDir: DocumentsTableSortDir
  onSort: (field: DocumentsTableSortField, dir: DocumentsTableSortDir) => void

  /** Permission gate for all write actions and the row overflow menu. */
  canEdit: boolean
  /** Pending state for the duplicate row action. */
  duplicatingDocId: string | null

  // Row action delegates — thin callbacks back to the controller's
  // mutations / dialog setters. Keeps this component decoupled from
  // the workspace hooks and global create store.
  onArchive: (doc: DocsDocument) => void
  onUnarchive: (doc: DocsDocument) => void
  onDelete: (doc: DocsDocument) => void
  onDuplicate: (doc: DocsDocument) => void
  onMove: (doc: DocsDocument) => void
  onPublish: (doc: DocsDocument) => void

  // Empty-state delegates. When no documents are present, the table
  // shows a CTA matching what the caller wants to create.
  onCreateDocument: () => void
  onCreateCollection: () => void
}

/**
 * DocumentsTable renders the document list the space detail page
 * shows under its header. Extracted verbatim from DocsSpaceDetail's
 * inline table — owns filter row, sort headers, row actions, and
 * the no-documents empty state.
 *
 * This component carries no data-fetching responsibilities: the
 * caller scopes the `documents` array first (via scopedDocuments()
 * or similar), then hands it in.
 */
export function DocumentsTable({
  documents,
  members,
  collectionNames,
  hasCollections,
  wsSlug,
  filterStatus,
  onFilterStatus,
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
  onCreateDocument,
  onCreateCollection,
}: DocumentsTableProps) {
  const navigate = useNavigate()

  const showStatusFilter = hasCollections && (documents.length > 0 || Boolean(filterStatus))

  // Sort the scoped documents. Kept local to match the existing
  // inline implementation — the caller does not need to pre-sort.
  const displayDocs = [...documents].sort((a, b) => {
    let cmp = 0
    switch (sortField) {
      case 'title':
        cmp = (a.title ?? '').localeCompare(b.title ?? '')
        break
      case 'status':
        cmp = (a.status ?? '').localeCompare(b.status ?? '')
        break
      case 'updated_at':
      default:
        cmp = new Date(a.updated_at).getTime() - new Date(b.updated_at).getTime()
        break
    }
    return sortDir === 'asc' ? cmp : -cmp
  })

  const cycleSort = (field: DocumentsTableSortField) => {
    if (sortField === field) {
      onSort(field, sortDir === 'asc' ? 'desc' : 'asc')
    } else {
      onSort(field, field === 'updated_at' ? 'desc' : 'asc')
    }
  }

  return (
    <>
      {showStatusFilter && (
        <div className="flex items-center justify-end gap-2">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className={`inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1 text-xs font-medium transition-colors ${
                  filterStatus
                    ? 'border-primary/30 bg-primary/5 text-foreground'
                    : 'border-border/60 text-muted-foreground hover:bg-muted/40 hover:text-foreground'
                }`}
              >
                <FilterHorizontalIcon className="h-3 w-3" />
                {filterStatus ? DOC_STATUS_LABELS[filterStatus] : 'Status'}
                <ArrowDown01Icon className="h-3 w-3 opacity-50" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-40">
              <DropdownMenuItem
                onClick={() => onFilterStatus(null)}
                className={!filterStatus ? 'font-medium' : ''}
              >
                All
                {!filterStatus && <Tick01Icon className="ml-auto h-3.5 w-3.5" />}
              </DropdownMenuItem>
              {(Object.entries(DOC_STATUS_LABELS) as [DocStatus, string][]).map(([key, label]) => (
                <DropdownMenuItem
                  key={key}
                  onClick={() => onFilterStatus(key)}
                  className={filterStatus === key ? 'font-medium' : ''}
                >
                  {label}
                  {filterStatus === key && <Tick01Icon className="ml-auto h-3.5 w-3.5" />}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      )}

      {displayDocs.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 px-4 max-w-md mx-auto">
          {hasCollections ? (
            <>
              <File01Icon className="h-10 w-10 text-muted-foreground/30 mb-3" />
              <p className="text-sm text-muted-foreground">No documents found.</p>
              {canEdit && (
                <button
                  type="button"
                  onClick={onCreateDocument}
                  className="mt-4 inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-background px-3 py-1.5 text-sm font-medium text-foreground shadow-sm transition-colors hover:bg-muted"
                >
                  <PlusSignIcon className="h-3.5 w-3.5" />
                  Create a document
                </button>
              )}
            </>
          ) : (
            <>
              <FolderOpenIcon className="h-10 w-10 text-muted-foreground/30 mb-3" />
              <p className="text-sm font-medium">No collections yet</p>
              <p className="mt-1.5 text-center text-xs text-muted-foreground leading-relaxed">
                Collections help you organize documents into groups — like topics, categories, or projects. Create your first collection to start adding documents.
              </p>
              {canEdit && (
                <button
                  type="button"
                  onClick={onCreateCollection}
                  className="mt-4 inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-background px-3 py-1.5 text-sm font-medium text-foreground shadow-sm transition-colors hover:bg-muted"
                >
                  <PlusSignIcon className="h-3.5 w-3.5" />
                  Create a collection
                </button>
              )}
            </>
          )}
        </div>
      ) : (
        <div className="rounded-lg border border-border/60 bg-card divide-y divide-border/40">
          <div className="flex items-center gap-3 px-4 py-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
            <button
              type="button"
              onClick={() => cycleSort('title')}
              className="min-w-0 flex-1 flex items-center gap-1 hover:text-foreground transition-colors text-left"
            >
              Title
              {sortField === 'title' && (sortDir === 'asc' ? <ArrowUp02Icon className="h-3 w-3" /> : <ArrowDown02Icon className="h-3 w-3" />)}
            </button>
            <span className="w-36 shrink-0">Owner</span>
            <span className="w-28 shrink-0">Collection</span>
            <button
              type="button"
              onClick={() => cycleSort('status')}
              className="w-20 shrink-0 flex items-center gap-1 hover:text-foreground transition-colors"
            >
              Status
              {sortField === 'status' && (sortDir === 'asc' ? <ArrowUp02Icon className="h-3 w-3" /> : <ArrowDown02Icon className="h-3 w-3" />)}
            </button>
            <button
              type="button"
              onClick={() => cycleSort('updated_at')}
              className="w-20 shrink-0 flex items-center gap-1 justify-end hover:text-foreground transition-colors"
            >
              Updated
              {sortField === 'updated_at' && (sortDir === 'asc' ? <ArrowUp02Icon className="h-3 w-3" /> : <ArrowDown02Icon className="h-3 w-3" />)}
            </button>
            {canEdit && <span className="w-8 shrink-0" />}
          </div>
          {displayDocs.map((doc) => {
            const owner = members.find((m) => m.id === doc.owner_id)
            return (
              <div
                key={doc.id}
                className="flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm transition-colors hover:bg-muted/40 group/row"
              >
                <button
                  type="button"
                  onClick={() =>
                    navigate({
                      to: '/w/$slug/docs/documents/$docId',
                      params: { slug: wsSlug, docId: doc.id },
                    })
                  }
                  className="flex min-w-0 flex-1 items-center gap-2 text-left justify-start"
                >
                  <File01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
                  <span className="min-w-0 truncate font-medium">{doc.title}</span>
                </button>
                <span className="w-36 shrink-0 truncate text-xs text-muted-foreground">
                  {owner ? (
                    <span className="flex items-center gap-1">
                      <UserAvatar
                        name={owner.display_name || owner.email}
                        avatarUrl={owner.avatar_url}
                        className="h-4 w-4"
                        fallbackClassName="text-[7px]"
                      />
                      <span className="truncate">{formatAssignableMemberName(owner)}</span>
                    </span>
                  ) : '—'}
                </span>
                <span className="w-28 shrink-0 truncate text-xs text-muted-foreground">
                  {doc.collection_id ? collectionNames.get(doc.collection_id) ?? '—' : '—'}
                </span>
                <span className={`w-20 shrink-0 text-xs font-medium ${statusColor(doc.status)}`}>
                  {DOC_STATUS_LABELS[doc.status] ?? doc.status}
                </span>
                <span className="w-20 shrink-0 text-right text-xs text-muted-foreground">
                  {timeAgo(doc.updated_at)}
                </span>
                {canEdit && (
                  <span className="w-8 shrink-0">
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <button
                          type="button"
                          className="rounded p-1 text-foreground/50 opacity-0 transition-opacity hover:text-foreground group-hover/row:opacity-100"
                          onClick={(e) => e.stopPropagation()}
                        >
                          <MoreHorizontalIcon className="h-4 w-4" />
                        </button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end" className="w-40">
                        <DropdownMenuItem
                          disabled={duplicatingDocId === doc.id}
                          onClick={() => onDuplicate(doc)}
                        >
                          <Copy01Icon className="h-3.5 w-3.5" />
                          {duplicatingDocId === doc.id ? 'Duplicating...' : 'Duplicate'}
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
                  </span>
                )}
              </div>
            )
          })}
        </div>
      )}
    </>
  )
}
