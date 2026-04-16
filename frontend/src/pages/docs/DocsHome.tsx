import { useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { timeAgo } from '@/lib/utils'
import { COLLECTION_ROW_CLASS, ARTICLE_ROW_CLASS, DOC_ICON_CLASS, STATUS_BADGE_CLASS, UPDATED_TEXT_CLASS, COUNT_BADGE_CLASS, statusColor } from './docsTreeStyles'
import {
  ArrowUpDownIcon,
  BookOpen01Icon,
  Tick01Icon,
  ArrowRight01Icon,
  HelpCircleIcon,
  File01Icon,
  Folder01Icon,
  GlobeIcon,
  Loading01Icon,
  PlusSignIcon,
  Search01Icon,
} from '@/lib/icons'
import { ICON_MAP, StoredIcon } from '@/components/ui/icon-picker'
import { Collapsible } from 'radix-ui'
import { toast } from 'sonner'
import { useTitle } from '@/hooks/useTitle'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import {
  useDocsSpaces,
  useAllDocsCollections,
  useDocsDocuments,
  useCreateDocsSpace,
  useWorkspaceAccess,
  usePermissions,
} from '@/hooks/queries'
import { useWorkspaceSettings } from '@/hooks/queries/useSettings'
import { Button } from '@/components/ui/button'
import { QuickTooltip } from '@/components/ui/quick-tooltip'
import { CreateSpaceDialog } from '@/components/docs/CreateSpaceDialog'
import { DocsArrangeTree } from '@/components/docs/DocsArrangeTree'
import type { DocsSpace, DocsCollection, DocsDocument, SpaceType } from '@/lib/docsTypes'
import { DOC_STATUS_LABELS } from '@/lib/docsTypes'

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
  wsSlug,
  navigate,
}: {
  doc: DocsDocument
  wsSlug: string
  navigate: ReturnType<typeof useNavigate>
}) {
  return (
    <button
      type="button"
      onClick={() =>
        navigate({
          to: '/w/$slug/docs/documents/$docId',
          params: { slug: wsSlug, docId: doc.id },
        })
      }
      className="group flex w-full items-center gap-2.5 rounded-md px-3 py-2 text-left text-sm transition-colors hover:bg-muted/60"
    >
      <File01Icon className={DOC_ICON_CLASS} />
      <span className={`min-w-0 flex-1 truncate ${ARTICLE_ROW_CLASS}`}>{doc.title}</span>
      <span className={`${STATUS_BADGE_CLASS} ${statusColor(doc.status)}`}>
        {DOC_STATUS_LABELS[doc.status] ?? doc.status}
      </span>
      <span className={UPDATED_TEXT_CLASS}>
        Updated: {timeAgo(doc.updated_at)}
      </span>
    </button>
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
  if (name) {
    const Icon = ICON_MAP[name]
    if (Icon) return <Icon className="h-3.5 w-3.5 shrink-0" />
  }
  return <Folder01Icon className="h-3.5 w-3.5 shrink-0" />
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
  items.sort((a, b) => a.position - b.position)
  return items
}

function CollectionSection({
  node,
  wsSlug,
  navigate,
}: {
  node: CollectionNode
  wsSlug: string
  navigate: ReturnType<typeof useNavigate>
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
              <CollectionSection key={item.node.id} node={item.node} wsSlug={wsSlug} navigate={navigate} />
            ) : (
              <DocRow key={item.doc.id} doc={item.doc} wsSlug={wsSlug} navigate={navigate} />
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
  wsSlug,
  navigate,
  teamNames,
}: {
  space: DocsSpace
  collections: DocsCollection[]
  documents: DocsDocument[]
  wsSlug: string
  navigate: ReturnType<typeof useNavigate>
  teamNames: string
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
            <CollectionSection key={node.id} node={node} wsSlug={wsSlug} navigate={navigate} />
          ))}

          {/* Uncategorized documents (no collection) */}
          {uncollected.length > 0 && (
            <CollectionSection
              node={{ id: '__uncategorized', name: 'Uncategorized', position: 0, depth: 0, documents: uncollected, children: [] }}
              wsSlug={wsSlug}
              navigate={navigate}
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
  const selectNone = () => setSelected(new Set())

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
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>
  }

  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <header className="space-y-3">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-xl font-semibold">All Docs</h2>
            <p className="text-sm text-muted-foreground">
              All spaces, collections, and articles in one place.
            </p>
          </div>
          {canEditDocs && spaces && spaces.length > 0 && (
            <Button
              variant={arrangeMode ? 'default' : 'outline'}
              size="sm"
              className="gap-1.5"
              onClick={() => setArrangeMode(!arrangeMode)}
            >
              <ArrowUpDownIcon className="h-3.5 w-3.5" />
              {arrangeMode ? 'Done arranging' : 'Arrange'}
            </Button>
          )}
        </div>
        {!arrangeMode && (
          <div className="relative">
            <Search01Icon className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <input
              type="text"
              placeholder="Search documents..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full rounded-lg border border-border/60 bg-background py-2 pl-9 pr-3 text-sm outline-none placeholder:text-muted-foreground/60 focus:border-primary/40 focus:ring-1 focus:ring-primary/20"
            />
          </div>
        )}
      </header>

      {arrangeMode && (
        <p className="text-xs text-amber-700 dark:text-amber-300 bg-amber-50 dark:bg-amber-950/30 border border-amber-200 dark:border-amber-800/40 rounded-md px-3 py-2 text-center">
          Drag to reorder spaces, collections, and documents within the same group. Changes to external spaces will be reflected on your public help center.
        </p>
      )}

      {searchResults !== null ? (
        <div className="rounded-lg border border-border/60 bg-card divide-y divide-border/40">
          {searchResults.length === 0 ? (
            <div className="flex flex-col items-center py-8 text-sm text-muted-foreground">
              <Search01Icon className="h-8 w-8 text-muted-foreground/30 mb-2" />
              No documents matching &ldquo;{searchQuery}&rdquo;
            </div>
          ) : (
            searchResults.map((doc) => (
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
                <File01Icon className={DOC_ICON_CLASS} />
                <span className={`min-w-0 flex-1 truncate ${ARTICLE_ROW_CLASS}`}>{doc.title}</span>
                <span className="shrink-0 text-[11px] text-muted-foreground">
                  {spaceNames.get(doc.space_id) ?? ''}
                  {doc.collection_id && allCollections
                    ? ` / ${allCollections.find((c) => c.id === doc.collection_id)?.name ?? ''}`
                    : ''}
                </span>
                <span className={`${STATUS_BADGE_CLASS} ${statusColor(doc.status)}`}>
                  {DOC_STATUS_LABELS[doc.status] ?? doc.status}
                </span>
              </button>
            ))
          )}
        </div>
      ) : isLoading ? (
        <div className="space-y-3 py-4">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-12 animate-pulse rounded-lg bg-muted/60" />
          ))}
        </div>
      ) : !spaces || spaces.length === 0 ? (
        <div className="py-8 px-4">
          <div className="text-center mb-8">
            <div className="flex h-14 w-14 items-center justify-center rounded-full bg-blue-500/10 mb-4 mx-auto">
              <BookOpen01Icon className="h-7 w-7 text-blue-500" />
            </div>
            <h3 className="text-lg font-semibold mb-1.5">Get started with Documentation</h3>
            <p className="text-sm text-muted-foreground max-w-lg mx-auto">
              Select the spaces you need and create them all at once — you can always add more later.
            </p>
          </div>

          {canEditDocs && (
            <>
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-3">
                  <span className="text-sm font-medium">{selected.size} of {SPACE_TEMPLATES.length} selected</span>
                  <button type="button" onClick={selectAll} className="text-xs text-primary hover:underline">Select all</button>
                  <button type="button" onClick={selectNone} className="text-xs text-muted-foreground hover:underline">Clear</button>
                </div>
                <Button
                  className="gap-2"
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
                      Create {selected.size} Space{selected.size !== 1 ? 's' : ''}
                    </>
                  )}
                </Button>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                {SPACE_TEMPLATES.map((t) => {
                  const isSelected = selected.has(t.slug)
                  return (
                    <button
                      key={t.slug}
                      type="button"
                      onClick={() => toggleTemplate(t.slug)}
                      className={`relative flex items-start gap-3 rounded-lg border p-4 text-left transition-all cursor-pointer ${
                        isSelected
                          ? 'border-primary bg-primary/5 ring-1 ring-primary/20'
                          : 'border-border/60 bg-card hover:border-border hover:bg-muted/30'
                      }`}
                    >
                      <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-muted/80 shrink-0 text-lg">
                        {t.icon}
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-semibold truncate">{t.name}</span>
                          {t.type === 'external_capable' && (
                            <QuickTooltip label="External">
                              <GlobeIcon className="h-3 w-3 shrink-0 text-blue-500" />
                            </QuickTooltip>
                          )}
                        </div>
                        <p className="text-xs text-muted-foreground mt-0.5 line-clamp-2">{t.description}</p>
                      </div>
                      <div
                        className={`absolute top-2 right-2 flex h-5 w-5 items-center justify-center rounded-full border transition-colors ${
                          isSelected
                            ? 'border-primary bg-primary text-primary-foreground'
                            : 'border-border bg-background'
                        }`}
                      >
                        {isSelected && <Tick01Icon className="h-3 w-3" />}
                      </div>
                    </button>
                  )
                })}
              </div>

              <div className="mt-4 text-center">
                <button
                  type="button"
                  onClick={() => setCreateSpaceOpen(true)}
                  className="text-xs text-muted-foreground hover:text-foreground hover:underline"
                >
                  Or create a custom space instead
                </button>
              </div>
            </>
          )}

          {!canEditDocs && (
            <p className="text-center text-sm text-muted-foreground">
              No documentation spaces have been created yet. Ask a workspace member to set them up.
            </p>
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
                <div className="divide-y divide-border/50 rounded-lg border border-border/60 bg-card">
                  {spaces.filter((s) => s.type === 'internal').map((space) => (
                    <SpaceSection
                      key={space.id}
                      space={space}
                      collections={collectionsBySpace.get(space.id) ?? []}
                      documents={documentsBySpace.get(space.id) ?? []}
                      wsSlug={wsSlug}
                      navigate={navigate}
                      teamNames={getTeamNames(space)}
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
                  <QuickTooltip label="These spaces are published to your public help center">
                    <HelpCircleIcon className="h-3.5 w-3.5 text-muted-foreground/50" />
                  </QuickTooltip>
                </h3>
                <div className="divide-y divide-border/50 rounded-lg border border-border/60 bg-card">
                  {spaces.filter((s) => s.type === 'external_capable').map((space) => (
                    <SpaceSection
                      key={space.id}
                      space={space}
                      collections={collectionsBySpace.get(space.id) ?? []}
                      documents={documentsBySpace.get(space.id) ?? []}
                      wsSlug={wsSlug}
                      navigate={navigate}
                      teamNames={getTeamNames(space)}
                    />
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      <CreateSpaceDialog wsId={wsId} open={createSpaceOpen} onOpenChange={setCreateSpaceOpen} />
      <div className="h-32" />
    </div>
  )
}
