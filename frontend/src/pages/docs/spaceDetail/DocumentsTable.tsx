import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import {
  ArchiveIcon,
  ArchiveRestoreIcon,
  ArrowDown01Icon,
  Copy01Icon,
  Delete01Icon,
  File01Icon,
  FilterHorizontalIcon,
  FolderInputIcon,
  FolderOpenIcon,
  MoreHorizontalIcon,
  PlusSignIcon,
  Search01Icon,
  SentIcon,
  Tick01Icon,
} from '@/lib/icons'
import { DOC_STATUS_LABELS } from '@/lib/docsTypes'
import type { DocsDocument, DocStatus } from '@/lib/docsTypes'
import { PendingProposalBadge } from '@/components/docs/proposals/PendingProposalBadge'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import {
  DocsLibraryList,
  DocsLibraryRow,
  DocsLibrarySortMenu,
  type DocsLibrarySortField,
} from '@/components/docs/DocsLibraryList'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

export type DocumentsTableSortField = DocsLibrarySortField
export type DocumentsTableSortDir = 'asc' | 'desc'

export interface DocumentsTableProps {
  /** Already-scoped documents (pre-sort, pre-filter). */
  documents: DocsDocument[]
  /** collection id → breadcrumb label, used only at the space root. */
  collectionPaths: Map<string, string>
  showCollectionPath: boolean
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
  collectionPaths,
  showCollectionPath,
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
  const [searchQuery, setSearchQuery] = useState('')

  const showStatusFilter = hasCollections && (documents.length > 0 || Boolean(filterStatus))
  const showToolbar = documents.length > 0 || Boolean(filterStatus)

  // Filter by search query, then sort.
  const filtered = searchQuery.trim()
    ? documents.filter((d) => d.title?.toLowerCase().includes(searchQuery.trim().toLowerCase()))
    : documents

  // Sort the scoped documents. Kept local to match the existing
  // inline implementation — the caller does not need to pre-sort.
  const displayDocs = [...filtered].sort((a, b) => {
    let cmp = 0
    switch (sortField) {
      case 'title':
        cmp = (a.title ?? '').localeCompare(b.title ?? '')
        break
      case 'position':
        cmp = (a.position ?? 0) - (b.position ?? 0)
        break
      case 'updated_at':
      default:
        cmp = new Date(a.updated_at).getTime() - new Date(b.updated_at).getTime()
        break
    }
    return sortDir === 'asc' ? cmp : -cmp
  })

  return (
    <>
      {showToolbar && (
        <div className="flex flex-col items-stretch gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 flex-1 items-center gap-2">
            <div className="relative min-w-0 flex-1 sm:max-w-64">
              <Search01Icon className="absolute left-2 top-1/2 h-3 w-3 -translate-y-1/2 text-muted-foreground/60" />
              <Input
                type="text"
                placeholder="Search..."
                aria-label="Search documents in this space"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="h-8 pl-7 text-xs"
              />
            </div>
            <span className="text-xs text-muted-foreground shrink-0">
              {displayDocs.length === 1 ? '1 document' : `${displayDocs.length} documents`}
            </span>
          </div>
          <div className="flex shrink-0 items-center justify-end gap-1">
          {showStatusFilter && <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="text-xs text-muted-foreground"
              >
                <FilterHorizontalIcon className="h-3 w-3" />
                {filterStatus ? DOC_STATUS_LABELS[filterStatus] : 'Status'}
                <ArrowDown01Icon className="h-3 w-3 opacity-50" />
              </Button>
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
          </DropdownMenu>}
          <DocsLibrarySortMenu
            value={{ field: sortField, direction: sortDir }}
            includeManualOrder
            onChange={(next) => onSort(next.field, next.direction)}
          />
          </div>
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
        <DocsLibraryList ariaLabel="Documents in this space">
          {displayDocs.map((doc) => (
            <DocsLibraryRow
              key={doc.id}
              title={doc.title}
              status={doc.status}
              updatedAt={doc.updated_at}
              location={showCollectionPath && doc.collection_id
                ? collectionPaths.get(doc.collection_id)
                : undefined}
              proposalBadge={<PendingProposalBadge count={doc.pending_change_proposal_count ?? 0} />}
              onOpen={() => navigate({
                to: '/w/$slug/docs/documents/$docId',
                params: { slug: wsSlug, docId: doc.id },
              })}
              actions={canEdit ? (
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <button
                      type="button"
                      className="rounded p-1.5 text-foreground/50 transition-colors hover:bg-muted hover:text-foreground"
                      aria-label={`More options for ${doc.title}`}
                    >
                      <MoreHorizontalIcon className="h-4 w-4" />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-40">
                    <DropdownMenuItem disabled={duplicatingDocId === doc.id} onClick={() => onDuplicate(doc)}>
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
              ) : undefined}
            />
          ))}
        </DocsLibraryList>
      )}
    </>
  )
}
