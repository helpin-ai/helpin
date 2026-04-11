import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { timeAgo } from '@/lib/utils'
import {
  LanguageCircleIcon,
} from '@/lib/icons'
import { toast } from 'sonner'
import { useTitle } from '@/hooks/useTitle'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { useGlobalCreateStore } from '@/stores/globalCreateStore'
import { buildCollectionTree } from '@/components/docs/docsCollectionTree'
import {
  useDocsSpace,
  useDocsCollections,
  useDocsDocuments,
  useDeleteDocsCollection,
  useUpdateDocsSpace,
  useDeleteDocsSpace,
  useArchiveDocsDocument,
  useUnarchiveDocsDocument,
  useDeleteDocsDocument,
  useDuplicateDocsDocument,
  useDocsHelpcenterCollectionTranslations,
  useDocsHelpcenterLocales,
  useDocsHelpcenterSpaceTranslations,
  useMarkDocsHelpcenterCollectionTranslationReviewed,
  useMarkDocsHelpcenterSpaceTranslationReviewed,
  usePublishDocsHelpcenterCollectionTranslation,
  usePublishDocsHelpcenterSpaceTranslation,
  useUnpublishDocsHelpcenterCollectionTranslation,
  useUnpublishDocsHelpcenterSpaceTranslation,
  useUpsertDocsHelpcenterCollectionTranslation,
  useUpsertDocsHelpcenterSpaceTranslation,
  usePublishDocsDocument,
  useAssignableMembers,
  useWorkspaceAccess,
  usePermissions,
} from '@/hooks/queries'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import type { DocsCollection, DocsDocument, DocStatus, DocsHelpcenterTranslationState } from '@/lib/docsTypes'
import { getHelpcenterLocaleLabel } from '@/lib/docsTypes'
import { ConfirmDialog } from '@/components/pm/ConfirmDialog'
import { CreateCollectionDialog } from '@/components/docs/CreateCollectionDialog'
import { TypedConfirmDialog } from '@/components/docs/TypedConfirmDialog'
import { SpaceDialog } from '@/components/docs/SpaceDialog'
import { MoveDocumentDialog } from '@/components/docs/MoveDocumentDialog'
import { DocumentsTable } from '@/pages/docs/spaceDetail/DocumentsTable'
import { SpaceNodeHeader } from '@/pages/docs/spaceDetail/SpaceNodeHeader'
import { CollectionCardGrid } from '@/pages/docs/spaceDetail/CollectionCardGrid'
import { EmptyNodeState } from '@/pages/docs/spaceDetail/EmptyNodeState'
import { UncategorizedSection } from '@/pages/docs/spaceDetail/UncategorizedSection'
import { NodeSkeleton } from '@/pages/docs/spaceDetail/NodeSkeleton'
import {
  countDirectDocs,
  directChildrenOfView,
  resolveView,
  scopedDocuments,
  type StatusFilter,
} from '@/pages/docs/spaceDetail/nodeSelection'
import { EditCollectionTranslationDialog } from '@/components/docs/helpcenter/EditCollectionTranslationDialog'
import { EditSpaceTranslationDialog } from '@/components/docs/helpcenter/EditSpaceTranslationDialog'
import { TranslationsPanel, type TranslationRow } from '@/components/docs/helpcenter/TranslationsPanel'
import { PublishSlugDialog } from '@/components/docs/helpcenter/PublishSlugDialog'
import { suggestDocsSlug } from '@/lib/docsSlugs'

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
  const [sortField, setSortField] = useState<'updated_at' | 'title' | 'status'>('updated_at')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc')

  const { data: space, isLoading: spaceLoading } = useDocsSpace(wsId, spaceId)
  const { data: collections } = useDocsCollections(wsId, spaceId)
  const docFilters = { space_id: spaceId, include_archived: 'true', ...(filterStatus ? { status: filterStatus } : {}) }
  const { data: documents } = useDocsDocuments(wsId, docFilters)
  const { data: localesConfig } = useDocsHelpcenterLocales(wsId)
  const { data: spaceTranslations = [] } = useDocsHelpcenterSpaceTranslations(wsId, spaceId)
  const { data: members = [] } = useAssignableMembers(wsId)
  const archiveDoc = useArchiveDocsDocument(wsId)
  const unarchiveDoc = useUnarchiveDocsDocument(wsId)
  const deleteDoc = useDeleteDocsDocument(wsId)
  const duplicateDoc = useDuplicateDocsDocument(wsId)
  const publishDoc = usePublishDocsDocument(wsId)
  const [editSpaceOpen, setEditSpaceOpen] = useState(false)
  const [translationsOpen, setTranslationsOpen] = useState(false)
  const [editingSpaceTranslationLocale, setEditingSpaceTranslationLocale] = useState<string | null>(null)
  const [editingCollectionTranslationLocale, setEditingCollectionTranslationLocale] = useState<string | null>(null)
  const [pendingTranslationPublish, setPendingTranslationPublish] = useState<{
    entity: 'space' | 'collection'
    locale: string
    slug: string
  } | null>(null)
  const [translationCollectionId, setTranslationCollectionId] = useState('')
  const [duplicatingDocId, setDuplicatingDocId] = useState<string | null>(null)
  const [movingDoc, setMovingDoc] = useState<DocsDocument | null>(null)
  const [deleteConfirmDoc, setDeleteConfirmDoc] = useState<DocsDocument | null>(null)

  useTitle(space?.name ?? 'Space')

  // Collection name lookup — still passed into DocumentsTable's
  // Collection column when rendered at space root.
  const collectionNames = useMemo(
    () => new Map<string, string>((collections ?? []).map((c) => [c.id, c.name])),
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
  const updateSpace = useUpdateDocsSpace(wsId)
  const deleteSpace = useDeleteDocsSpace(wsId)
  const upsertSpaceTranslation = useUpsertDocsHelpcenterSpaceTranslation(wsId, spaceId)
  const publishSpaceTranslation = usePublishDocsHelpcenterSpaceTranslation(wsId, spaceId)
  const unpublishSpaceTranslation = useUnpublishDocsHelpcenterSpaceTranslation(wsId, spaceId)
  const markSpaceTranslationReviewed = useMarkDocsHelpcenterSpaceTranslationReviewed(wsId, spaceId)
  const [editingCollection, setEditingCollection] = useState<DocsCollection | null>(null)

  // Inline space rename
  const [renamingSpace, setRenamingSpace] = useState(false)
  const [spaceNameValue, setSpaceNameValue] = useState('')

  const startRenameSpace = () => {
    setRenamingSpace(true)
    setSpaceNameValue(space?.name ?? '')
  }

  const commitRenameSpace = async () => {
    setRenamingSpace(false)
    const name = spaceNameValue.trim()
    if (!name || !space || name === space.name) return
    try {
      await updateSpace.mutateAsync({ id: spaceId, name })
      toast.success('Space renamed')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to rename space')
    }
  }

  // Typed confirm dialog state
  const [confirmDelete, setConfirmDelete] = useState<{
    type: 'collection' | 'space'
    id: string
    name: string
  } | null>(null)

  useEffect(() => {
    const availableCollections = collections ?? []
    if (availableCollections.length === 0) {
      setTranslationCollectionId('')
      return
    }
    if (activeCollectionParam && activeCollectionParam !== '__uncollected__' && availableCollections.some((collection) => collection.id === activeCollectionParam)) {
      setTranslationCollectionId(activeCollectionParam)
      return
    }
    setTranslationCollectionId((current) => (
      current && availableCollections.some((collection) => collection.id === current)
        ? current
        : availableCollections[0].id
    ))
  }, [collections, activeCollectionParam])

  const { data: collectionTranslations = [] } = useDocsHelpcenterCollectionTranslations(wsId, translationCollectionId)
  const upsertCollectionTranslation = useUpsertDocsHelpcenterCollectionTranslation(wsId, translationCollectionId)
  const publishCollectionTranslation = usePublishDocsHelpcenterCollectionTranslation(wsId, translationCollectionId)
  const unpublishCollectionTranslation = useUnpublishDocsHelpcenterCollectionTranslation(wsId, translationCollectionId)
  const markCollectionTranslationReviewed = useMarkDocsHelpcenterCollectionTranslationReviewed(wsId, translationCollectionId)

  const defaultLocale = localesConfig?.default_locale ?? 'en'
  const enabledLocales = localesConfig?.enabled_locales?.length
    ? localesConfig.enabled_locales
    : [defaultLocale]
  const spaceTranslationsByLocale = new Map(spaceTranslations.map((translation) => [translation.locale, translation]))
  const collectionTranslationsByLocale = new Map(collectionTranslations.map((translation) => [translation.locale, translation]))
  const selectedTranslationCollection = (collections ?? []).find((collection) => collection.id === translationCollectionId) ?? null

  const spaceTranslationRows: TranslationRow[] = enabledLocales.map((locale) => {
    const translation = spaceTranslationsByLocale.get(locale)
    const isDefaultLocale = locale === defaultLocale
    return {
      locale,
      state: (translation?.status ?? 'missing') as DocsHelpcenterTranslationState,
      updatedAtLabel: translation ? `Updated ${timeAgo(translation.updated_at)}` : undefined,
      helperText: isDefaultLocale
        ? 'Mirrored from the source space and refreshed automatically when the source changes.'
        : translation?.source_synced === false
          ? 'Source changes landed after this translation. Review the copy before publishing again.'
          : 'Localized space copy is used in the public navigation and locale-aware URLs.',
      isDefaultLocale,
      sourceMirrorLabel: 'Source mirror',
    }
  })

  const collectionTranslationRows: TranslationRow[] = enabledLocales.map((locale) => {
    const translation = collectionTranslationsByLocale.get(locale)
    const isDefaultLocale = locale === defaultLocale
    const parentPublished = spaceTranslationsByLocale.get(locale)?.status === 'published'
    return {
      locale,
      state: (translation?.status ?? 'missing') as DocsHelpcenterTranslationState,
      updatedAtLabel: translation ? `Updated ${timeAgo(translation.updated_at)}` : undefined,
      helperText: isDefaultLocale
        ? 'Mirrored from the source collection and kept in sync automatically.'
        : translation?.source_synced === false
          ? 'Source collection details changed. Review this translation before republishing.'
          : 'Localized collection labels and descriptions shape the public information architecture for this locale.',
      publishBlockedReason: !isDefaultLocale && translation && !parentPublished
        ? 'Publish the parent translation first'
        : undefined,
      isDefaultLocale,
      sourceMirrorLabel: 'Source mirror',
    }
  })

  const handleSaveSpaceTranslation = async (data: Parameters<typeof upsertSpaceTranslation.mutateAsync>[0]) => {
    try {
      await upsertSpaceTranslation.mutateAsync(data)
      toast.success('Space translation saved')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save translation')
      throw err
    }
  }

  const handleSaveCollectionTranslation = async (data: Parameters<typeof upsertCollectionTranslation.mutateAsync>[0]) => {
    try {
      await upsertCollectionTranslation.mutateAsync(data)
      toast.success('Collection translation saved')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save translation')
      throw err
    }
  }

  const handleConfirmTranslationPublish = async () => {
    if (!pendingTranslationPublish) return

    try {
      if (pendingTranslationPublish.entity === 'space') {
        await publishSpaceTranslation.mutateAsync({
          locale: pendingTranslationPublish.locale,
          slug: pendingTranslationPublish.slug,
        })
        toast.success('Space translation published')
      } else {
        await publishCollectionTranslation.mutateAsync({
          locale: pendingTranslationPublish.locale,
          slug: pendingTranslationPublish.slug,
        })
        toast.success('Collection translation published')
      }
      setPendingTranslationPublish(null)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to publish translation')
    }
  }

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
        docCount={scopedDocs.length}
        subCount={childNodes.length}
        canEdit={canEditDocs}
        onNavigateToDocs={() => navigate({ to: '/w/$slug/docs', params: { slug: wsSlug } })}
        onNavigateToCollection={navigateToCollection}
        onCreateDocument={() => openCreate('docs_document', {
          spaceId,
          collectionId:
            view.kind === 'collection' ? view.node.collection.id : undefined,
        })}
        onCreateChildCollection={
          view.kind === 'uncategorized' ||
          (view.kind === 'collection' && view.node.collection.depth >= 2)
            ? null
            : () => openCreate('docs_collection', {
                spaceId,
                parentCollectionId:
                  view.kind === 'collection' ? view.node.collection.id : undefined,
              })
        }
        renamingSpace={renamingSpace}
        spaceNameValue={spaceNameValue}
        onStartRenameSpace={startRenameSpace}
        onSpaceNameChange={setSpaceNameValue}
        onCommitRenameSpace={commitRenameSpace}
        onCancelRenameSpace={() => setRenamingSpace(false)}
        onEditSpace={() => setEditSpaceOpen(true)}
        onDeleteSpace={() => setConfirmDelete({ type: 'space', id: spaceId, name: space.name })}
        onOpenTranslations={() => setTranslationsOpen(true)}
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

          {scopedDocs.length > 0 ? (
            <section className="space-y-3">
              {childNodes.length > 0 && (
                <h2 className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                  {view.kind === 'space_root' ? 'All documents' : 'Documents'}
                </h2>
              )}
              <DocumentsTable
                documents={scopedDocs}
                members={members}
                collectionNames={collectionNames}
                showCollectionColumn={view.kind === 'space_root'}
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
            </section>
          ) : childNodes.length === 0 ? (
            <EmptyNodeState
              view={view}
              canEdit={canEditDocs}
              canAddChildCollection={
                view.kind !== 'uncategorized' &&
                !(view.kind === 'collection' && view.node.collection.depth >= 2)
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

      <Sheet open={translationsOpen} onOpenChange={setTranslationsOpen}>
        <SheetContent className="w-full data-[side=right]:w-full data-[side=right]:sm:max-w-2xl">
          <SheetHeader className="border-b border-border/60">
            <SheetTitle className="flex items-center gap-2">
              <LanguageCircleIcon className="h-4 w-4 text-primary" />
              Public translations
            </SheetTitle>
            <SheetDescription>
              Manage the language variants for this space and its collections without changing the source docs structure.
            </SheetDescription>
          </SheetHeader>

          <div className="flex-1 space-y-6 overflow-y-auto p-4">
            <TranslationsPanel
              title="Space translations"
              description="These localized labels and slugs power the space-level public route and navigation."
              locales={enabledLocales}
              rows={spaceTranslationRows}
              onAdd={setEditingSpaceTranslationLocale}
              onEdit={setEditingSpaceTranslationLocale}
              onPublish={(locale) => {
                const translation = spaceTranslationsByLocale.get(locale)
                if (translation && !translation.slug) {
                  setPendingTranslationPublish({
                    entity: 'space',
                    locale,
                    slug: suggestDocsSlug(translation.name || 'space', 'space'),
                  })
                  return
                }
                publishSpaceTranslation.mutate({ locale }, {
                  onSuccess: () => toast.success('Space translation published'),
                  onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to publish translation'),
                })
              }}
              onUnpublish={(locale) => {
                unpublishSpaceTranslation.mutate(locale, {
                  onSuccess: () => toast.success('Space translation reverted to draft'),
                  onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to unpublish translation'),
                })
              }}
              onMarkReviewed={(locale) => {
                markSpaceTranslationReviewed.mutate(locale, {
                  onSuccess: () => toast.success('Space translation marked as reviewed'),
                  onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to mark translation reviewed'),
                })
              }}
            />

            <div className="space-y-4 rounded-2xl border border-border/60 bg-card p-4">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
                <div>
                  <h3 className="text-sm font-semibold">Collection translations</h3>
                  <p className="text-xs text-muted-foreground">
                    Pick a collection to manage its public label, slug, and description for each enabled locale.
                  </p>
                </div>
                {selectedTranslationCollection && (
                  <Select value={translationCollectionId} onValueChange={setTranslationCollectionId}>
                    <SelectTrigger className="w-full sm:w-72">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {(collections ?? []).map((collection) => (
                        <SelectItem key={collection.id} value={collection.id}>
                          {collection.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </div>

              {selectedTranslationCollection ? (
                <TranslationsPanel
                  title={selectedTranslationCollection.name}
                  description="Collection translations inherit readiness from the parent space translation in the same locale."
                  locales={enabledLocales}
                  rows={collectionTranslationRows}
                  onAdd={setEditingCollectionTranslationLocale}
                  onEdit={setEditingCollectionTranslationLocale}
                  onPublish={(locale) => {
                    const translation = collectionTranslationsByLocale.get(locale)
                    if (translation && !translation.slug) {
                      setPendingTranslationPublish({
                        entity: 'collection',
                        locale,
                        slug: suggestDocsSlug(translation.name || 'collection', 'collection'),
                      })
                      return
                    }
                    publishCollectionTranslation.mutate({ locale }, {
                      onSuccess: () => toast.success('Collection translation published'),
                      onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to publish translation'),
                    })
                  }}
                  onUnpublish={(locale) => {
                    unpublishCollectionTranslation.mutate(locale, {
                      onSuccess: () => toast.success('Collection translation reverted to draft'),
                      onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to unpublish translation'),
                    })
                  }}
                  onMarkReviewed={(locale) => {
                    markCollectionTranslationReviewed.mutate(locale, {
                      onSuccess: () => toast.success('Collection translation marked as reviewed'),
                      onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to mark translation reviewed'),
                    })
                  }}
                />
              ) : (
                <div className="rounded-2xl border border-dashed border-border/60 bg-muted/20 px-4 py-6 text-center">
                  <p className="text-sm font-medium">No collections yet</p>
                  <p className="mt-1 text-xs text-muted-foreground">
                    Create a collection first, then translate its public-facing name and description here.
                  </p>
                </div>
              )}
            </div>
          </div>
        </SheetContent>
      </Sheet>

      {editingSpaceTranslationLocale && (
        <EditSpaceTranslationDialog
          key={`${editingSpaceTranslationLocale}-${spaceTranslationsByLocale.get(editingSpaceTranslationLocale)?.updated_at ?? 'new'}`}
          open={Boolean(editingSpaceTranslationLocale)}
          onOpenChange={(open) => {
            if (!open) setEditingSpaceTranslationLocale(null)
          }}
          locale={editingSpaceTranslationLocale}
          sourceName={space.name}
          sourceSlug={space.slug}
          translation={spaceTranslationsByLocale.get(editingSpaceTranslationLocale) ?? null}
          isSaving={upsertSpaceTranslation.isPending}
          onSave={handleSaveSpaceTranslation}
        />
      )}

      {editingCollectionTranslationLocale && selectedTranslationCollection && (
        <EditCollectionTranslationDialog
          key={`${selectedTranslationCollection.id}-${editingCollectionTranslationLocale}-${collectionTranslationsByLocale.get(editingCollectionTranslationLocale)?.updated_at ?? 'new'}`}
          open={Boolean(editingCollectionTranslationLocale)}
          onOpenChange={(open) => {
            if (!open) setEditingCollectionTranslationLocale(null)
          }}
          locale={editingCollectionTranslationLocale}
          sourceName={selectedTranslationCollection.name}
          sourceSlug={selectedTranslationCollection.slug}
          sourceDescription={selectedTranslationCollection.description}
          translation={collectionTranslationsByLocale.get(editingCollectionTranslationLocale) ?? null}
          isSaving={upsertCollectionTranslation.isPending}
          onSave={handleSaveCollectionTranslation}
        />
      )}

      <PublishSlugDialog
        open={pendingTranslationPublish !== null}
        onOpenChange={(open) => {
          if (!open) setPendingTranslationPublish(null)
        }}
        title={
          pendingTranslationPublish?.entity === 'collection'
            ? `Confirm ${pendingTranslationPublish ? `${getHelpcenterLocaleLabel(pendingTranslationPublish.locale)} ` : ''}collection slug`
            : `Confirm ${pendingTranslationPublish ? `${getHelpcenterLocaleLabel(pendingTranslationPublish.locale)} ` : ''}space slug`
        }
        description={
          pendingTranslationPublish?.entity === 'collection'
            ? 'This slug will be used in the localized public help center collection path on first publish.'
            : 'This slug will be stored with the localized space record on first publish.'
        }
        slug={pendingTranslationPublish?.slug ?? ''}
        onSlugChange={(slug) => {
          setPendingTranslationPublish((current) => (current ? { ...current, slug } : current))
        }}
        onConfirm={handleConfirmTranslationPublish}
        isPublishing={pendingTranslationPublish?.entity === 'collection' ? publishCollectionTranslation.isPending : publishSpaceTranslation.isPending}
      />

      {/* Typed confirm delete dialog */}
      <TypedConfirmDialog
        open={confirmDelete !== null}
        onOpenChange={(open) => { if (!open) setTimeout(() => setConfirmDelete(null), 150) }}
        title={confirmDelete?.type === 'space' ? 'Delete space' : 'Delete collection'}
        description={
          confirmDelete?.type === 'space'
            ? 'This will permanently delete this space and all its documents. This action cannot be undone.'
            : 'This will permanently delete this collection. Documents in this collection will become uncategorized.'
        }
        confirmText={confirmDelete?.name ?? ''}
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
