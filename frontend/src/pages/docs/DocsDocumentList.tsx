import { useMemo, useState } from 'react'
import { Clock01Icon, PencilEdit02Icon, UserIcon } from '@/lib/icons'
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
  useAssignableMembers,
  useWorkspaceAccess,
  usePermissions,
} from '@/hooks/queries'
import { MoveDocumentDialog } from '@/components/docs/MoveDocumentDialog'
import { ConfirmDialog } from '@/components/pm/ConfirmDialog'
import type { DocsDocument, DocStatus } from '@/lib/docsTypes'
import { buildCollectionPathLabels } from '@/components/docs/docsCollectionTree'
import { QuietPageHeader, QuietPageViewport } from '@/components/design-system/quiet'
import {
  DocumentsTable,
  type DocumentsTablePathColumn,
  type DocumentsTableSortDir,
  type DocumentsTableSortField,
} from '@/pages/docs/spaceDetail/DocumentsTable'

interface DocsDocumentListProps {
  title: string
  description: string
  filterMode: 'my' | 'drafts' | 'recent'
}

// Same default as space pages: archived documents stay hidden until asked for.
const DEFAULT_STATUSES: DocStatus[] = ['draft', 'published']

export function DocsDocumentList({ title, description, filterMode }: DocsDocumentListProps) {
  useTitle(title)
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id ?? ''
  const wsSlug = workspace?.slug ?? ''

  const { data: access, isLoading: isAccessLoading } = useWorkspaceAccess(wsId)
  const { canEditDocs } = usePermissions(access)
  const membershipId = access?.membership.id

  // Drafts is one status, so it shows every row and no status filter.
  const [statuses, setStatuses] = useState<DocStatus[]>(filterMode === 'drafts' ? [] : DEFAULT_STATUSES)
  const [sortField, setSortField] = useState<DocumentsTableSortField>('updated_at')
  const [sortDir, setSortDir] = useState<DocumentsTableSortDir>('desc')
  const [duplicatingDocId, setDuplicatingDocId] = useState<string | null>(null)
  const [movingDoc, setMovingDoc] = useState<DocsDocument | null>(null)
  const [deleteConfirmDoc, setDeleteConfirmDoc] = useState<DocsDocument | null>(null)

  const filters = useMemo(() => {
    if (filterMode === 'drafts') return { status: 'draft' }
    // Owned or created by the caller; the server resolves who that is.
    if (filterMode === 'my') return { mine: 'true', include_archived: 'true' }
    return {}
  }, [filterMode])

  const { data: rawDocuments, isLoading } = useDocsDocuments(wsId, filters)
  const { data: spaces = [] } = useDocsSpaces(wsId)
  const { data: collections = [] } = useAllDocsCollections(wsId)
  const { data: members = [] } = useAssignableMembers(wsId)
  const archiveDoc = useArchiveDocsDocument(wsId)
  const unarchiveDoc = useUnarchiveDocsDocument(wsId)
  const deleteDoc = useDeleteDocsDocument(wsId)
  const duplicateDoc = useDuplicateDocsDocument(wsId)
  const publishDoc = usePublishDocsDocument(wsId)

  const pathColumn = useMemo<DocumentsTablePathColumn>(() => {
    const spaceNames = new Map(spaces.map((space) => [space.id, space.name]))
    const collectionPaths = buildCollectionPathLabels(collections)
    return {
      label: 'Location',
      emptyLabel: '—',
      pathFor: (doc) => {
        const collectionPath = doc.collection_id ? collectionPaths.get(doc.collection_id) : undefined
        return [spaceNames.get(doc.space_id), collectionPath].filter(Boolean).join(' › ') || null
      },
    }
  }, [spaces, collections])

  // Recent is the 20 most recently updated documents; the table sorts within them.
  const documents = useMemo(() => {
    if (!rawDocuments || filterMode !== 'recent') return rawDocuments
    return [...rawDocuments]
      .sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
      .slice(0, 20)
  }, [rawDocuments, filterMode])

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

  const EmptyIcon = filterMode === 'recent' ? Clock01Icon : filterMode === 'my' ? UserIcon : PencilEdit02Icon

  if (!workspace) {
    return (
      <div className="flex h-full min-h-0 flex-col">
        <QuietPageHeader variant="shell" title={title} description={description} />
        <QuietPageViewport className="min-h-0 flex-1">
          <p className="text-sm text-muted-foreground">Workspace not found.</p>
        </QuietPageViewport>
      </div>
    )
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <QuietPageHeader variant="shell" title={title} description={description} />

      <QuietPageViewport className="min-h-0 flex-1">
        {isLoading || (filterMode === 'my' && isAccessLoading) ? (
          <div className="space-y-2 py-4">
            {[1, 2, 3, 4].map((i) => (
              <div key={i} className="h-10 animate-pulse rounded-md bg-muted/60" />
            ))}
          </div>
        ) : !documents || documents.length === 0 ? (
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
        ) : (
          <DocumentsTable
            wsId={wsId}
            wsSlug={wsSlug}
            selectionScope={`${filterMode}:${membershipId ?? ''}`}
            documents={documents}
            members={members}
            pathColumn={pathColumn}
            showStatusFilter={filterMode !== 'drafts'}
            listLabel={title}
            searchLabel={`Search ${title.toLowerCase()}`}
            order={null}
            statuses={statuses}
            onStatusesChange={setStatuses}
            sortField={sortField}
            sortDir={sortDir}
            onSort={(field, dir) => { setSortField(field); setSortDir(dir) }}
            canEdit={canEditDocs}
            duplicatingDocId={duplicatingDocId}
            onArchive={(doc) => archiveDoc.mutate(doc.id, {
              onSuccess: () => toast.success('Document archived'),
              onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to archive'),
            })}
            onUnarchive={(doc) => unarchiveDoc.mutate(doc.id, {
              onSuccess: () => toast.success('Document unarchived'),
              onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to unarchive'),
            })}
            onDelete={setDeleteConfirmDoc}
            onDuplicate={(doc) => void handleDuplicateDoc(doc)}
            onMove={setMovingDoc}
            onPublish={(doc) => publishDoc.mutate({ id: doc.id }, {
              onSuccess: () => toast.success('Document published'),
              onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to publish'),
            })}
          />
        )}
      </QuietPageViewport>

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
