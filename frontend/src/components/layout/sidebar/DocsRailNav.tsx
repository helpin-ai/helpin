import { Fragment, useEffect, useMemo, useState } from 'react'
import { useLocation } from '@tanstack/react-router'
import {
  ArrowDown01Icon,
  ArrowRight01Icon,
  Clock01Icon,
  Delete01Icon,
  File01Icon,
  FolderOpenIcon,
  InboxIcon,
  MoreVerticalIcon,
  PencilEdit01Icon,
  PlusSignIcon,
  Search01Icon,
  StarIcon,
  Tick01Icon,
  UserIcon,
} from '@/lib/icons'
import { cn, getInitials } from '@/lib/utils'
import { StoredIcon } from '@/components/ui/icon-picker'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  useDeleteDocsCollection,
  useDeleteDocsSpace,
  useDocsCollections,
  useDocsDocument,
  useDocsDocuments,
  useDocsSpaces,
} from '@/hooks/queries'
import type { DocsCollection, DocsDocument, DocsSpace } from '@/lib/docsTypes'
import { SpaceDialog } from '@/components/docs/SpaceDialog'
import { CreateCollectionDialog } from '@/components/docs/CreateCollectionDialog'
import { DeleteCollectionDialog } from '@/components/docs/DeleteCollectionDialog'
import { DeleteSpaceDialog } from '@/components/docs/DeleteSpaceDialog'
import { useSearchCommandStore } from '@/stores/searchCommandStore'
import type { SidebarNavigateTarget } from './navigation'
import {
  buildDocsSidebarTree,
  findCollectionAncestry,
  getStoredDocsCollectionId,
  getStoredDocsSpaceId,
  getStoredExpandedCollections,
  resolveDocsSidebarSpaceId,
  saveStoredDocsCollectionId,
  saveStoredDocsSpaceId,
  saveStoredExpandedCollections,
  type DocsSidebarCollectionNode,
} from './docsSidebarState'

type OpenDocsCreate = (
  modal: 'docs_document' | 'docs_collection' | 'docs_space',
  options?: { spaceId?: string; collectionId?: string; parentCollectionId?: string },
) => void

type DocsRailNavProps = {
  wsId: string
  wsSlug: string
  canEditDocs: boolean
  isActive: (link: string) => boolean
  openCreate: OpenDocsCreate
  onNavigate: (target: SidebarNavigateTarget) => void
}

function SpaceMark({ space, size = 'md' }: { space: DocsSpace; size?: 'sm' | 'md' }) {
  return (
    <span
      className={cn(
        'flex shrink-0 items-center justify-center overflow-hidden bg-foreground font-semibold text-background',
        size === 'md' ? 'h-7 w-7 rounded-lg text-xs' : 'h-6 w-6 rounded-md text-[11px]',
      )}
    >
      <StoredIcon
        name={space.icon}
        className={size === 'md' ? 'h-4 w-4' : 'h-3.5 w-3.5'}
        textClassName="leading-none"
        fallback={<span>{getInitials(space.name).slice(0, 2)}</span>}
      />
    </span>
  )
}

function SpacePickerRow({
  space,
  wsId,
  active,
  highlighted,
  canEditDocs,
  onSelect,
  onEdit,
  onDelete,
}: {
  space: DocsSpace
  wsId: string
  active: boolean
  highlighted: boolean
  canEditDocs: boolean
  onSelect: () => void
  onEdit: () => void
  onDelete: () => void
}) {
  const { data: documents = [] } = useDocsDocuments(wsId, { space_id: space.id })

  return (
    <div
      role="option"
      aria-selected={active}
      id={`docs-space-option-${space.id}`}
      className={cn(
        'group/space-option flex min-h-10 items-center gap-2 rounded-lg px-2 py-1.5 transition-colors hover:bg-muted',
        (active || highlighted) && 'bg-muted',
      )}
    >
      <button type="button" className="flex min-w-0 flex-1 items-center gap-2 text-left" onClick={onSelect}>
        <SpaceMark space={space} size="sm" />
        <span className="min-w-0 flex-1">
          <span className="block truncate text-[13px] font-medium">{space.name}</span>
          <span className="block truncate text-[11px] text-muted-foreground">
            {documents.length} {documents.length === 1 ? 'doc' : 'docs'} · {space.type === 'external_capable' ? 'external' : 'team'}
          </span>
        </span>
      </button>
      {active && <Tick01Icon className="h-3.5 w-3.5 shrink-0" />}
      {canEditDocs && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              aria-label={`Manage ${space.name}`}
              className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md opacity-0 transition-opacity hover:bg-background group-hover/space-option:opacity-100 data-[state=open]:opacity-100"
            >
              <MoreVerticalIcon className="h-3.5 w-3.5" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent side="right" align="start">
            <DropdownMenuItem onClick={onEdit}>
              <PencilEdit01Icon className="h-4 w-4" />
              Edit space
            </DropdownMenuItem>
            <DropdownMenuItem onClick={onDelete} className="text-destructive focus:text-destructive">
              <Delete01Icon className="h-4 w-4" />
              Delete space
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  )
}

function DocumentRow({
  document,
  active,
  pinned = false,
  onOpen,
}: {
  document: DocsDocument
  active: boolean
  pinned?: boolean
  onOpen: () => void
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          data-slot="docs-document-row"
          data-pinned={pinned || undefined}
          aria-current={active ? 'page' : undefined}
          onClick={onOpen}
          className={cn(
            'flex min-h-8 w-full min-w-0 items-center gap-2 rounded-md px-2 py-1.5 text-left text-[13px] text-sidebar-foreground/80 transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
            active && 'bg-sidebar-accent font-medium text-sidebar-accent-foreground',
          )}
        >
          {pinned && (
            <StarIcon className="h-3 w-3 shrink-0 text-amber-500" />
          )}
          <span className="truncate">{document.title || 'Untitled'}</span>
        </button>
      </TooltipTrigger>
      <TooltipContent side="right">{document.title || 'Untitled'}</TooltipContent>
    </Tooltip>
  )
}

function CollectionBranch({
  node,
  expanded,
  activeDocumentId,
  activeCollectionId,
  canEditDocs,
  onToggle,
  onOpenCollection,
  onOpenDocument,
  onCreateDocument,
  onEditCollection,
  onDeleteCollection,
}: {
  node: DocsSidebarCollectionNode
  expanded: Set<string>
  activeDocumentId: string
  activeCollectionId: string
  canEditDocs: boolean
  onToggle: (collectionId: string) => void
  onOpenCollection: (collection: DocsCollection) => void
  onOpenDocument: (document: DocsDocument) => void
  onCreateDocument: (collectionId: string) => void
  onEditCollection: (collection: DocsCollection) => void
  onDeleteCollection: (collection: DocsCollection) => void
}) {
  const isExpanded = expanded.has(node.collection.id)
  const isActive = activeCollectionId === node.collection.id
  const directCount = node.documents.length

  return (
    <div>
      <div data-slot="docs-tree-row" className="group/collection relative flex min-h-8 items-center">
        <button
          type="button"
          data-slot="docs-tree-disclosure"
          aria-label={`${isExpanded ? 'Collapse' : 'Expand'} ${node.collection.name}`}
          aria-expanded={isExpanded}
          onClick={() => onToggle(node.collection.id)}
          className="absolute -left-2 z-10 flex h-8 w-2 items-center justify-start text-muted-foreground transition-colors before:absolute before:inset-y-0 before:left-0 before:w-4 hover:text-sidebar-accent-foreground"
        >
          <ArrowRight01Icon
            className={cn(
              'h-2.5 w-2.5 max-w-none shrink-0 transition-transform',
              isExpanded && 'rotate-90',
            )}
          />
        </button>
        <button
          type="button"
          data-slot="docs-tree-item"
          aria-current={isActive ? 'location' : undefined}
          onClick={() => onOpenCollection(node.collection)}
          className={cn(
            'flex min-w-0 flex-1 items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
            isActive && 'bg-sidebar-accent text-sidebar-accent-foreground',
          )}
        >
          <StoredIcon
            name={node.collection.icon}
            className="h-3.5 w-3.5 shrink-0"
            fallback={<FolderOpenIcon className="h-3.5 w-3.5 shrink-0" />}
          />
          <span className="truncate text-[13px] font-medium">{node.collection.name}</span>
          <span className="ml-auto w-6 shrink-0 text-right text-[11px] tabular-nums text-muted-foreground">{directCount}</span>
        </button>
        {canEditDocs && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                aria-label={`Manage ${node.collection.name}`}
                className="absolute inset-y-0 right-0 flex w-8 items-center justify-center rounded-md bg-gradient-to-l from-sidebar from-65% to-transparent opacity-0 transition-opacity group-hover/collection:opacity-100 data-[state=open]:opacity-100"
                onClick={(event) => event.stopPropagation()}
              >
                <MoreVerticalIcon className="h-3.5 w-3.5" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent side="right" align="start">
              <DropdownMenuItem onClick={() => onEditCollection(node.collection)}>
                <PencilEdit01Icon className="h-4 w-4" />
                Edit collection
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => onDeleteCollection(node.collection)}
                className="text-destructive focus:text-destructive"
              >
                <Delete01Icon className="h-4 w-4" />
                Delete collection
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>

      {isExpanded && (
        <div className="ml-4 border-l border-sidebar-border/80 pl-2">
          {node.documents.map((document) => (
            <DocumentRow
              key={document.id}
              document={document}
              active={document.id === activeDocumentId}
              onOpen={() => onOpenDocument(document)}
            />
          ))}
          {node.children.map((child) => (
            <CollectionBranch
              key={child.collection.id}
              node={child}
              expanded={expanded}
              activeDocumentId={activeDocumentId}
              activeCollectionId={activeCollectionId}
              canEditDocs={canEditDocs}
              onToggle={onToggle}
              onOpenCollection={onOpenCollection}
              onOpenDocument={onOpenDocument}
              onCreateDocument={onCreateDocument}
              onEditCollection={onEditCollection}
              onDeleteCollection={onDeleteCollection}
            />
          ))}
          {canEditDocs && (
            <button
              type="button"
              onClick={() => onCreateDocument(node.collection.id)}
              className="flex min-h-8 w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
            >
              <PlusSignIcon className="h-3 w-3" />
              New document
            </button>
          )}
        </div>
      )}
    </div>
  )
}

export function DocsRailNav({
  wsId,
  wsSlug,
  canEditDocs,
  isActive,
  openCreate,
  onNavigate,
}: DocsRailNavProps) {
  const location = useLocation()
  const { data: spaces = [], isLoading: spacesLoading } = useDocsSpaces(wsId)
  const activeDocumentId = location.pathname.match(/\/docs\/documents\/([^/]+)/)?.[1] ?? ''
  const routeSpaceId = location.pathname.match(/\/docs\/spaces\/([^/]+)/)?.[1] ?? ''
  const { data: activeDocument } = useDocsDocument(wsId, activeDocumentId)
  const [selectedSpaceId, setSelectedSpaceId] = useState<string | null>(() => getStoredDocsSpaceId(wsId))
  const [switcherOpen, setSwitcherOpen] = useState(false)
  const [spaceQuery, setSpaceQuery] = useState('')
  const [highlightedSpaceIndex, setHighlightedSpaceIndex] = useState(0)
  const [expansionOverrides, setExpansionOverrides] = useState<Record<string, Record<string, boolean>>>({})
  const [editingSpace, setEditingSpace] = useState<DocsSpace | null>(null)
  const [deletingSpace, setDeletingSpace] = useState<DocsSpace | null>(null)
  const [editingCollection, setEditingCollection] = useState<DocsCollection | null>(null)
  const [deletingCollection, setDeletingCollection] = useState<DocsCollection | null>(null)
  const openSearch = useSearchCommandStore((state) => state.openSearch)
  const deleteSpace = useDeleteDocsSpace(wsId)
  const deleteCollection = useDeleteDocsCollection(wsId)

  const activeSpaceId = resolveDocsSidebarSpaceId({
    spaces,
    routeSpaceId,
    documentSpaceId: activeDocument?.space_id,
    selectedSpaceId,
    storedSpaceId: getStoredDocsSpaceId(wsId),
  })
  const activeSpace = spaces.find((space) => space.id === activeSpaceId)
  const { data: collections = [], isLoading: collectionsLoading } = useDocsCollections(wsId, activeSpaceId)
  const { data: documents = [], isLoading: documentsLoading } = useDocsDocuments(
    wsId,
    { space_id: activeSpaceId },
    { enabled: !!activeSpaceId },
  )

  useEffect(() => {
    if (activeSpaceId) saveStoredDocsSpaceId(wsId, activeSpaceId)
  }, [activeSpaceId, wsId])

  const search = location.search as Record<string, string | undefined>
  const routeCollectionId = search.collection && search.collection !== '__uncollected__'
    ? search.collection
    : activeDocument?.collection_id
  useEffect(() => {
    if (activeSpaceId && routeCollectionId) {
      saveStoredDocsCollectionId(wsId, activeSpaceId, routeCollectionId)
    }
  }, [activeSpaceId, routeCollectionId, wsId])

  const tree = useMemo(
    () => buildDocsSidebarTree(collections, documents),
    [collections, documents],
  )
  const focusedExpansionIds = useMemo(
    () => search.collection === '__uncollected__'
      ? ['__uncategorized__']
      : findCollectionAncestry(collections, routeCollectionId),
    [collections, routeCollectionId, search.collection],
  )
  const focusedExpansionKey = focusedExpansionIds.join(':')
  const expansionContextKey = `${activeSpaceId}:${focusedExpansionKey}`
  const expanded = getStoredExpandedCollections(wsId, activeSpaceId)
  for (const collectionId of focusedExpansionIds) expanded.add(collectionId)
  for (const [collectionId, isExpanded] of Object.entries(expansionOverrides[expansionContextKey] ?? {})) {
    if (isExpanded) expanded.add(collectionId)
    else expanded.delete(collectionId)
  }
  const pinnedDocuments = documents.filter((document) => document.is_pinned)
  const uncollectedDocuments = documents
    .filter((document) => !document.collection_id)
    .sort((a, b) => a.position - b.position || a.title.localeCompare(b.title))
  const uncategorizedExpanded = expanded.has('__uncategorized__')

  const filteredSpaces = spaces.filter((space) =>
    space.name.toLowerCase().includes(spaceQuery.trim().toLowerCase()),
  )
  const internalSpaces = filteredSpaces.filter((space) => space.type === 'internal')
  const externalSpaces = filteredSpaces.filter((space) => space.type === 'external_capable')
  const orderedFilteredSpaces = [...internalSpaces, ...externalSpaces]

  const navigateToSpace = (spaceId: string) => {
    setSelectedSpaceId(spaceId)
    setSwitcherOpen(false)
    onNavigate({
      to: '/w/$slug/docs/spaces/$spaceId',
      params: { slug: wsSlug, spaceId },
    })
  }

  const navigateToDocument = (document: DocsDocument) => {
    if (document.collection_id) {
      saveStoredDocsCollectionId(wsId, document.space_id, document.collection_id)
    }
    onNavigate({
      to: '/w/$slug/docs/documents/$docId',
      params: { slug: wsSlug, docId: document.id },
    })
  }

  const navigateToCollection = (collection: DocsCollection) => {
    saveStoredDocsCollectionId(wsId, collection.space_id, collection.id)
    onNavigate({
      to: '/w/$slug/docs/spaces/$spaceId',
      params: { slug: wsSlug, spaceId: collection.space_id },
      search: { collection: collection.id },
    })
  }

  const toggleCollection = (collectionId: string) => {
    const nextIsExpanded = !expanded.has(collectionId)
    const next = new Set(expanded)
    if (nextIsExpanded) next.add(collectionId)
    else next.delete(collectionId)
    saveStoredExpandedCollections(wsId, activeSpaceId, next)
    setExpansionOverrides((current) => ({
      ...current,
      [expansionContextKey]: {
        ...current[expansionContextKey],
        [collectionId]: nextIsExpanded,
      },
    }))
  }

  const validCollectionIds = new Set(collections.map((collection) => collection.id))
  const storedCollectionId = getStoredDocsCollectionId(wsId, activeSpaceId)
  const defaultCollectionId = routeCollectionId && validCollectionIds.has(routeCollectionId)
    ? routeCollectionId
    : storedCollectionId && validCollectionIds.has(storedCollectionId)
      ? storedCollectionId
      : collections[0]?.id

  if (spacesLoading) {
    return (
      <div className="flex min-h-0 flex-1 flex-col gap-2 p-1">
        <Skeleton className="h-12 rounded-lg" />
        <Skeleton className="h-8 rounded-md" />
        <div className="space-y-1 pt-3">
          {[1, 2, 3, 4].map((row) => <Skeleton key={row} className="h-8 rounded-md" />)}
        </div>
      </div>
    )
  }

  if (!activeSpace) {
    return (
      <div className="flex min-h-0 flex-1 flex-col p-2">
        <div className="rounded-lg border border-dashed p-3 text-center">
          <p className="text-sm font-medium">No documentation spaces</p>
          <p className="mt-1 text-xs text-muted-foreground">Create a space to organize your documents.</p>
          {canEditDocs && (
            <Button size="sm" className="mt-3 h-8" onClick={() => openCreate('docs_space')}>
              <PlusSignIcon className="h-3.5 w-3.5" />
              New space
            </Button>
          )}
        </div>
      </div>
    )
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="px-1 pb-2">
        <Popover
          open={switcherOpen}
          onOpenChange={(open) => {
            setSwitcherOpen(open)
            if (open) {
              setSpaceQuery('')
              setHighlightedSpaceIndex(0)
            }
          }}
        >
          <PopoverTrigger asChild>
            <button
              type="button"
              aria-haspopup="listbox"
              aria-expanded={switcherOpen}
              className="flex min-h-13 w-full items-center gap-2 rounded-lg px-2 py-2 text-left transition-colors hover:bg-sidebar-accent"
            >
              <SpaceMark space={activeSpace} />
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-semibold tracking-tight">{activeSpace.name}</span>
                <span className="block truncate text-[11px] text-muted-foreground">
                  {documents.length} {documents.length === 1 ? 'doc' : 'docs'} · {activeSpace.type === 'external_capable' ? 'external' : 'team'}
                </span>
              </span>
              <ArrowDown01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            </button>
          </PopoverTrigger>
          <PopoverContent align="start" sideOffset={-52} className="w-72 gap-1 p-2 shadow-xl">
            <Input
              autoFocus
              role="combobox"
              aria-controls="docs-space-options"
              aria-expanded={switcherOpen}
              aria-activedescendant={orderedFilteredSpaces[highlightedSpaceIndex]
                ? `docs-space-option-${orderedFilteredSpaces[highlightedSpaceIndex].id}`
                : undefined}
              value={spaceQuery}
              onChange={(event) => {
                setSpaceQuery(event.target.value)
                setHighlightedSpaceIndex(0)
              }}
              onKeyDown={(event) => {
                if (event.key === 'ArrowDown') {
                  event.preventDefault()
                  setHighlightedSpaceIndex((index) => Math.min(index + 1, Math.max(orderedFilteredSpaces.length - 1, 0)))
                } else if (event.key === 'ArrowUp') {
                  event.preventDefault()
                  setHighlightedSpaceIndex((index) => Math.max(index - 1, 0))
                } else if (event.key === 'Enter') {
                  const highlighted = orderedFilteredSpaces[highlightedSpaceIndex]
                  if (highlighted) {
                    event.preventDefault()
                    navigateToSpace(highlighted.id)
                  }
                }
              }}
              placeholder="Find a space…"
              className="mb-1 h-8 border-0 bg-muted/70 text-sm shadow-none focus-visible:ring-1"
            />
            <div id="docs-space-options" role="listbox" aria-label="Documentation spaces" className="max-h-72 overflow-y-auto">
              {[
                ['Team spaces', internalSpaces],
                ['External spaces', externalSpaces],
              ].map(([label, group]) => {
                const groupSpaces = group as DocsSpace[]
                if (groupSpaces.length === 0) return null
                return (
                  <Fragment key={label as string}>
                    <div className="px-2 pb-1 pt-2 text-[10px] uppercase tracking-[0.12em] text-muted-foreground">
                      {label as string}
                    </div>
                    {groupSpaces.map((space) => (
                      <SpacePickerRow
                        key={space.id}
                        space={space}
                        wsId={wsId}
                        active={space.id === activeSpaceId}
                        highlighted={orderedFilteredSpaces[highlightedSpaceIndex]?.id === space.id}
                        canEditDocs={canEditDocs}
                        onSelect={() => navigateToSpace(space.id)}
                        onEdit={() => { setSwitcherOpen(false); setEditingSpace(space) }}
                        onDelete={() => { setSwitcherOpen(false); setDeletingSpace(space) }}
                      />
                    ))}
                  </Fragment>
                )
              })}
              {internalSpaces.length === 0 && externalSpaces.length === 0 && (
                <p className="px-2 py-6 text-center text-sm text-muted-foreground">No spaces found.</p>
              )}
            </div>
            {canEditDocs && (
              <>
                <div className="mx-1 my-1 h-px bg-border/70" />
                <button
                  type="button"
                  className="flex min-h-9 w-full items-center gap-2 rounded-lg px-2 text-left text-[13px] transition-colors hover:bg-muted"
                  onClick={() => { setSwitcherOpen(false); openCreate('docs_space') }}
                >
                  <span className="flex h-6 w-6 items-center justify-center"><PlusSignIcon className="h-3.5 w-3.5" /></span>
                  New space
                </button>
              </>
            )}
          </PopoverContent>
        </Popover>
      </div>

      {canEditDocs && (
        <div className="flex px-2 pb-3">
          <Button
            size="sm"
            className="h-8 flex-1 rounded-r-none text-xs"
            onClick={() => openCreate('docs_document', { spaceId: activeSpaceId, collectionId: defaultCollectionId })}
          >
            <PlusSignIcon className="h-3.5 w-3.5" />
            New document
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button size="sm" className="h-8 rounded-l-none border-l border-primary-foreground/20 px-2" aria-label="More create options">
                <ArrowDown01Icon className="h-3 w-3" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              <DropdownMenuItem onClick={() => openCreate('docs_document', { spaceId: activeSpaceId })}>
                <File01Icon className="h-4 w-4" />
                New document in…
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => openCreate('docs_collection', { spaceId: activeSpaceId })}>
                <FolderOpenIcon className="h-4 w-4" />
                New collection
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => openCreate('docs_space')}>
                <PlusSignIcon className="h-4 w-4" />
                New space
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      )}

      <nav data-slot="docs-quick-links" aria-label="Docs views" className="space-y-0.5 px-2 pb-2">
        {[
          { label: 'Recent docs', icon: Clock01Icon, link: `/w/${wsSlug}/docs/recent` },
          { label: 'My documents', icon: UserIcon, link: `/w/${wsSlug}/docs/my` },
        ].map((item) => (
          <button
            key={item.link}
            type="button"
            onClick={() => onNavigate(item.link)}
            className={cn(
              'flex min-h-8 w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[13px] transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
              isActive(item.link) && 'bg-sidebar-accent font-medium text-sidebar-accent-foreground',
            )}
          >
            <item.icon className="h-3.5 w-3.5 text-muted-foreground" />
            <span className="truncate">{item.label}</span>
          </button>
        ))}
        <button
          type="button"
          onClick={() => navigateToSpace(activeSpaceId)}
          className={cn(
            'flex min-h-8 w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[13px] transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
            isActive(`/w/${wsSlug}/docs/spaces/${activeSpaceId}`) && 'bg-sidebar-accent font-medium text-sidebar-accent-foreground',
          )}
        >
          <File01Icon className="h-3.5 w-3.5 text-muted-foreground" />
          <span className="truncate">All docs in space</span>
          <span className="ml-auto w-6 shrink-0 text-right text-[11px] tabular-nums text-muted-foreground">{documents.length}</span>
        </button>
      </nav>

      <div className="mx-3 h-px bg-sidebar-border/80" />

      <div data-slot="docs-tree-scroll" className="no-scrollbar min-h-0 flex-1 overflow-y-auto px-2 pb-3 pt-2">
        <div className="flex min-h-7 items-center justify-between px-2 text-[10px] uppercase tracking-[0.12em] text-muted-foreground">
          <span>In this space</span>
          {canEditDocs && (
            <button
              type="button"
              aria-label="New collection"
              className="flex h-6 w-6 items-center justify-center rounded-md hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
              onClick={() => openCreate('docs_collection', { spaceId: activeSpaceId })}
            >
              <PlusSignIcon className="h-3.5 w-3.5" />
            </button>
          )}
        </div>

        {pinnedDocuments.length > 0 && (
          <div className="mb-2 space-y-0.5">
            {pinnedDocuments.map((document) => (
              <DocumentRow
                key={`pinned-${document.id}`}
                document={document}
                pinned
                active={document.id === activeDocumentId}
                onOpen={() => navigateToDocument(document)}
              />
            ))}
          </div>
        )}

        {collectionsLoading || documentsLoading ? (
          <div className="space-y-1">
            {[1, 2, 3, 4].map((row) => <Skeleton key={row} className="h-8 rounded-md" />)}
          </div>
        ) : (
          <div className="space-y-0.5">
            {tree.map((node) => (
              <CollectionBranch
                key={node.collection.id}
                node={node}
                expanded={expanded}
                activeDocumentId={activeDocumentId}
                activeCollectionId={location.pathname.includes('/docs/spaces/') && search.collection !== '__uncollected__' ? (search.collection ?? '') : ''}
                canEditDocs={canEditDocs}
                onToggle={toggleCollection}
                onOpenCollection={navigateToCollection}
                onOpenDocument={navigateToDocument}
                onCreateDocument={(collectionId) => openCreate('docs_document', { spaceId: activeSpaceId, collectionId })}
                onEditCollection={setEditingCollection}
                onDeleteCollection={setDeletingCollection}
              />
            ))}

            {uncollectedDocuments.length > 0 && (
              <div>
                <div data-slot="docs-tree-row" className="relative flex min-h-8 items-center">
                  <button
                    type="button"
                    data-slot="docs-tree-disclosure"
                    aria-label={`${uncategorizedExpanded ? 'Collapse' : 'Expand'} Uncategorized`}
                    aria-expanded={uncategorizedExpanded}
                    onClick={() => toggleCollection('__uncategorized__')}
                    className="absolute -left-2 z-10 flex h-8 w-2 items-center justify-start text-muted-foreground transition-colors before:absolute before:inset-y-0 before:left-0 before:w-4 hover:text-sidebar-accent-foreground"
                  >
                    <ArrowRight01Icon className={cn('h-2.5 w-2.5 max-w-none shrink-0 transition-transform', uncategorizedExpanded && 'rotate-90')} />
                  </button>
                  <button
                    type="button"
                    data-slot="docs-tree-item"
                    aria-current={location.pathname.includes('/docs/spaces/') && search.collection === '__uncollected__' ? 'location' : undefined}
                    onClick={() => onNavigate({
                      to: '/w/$slug/docs/spaces/$spaceId',
                      params: { slug: wsSlug, spaceId: activeSpaceId },
                      search: { collection: '__uncollected__' },
                    })}
                    className={cn(
                      'flex min-w-0 flex-1 items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
                      location.pathname.includes('/docs/spaces/') && search.collection === '__uncollected__' && 'bg-sidebar-accent text-sidebar-accent-foreground',
                    )}
                  >
                    <InboxIcon className="h-3.5 w-3.5" />
                    <span className="truncate text-[13px] font-medium">Uncategorized</span>
                    <span className="ml-auto w-6 shrink-0 text-right text-[11px] tabular-nums text-muted-foreground">{uncollectedDocuments.length}</span>
                  </button>
                </div>
                {uncategorizedExpanded && (
                  <div className="ml-4 border-l border-sidebar-border/80 pl-2">
                    {uncollectedDocuments.map((document) => (
                      <DocumentRow
                        key={document.id}
                        document={document}
                        active={document.id === activeDocumentId}
                        onOpen={() => navigateToDocument(document)}
                      />
                    ))}
                    {canEditDocs && (
                      <button
                        type="button"
                        onClick={() => openCreate('docs_document', { spaceId: activeSpaceId })}
                        className="flex min-h-8 w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
                      >
                        <PlusSignIcon className="h-3 w-3" />
                        New document
                      </button>
                    )}
                  </div>
                )}
              </div>
            )}

            {tree.length === 0 && uncollectedDocuments.length === 0 && (
              <p className="px-2 py-3 text-xs text-muted-foreground">No collections or documents yet.</p>
            )}
            {canEditDocs && (
              <button
                type="button"
                onClick={() => openCreate('docs_collection', { spaceId: activeSpaceId })}
                className="mt-1 flex min-h-8 w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
              >
                <PlusSignIcon className="h-3 w-3" />
                New collection
              </button>
            )}
          </div>
        )}
      </div>

      <button
        type="button"
        onClick={openSearch}
        className="flex min-h-11 items-center gap-2 border-t border-sidebar-border/80 px-3 text-left text-xs text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
      >
        <Search01Icon className="h-3.5 w-3.5" />
        <span className="min-w-0 flex-1 truncate">Search all spaces</span>
        <span className="text-[11px] text-muted-foreground/70">⌘K</span>
      </button>

      <SpaceDialog
        wsId={wsId}
        open={editingSpace !== null}
        onOpenChange={(open) => { if (!open) setEditingSpace(null) }}
        space={editingSpace}
      />
      <DeleteSpaceDialog
        wsId={wsId}
        space={deletingSpace}
        open={deletingSpace !== null}
        onOpenChange={(open) => { if (!open) setDeletingSpace(null) }}
        onConfirm={async () => {
          if (!deletingSpace) return
          await deleteSpace.mutateAsync(deletingSpace.id)
          setDeletingSpace(null)
          onNavigate({ to: '/w/$slug/docs', params: { slug: wsSlug } })
        }}
      />
      <CreateCollectionDialog
        wsId={wsId}
        open={editingCollection !== null}
        onOpenChange={(open) => { if (!open) setEditingCollection(null) }}
        collection={editingCollection}
      />
      <DeleteCollectionDialog
        wsId={wsId}
        collection={deletingCollection}
        open={deletingCollection !== null}
        onOpenChange={(open) => { if (!open) setDeletingCollection(null) }}
        onConfirm={async () => {
          if (!deletingCollection) return
          await deleteCollection.mutateAsync({ id: deletingCollection.id, spaceId: deletingCollection.space_id })
          setDeletingCollection(null)
        }}
      />
    </div>
  )
}
