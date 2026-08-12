import { useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import {
  ArchiveIcon,
  Tick01Icon,
  Clock01Icon,
  Copy01Icon,
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
  useDocsSpaces,
  useAllDocsCollections,
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
import { MoveDocumentDialog } from '@/components/docs/MoveDocumentDialog'
import { ConfirmDialog } from '@/components/pm/ConfirmDialog'
import { Button } from '@/components/ui/button'
import type { DocsDocument, DocStatus } from '@/lib/docsTypes'
import { DOC_STATUS_LABELS } from '@/lib/docsTypes'
import {
  DocsLibraryList,
  DocsLibraryRow,
  DocsLibrarySortMenu,
  type DocsLibrarySortField,
} from '@/components/docs/DocsLibraryList'
import { buildCollectionPathLabels } from '@/components/docs/docsCollectionTree'

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
  const membershipId = access?.membership.id

  const [filterStatus, setFilterStatus] = useState<DocStatus | null>(null)
  const [sortField, setSortField] = useState<DocsLibrarySortField>('updated_at')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc')
  const [duplicatingDocId, setDuplicatingDocId] = useState<string | null>(null)
  const [movingDoc, setMovingDoc] = useState<DocsDocument | null>(null)
  const [deleteConfirmDoc, setDeleteConfirmDoc] = useState<DocsDocument | null>(null)

  const filters = useMemo(() => {
    if (filterMode === 'drafts') return { status: 'draft' }
    if (filterMode === 'my' && membershipId) {
      return { owner_id: membershipId, include_archived: 'true' }
    }
    return {}
  }, [filterMode, membershipId])

  const { data: rawDocuments, isLoading } = useDocsDocuments(wsId, filters, {
    enabled: filterMode !== 'my' || !!membershipId,
  })
  const { data: spaces = [] } = useDocsSpaces(wsId)
  const { data: collections = [] } = useAllDocsCollections(wsId)
  const archiveDoc = useArchiveDocsDocument(wsId)
  const unarchiveDoc = useUnarchiveDocsDocument(wsId)
  const deleteDoc = useDeleteDocsDocument(wsId)
  const duplicateDoc = useDuplicateDocsDocument(wsId)
  const publishDoc = usePublishDocsDocument(wsId)

  const spaceNames = useMemo(
    () => new Map(spaces.map((space) => [space.id, space.name])),
    [spaces],
  )
  const collectionPaths = useMemo(
    () => buildCollectionPathLabels(collections),
    [collections],
  )

  const locationFor = (doc: DocsDocument) => {
    const spaceName = spaceNames.get(doc.space_id)
    const collectionPath = doc.collection_id ? collectionPaths.get(doc.collection_id) : undefined
    return [spaceName, collectionPath].filter(Boolean).join(' › ')
  }

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
    <div className="space-y-4">
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
          <div className="flex items-center justify-end gap-2">
            <DropdownMenu>
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
            <DocsLibrarySortMenu
              value={{ field: sortField, direction: sortDir }}
              onChange={(next) => {
                setSortField(next.field)
                setSortDir(next.direction)
              }}
            />
          </div>

          <DocsLibraryList ariaLabel={title}>
            {displayDocs.map((doc: DocsDocument) => (
              <DocsLibraryRow
                key={doc.id}
                title={doc.title}
                status={doc.status}
                updatedAt={doc.updated_at}
                location={locationFor(doc)}
                onOpen={() => navigate({
                  to: '/w/$slug/docs/documents/$docId',
                  params: { slug: wsSlug, docId: doc.id },
                })}
                actions={canEditDocs ? (
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
                        <DropdownMenuItem onClick={() => publishDoc.mutate({ id: doc.id }, {
                          onSuccess: () => toast.success('Document published'),
                          onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to publish'),
                        })}>
                          <SentIcon className="h-3.5 w-3.5" />
                          Publish
                        </DropdownMenuItem>
                      )}
                      {doc.status === 'archived' ? (
                        <DropdownMenuItem onClick={() => unarchiveDoc.mutate(doc.id, {
                          onSuccess: () => toast.success('Document unarchived'),
                          onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to unarchive'),
                        })}>
                          <ArchiveRestoreIcon className="h-3.5 w-3.5" />
                          Unarchive
                        </DropdownMenuItem>
                      ) : (
                        <DropdownMenuItem onClick={() => archiveDoc.mutate(doc.id, {
                          onSuccess: () => toast.success('Document archived'),
                          onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to archive'),
                        })}>
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
                ) : undefined}
              />
            ))}
          </DocsLibraryList>
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
