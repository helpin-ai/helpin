import { useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { timeAgo } from '@/lib/utils'
import {
  ArchiveIcon,
  ArrowDown02Icon,
  ArrowUp02Icon,
  Tick01Icon,
  Clock01Icon,
  Copy01Icon,
  File01Icon,
  FilterHorizontalIcon,
  ArrowDown01Icon,
  MoreHorizontalIcon,
  PencilEdit02Icon,
  SentIcon,
  Delete01Icon,
  UserIcon,
  ArchiveRestoreIcon,
  FolderInputIcon,
} from '@/lib/icons'
import { toast } from 'sonner'
import { useTitle } from '@/hooks/useTitle'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import {
  useDocsDocuments,
  useArchiveDocsDocument,
  useUnarchiveDocsDocument,
  useDeleteDocsDocument,
  useDuplicateDocsDocument,
  usePublishDocsDocument,
  useAssignableMembers,
  useWorkspaceAccess,
  usePermissions,
} from '@/hooks/queries'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { UserAvatar } from '@/components/pm/UserAvatar'
import { formatAssignableMemberName } from '@/lib/assignableMembers'
import { MoveDocumentDialog } from '@/components/docs/MoveDocumentDialog'
import { ConfirmDialog } from '@/components/pm/ConfirmDialog'
import type { DocsDocument, DocStatus } from '@/lib/docsTypes'
import { DOC_STATUS_LABELS } from '@/lib/docsTypes'

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

interface DocsDocumentListProps {
  title: string
  description: string
  filterMode: 'my' | 'drafts' | 'recent'
}

export function DocsDocumentList({ title, description, filterMode }: DocsDocumentListProps) {
  useTitle(title)
  const navigate = useNavigate()
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id ?? ''
  const wsSlug = workspace?.slug ?? ''

  const { data: access, isLoading: isAccessLoading } = useWorkspaceAccess(wsId)
  const { canEditDocs } = usePermissions(access)

  const [filterStatus, setFilterStatus] = useState<DocStatus | null>(null)
  const [sortField, setSortField] = useState<'updated_at' | 'title' | 'status'>('updated_at')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc')
  const [duplicatingDocId, setDuplicatingDocId] = useState<string | null>(null)
  const [movingDoc, setMovingDoc] = useState<DocsDocument | null>(null)
  const [deleteConfirmDoc, setDeleteConfirmDoc] = useState<DocsDocument | null>(null)

  const filters = useMemo(() => {
    if (filterMode === 'drafts') return { status: 'draft' }
    if (filterMode === 'my' && access?.membership.id) {
      return { owner_id: access.membership.id, include_archived: 'true' }
    }
    return {}
  }, [filterMode, access?.membership.id])

  const { data: rawDocuments, isLoading } = useDocsDocuments(wsId, filters, {
    enabled: filterMode !== 'my' || !!access?.membership.id,
  })
  const { data: members = [] } = useAssignableMembers(wsId)
  const archiveDoc = useArchiveDocsDocument(wsId)
  const unarchiveDoc = useUnarchiveDocsDocument(wsId)
  const deleteDoc = useDeleteDocsDocument(wsId)
  const duplicateDoc = useDuplicateDocsDocument(wsId)
  const publishDoc = usePublishDocsDocument(wsId)

  // For recent, sort by updated_at descending and limit
  const documents = useMemo(() => {
    if (!rawDocuments) return rawDocuments
    let docs = rawDocuments
    if (filterMode === 'recent') {
      docs = [...docs].sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()).slice(0, 20)
    }
    if (filterStatus) {
      docs = docs.filter((d) => d.status === filterStatus)
    }
    return docs
  }, [rawDocuments, filterMode, filterStatus])

  const displayDocs = useMemo(() => {
    if (!documents) return []
    return [...documents].sort((a, b) => {
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
  }, [documents, sortField, sortDir])

  const handleDuplicateDoc = async (doc: DocsDocument) => {
    setDuplicatingDocId(doc.id)
    try {
      await duplicateDoc.mutateAsync(doc)
      toast.success('Document duplicated')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to duplicate')
    } finally {
      setDuplicatingDocId((current) => (current === doc.id ? null : current))
    }
  }

  const handleDeleteDoc = async () => {
    if (!deleteConfirmDoc) return
    try {
      await deleteDoc.mutateAsync(deleteConfirmDoc.id)
      toast.success('Document deleted')
      setDeleteConfirmDoc(null)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete')
    }
  }

  const emptyIcon = filterMode === 'recent' ? Clock01Icon : filterMode === 'my' ? UserIcon : PencilEdit02Icon
  const EmptyIcon = emptyIcon

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>
  }

  return (
    <div className="mx-auto max-w-7xl space-y-4">
      <header>
        <h2 className="text-xl font-semibold">{title}</h2>
        <p className="text-sm text-muted-foreground">{description}</p>
      </header>

      {isLoading || (filterMode === 'my' && isAccessLoading) ? (
        <div className="space-y-2 py-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-10 animate-pulse rounded-md bg-muted/60" />
          ))}
        </div>
      ) : !documents || documents.length === 0 ? (
        filterStatus && rawDocuments && rawDocuments.length > 0 ? (
          <div className="flex flex-col items-center justify-center py-16 px-4">
            <div className="flex h-14 w-14 items-center justify-center rounded-full bg-muted/40 mb-5">
              <FilterHorizontalIcon className="h-7 w-7 text-muted-foreground/60" />
            </div>
            <h3 className="text-lg font-semibold mb-1.5">
              No {DOC_STATUS_LABELS[filterStatus].toLowerCase()} documents
            </h3>
            <p className="text-sm text-muted-foreground text-center max-w-md mb-4">
              {filterStatus === 'draft'
                ? 'There are no draft documents matching your current view.'
                : filterStatus === 'published'
                  ? 'There are no published documents matching your current view.'
                  : 'There are no archived documents matching your current view.'}
            </p>
            <button
              type="button"
              onClick={() => setFilterStatus(null)}
              className="text-sm text-primary hover:underline"
            >
              Clear filter and show all documents
            </button>
          </div>
        ) : (
          <div className="flex flex-col items-center justify-center py-16 px-4">
            <div className="flex h-14 w-14 items-center justify-center rounded-full bg-muted/40 mb-5">
              <EmptyIcon className="h-7 w-7 text-muted-foreground/60" />
            </div>
            <h3 className="text-lg font-semibold mb-1.5">
              {filterMode === 'recent' ? 'No recent documents' : filterMode === 'my' ? 'No documents yet' : 'No drafts'}
            </h3>
            <p className="text-sm text-muted-foreground text-center max-w-md">
              {filterMode === 'recent'
                ? 'Recently updated documents will appear here.'
                : filterMode === 'my'
                  ? 'Documents you create or own will appear here.'
                  : 'Draft documents will appear here until they are published.'}
            </p>
          </div>
        )
      ) : (
        <>
          {/* Filters row */}
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
                  onClick={() => setFilterStatus(null)}
                  className={!filterStatus ? 'font-medium' : ''}
                >
                  All
                  {!filterStatus && <Tick01Icon className="ml-auto h-3.5 w-3.5" />}
                </DropdownMenuItem>
                {(Object.entries(DOC_STATUS_LABELS) as [DocStatus, string][]).map(([key, label]) => (
                  <DropdownMenuItem
                    key={key}
                    onClick={() => setFilterStatus(key)}
                    className={filterStatus === key ? 'font-medium' : ''}
                  >
                    {label}
                    {filterStatus === key && <Tick01Icon className="ml-auto h-3.5 w-3.5" />}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>

          <div className="rounded-lg border border-border/60 bg-card divide-y divide-border/40">
            {/* Table header */}
            <div className="flex items-center gap-3 px-4 py-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
              <button type="button" onClick={() => { if (sortField === 'title') { setSortDir(d => d === 'asc' ? 'desc' : 'asc') } else { setSortField('title'); setSortDir('asc') } }} className="min-w-0 flex-1 flex items-center gap-1 hover:text-foreground transition-colors text-left">
                Title
                {sortField === 'title' && (sortDir === 'asc' ? <ArrowUp02Icon className="h-3 w-3" /> : <ArrowDown02Icon className="h-3 w-3" />)}
              </button>
              <span className="w-36 shrink-0">Owner</span>
              <button type="button" onClick={() => { if (sortField === 'status') { setSortDir(d => d === 'asc' ? 'desc' : 'asc') } else { setSortField('status'); setSortDir('asc') } }} className="w-20 shrink-0 flex items-center gap-1 hover:text-foreground transition-colors">
                Status
                {sortField === 'status' && (sortDir === 'asc' ? <ArrowUp02Icon className="h-3 w-3" /> : <ArrowDown02Icon className="h-3 w-3" />)}
              </button>
              <button type="button" onClick={() => { if (sortField === 'updated_at') { setSortDir(d => d === 'asc' ? 'desc' : 'asc') } else { setSortField('updated_at'); setSortDir('desc') } }} className="w-20 shrink-0 flex items-center gap-1 justify-end hover:text-foreground transition-colors">
                Updated
                {sortField === 'updated_at' && (sortDir === 'asc' ? <ArrowUp02Icon className="h-3 w-3" /> : <ArrowDown02Icon className="h-3 w-3" />)}
              </button>
              {canEditDocs && <span className="w-8 shrink-0" />}
            </div>
            {displayDocs.map((doc: DocsDocument) => {
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
                  <span className={`w-20 shrink-0 text-xs font-medium ${statusColor(doc.status)}`}>
                    {DOC_STATUS_LABELS[doc.status] ?? doc.status}
                  </span>
                  <span className="w-20 shrink-0 text-right text-xs text-muted-foreground">
                    {timeAgo(doc.updated_at)}
                  </span>
                  {canEditDocs && (
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
                            onClick={() => void handleDuplicateDoc(doc)}
                          >
                            <Copy01Icon className="h-3.5 w-3.5" />
                            {duplicatingDocId === doc.id ? 'Duplicating...' : 'Duplicate'}
                          </DropdownMenuItem>
                          <DropdownMenuItem onClick={() => setMovingDoc(doc)}>
                            <FolderInputIcon className="h-3.5 w-3.5" />
                            Move to...
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          {doc.status === 'draft' && (
                            <DropdownMenuItem
                              onClick={() => {
                                publishDoc.mutate({ id: doc.id }, {
                                  onSuccess: () => toast.success('Document published'),
                                  onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to publish'),
                                })
                              }}
                            >
                              <SentIcon className="h-3.5 w-3.5" />
                              Publish
                            </DropdownMenuItem>
                          )}
                          {doc.status === 'archived' ? (
                            <DropdownMenuItem
                              onClick={() => {
                                unarchiveDoc.mutate(doc.id, {
                                  onSuccess: () => toast.success('Document unarchived'),
                                  onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to unarchive'),
                                })
                              }}
                            >
                              <ArchiveRestoreIcon className="h-3.5 w-3.5" />
                              Unarchive
                            </DropdownMenuItem>
                          ) : (
                            <DropdownMenuItem
                              onClick={() => {
                                archiveDoc.mutate(doc.id, {
                                  onSuccess: () => toast.success('Document archived'),
                                  onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to archive'),
                                })
                              }}
                            >
                              <ArchiveIcon className="h-3.5 w-3.5" />
                              Archive
                            </DropdownMenuItem>
                          )}
                          <DropdownMenuSeparator />
                          <DropdownMenuItem
                            className="text-destructive focus:text-destructive"
                            onClick={() => setDeleteConfirmDoc(doc)}
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
        </>
      )}

      {movingDoc && (
        <MoveDocumentDialog
          wsId={wsId}
          open={!!movingDoc}
          onOpenChange={(open) => { if (!open) setMovingDoc(null) }}
          docId={movingDoc.id}
          docTitle={movingDoc.title}
          currentSpaceId={movingDoc.space_id}
          currentCollectionId={movingDoc.collection_id}
        />
      )}

      <ConfirmDialog
        open={deleteConfirmDoc !== null}
        onOpenChange={(open) => { if (!open) setDeleteConfirmDoc(null) }}
        title="Delete document"
        description="This will permanently delete the document and its saved content. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDeleteDoc}
      />
    </div>
  )
}
