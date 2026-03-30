import { useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { timeAgo } from '@/lib/utils'
import {
  ArrowUpDown,
  BookOpen,
  Check,
  ChevronRight,
  CircleHelp,
  FileText,
  Folder,
  Globe,
  Loader2,
  Plus,
} from 'lucide-react'
import { ICON_MAP } from '@/components/ui/icon-picker'
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
      <FileText className="h-4 w-4 shrink-0 text-muted-foreground" />
      <span className="min-w-0 flex-1 truncate font-medium">{doc.title}</span>
      <span className={`shrink-0 text-xs font-medium ${statusColor(doc.status)}`}>
        {DOC_STATUS_LABELS[doc.status] ?? doc.status}
      </span>
      <span className="shrink-0 text-[11px] text-muted-foreground">
        Updated: {timeAgo(doc.updated_at)}
      </span>
    </button>
  )
}

// ── Collection section ──────────────────────────────────────────────────────

function CollectionIcon({ name }: { name?: string }) {
  if (name) {
    const Icon = ICON_MAP[name]
    if (Icon) return <Icon className="h-3.5 w-3.5 shrink-0" />
  }
  return <Folder className="h-3.5 w-3.5 shrink-0" />
}

function CollectionSection({
  name,
  icon,
  documents,
  wsSlug,
  navigate,
}: {
  name: string
  icon?: string
  documents: DocsDocument[]
  wsSlug: string
  navigate: ReturnType<typeof useNavigate>
}) {
  const [open, setOpen] = useState(documents.length > 0)

  return (
    <Collapsible.Root open={open} onOpenChange={setOpen}>
      <Collapsible.Trigger asChild>
        <button
          type="button"
          className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-[13px] font-medium text-foreground/70 transition-colors hover:bg-muted/40"
        >
          <ChevronRight
            className={`h-3.5 w-3.5 shrink-0 transition-transform ${open ? 'rotate-90' : ''}`}
          />
          <CollectionIcon name={icon} />
          <span className="truncate">{name}</span>
          <span className="rounded-full bg-muted px-1.5 text-[10px] tabular-nums text-muted-foreground">{documents.length} {documents.length === 1 ? 'doc' : 'docs'}</span>
        </button>
      </Collapsible.Trigger>
      <Collapsible.Content>
        <div className="ml-4 border-l border-border/50 pl-1">
          {documents.length > 0 ? (
            documents.map((doc) => (
              <DocRow key={doc.id} doc={doc} wsSlug={wsSlug} navigate={navigate} />
            ))
          ) : (
            <p className="px-3 py-1.5 text-[11px] text-muted-foreground/60">No documents</p>
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
            <ChevronRight
              className={`h-4 w-4 shrink-0 text-muted-foreground transition-transform ${expanded ? 'rotate-90' : ''}`}
            />
            {space.icon ? (
              <span className="text-base">{space.icon}</span>
            ) : (
              <Folder className="h-4 w-4 shrink-0 text-muted-foreground" />
            )}
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
            <BookOpen className="h-3.5 w-3.5" />
          </Button>
        </QuickTooltip>
      </div>

      <Collapsible.Content>
        <div className="ml-5 border-l border-border/50 pb-2 pl-2">
          {/* Collections */}
          {(collections ?? []).map((col) => (
            <CollectionSection
              key={col.id}
              name={col.name}
              icon={col.icon}
              documents={collectionMap.get(col.id) ?? []}
              wsSlug={wsSlug}
              navigate={navigate}
            />
          ))}

          {/* Uncategorized documents (no collection) */}
          {uncollected.length > 0 && (
            <CollectionSection
              name="Uncategorized"
              documents={uncollected}
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
    return map
  }, [allDocuments])

  const [createSpaceOpen, setCreateSpaceOpen] = useState(false)
  const [arrangeMode, setArrangeMode] = useState(false)

  const { data: settings } = useWorkspaceSettings(wsId)
  const allTeams = settings?.teams ?? []
  const teamMap = new Map(allTeams.map((t) => [t.id, t.name]))

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
      <header className="flex items-center justify-between">
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
            <ArrowUpDown className="h-3.5 w-3.5" />
            {arrangeMode ? 'Done arranging' : 'Arrange'}
          </Button>
        )}
      </header>

      {arrangeMode && (
        <p className="text-xs text-amber-700 dark:text-amber-300 bg-amber-50 dark:bg-amber-950/30 border border-amber-200 dark:border-amber-800/40 rounded-md px-3 py-2 text-center">
          <span className="font-semibold">Note:</span> Drag and drop to reorder spaces, collections, and articles. Changes to external spaces will be reflected on your public help center.
        </p>
      )}

      {isLoading ? (
        <div className="space-y-3 py-4">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-12 animate-pulse rounded-lg bg-muted/60" />
          ))}
        </div>
      ) : !spaces || spaces.length === 0 ? (
        <div className="py-8 px-4">
          <div className="text-center mb-8">
            <div className="flex h-14 w-14 items-center justify-center rounded-full bg-blue-500/10 mb-4 mx-auto">
              <BookOpen className="h-7 w-7 text-blue-500" />
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
                      <Loader2 className="h-4 w-4 animate-spin" />
                      Creating...
                    </>
                  ) : (
                    <>
                      <Plus className="h-4 w-4" />
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
                              <Globe className="h-3 w-3 shrink-0 text-blue-500" />
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
                        {isSelected && <Check className="h-3 w-3" />}
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
                    <CircleHelp className="h-3.5 w-3.5 text-muted-foreground/50" />
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
    </div>
  )
}
