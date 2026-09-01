import { useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { COLLECTION_ROW_CLASS, COUNT_BADGE_CLASS } from './docsTreeStyles'
import {
  ArrowUpDownIcon,
  BookOpen01Icon,
  Cancel01Icon,
  Tick01Icon,
  ArrowRight01Icon,
  HelpCircleIcon,
  Folder01Icon,
  GlobeIcon,
  Loading01Icon,
  PencilEdit02Icon,
  PlusSignIcon,
  Search01Icon,
} from '@/lib/icons'
import { StoredIcon } from '@/components/ui/icon-picker'
import { Collapsible } from 'radix-ui'
import { toast } from 'sonner'
import { useTitle } from '@/hooks/useTitle'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import {
  useDocsSpaces,
  useAllDocsCollections,
  useDocsDocuments,
  useCreateDocsSpace,
  useUpdateDocsDocument,
  useWorkspaceAccess,
  usePermissions,
} from '@/hooks/queries'
import { useWorkspaceSettings } from '@/hooks/queries/useSettings'
import { Button } from '@/components/ui/button'
import { QuickTooltip } from '@/components/ui/quick-tooltip'
import { CreateSpaceDialog } from '@/components/docs/CreateSpaceDialog'
import { DocsArrangeTree } from '@/components/docs/DocsArrangeTree'
import { DocsLibraryList, DocsLibraryRow } from '@/components/docs/DocsLibraryList'
import { buildCollectionPathLabels } from '@/components/docs/docsCollectionTree'
import type { DocsSpace, DocsCollection, DocsDocument, SpaceType } from '@/lib/docsTypes'
import { QuietPageHeader, QuietPageViewport, QuietSearchInput, QuietTextAction } from '@/components/design-system/quiet'

// ── Space templates for quick setup ─────────────────────────────────────────

interface SpaceTemplate {
  name: string
  slug: string
  icon: string
  description: string
  type: SpaceType
}

const SPACE_TEMPLATES: SpaceTemplate[] = [
  { name: 'Company Wiki', slug: 'company-wiki', icon: '📖', description: 'General knowledge base for the whole team', type: 'internal' },
  { name: 'Engineering', slug: 'engineering', icon: '⚙️', description: 'Technical docs, architecture, and runbooks', type: 'internal' },
  { name: 'Support Knowledge', slug: 'support-knowledge', icon: '🎧', description: 'Internal support guides and troubleshooting', type: 'internal' },
  { name: 'Marketing', slug: 'marketing', icon: '📣', description: 'Brand guidelines, campaigns, and content', type: 'internal' },
  { name: 'Sales', slug: 'sales', icon: '💼', description: 'Sales playbooks, proposals, and enablement', type: 'internal' },
  { name: 'HR & People', slug: 'hr-people', icon: '👥', description: 'Policies, onboarding, and people ops', type: 'internal' },
  { name: 'Operations', slug: 'operations', icon: '📋', description: 'Processes, SOPs, and operational docs', type: 'internal' },
  { name: 'Help Center', slug: 'help-center', icon: '💡', description: 'Public-facing help articles for customers', type: 'external_capable' },
  { name: 'Developer / API Docs', slug: 'developer-api-docs', icon: '🔧', description: 'API references and developer guides', type: 'external_capable' },
]

// ── Status color ─────────────────────────────────────────────────────────────


// ── Document row ────────────────────────────────────────────────────────────

function DocRow({
  doc,
  wsId,
  wsSlug,
  navigate,
  canEdit,
}: {
  doc: DocsDocument
  wsId: string
  wsSlug: string
  navigate: ReturnType<typeof useNavigate>
  canEdit: boolean
}) {
  const updateDoc = useUpdateDocsDocument(wsId)
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(doc.title)
  const canRename = canEdit && doc.status !== 'archived' && !doc.is_locked

  const startEditing = () => {
    if (!canRename) return
    setDraft(doc.title)
    setEditing(true)
  }

  const cancelEditing = () => {
    setDraft(doc.title)
    setEditing(false)
  }

  const saveTitle = async () => {
    const nextTitle = draft.trim()
    if (!nextTitle) {
      toast.error('Document title is required')
      return
    }
    if (nextTitle === doc.title) {
      setEditing(false)
      return
    }
    try {
      await updateDoc.mutateAsync({ id: doc.id, title: nextTitle })
      toast.success('Document title updated')
      setEditing(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update title')
    }
  }

  if (editing) {
    return (
      <div className="flex w-full items-center gap-2.5 rounded-md px-3 py-2 text-sm">
        <input
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.preventDefault()
              void saveTitle()
            } else if (event.key === 'Escape') {
              event.preventDefault()
              cancelEditing()
            }
          }}
          onBlur={() => {
            if (!updateDoc.isPending) void saveTitle()
          }}
          autoFocus
          className="min-w-0 flex-1 rounded-md border border-border/60 bg-background px-2 py-1 text-sm outline-none focus:border-primary/40 focus:ring-1 focus:ring-primary/20"
        />
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="h-7 w-7 shrink-0"
          disabled={updateDoc.isPending}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => void saveTitle()}
        >
          <Tick01Icon className="h-3.5 w-3.5" />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="h-7 w-7 shrink-0"
          disabled={updateDoc.isPending}
          onMouseDown={(event) => event.preventDefault()}
          onClick={cancelEditing}
        >
          <Cancel01Icon className="h-3.5 w-3.5" />
        </Button>
      </div>
    )
  }

  return (
    <DocsLibraryRow
      compact
      title={doc.title}
      status={doc.status}
      updatedAt={doc.updated_at}
      onOpen={() => navigate({
        to: '/w/$slug/docs/documents/$docId',
        params: { slug: wsSlug, docId: doc.id },
      })}
      actions={canRename ? (
        <QuickTooltip label="Rename document">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-7 w-7 shrink-0"
            onClick={startEditing}
          >
            <PencilEdit02Icon className="h-3.5 w-3.5" />
          </Button>
        </QuickTooltip>
      ) : undefined}
    />
  )
}

// ── Collection tree helper ──────────────────────────────────────────────────

/** Builds a recursive tree of CollectionNodes from a flat collection list. */
function buildCollectionNodeTree(
  collections: DocsCollection[],
  docsByCollection: Map<string, DocsDocument[]>,
): CollectionNode[] {
  const byParent = new Map<string | null, DocsCollection[]>()
  for (const c of collections) {
    const key = c.parent_collection_id ?? null
    const list = byParent.get(key) ?? []
    list.push(c)
    byParent.set(key, list)
  }
  // Sort siblings by position.
  for (const list of byParent.values()) {
    list.sort((a, b) => a.position - b.position)
  }

  function build(parentId: string | null): CollectionNode[] {
    return (byParent.get(parentId) ?? []).map((c) => ({
      id: c.id,
      name: c.name,
      icon: c.icon,
      depth: c.depth,
      position: c.position,
      documents: docsByCollection.get(c.id) ?? [],
      children: build(c.id),
    }))
  }
  return build(null)
}

// ── Collection section ──────────────────────────────────────────────────────

function CollectionIcon({ name }: { name?: string }) {
  return (
    <StoredIcon
      name={name}
      className="h-3.5 w-3.5 shrink-0"
      fallback={<Folder01Icon className="h-3.5 w-3.5 shrink-0" />}
    />
  )
}

interface CollectionNode {
  id: string
  name: string
  icon?: string | null
  depth: number
  position: number
  documents: DocsDocument[]
  children: CollectionNode[]
}

type MergedChild =
  | { kind: 'collection'; position: number; node: CollectionNode }
  | { kind: 'doc'; position: number; doc: DocsDocument }

/**
 * Merge a collection's direct documents with its sub-collections into a
 * single list sorted by position, so the rendered order matches the
 * author's intent (e.g., the Nextra _meta order when imported).
 */
function mergeChildren(node: CollectionNode): MergedChild[] {
  const items: MergedChild[] = []
  for (const child of node.children) {
    items.push({ kind: 'collection', position: child.position, node: child })
  }
  for (const doc of node.documents) {
    items.push({ kind: 'doc', position: doc.position, doc })
  }
  // Primary: position. Tie-break: collections before docs (deterministic
  // for legacy data where articles/sub-collections had independent
  // 0..N sequences), then by id for full determinism.
  items.sort((a, b) => {
    if (a.position !== b.position) return a.position - b.position
    if (a.kind !== b.kind) return a.kind === 'collection' ? -1 : 1
    const aId = a.kind === 'doc' ? a.doc.id : a.node.id
    const bId = b.kind === 'doc' ? b.doc.id : b.node.id
    return aId.localeCompare(bId)
  })
  return items
}

function CollectionSection({
  node,
  wsId,
  wsSlug,
  navigate,
  canEdit,
}: {
  node: CollectionNode
  wsId: string
  wsSlug: string
  navigate: ReturnType<typeof useNavigate>
  canEdit: boolean
}) {
  const [open, setOpen] = useState(false)

  return (
    <Collapsible.Root open={open} onOpenChange={setOpen}>
      <Collapsible.Trigger asChild>
        <button
          type="button"
          className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 ${COLLECTION_ROW_CLASS} transition-colors hover:bg-muted/40`}
        >
          <ArrowRight01Icon
            className={`h-3.5 w-3.5 shrink-0 transition-transform ${open ? 'rotate-90' : ''}`}
          />
          <CollectionIcon name={node.icon ?? undefined} />
          <span className="truncate">{node.name}</span>
          {node.documents.length > 0 && (
            <span className={COUNT_BADGE_CLASS}>{node.documents.length} {node.documents.length === 1 ? 'doc' : 'docs'}</span>
          )}
          {node.children.length > 0 && (
            <span className={COUNT_BADGE_CLASS}>{node.children.length} sub</span>
          )}
        </button>
      </Collapsible.Trigger>
      <Collapsible.Content>
        <div className="ml-[14px] border-l border-border/50 pl-3">
          {mergeChildren(node).map((item) =>
            item.kind === 'collection' ? (
              <CollectionSection
                key={item.node.id}
                node={item.node}
                wsId={wsId}
                wsSlug={wsSlug}
                navigate={navigate}
                canEdit={canEdit}
              />
            ) : (
              <DocRow key={item.doc.id} doc={item.doc} wsId={wsId} wsSlug={wsSlug} navigate={navigate} canEdit={canEdit} />
            ),
          )}
        </div>
      </Collapsible.Content>
    </Collapsible.Root>
  )
}

// ── Space section ───────────────────────────────────────────────────────────

function SpaceSection({
  space,
  collections,
  documents,
  wsId,
  wsSlug,
  navigate,
  teamNames,
  canEdit,
}: {
  space: DocsSpace
  collections: DocsCollection[]
  documents: DocsDocument[]
  wsId: string
  wsSlug: string
  navigate: ReturnType<typeof useNavigate>
  teamNames: string
  canEdit: boolean
}) {
  const [expanded, setExpanded] = useState(false)

  const collectionMap = new Map<string, DocsDocument[]>()
  const uncollected: DocsDocument[] = []

  for (const doc of documents) {
    if (doc.collection_id) {
      const list = collectionMap.get(doc.collection_id) ?? []
      list.push(doc)
      collectionMap.set(doc.collection_id, list)
    } else {
      uncollected.push(doc)
    }
  }

  // Build recursive tree of CollectionNodes.
  const collectionTree = buildCollectionNodeTree(collections, collectionMap)
  const docCount = documents.length
  const collCount = collections.length

  return (
    <Collapsible.Root open={expanded} onOpenChange={setExpanded}>
      <div className="group/space flex items-center">
        <Collapsible.Trigger asChild>
          <button
            type="button"
            className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2.5 text-left transition-colors hover:bg-muted/60"
          >
            <ArrowRight01Icon
              className={`h-4 w-4 shrink-0 text-muted-foreground transition-transform ${expanded ? 'rotate-90' : ''}`}
            />
            <StoredIcon
              name={space.icon}
              className="h-4 w-4 shrink-0 text-muted-foreground"
              textClassName="text-base"
              fallback={<Folder01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />}
            />
            <div className="min-w-0 flex-1">
              <span className="truncate text-sm font-semibold">{space.name}</span>
            </div>
            {!expanded && (
              <div className="flex items-center gap-5 shrink-0">
                {collCount > 0 && (
                  <span className="text-[11px] text-muted-foreground">{collCount} {collCount === 1 ? 'collection' : 'collections'}</span>
                )}
                {docCount > 0 && (
                  <span className="text-[11px] text-muted-foreground">{docCount} {docCount === 1 ? 'doc' : 'docs'}</span>
                )}
                {teamNames && (
                  <span className="text-[11px] text-muted-foreground/60 truncate max-w-[150px]">{teamNames}</span>
                )}
              </div>
            )}
          </button>
        </Collapsible.Trigger>
        <QuickTooltip label="Open space">
          <Button
            variant="ghost"
            size="icon"
            className="mr-1 h-7 w-7 shrink-0 opacity-0 transition-opacity group-hover/space:opacity-100"
            onClick={() =>
              navigate({
                to: '/w/$slug/docs/spaces/$spaceId',
                params: { slug: wsSlug, spaceId: space.id },
              })
            }
          >
            <BookOpen01Icon className="h-3.5 w-3.5" />
          </Button>
        </QuickTooltip>
      </div>

      <Collapsible.Content>
        <div className="ml-5 border-l border-border/50 pb-2 pl-2">
          {/* Collections — rendered as recursive tree */}
          {collectionTree.map((node) => (
            <CollectionSection key={node.id} node={node} wsId={wsId} wsSlug={wsSlug} navigate={navigate} canEdit={canEdit} />
          ))}

          {/* Uncategorized documents (no collection) */}
          {uncollected.length > 0 && (
            <CollectionSection
              node={{ id: '__uncategorized', name: 'Uncategorized', position: 0, depth: 0, documents: uncollected, children: [] }}
              wsId={wsId}
              wsSlug={wsSlug}
              navigate={navigate}
              canEdit={canEdit}
            />
          )}

          {expanded && docCount === 0 && (
            <p className="px-3 py-2 text-xs text-muted-foreground">No documents in this space yet.</p>
          )}
        </div>
      </Collapsible.Content>
    </Collapsible.Root>
  )
}

// ── Main page ───────────────────────────────────────────────────────────────

export function DocsHome() {
  useTitle('Documentation')
  const navigate = useNavigate()
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id ?? ''
  const wsSlug = workspace?.slug ?? ''

  const { data: access } = useWorkspaceAccess(wsId)
  const { canEditDocs } = usePermissions(access)
  const { data: spaces, isLoading } = useDocsSpaces(wsId)
  const { data: allCollections } = useAllDocsCollections(wsId)
  const { data: allDocuments } = useDocsDocuments(wsId, {})

  // Build per-space lookup maps from batch data
  const collectionsBySpace = useMemo(() => {
    const map = new Map<string, DocsCollection[]>()
    for (const c of allCollections ?? []) {
      const list = map.get(c.space_id) ?? []
      list.push(c)
      map.set(c.space_id, list)
    }
    return map
  }, [allCollections])

  const documentsBySpace = useMemo(() => {
    const map = new Map<string, DocsDocument[]>()
    for (const d of allDocuments ?? []) {
      const list = map.get(d.space_id) ?? []
      list.push(d)
      map.set(d.space_id, list)
    }
    // Sort by position within each space for correct display order.
    for (const list of map.values()) {
      list.sort((a, b) => (a.position ?? 0) - (b.position ?? 0))
    }
    return map
  }, [allDocuments])

  const [createSpaceOpen, setCreateSpaceOpen] = useState(false)
  const [arrangeMode, setArrangeMode] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')

  const { data: settings } = useWorkspaceSettings(wsId)
  const allTeams = settings?.teams ?? []
  const teamMap = new Map(allTeams.map((t) => [t.id, t.name]))

  // Search: filter documents across all spaces by title.
  const searchResults = useMemo(() => {
    const q = searchQuery.trim().toLowerCase()
    if (!q || !allDocuments) return null
    return allDocuments.filter((d) =>
      d.title?.toLowerCase().includes(q)
    )
  }, [searchQuery, allDocuments])

  // Space name lookup for search results.
  const spaceNames = useMemo(
    () => new Map<string, string>((spaces ?? []).map((s) => [s.id, s.name])),
    [spaces],
  )
  const collectionPaths = useMemo(
    () => buildCollectionPathLabels(allCollections ?? []),
    [allCollections],
  )

  const getTeamNames = (space: DocsSpace): string => {
    if (space.visibility === 'workspace_wide') return 'All teams'
    if (space.team_ids?.length > 0) {
      return space.team_ids.map((id) => teamMap.get(id) ?? 'Unknown').join(', ')
    }
    return ''
  }

  // Template picker state
  const [selected, setSelected] = useState<Set<string>>(() => new Set(SPACE_TEMPLATES.map((t) => t.slug)))
  const createSpace = useCreateDocsSpace(wsId)
  const [creating, setCreating] = useState(false)

  const toggleTemplate = (slug: string) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(slug)) next.delete(slug)
      else next.add(slug)
      return next
    })
  }

  const selectAll = () => setSelected(new Set(SPACE_TEMPLATES.map((t) => t.slug)))

  const handleCreateSelected = async () => {
    const toCreate = SPACE_TEMPLATES.filter((t) => selected.has(t.slug))
    if (toCreate.length === 0) return

    setCreating(true)
    let created = 0
    for (const t of toCreate) {
      try {
        await createSpace.mutateAsync({
          name: t.name,
          slug: t.slug,
          visibility: 'workspace_wide',
          type: t.type,
        })
        created++
      } catch {
        // skip duplicates silently (e.g. slug conflict)
      }
    }
    setCreating(false)
    if (created > 0) {
      toast.success(`Created ${created} space${created > 1 ? 's' : ''}`)
    }
  }

  if (!workspace) {
    return (
      <div className="flex h-full min-h-0 flex-col">
        <QuietPageHeader variant="shell" title="All Docs" />
        <QuietPageViewport className="min-h-0 flex-1">
          <p className="text-sm text-muted-foreground">Workspace not found.</p>
        </QuietPageViewport>
      </div>
    )
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <QuietPageHeader
        variant="shell"
        title="All Docs"
        description="All spaces, collections, and articles in one place."
      />

      <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-quiet-divider-light px-4 py-2 sm:px-6 lg:px-8">
        {!arrangeMode && (
          <QuietSearchInput
            containerClassName="min-w-0 flex-1 sm:w-64 sm:flex-none"
            placeholder="Search documents..."
            aria-label="Search all documents"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
        )}
        {canEditDocs && spaces && spaces.length > 0 ? (
          <QuietTextAction className="ml-auto gap-1.5" onClick={() => setArrangeMode(!arrangeMode)}>
            <ArrowUpDownIcon className="h-3.5 w-3.5" />
            {arrangeMode ? 'Done arranging' : 'Arrange'}
          </QuietTextAction>
        ) : null}
      </div>

      <QuietPageViewport className="min-h-0 flex-1">
        <div className="space-y-4">
          {arrangeMode && (
            <p className="text-xs text-amber-700 dark:text-amber-300 bg-amber-50 dark:bg-amber-950/30 border border-amber-200 dark:border-amber-800/40 rounded-md px-3 py-2 text-center">
              Drag to reorder spaces, collections, and documents within the same group. Changes to external spaces will be reflected on your public help center.
            </p>
          )}

      {searchResults !== null ? (
        <DocsLibraryList ariaLabel="Search results">
          {searchResults.length === 0 ? (
            <div className="flex flex-col items-center py-8 text-sm text-muted-foreground">
              <Search01Icon className="h-8 w-8 text-muted-foreground/30 mb-2" />
              No documents matching &ldquo;{searchQuery}&rdquo;
            </div>
          ) : (
            searchResults.map((doc) => (
              <DocsLibraryRow
                key={doc.id}
                title={doc.title}
                status={doc.status}
                updatedAt={doc.updated_at}
                location={[
                  spaceNames.get(doc.space_id),
                  doc.collection_id ? collectionPaths.get(doc.collection_id) : undefined,
                ].filter(Boolean).join(' › ')}
                onOpen={() => navigate({
                  to: '/w/$slug/docs/documents/$docId',
                  params: { slug: wsSlug, docId: doc.id },
                })}
              />
            ))
          )}
        </DocsLibraryList>
      ) : isLoading ? (
        <div className="space-y-3 py-4">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-12 animate-pulse rounded-lg bg-muted/60" />
          ))}
        </div>
      ) : !spaces || spaces.length === 0 ? (
        <div className="rounded-lg border border-border/60 bg-card p-6 sm:p-8">
          <div className="mx-auto max-w-2xl text-center">
            <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-full bg-muted text-muted-foreground">
              <BookOpen01Icon className="h-5 w-5" />
            </div>
            <h3 className="mt-4 text-lg font-semibold">No docs spaces yet</h3>
            <p className="mt-2 text-sm leading-6 text-muted-foreground">
              Create a space for internal docs, team knowledge, or public help articles. You can add collections and articles after.
            </p>
          </div>
          {canEditDocs ? (
            <div className="mt-6">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <p className="text-sm font-medium">Starter spaces</p>
                <Button
                  size="sm"
                  onClick={handleCreateSelected}
                  disabled={selected.size === 0 || creating}
                >
                  {creating ? (
                    <>
                      <Loading01Icon className="h-4 w-4 animate-spin" />
                      Creating...
                    </>
                  ) : (
                    <>
                      <PlusSignIcon className="h-4 w-4" />
                      Create selected
                    </>
                  )}
                </Button>
              </div>

              <div className="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
                {SPACE_TEMPLATES.map((t) => {
                  const isSelected = selected.has(t.slug)
                  return (
                    <button
                      key={t.slug}
                      type="button"
                      onClick={() => toggleTemplate(t.slug)}
                      className={`relative flex min-h-[88px] items-start gap-3 rounded-md border p-3 text-left transition-all ${
                        isSelected
                          ? 'border-primary/45 bg-primary/5 ring-1 ring-primary/15'
                          : 'border-border/60 bg-background hover:border-border hover:bg-muted/30'
                      }`}
                    >
                      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md border bg-muted/60 text-base">
                        {t.icon}
                      </div>
                      <div className="min-w-0 flex-1 pr-5">
                        <div className="flex items-center gap-2">
                          <span className="truncate text-sm font-semibold">{t.name}</span>
                          {t.type === 'external_capable' && (
                            <QuickTooltip label="Can publish to the public help center">
                              <span className="inline-flex h-3.5 w-3.5 shrink-0 cursor-help items-center justify-center rounded text-primary">
                                <GlobeIcon className="h-3 w-3" />
                              </span>
                            </QuickTooltip>
                          )}
                        </div>
                        <p className="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground">{t.description}</p>
                      </div>
                      <div
                        className={`absolute right-3 top-3 flex h-4 w-4 items-center justify-center rounded-full border transition-colors ${
                          isSelected
                            ? 'border-primary bg-primary text-primary-foreground'
                            : 'border-border bg-background'
                        }`}
                      >
                        {isSelected && <Tick01Icon className="h-2.5 w-2.5" />}
                      </div>
                    </button>
                  )
                })}
              </div>

              <div className="mt-4 flex flex-wrap items-center justify-center gap-3">
                <button type="button" onClick={selectAll} className="text-xs font-medium text-muted-foreground transition-colors hover:text-foreground">
                  Select all
                </button>
                <button
                  type="button"
                  onClick={() => setCreateSpaceOpen(true)}
                  className="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground transition-colors hover:text-foreground"
                >
                  Create a custom space
                  <ArrowRight01Icon className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>
          ) : (
            <div className="border-t border-border/60 p-5 text-center">
              <p className="text-sm font-medium">No knowledge spaces yet</p>
              <p className="mt-1 text-sm text-muted-foreground">
                Ask a workspace admin or docs editor to create the first space.
              </p>
            </div>
          )}
        </div>
      ) : arrangeMode ? (
        <DocsArrangeTree spaces={spaces} wsId={wsId} />
      ) : (
          <div className="space-y-6">
            {/* Internal spaces */}
            {spaces.filter((s) => s.type === 'internal').length > 0 && (
              <div>
                <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Team Spaces</h3>
                <div className="divide-y divide-border/45" data-slot="docs-space-library">
                  {spaces.filter((s) => s.type === 'internal').map((space) => (
                    <SpaceSection
                      key={space.id}
                      space={space}
                      collections={collectionsBySpace.get(space.id) ?? []}
                      documents={documentsBySpace.get(space.id) ?? []}
                      wsId={wsId}
                      wsSlug={wsSlug}
                      navigate={navigate}
                      teamNames={getTeamNames(space)}
                      canEdit={canEditDocs}
                    />
                  ))}
                </div>
              </div>
            )}

            {/* External spaces */}
            {spaces.filter((s) => s.type === 'external_capable').length > 0 && (
              <div>
                <h3 className="mb-2 flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  External Spaces
                  <QuickTooltip label="These spaces are published to your public help center" side="right">
                    <span className="inline-flex h-4 w-4 cursor-help items-center justify-center rounded text-muted-foreground/50 hover:text-muted-foreground">
                      <HelpCircleIcon className="h-3.5 w-3.5" />
                    </span>
                  </QuickTooltip>
                </h3>
                <div className="divide-y divide-border/45" data-slot="docs-space-library">
                  {spaces.filter((s) => s.type === 'external_capable').map((space) => (
                    <SpaceSection
                      key={space.id}
                      space={space}
                      collections={collectionsBySpace.get(space.id) ?? []}
                      documents={documentsBySpace.get(space.id) ?? []}
                      wsId={wsId}
                      wsSlug={wsSlug}
                      navigate={navigate}
                      teamNames={getTeamNames(space)}
                      canEdit={canEditDocs}
                    />
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
          <div className="h-32" />
        </div>
      </QuietPageViewport>
      <CreateSpaceDialog wsId={wsId} open={createSpaceOpen} onOpenChange={setCreateSpaceOpen} />
    </div>
  )
}
