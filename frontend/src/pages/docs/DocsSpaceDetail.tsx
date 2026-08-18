import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { toast } from 'sonner'
import { useTitle } from '@/hooks/useTitle'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { useGlobalCreateStore } from '@/stores/globalCreateStore'
import { buildCollectionPathLabels, buildCollectionTree } from '@/components/docs/docsCollectionTree'
import {
  useDocsSpace,
  useDocsCollections,
  useDocsDocuments,
  useDeleteDocsCollection,
  useDeleteDocsSpace,
  useArchiveDocsDocument,
  useUnarchiveDocsDocument,
  useDeleteDocsDocument,
  useDuplicateDocsDocument,
  usePublishDocsDocument,
  useWorkspaceAccess,
  usePermissions,
} from '@/hooks/queries'
import type { DocsCollection, DocsDocument, DocStatus } from '@/lib/docsTypes'
import { ConfirmDialog } from '@/components/pm/ConfirmDialog'
import { CreateCollectionDialog } from '@/components/docs/CreateCollectionDialog'
import { DeleteCollectionDialog } from '@/components/docs/DeleteCollectionDialog'
import { DeleteSpaceDialog } from '@/components/docs/DeleteSpaceDialog'
import { SpaceDialog } from '@/components/docs/SpaceDialog'
import { MoveDocumentDialog } from '@/components/docs/MoveDocumentDialog'
import { DocumentsTable, type DocumentsTableSortField } from '@/pages/docs/spaceDetail/DocumentsTable'
import { SpaceNodeHeader } from '@/pages/docs/spaceDetail/SpaceNodeHeader'
import { CollectionCardGrid } from '@/pages/docs/spaceDetail/CollectionCardGrid'
import { EmptyNodeState } from '@/pages/docs/spaceDetail/EmptyNodeState'
import { UncategorizedSection } from '@/pages/docs/spaceDetail/UncategorizedSection'
import { NodeSkeleton } from '@/pages/docs/spaceDetail/NodeSkeleton'
import { APIReferenceSection } from '@/components/docs/APIReferenceSection'
import {
  countDirectDocs,
  directChildrenOfView,
  resolveView,
  scopedDocuments,
  type StatusFilter,
} from '@/pages/docs/spaceDetail/nodeSelection'

export function DocsSpaceDetail() {
  const navigate = useNavigate()
  const { collection: activeCollectionParam } = useSearch({
    from: '/_authenticated/w/$slug/docs/spaces/$spaceId',
  })
  const { spaceId } = useParams({ strict: false }) as { spaceId: string }
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id ?? ''
  const wsSlug = workspace?.slug ?? ''

  const openCreate = useGlobalCreateStore((s) => s.openCreate)

  const { data: access } = useWorkspaceAccess(wsId)
  const { canEditDocs } = usePermissions(access)

  const [filterStatus, setFilterStatus] = useState<DocStatus | null>(null)
  const [sortField, setSortField] = useState<DocumentsTableSortField>('position')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc')

  const { data: space, isLoading: spaceLoading } = useDocsSpace(wsId, spaceId)
  const { data: collections } = useDocsCollections(wsId, spaceId)
  const docFilters = { space_id: spaceId, include_archived: 'true', ...(filterStatus ? { status: filterStatus } : {}) }
  const { data: documents } = useDocsDocuments(wsId, docFilters)
  const archiveDoc = useArchiveDocsDocument(wsId)
  const unarchiveDoc = useUnarchiveDocsDocument(wsId)
  const deleteDoc = useDeleteDocsDocument(wsId)
  const duplicateDoc = useDuplicateDocsDocument(wsId)
  const publishDoc = usePublishDocsDocument(wsId)
  const [editSpaceOpen, setEditSpaceOpen] = useState(false)
  const [duplicatingDocId, setDuplicatingDocId] = useState<string | null>(null)
  const [movingDoc, setMovingDoc] = useState<DocsDocument | null>(null)
  const [deleteConfirmDoc, setDeleteConfirmDoc] = useState<DocsDocument | null>(null)

  useTitle(space?.name ?? 'Space')

  const collectionPaths = useMemo(
    () => buildCollectionPathLabels(collections ?? []),
    [collections],
  )

  // Pre-partitioned tree. Single source of ordering (sibling bucket
  // sort is (position, created_at)) and document partitioning
  // (per-collection .documents and .uncategorizedDocuments). Null
  // until both collections and documents resolve so downstream
  // helpers can render a loading state instead of a flash of
  // space_root.
  const tree = useMemo(() => {
    if (!collections || !documents) return null
    return buildCollectionTree(spaceId, collections, documents)
  }, [spaceId, collections, documents])

  // Status filter is the shared predicate across the table and the
  // header/card counts — see §5.3 of the plan.
  const filter = useMemo<StatusFilter>(() => ({ status: filterStatus }), [filterStatus])

  const view = useMemo(
    () => resolveView(activeCollectionParam ?? null, tree),
    [activeCollectionParam, tree],
  )

  const childNodes = useMemo(() => directChildrenOfView(view, tree), [view, tree])
  const scopedDocs = useMemo(
    () => scopedDocuments(view, tree, filter),
    [view, tree, filter],
  )
  const uncategorizedCount = useMemo(
    () => countDirectDocs(null, tree, filter),
    [tree, filter],
  )

  // Deep-link fallback: if the URL points at a collection id that no
  // longer exists (user deleted it in another tab, or a stale link),
  // strip the param and surface a toast. Only runs after collections
  // load so we never clean up a merely-pending deep-link.
  useEffect(() => {
    if (!collections || !activeCollectionParam || activeCollectionParam === '__uncollected__') return
    const exists = collections.some((c) => c.id === activeCollectionParam)
    if (!exists) {
      toast.error('That collection is no longer available.')
      navigate({
        to: '/w/$slug/docs/spaces/$spaceId',
        params: { slug: wsSlug, spaceId },
        search: {},
        replace: true,
      })
    }
  }, [collections, activeCollectionParam, wsSlug, spaceId, navigate])

  // Navigation helper — all drill/breadcrumb/card clicks flow through
  // navigate() so the URL stays the source of truth.
  const navigateToCollection = (target: string | null) => {
    navigate({
      to: '/w/$slug/docs/spaces/$spaceId',
      params: { slug: wsSlug, spaceId },
      search: target === null ? {} : { collection: target },
    })
  }

  const deleteCollection = useDeleteDocsCollection(wsId)
  const deleteSpace = useDeleteDocsSpace(wsId)
  const [editingCollection, setEditingCollection] = useState<DocsCollection | null>(null)

  // Typed confirm dialog state
  const [confirmDelete, setConfirmDelete] = useState<{
    type: 'collection' | 'space'
    id: string
    name: string
  } | null>(null)

  const handleConfirmDelete = async () => {
    if (!confirmDelete) return
    const { type, id, name } = confirmDelete
    try {
      if (type === 'collection') {
        await deleteCollection.mutateAsync({ id, spaceId })
        toast.success(`Collection "${name}" deleted`)
        if (activeCollectionParam === id) navigateToCollection(null)
      } else {
        await deleteSpace.mutateAsync(id)
        toast.success(`Space "${name}" deleted`)
        navigate({ to: '/w/$slug/docs', params: { slug: wsSlug } })
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : `Failed to delete ${type}`)
    }
  }

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

  const handleConfirmDeleteDoc = async () => {
    if (!deleteConfirmDoc) return
    try {
      await deleteDoc.mutateAsync(deleteConfirmDoc.id)
      toast.success('Document deleted')
      setDeleteConfirmDoc(null)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete')
    }
  }

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>
  }

  if (spaceLoading) {
    return (
      <div className="space-y-3 py-4">
        <div className="h-8 w-48 animate-pulse rounded bg-muted/60" />
        <div className="h-10 animate-pulse rounded-lg bg-muted/60" />
        <div className="h-10 animate-pulse rounded-lg bg-muted/60" />
      </div>
    )
  }

  if (!space) {
    return <p className="text-sm text-muted-foreground">Space not found.</p>
  }

  return (
    <div className="space-y-4">
      <SpaceNodeHeader
        space={space}
        view={view}
        canEdit={canEditDocs}
        onNavigateToCollection={navigateToCollection}
        onCreateDocument={() => openCreate('docs_document', {
          spaceId,
          collectionId:
            view.kind === 'collection' ? view.node.collection.id : undefined,
        })}
        onCreateChildCollection={
          view.kind === 'uncategorized' ||
          (view.kind === 'collection' && view.node.collection.depth >= 1)
            ? null
            : () => openCreate('docs_collection', {
                spaceId,
                parentCollectionId:
                  view.kind === 'collection' ? view.node.collection.id : undefined,
              })
        }
        onEditSpace={() => setEditSpaceOpen(true)}
        onDeleteSpace={() => setConfirmDelete({ type: 'space', id: spaceId, name: space.name })}
        onEditCollection={
          view.kind === 'collection'
            ? () => setEditingCollection(view.node.collection)
            : null
        }
        onDeleteCollection={
          view.kind === 'collection'
            ? () => setConfirmDelete({
                type: 'collection',
                id: view.node.collection.id,
                name: view.node.collection.name,
              })
            : null
        }
      />

      {view.kind === 'loading' ? (
        <NodeSkeleton />
      ) : (
        <>
          {view.kind === 'space_root' && space.type === 'external_capable' && (
            <APIReferenceSection wsId={wsId} spaceId={spaceId} canEdit={canEditDocs} />
          )}

          <CollectionCardGrid
            title={view.kind === 'space_root' ? 'Collections' : 'Sub-collections'}
            nodes={childNodes}
            filter={filter}
            canEdit={canEditDocs}
            onOpen={(node) => navigateToCollection(node.collection.id)}
            onEdit={(node) => setEditingCollection(node.collection)}
            onDelete={(node) => setConfirmDelete({
              type: 'collection',
              id: node.collection.id,
              name: node.collection.name,
            })}
          />

          {(collections ?? []).length > 0 && scopedDocs.length > 0 && (
            <hr className="border-border/40" />
          )}

          {scopedDocs.length > 0 ? (
            <DocumentsTable
                documents={scopedDocs}
                collectionPaths={collectionPaths}
                showCollectionPath={view.kind === 'space_root'}
                hasCollections={(collections ?? []).length > 0}
                wsSlug={wsSlug}
              filterStatus={filterStatus}
              onFilterStatus={setFilterStatus}
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
              onCreateDocument={() => openCreate('docs_document', {
                spaceId,
                collectionId:
                  view.kind === 'collection' ? view.node.collection.id : undefined,
              })}
              onCreateCollection={() => openCreate('docs_collection', {
                spaceId,
                parentCollectionId:
                  view.kind === 'collection' ? view.node.collection.id : undefined,
              })}
            />
          ) : childNodes.length === 0 ? (
            <EmptyNodeState
              view={view}
              canEdit={canEditDocs}
              canAddChildCollection={
                view.kind !== 'uncategorized' &&
                !(view.kind === 'collection' && view.node.collection.depth >= 1)
              }
              onCreateDocument={() => openCreate('docs_document', {
                spaceId,
                collectionId:
                  view.kind === 'collection' ? view.node.collection.id : undefined,
              })}
              onCreateChildCollection={() => openCreate('docs_collection', {
                spaceId,
                parentCollectionId:
                  view.kind === 'collection' ? view.node.collection.id : undefined,
              })}
            />
          ) : null}

          {view.kind === 'space_root' && uncategorizedCount > 0 && (
            <UncategorizedSection
              count={uncategorizedCount}
              onOpen={() => navigateToCollection('__uncollected__')}
            />
          )}
        </>
      )}

      {/* Translation management (space/collection names, descriptions,
          slugs) lives in Settings > Help Center > Languages & Translation.
          Do not re-add a translations sheet here — the settings table
          supports Names/Descriptions modes, bulk auto-translate, and
          optimistic updates. See commit history for the migration. */}

      {/* Space delete — shared impact-aware dialog */}
      <DeleteSpaceDialog
        wsId={wsId}
        space={
          confirmDelete?.type === 'space'
            ? { id: confirmDelete.id, name: confirmDelete.name }
            : null
        }
        open={confirmDelete?.type === 'space'}
        onOpenChange={(open) => { if (!open) setTimeout(() => setConfirmDelete(null), 150) }}
        onConfirm={handleConfirmDelete}
      />

      {/* Collection delete — shared impact-aware dialog */}
      <DeleteCollectionDialog
        wsId={wsId}
        collection={
          confirmDelete?.type === 'collection'
            ? { id: confirmDelete.id, name: confirmDelete.name, space_id: spaceId }
            : null
        }
        open={confirmDelete?.type === 'collection'}
        onOpenChange={(open) => { if (!open) setTimeout(() => setConfirmDelete(null), 150) }}
        onConfirm={handleConfirmDelete}
      />

      <ConfirmDialog
        open={deleteConfirmDoc !== null}
        onOpenChange={(open) => { if (!open) setDeleteConfirmDoc(null) }}
        title="Delete document"
        description="This will permanently delete the document and its saved content. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleConfirmDeleteDoc}
      />

      <SpaceDialog
        wsId={wsId}
        open={editSpaceOpen}
        onOpenChange={setEditSpaceOpen}
        space={space ?? null}
      />

      <CreateCollectionDialog
        wsId={wsId}
        open={editingCollection !== null}
        onOpenChange={(open) => {
          if (!open) setEditingCollection(null)
        }}
        collection={editingCollection}
      />

      {movingDoc && (
        <MoveDocumentDialog
          wsId={wsId}
          open={!!movingDoc}
          onOpenChange={(open) => { if (!open) setMovingDoc(null) }}
          docId={movingDoc.id}
          docTitle={movingDoc.title}
          currentSpaceId={movingDoc.space_id ?? spaceId}
          currentCollectionId={movingDoc.collection_id}
        />
      )}
    </div>
  )
}
