import { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams, useLocation } from '@tanstack/react-router'
import { timeAgo } from '@/lib/utils'
import {
  ArrowLeft,
  Check,
  ChevronDown,
  FileText,
  FolderOpen,
  Globe,
  ListFilter,
  Lock,
  MoreHorizontal,
  Pencil,
  Plus,
  Trash2,
  X,
} from 'lucide-react'
import { toast } from 'sonner'
import { useTitle } from '@/hooks/useTitle'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import {
  useDocsSpace,
  useDocsCollections,
  useDocsDocuments,
  useCreateDocsCollection,
  useUpdateDocsCollection,
  useDeleteDocsCollection,
  useDeleteDocsSpace,
  useWorkspaceAccess,
  usePermissions,
} from '@/hooks/queries'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import type { DocsDocument, DocType, DocStatus } from '@/lib/docsTypes'
import { DOC_TYPE_LABELS, DOC_STATUS_LABELS } from '@/lib/docsTypes'
import { TypedConfirmDialog } from '@/components/docs/TypedConfirmDialog'

function statusVariant(status: string): 'default' | 'secondary' | 'outline' {
  switch (status) {
    case 'published':
      return 'default'
    case 'archived':
      return 'outline'
    default:
      return 'secondary'
  }
}

export function DocsSpaceDetail() {
  const navigate = useNavigate()
  const location = useLocation()
  const { spaceId } = useParams({ strict: false }) as { spaceId: string }
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id ?? ''
  const wsSlug = workspace?.slug ?? ''

  const { data: access } = useWorkspaceAccess(wsId)
  const { canEditDocs } = usePermissions(access)

  const { data: space, isLoading: spaceLoading } = useDocsSpace(wsId, spaceId)
  const { data: collections } = useDocsCollections(wsId, spaceId)
  const { data: documents } = useDocsDocuments(wsId, { space_id: spaceId })
  const [filterType, setFilterType] = useState<DocType | null>(null)
  const [filterStatus, setFilterStatus] = useState<DocStatus | null>(null)

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

  // Inline collection creation
  const [addingCollection, setAddingCollection] = useState(false)
  const [newCollName, setNewCollName] = useState('')
  const newCollRef = useRef<HTMLInputElement>(null)
  const createCollection = useCreateDocsCollection(wsId, spaceId)

  // Inline collection rename
  const [renamingId, setRenamingId] = useState<string | null>(null)
  const [renameValue, setRenameValue] = useState('')
  const renameRef = useRef<HTMLInputElement>(null)
  const updateCollection = useUpdateDocsCollection(wsId)
  const deleteCollection = useDeleteDocsCollection(wsId)
  const deleteSpace = useDeleteDocsSpace(wsId)

  // Typed confirm dialog state
  const [confirmDelete, setConfirmDelete] = useState<{
    type: 'collection' | 'space'
    id: string
    name: string
  } | null>(null)

  const startAddCollection = () => {
    setAddingCollection(true)
    setNewCollName('')
    requestAnimationFrame(() => newCollRef.current?.focus())
  }

  const commitAddCollection = async () => {
    const name = newCollName.trim()
    setAddingCollection(false)
    if (!name) return
    try {
      await createCollection.mutateAsync({ name })
      toast.success(`Collection "${name}" created`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to create collection')
    }
  }

  const startRename = (id: string, currentName: string) => {
    setRenamingId(id)
    setRenameValue(currentName)
    requestAnimationFrame(() => renameRef.current?.select())
  }

  const commitRename = async () => {
    const id = renamingId
    setRenamingId(null)
    if (!id) return
    const name = renameValue.trim()
    if (!name) return
    const col = (collections ?? []).find((c) => c.id === id)
    if (col && name !== col.name) {
      try {
        await updateCollection.mutateAsync({ id, spaceId, name })
      } catch (err) {
        toast.error(err instanceof Error ? err.message : 'Failed to rename')
      }
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
              <span className="text-xl">{space.icon ?? '📁'}</span>
              <h2 className="text-xl font-semibold">{space.name}</h2>
              {space.type === 'external_capable' && (
                <Globe className="h-4 w-4 text-blue-500" title="External capable" />
              )}
              {space.restrict_to_owners && (
                <Lock className="h-4 w-4 text-amber-500" title="Restricted" />
              )}
            </div>
            <p className="mt-0.5 text-sm text-muted-foreground">
              {space.visibility === 'team_only' ? 'Team only' : 'Workspace wide'} ·{' '}
              {documents?.length ?? 0} documents
            </p>
          </div>
        </div>
        {canEditDocs && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0">
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-40">
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
      </header>

      {/* Collection tabs */}
      <div className="flex items-center gap-1 overflow-x-auto pb-1">
        <button
          type="button"
          onClick={() => { setActiveCollection(null);  }}
          className={`shrink-0 rounded-full px-3 py-1 text-xs font-medium transition-colors ${
            !activeCollection              ? 'bg-foreground text-background'
              : 'bg-muted/60 text-muted-foreground hover:bg-muted'
          }`}
        >
          All ({documents?.length ?? 0})
        </button>

        {(collections ?? []).map((col) => (
          <div key={col.id} className="group/tab relative flex shrink-0 items-center">
            {renamingId === col.id ? (
              <input
                ref={renameRef}
                value={renameValue}
                onChange={(e) => setRenameValue(e.target.value)}
                onBlur={commitRename}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') commitRename()
                  if (e.key === 'Escape') setRenamingId(null)
                }}
                className="h-7 w-32 rounded-full border border-primary/40 bg-background px-3 text-xs font-medium outline-none"
              />
            ) : (
              <button
                type="button"
                onClick={() => { setActiveCollection(col.id);  }}
                className={`shrink-0 rounded-full px-3 py-1 pr-7 text-xs font-medium transition-colors ${
                  activeCollection === col.id                    ? 'bg-foreground text-background'
                    : 'bg-muted/60 text-muted-foreground hover:bg-muted'
                }`}
              >
                <FolderOpen className="mr-1 inline h-3 w-3" />
                {col.icon ? `${col.icon} ` : ''}{col.name} ({collectionMap.get(col.id)?.length ?? 0})
              </button>
            )}

            {canEditDocs && renamingId !== col.id && (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <button
                    type="button"
                    className={`absolute right-1 top-1/2 -translate-y-1/2 rounded-full p-0.5 opacity-0 transition-opacity group-hover/tab:opacity-100 ${
                      activeCollection === col.id
                        ? 'text-background/70 hover:text-background'
                        : 'text-muted-foreground/60 hover:text-foreground'
                    }`}
                    onClick={(e) => e.stopPropagation()}
                  >
                    <MoreHorizontal className="h-3.5 w-3.5" />
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start" className="w-36">
                  <DropdownMenuItem onClick={() => startRename(col.id, col.name)}>
                    <Pencil className="h-3.5 w-3.5" />
                    Rename
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

        {uncollected.length > 0 && (collections ?? []).length > 0 && (
          <button
            type="button"
            onClick={() => { setActiveCollection('__uncollected__');  }}
            className={`shrink-0 rounded-full px-3 py-1 text-xs font-medium transition-colors ${
              activeCollection === '__uncollected__'                ? 'bg-foreground text-background'
                : 'bg-muted/60 text-muted-foreground hover:bg-muted'
            }`}
          >
            Uncategorized ({uncollected.length})
          </button>
        )}

        {/* Inline add collection */}
        {canEditDocs && (
          addingCollection ? (
            <div className="flex shrink-0 items-center gap-1">
              <input
                ref={newCollRef}
                value={newCollName}
                onChange={(e) => setNewCollName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') commitAddCollection()
                  if (e.key === 'Escape') setAddingCollection(false)
                }}
                onBlur={commitAddCollection}
                placeholder="Collection name…"
                className="h-7 w-36 rounded-full border border-primary/40 bg-background px-3 text-xs font-medium outline-none placeholder:text-muted-foreground/50"
              />
            </div>
          ) : (
            <button
              type="button"
              onClick={startAddCollection}
              className="flex shrink-0 items-center gap-1 rounded-full bg-muted/40 px-2.5 py-1 text-xs font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
              title="Add collection"
            >
              <Plus className="h-3 w-3" />
              Collection
            </button>
          )
        )}

        {/* Type & Status filters (right-aligned) */}
        {(() => {
          const hasActiveFilters = filterType !== null || filterStatus !== null
          return (
            <div className="flex shrink-0 items-center gap-2 ml-auto pl-3">
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <button
                    type="button"
                    className={`inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1 text-xs font-medium transition-colors ${
                      filterType
                        ? 'border-primary/30 bg-primary/5 text-foreground'
                        : 'border-border/60 text-muted-foreground hover:bg-muted/40 hover:text-foreground'
                    }`}
                  >
                    <ListFilter className="h-3 w-3" />
                    {filterType ? DOC_TYPE_LABELS[filterType] : 'Type'}
                    <ChevronDown className="h-3 w-3 opacity-50" />
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-44">
                  <DropdownMenuItem
                    onClick={() => setFilterType(null)}
                    className={!filterType ? 'font-medium' : ''}
                  >
                    All Types
                    {!filterType && <Check className="ml-auto h-3.5 w-3.5" />}
                  </DropdownMenuItem>
                  {(Object.entries(DOC_TYPE_LABELS) as [DocType, string][]).map(([key, label]) => (
                    <DropdownMenuItem
                      key={key}
                      onClick={() => setFilterType(key)}
                      className={filterType === key ? 'font-medium' : ''}
                    >
                      {label}
                      {filterType === key && <Check className="ml-auto h-3.5 w-3.5" />}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>

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
                <DropdownMenuContent align="end" className="w-36">
                  <DropdownMenuItem
                    onClick={() => setFilterStatus(null)}
                    className={!filterStatus ? 'font-medium' : ''}
                  >
                    All Statuses
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

              {hasActiveFilters && (
                <button
                  type="button"
                  onClick={() => { setFilterType(null); setFilterStatus(null) }}
                  className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
                >
                  <X className="h-3 w-3" />
                  Clear
                </button>
              )}
            </div>
          )
        })()}
      </div>

      {/* Document list */}
      {(() => {
        const baseDocs = activeCollection === '__uncollected__'
          ? uncollected
          : activeCollection
            ? collectionMap.get(activeCollection) ?? []
            : documents ?? []

        // Apply type and status filters
        const displayDocs = baseDocs.filter((doc) => {
          if (filterType && doc.doc_type !== filterType) return false
          if (filterStatus && doc.status !== filterStatus) return false
          return true
        })

        if (displayDocs.length === 0) {
          return (
            <div className="flex flex-col items-center justify-center py-12 px-4">
              <FileText className="h-10 w-10 text-muted-foreground/30 mb-3" />
              <p className="text-sm text-muted-foreground">No documents found.</p>
            </div>
          )
        }

        return (
          <div className="rounded-lg border border-border/60 bg-card divide-y divide-border/40">
            <div className="flex items-center gap-3 px-4 py-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
              <span className="min-w-0 flex-1">Title</span>
              <span className="w-32 shrink-0">Collection</span>
              <span className="w-28 shrink-0">Type</span>
              <span className="w-20 shrink-0">Status</span>
              <span className="w-20 shrink-0 text-right">Updated</span>
            </div>
            {displayDocs.map((doc: DocsDocument) => (
              <button
                key={doc.id}
                type="button"
                onClick={() =>
                  navigate({
                    to: '/w/$slug/docs/documents/$docId',
                    params: { slug: wsSlug, docId: doc.id },
                  })
                }
                className="flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm transition-colors hover:bg-muted/40"
              >
                <FileText className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="min-w-0 flex-1 truncate font-medium">{doc.title}</span>
                <span className="w-32 shrink-0 truncate text-xs text-muted-foreground">
                  {doc.collection_id ? collectionNames.get(doc.collection_id) ?? '—' : '—'}
                </span>
                <span className="w-28 shrink-0 text-xs text-muted-foreground">
                  {DOC_TYPE_LABELS[doc.doc_type] ?? doc.doc_type}
                </span>
                <span className="w-20 shrink-0">
                  <Badge
                    variant={statusVariant(doc.status)}
                    className="text-[10px] px-1.5 py-0"
                  >
                    {DOC_STATUS_LABELS[doc.status] ?? doc.status}
                  </Badge>
                </span>
                <span className="w-20 shrink-0 text-right text-xs text-muted-foreground">
                  {timeAgo(doc.updated_at)}
                </span>
              </button>
            ))}
          </div>
        )
      })()}

      {/* Typed confirm delete dialog */}
      <TypedConfirmDialog
        open={confirmDelete !== null}
        onOpenChange={(open) => { if (!open) setConfirmDelete(null) }}
        title={confirmDelete?.type === 'space' ? 'Delete space' : 'Delete collection'}
        description={
          confirmDelete?.type === 'space'
            ? 'This will permanently delete this space and all its documents. This action cannot be undone.'
            : 'This will permanently delete this collection. Documents in this collection will become uncategorized.'
        }
        confirmText={confirmDelete?.name ?? ''}
        onConfirm={handleConfirmDelete}
      />
    </div>
  )
}
