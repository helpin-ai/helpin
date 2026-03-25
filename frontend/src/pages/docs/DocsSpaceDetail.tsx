import { useEffect, useState } from 'react'
import { useNavigate, useParams, useLocation } from '@tanstack/react-router'
import { timeAgo } from '@/lib/utils'
import {
  Archive,
  ArchiveRestore,
  ArrowDown,
  ArrowLeft,
  ArrowUp,
  Check,
  ChevronDown,
  Copy,
  FileText,
  Folder,
  FolderInput,
  FolderOpen,
  Globe,
  Languages,
  ListFilter,
  MoreHorizontal,
  Pencil,
  Plus,
  Send,
  Settings,
  Trash2,
} from 'lucide-react'
import { toast } from 'sonner'
import { ICON_MAP } from '@/components/ui/icon-picker'
import { useTitle } from '@/hooks/useTitle'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { useGlobalCreateStore } from '@/stores/globalCreateStore'
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
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import type { DocsCollection, DocsDocument, DocStatus } from '@/lib/docsTypes'
import { DOC_STATUS_LABELS } from '@/lib/docsTypes'
import { UserAvatar } from '@/components/pm/UserAvatar'
import { ConfirmDialog } from '@/components/pm/ConfirmDialog'
import { formatAssignableMemberName } from '@/lib/assignableMembers'
import { QuickTooltip } from '@/components/ui/quick-tooltip'
import { CreateCollectionDialog } from '@/components/docs/CreateCollectionDialog'
import { TypedConfirmDialog } from '@/components/docs/TypedConfirmDialog'
import { SpaceDialog } from '@/components/docs/SpaceDialog'
import { MoveDocumentDialog } from '@/components/docs/MoveDocumentDialog'
import { EditCollectionTranslationDialog } from '@/components/docs/helpcenter/EditCollectionTranslationDialog'
import { EditSpaceTranslationDialog } from '@/components/docs/helpcenter/EditSpaceTranslationDialog'
import { TranslationsPanel, type TranslationRow } from '@/components/docs/helpcenter/TranslationsPanel'

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

function CollectionTabIcon({ name }: { name?: string | null }) {
  if (name) {
    const Icon = ICON_MAP[name]
    if (Icon) return <Icon className="h-3.5 w-3.5 shrink-0" />
  }
  return <FolderOpen className="h-3.5 w-3.5 shrink-0" />
}

export function DocsSpaceDetail() {
  const navigate = useNavigate()
  const location = useLocation()
  const { spaceId } = useParams({ strict: false }) as { spaceId: string }
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id ?? ''
  const wsSlug = workspace?.slug ?? ''

  const openCreate = useGlobalCreateStore((s) => s.openCreate)

  const { data: access } = useWorkspaceAccess(wsId)
  const { canEditDocs, canAdminDocs } = usePermissions(access)

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
  const [translationCollectionId, setTranslationCollectionId] = useState('')
  const [duplicatingDocId, setDuplicatingDocId] = useState<string | null>(null)
  const [movingDoc, setMovingDoc] = useState<DocsDocument | null>(null)
  const [deleteConfirmDoc, setDeleteConfirmDoc] = useState<DocsDocument | null>(null)

  useTitle(space?.name ?? 'Space')

  // Collection name lookup
  const collectionNames = new Map<string, string>(
    (collections ?? []).map((c) => [c.id, c.name])
  )

  // Group documents by collection
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
  // Read collection from URL search param
  const collectionParam = (location.search as Record<string, string | undefined>).collection ?? null
  const [activeCollection, setActiveCollection] = useState<string | null>(collectionParam)

  // Sync when URL search param changes (e.g. sidebar navigation)
  useEffect(() => {
    setActiveCollection(collectionParam)
  }, [collectionParam])

  const activeCollectionDocuments = activeCollection === '__uncollected__'
    ? uncollected
    : activeCollection
      ? collectionMap.get(activeCollection) ?? []
      : documents ?? []
  const showStatusFilter = (collections ?? []).length > 0 && (activeCollectionDocuments.length > 0 || Boolean(filterStatus))

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
    if (activeCollection && activeCollection !== '__uncollected__' && availableCollections.some((collection) => collection.id === activeCollection)) {
      setTranslationCollectionId(activeCollection)
      return
    }
    setTranslationCollectionId((current) => (
      current && availableCollections.some((collection) => collection.id === current)
        ? current
        : availableCollections[0].id
    ))
  }, [collections, activeCollection])

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
      state: translation?.status ?? 'missing',
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
      state: translation?.status ?? 'missing',
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

  const handleConfirmDelete = async () => {
    if (!confirmDelete) return
    const { type, id, name } = confirmDelete
    try {
      if (type === 'collection') {
        await deleteCollection.mutateAsync({ id, spaceId })
        toast.success(`Collection "${name}" deleted`)
        if (activeCollection === id) setActiveCollection(null)
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
      {/* Header */}
      <header className="flex items-start justify-between gap-4">
        <div className="flex items-start gap-3">
          <Button
            variant="ghost"
            size="icon"
            className="mt-0.5 h-8 w-8 shrink-0"
            onClick={() => navigate({ to: '/w/$slug/docs', params: { slug: wsSlug } })}
          >
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div>
            <div className="flex items-center gap-2">
              {space.icon ? (
                <span className="text-xl">{space.icon}</span>
              ) : (
                <Folder className="h-5 w-5 shrink-0 text-muted-foreground" />
              )}
              {renamingSpace ? (
                <input
                  key="space-rename-input"
                  autoFocus
                  value={spaceNameValue}
                  onChange={(e) => setSpaceNameValue(e.target.value)}
                  onFocus={(e) => e.target.select()}
                  onBlur={commitRenameSpace}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') commitRenameSpace()
                    if (e.key === 'Escape') setRenamingSpace(false)
                  }}
                  className="text-xl font-semibold bg-transparent border-b border-primary/40 outline-none px-0 py-0 min-w-0"
                />
              ) : (
                <h2
                  className="text-xl font-semibold cursor-text hover:text-foreground/80 transition-colors"
                  onClick={() => canEditDocs && startRenameSpace()}
                >
                  {space.name}
                </h2>
              )}
              {space.type === 'external_capable' && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Globe className="h-4 w-4 text-blue-500" />
                  </TooltipTrigger>
                  <TooltipContent>External</TooltipContent>
                </Tooltip>
              )}
            </div>
            <p className="mt-0.5 text-sm text-muted-foreground">
              {space.visibility === 'team_only' ? 'Team only' : 'All teams'} ·{' '}
              {documents?.length ?? 0} documents
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {space.type === 'external_capable' && canAdminDocs && (
            <Button
              variant="outline"
              size="sm"
              className="h-8 gap-1.5 text-xs"
              onClick={() => setTranslationsOpen(true)}
            >
              <Languages className="h-3.5 w-3.5" />
              Translations
            </Button>
          )}
          {canEditDocs && (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0">
                  <MoreHorizontal className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-44">
                <DropdownMenuItem onClick={() => setEditSpaceOpen(true)}>
                  <Settings className="h-3.5 w-3.5" />
                  Edit space
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => setConfirmDelete({ type: 'space', id: spaceId, name: space.name })}
                  className="text-destructive focus:text-destructive"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                  Delete space
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>
      </header>

      {/* Collection tabs */}
      {((collections ?? []).length > 0 || uncollected.length > 0) && <div className="flex flex-wrap items-center gap-1.5">
        <button
          type="button"
          onClick={() => setActiveCollection(null)}
          className={`rounded-full px-3.5 py-1 text-[13px] font-medium transition-colors ${
            !activeCollection
              ? 'bg-foreground text-background'
              : 'bg-muted/60 text-muted-foreground hover:bg-muted border border-border/40'
          }`}
        >
          All Collections ({documents?.length ?? 0})
        </button>

        {(collections ?? []).map((col) => (
          <div key={col.id} className="group/tab relative flex items-center">
            <button
              type="button"
              onClick={() => setActiveCollection(col.id)}
              className={`inline-flex items-center gap-1.5 rounded-full px-3.5 py-1 pr-7 text-[13px] font-medium transition-colors ${
                activeCollection === col.id
                  ? 'bg-foreground text-background'
                  : 'bg-muted/60 text-muted-foreground hover:bg-muted border border-border/40'
              }`}
            >
              <CollectionTabIcon name={col.icon} />
              <span>{col.name} ({collectionMap.get(col.id)?.length ?? 0})</span>
            </button>

            {canEditDocs && (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <button
                    type="button"
                    className={`absolute right-2 top-1/2 -translate-y-1/2 rounded-full p-0.5 opacity-0 transition-opacity outline-none focus:outline-none group-hover/tab:opacity-100 ${
                      activeCollection === col.id
                        ? 'text-background/80 hover:text-background'
                        : 'text-muted-foreground hover:text-foreground'
                    }`}
                    onClick={(e) => e.stopPropagation()}
                  >
                    <MoreHorizontal className="h-3.5 w-3.5" />
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start" className="w-44">
                  <DropdownMenuItem onClick={() => setEditingCollection(col)}>
                    <Pencil className="h-3.5 w-3.5" />
                    Edit collection
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    onClick={() => setConfirmDelete({ type: 'collection', id: col.id, name: col.name })}
                    className="text-destructive focus:text-destructive"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                    Delete
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            )}
          </div>
        ))}

        {uncollected.length > 0 && (
          <button
            type="button"
            onClick={() => setActiveCollection('__uncollected__')}
            className={`rounded-full px-3 py-1 text-xs font-medium transition-colors ${
              activeCollection === '__uncollected__'
                ? 'bg-foreground text-background'
                : 'bg-muted/60 text-muted-foreground hover:bg-muted border border-border/40'
            }`}
          >
            Uncategorized ({uncollected.length})
          </button>
        )}

        {/* Add collection */}
        {canEditDocs && (
          <QuickTooltip label="Add collection">
            <button
              type="button"
              onClick={() => openCreate('docs_collection', { spaceId })}
              className="flex items-center gap-1 rounded-full border border-dashed border-primary/40 px-2.5 py-1 text-xs font-medium text-primary/70 transition-colors hover:border-primary hover:text-primary hover:bg-primary/5"
            >
              <Plus className="h-3 w-3" />
              Collection
            </button>
          </QuickTooltip>
        )}
      </div>}

      {/* Filters row */}
      {showStatusFilter && <div className="flex items-center justify-end gap-2">
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
              <ListFilter className="h-3 w-3" />
              {filterStatus ? DOC_STATUS_LABELS[filterStatus] : 'Status'}
              <ChevronDown className="h-3 w-3 opacity-50" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-40">
            <DropdownMenuItem
              onClick={() => setFilterStatus(null)}
              className={!filterStatus ? 'font-medium' : ''}
            >
              All
              {!filterStatus && <Check className="ml-auto h-3.5 w-3.5" />}
            </DropdownMenuItem>
            {(Object.entries(DOC_STATUS_LABELS) as [DocStatus, string][]).map(([key, label]) => (
              <DropdownMenuItem
                key={key}
                onClick={() => setFilterStatus(key)}
                className={filterStatus === key ? 'font-medium' : ''}
              >
                {label}
                {filterStatus === key && <Check className="ml-auto h-3.5 w-3.5" />}
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>}

      {/* Document list */}
      {(() => {
        const baseDocs = activeCollectionDocuments

        const filtered = baseDocs

        // Sort
        const displayDocs = [...filtered].sort((a, b) => {
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

        if (displayDocs.length === 0) {
          const hasCollections = (collections ?? []).length > 0
          return (
            <div className="flex flex-col items-center justify-center py-12 px-4 max-w-md mx-auto">
              {hasCollections ? (
                <>
                  <FileText className="h-10 w-10 text-muted-foreground/30 mb-3" />
                  <p className="text-sm text-muted-foreground">No documents found.</p>
                  {canEditDocs && (
                    <button
                      type="button"
                      onClick={() => openCreate('docs_document', {
                        spaceId,
                        collectionId: activeCollection && activeCollection !== '__uncollected__' ? activeCollection : undefined,
                      })}
                      className="mt-4 inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-background px-3 py-1.5 text-sm font-medium text-foreground shadow-sm transition-colors hover:bg-muted"
                    >
                      <Plus className="h-3.5 w-3.5" />
                      Create a document
                    </button>
                  )}
                </>
              ) : (
                <>
                  <FolderOpen className="h-10 w-10 text-muted-foreground/30 mb-3" />
                  <p className="text-sm font-medium">No collections yet</p>
                  <p className="mt-1.5 text-center text-xs text-muted-foreground leading-relaxed">
                    Collections help you organize documents into groups — like topics, categories, or projects. Create your first collection to start adding documents.
                  </p>
                  {canEditDocs && (
                    <button
                      type="button"
                      onClick={() => openCreate('docs_collection', { spaceId })}
                      className="mt-4 inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-background px-3 py-1.5 text-sm font-medium text-foreground shadow-sm transition-colors hover:bg-muted"
                    >
                      <Plus className="h-3.5 w-3.5" />
                      Create a collection
                    </button>
                  )}
                </>
              )}
            </div>
          )
        }

        return (
          <div className="rounded-lg border border-border/60 bg-card divide-y divide-border/40">
            <div className="flex items-center gap-3 px-4 py-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
              <button type="button" onClick={() => { if (sortField === 'title') { setSortDir(d => d === 'asc' ? 'desc' : 'asc') } else { setSortField('title'); setSortDir('asc') } }} className="min-w-0 flex-1 flex items-center gap-1 hover:text-foreground transition-colors text-left">
                Title
                {sortField === 'title' && (sortDir === 'asc' ? <ArrowUp className="h-3 w-3" /> : <ArrowDown className="h-3 w-3" />)}
              </button>
              <span className="w-36 shrink-0">Owner</span>
              <span className="w-28 shrink-0">Collection</span>
              <button type="button" onClick={() => { if (sortField === 'status') { setSortDir(d => d === 'asc' ? 'desc' : 'asc') } else { setSortField('status'); setSortDir('asc') } }} className="w-20 shrink-0 flex items-center gap-1 hover:text-foreground transition-colors">
                Status
                {sortField === 'status' && (sortDir === 'asc' ? <ArrowUp className="h-3 w-3" /> : <ArrowDown className="h-3 w-3" />)}
              </button>
              <button type="button" onClick={() => { if (sortField === 'updated_at') { setSortDir(d => d === 'asc' ? 'desc' : 'asc') } else { setSortField('updated_at'); setSortDir('desc') } }} className="w-20 shrink-0 flex items-center gap-1 justify-end hover:text-foreground transition-colors">
                Updated
                {sortField === 'updated_at' && (sortDir === 'asc' ? <ArrowUp className="h-3 w-3" /> : <ArrowDown className="h-3 w-3" />)}
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
                  <FileText className="h-4 w-4 shrink-0 text-muted-foreground" />
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
                {canEditDocs && (
                  <span className="w-8 shrink-0">
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <button
                          type="button"
                          className="rounded p-1 text-foreground/50 opacity-0 transition-opacity hover:text-foreground group-hover/row:opacity-100"
                          onClick={(e) => e.stopPropagation()}
                        >
                          <MoreHorizontal className="h-4 w-4" />
                        </button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end" className="w-40">
                        <DropdownMenuItem
                          disabled={duplicatingDocId === doc.id}
                          onClick={() => void handleDuplicateDoc(doc)}
                        >
                          <Copy className="h-3.5 w-3.5" />
                          {duplicatingDocId === doc.id ? 'Duplicating...' : 'Duplicate'}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          onClick={() => setMovingDoc(doc)}
                        >
                          <FolderInput className="h-3.5 w-3.5" />
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
                            <Send className="h-3.5 w-3.5" />
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
                            <ArchiveRestore className="h-3.5 w-3.5" />
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
                            <Archive className="h-3.5 w-3.5" />
                            Archive
                          </DropdownMenuItem>
                        )}
                        <DropdownMenuSeparator />
                        <DropdownMenuItem
                          className="text-destructive focus:text-destructive"
                          onClick={() => setDeleteConfirmDoc(doc)}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
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
        )
      })()}

      <Sheet open={translationsOpen} onOpenChange={setTranslationsOpen}>
        <SheetContent className="w-full sm:max-w-2xl">
          <SheetHeader className="border-b border-border/60">
            <SheetTitle className="flex items-center gap-2">
              <Languages className="h-4 w-4 text-primary" />
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
                publishSpaceTranslation.mutate(locale, {
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
                    publishCollectionTranslation.mutate(locale, {
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
