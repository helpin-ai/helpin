import { useDeferredValue, useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { toast } from 'sonner'
import {
  Alert01Icon,
  BulbIcon,
  Cancel01Icon,
  ChartIncreaseIcon,
  Clock03Icon,
  DollarCircleIcon,
  InboxIcon,
  Mail01Icon,
  Search01Icon,
  SparklesIcon,
} from '@/lib/icons'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { useAcceptSuggestion, useDismissSuggestion, usePendingSuggestions } from '@/hooks/queries'
import { SuggestionCard } from '@/components/crm/SuggestionCard'
import { useTitle } from '@/hooks/useTitle'
import { cn, timeAgo } from '@/lib/utils'
import type { CRMSignalDismissalReason, CRMSuggestion, CRMSuggestionType } from '@/lib/crmTypes'

type FilterTab = 'all' | CRMSuggestionType
type SortMode = 'recommended' | 'newest' | 'oldest'
type Tone = 'neutral' | 'good' | 'warn' | 'danger' | 'accent'

const tabs: Array<{ key: FilterTab; label: string; icon: typeof SparklesIcon }> = [
  { key: 'all', label: 'All', icon: SparklesIcon },
  { key: 'deal_create', label: 'New deals', icon: DollarCircleIcon },
  { key: 'deal_advance', label: 'Advances', icon: ChartIncreaseIcon },
  { key: 'follow_up', label: 'Follow ups', icon: Mail01Icon },
  { key: 'risk_alert', label: 'Risks', icon: Alert01Icon },
  { key: 'enrichment', label: 'Enrichment', icon: BulbIcon },
]

const sortOptions: Array<{ key: SortMode; label: string }> = [
  { key: 'recommended', label: 'Recommended' },
  { key: 'newest', label: 'Newest' },
  { key: 'oldest', label: 'Oldest' },
]

const typeLabels: Record<CRMSuggestionType, string> = {
  deal_create: 'New deal',
  deal_advance: 'Stage advance',
  follow_up: 'Follow up',
  enrichment: 'Enrichment',
  risk_alert: 'Risk alert',
}

function toneClasses(tone: Tone) {
  switch (tone) {
    case 'good':
      return 'border-emerald-500/20 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
    case 'warn':
      return 'border-amber-500/20 bg-amber-500/10 text-amber-700 dark:text-amber-300'
    case 'danger':
      return 'border-rose-500/20 bg-rose-500/10 text-rose-700 dark:text-rose-300'
    case 'accent':
      return 'border-sky-500/20 bg-sky-500/10 text-sky-700 dark:text-sky-300'
    default:
      return 'border-border bg-muted text-muted-foreground'
  }
}

function typeTone(type: CRMSuggestionType): Tone {
  switch (type) {
    case 'deal_create':
      return 'good'
    case 'deal_advance':
      return 'accent'
    case 'risk_alert':
      return 'danger'
    case 'enrichment':
      return 'warn'
    default:
      return 'neutral'
  }
}

function matchesSearch(suggestion: CRMSuggestion, query: string) {
  if (!query) return true
  const haystack = [
    suggestion.title,
    suggestion.description,
    suggestion.suggestion_type,
    suggestion.object_type,
    JSON.stringify(suggestion.context ?? {}),
  ].join(' ').toLowerCase()
  return haystack.includes(query.toLowerCase())
}

function sortSuggestions(items: CRMSuggestion[], mode: SortMode) {
  return [...items].sort((a, b) => {
    if (mode === 'newest') return new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
    if (mode === 'oldest') return new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
    const confidenceDelta = b.confidence - a.confidence
    if (Math.abs(confidenceDelta) > 0.001) return confidenceDelta
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  })
}

function FilterEmptyState({ activeTab, hasSearch, onClearSearch }: { activeTab: FilterTab; hasSearch: boolean; onClearSearch: () => void }) {
  return (
    <div className="flex min-h-72 flex-col items-center justify-center rounded-lg border border-dashed border-border/80 bg-muted/20 px-6 py-10 text-center">
      <div className="flex h-11 w-11 items-center justify-center rounded-lg border bg-background">
        <InboxIcon className="h-5 w-5 text-muted-foreground" />
      </div>
      <h3 className="mt-3 text-sm font-medium">
        {hasSearch ? 'No matching suggestions' : activeTab === 'all' ? 'No pending suggestions' : `No ${activeTab.replace(/_/g, ' ')} suggestions`}
      </h3>
      <p className="mt-1 max-w-md text-xs leading-5 text-muted-foreground">
        {hasSearch
          ? 'Try a different search term or clear the current filter.'
          : 'CRM suggestions will appear here when automation detects deal creation, progression, follow-up, enrichment, or risk opportunities.'}
      </p>
      {hasSearch ? (
        <Button variant="outline" size="sm" className="mt-4" onClick={onClearSearch}>
          <Cancel01Icon className="h-3.5 w-3.5" />
          Clear search
        </Button>
      ) : null}
    </div>
  )
}

function QueueClearState({
  onOpenInsights,
  onOpenEmailSettings,
  onOpenAutonomySettings,
}: {
  onOpenInsights: () => void
  onOpenEmailSettings: () => void
  onOpenAutonomySettings: () => void
}) {
  return (
    <div className="rounded-lg border bg-card">
      <div className="grid gap-6 p-6 lg:grid-cols-[minmax(0,1fr)_360px]">
        <div className="flex min-h-64 flex-col justify-center">
          <div className="flex h-11 w-11 items-center justify-center rounded-lg border bg-muted/40">
            <InboxIcon className="h-5 w-5 text-muted-foreground" />
          </div>
          <h2 className="mt-4 text-lg font-semibold">Review queue is clear</h2>
          <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
            There are no CRM suggestions waiting for a human decision. This page becomes useful when Helpin finds a deal to create,
            a stage change to approve, a follow-up to send, enrichment to apply, or a risk alert that should not run automatically.
          </p>
          <div className="mt-5 flex flex-wrap gap-2">
            <Button size="sm" onClick={onOpenInsights}>
              <SparklesIcon className="h-4 w-4" />
              Open insights
            </Button>
            <Button size="sm" variant="outline" onClick={onOpenEmailSettings}>
              <Mail01Icon className="h-4 w-4" />
              Email sync settings
            </Button>
            <Button size="sm" variant="outline" onClick={onOpenAutonomySettings}>
              <ChartIncreaseIcon className="h-4 w-4" />
              Review thresholds
            </Button>
          </div>
        </div>

        <div className="space-y-3">
          <div className="rounded-lg border bg-muted/20 p-4">
            <p className="text-sm font-medium">What creates review work</p>
            <p className="mt-1 text-xs leading-5 text-muted-foreground">
              Items land here when confidence is high enough to recommend action but below the auto-execute threshold.
            </p>
          </div>
          <div className="rounded-lg border bg-muted/20 p-4">
            <p className="text-sm font-medium">Where to look next</p>
            <p className="mt-1 text-xs leading-5 text-muted-foreground">
              Use Insights to see raw buyer signals. Use thresholds when the queue is too noisy or too quiet.
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}

function StatCard({
  label,
  value,
  detail,
  icon: Icon,
  tone = 'neutral',
}: {
  label: string
  value: string | number
  detail: string
  icon: typeof SparklesIcon
  tone?: Tone
}) {
  return (
    <Card size="sm" className="rounded-lg py-4">
      <CardContent className="flex items-start justify-between gap-3 px-4">
        <div className="min-w-0">
          <p className="text-xs text-muted-foreground">{label}</p>
          <p className="mt-1 text-2xl font-semibold tabular-nums">{value}</p>
          <p className="mt-1 truncate text-xs text-muted-foreground">{detail}</p>
        </div>
        <div className={cn('flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border', toneClasses(tone))}>
          <Icon className="h-4 w-4" />
        </div>
      </CardContent>
    </Card>
  )
}

function QueueBreakdown({ counts }: { counts: Record<CRMSuggestionType, number> }) {
  const rows = Object.entries(counts)
    .map(([type, count]) => ({ type: type as CRMSuggestionType, count }))
    .filter((row) => row.count > 0)
    .sort((a, b) => b.count - a.count)

  return (
    <Card className="rounded-lg">
      <CardHeader className="pb-0">
        <CardTitle className="text-base">Queue Mix</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        {rows.length === 0 ? (
          <p className="py-3 text-sm text-muted-foreground">No pending categories.</p>
        ) : (
          rows.map((row) => (
            <div key={row.type} className="flex items-center justify-between gap-3 rounded-md border px-3 py-2">
              <span className="flex min-w-0 items-center gap-2">
                <span className={cn('h-2 w-2 shrink-0 rounded-full border', toneClasses(typeTone(row.type)))} />
                <span className="truncate text-sm">{typeLabels[row.type]}</span>
              </span>
              <Badge variant="secondary" className="font-normal tabular-nums">
                {row.count}
              </Badge>
            </div>
          ))
        )}
      </CardContent>
    </Card>
  )
}

function PrioritySnapshot({ suggestions }: { suggestions: CRMSuggestion[] }) {
  const top = suggestions[0]
  const oldest = suggestions.length
    ? suggestions.reduce((candidate, item) => (
      new Date(item.created_at).getTime() < new Date(candidate.created_at).getTime() ? item : candidate
    ), suggestions[0])
    : null

  return (
    <Card className="rounded-lg">
      <CardHeader className="pb-0">
        <CardTitle className="text-base">Priority Snapshot</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {top ? (
          <div className="rounded-lg border p-3">
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant="outline" className={cn('text-[11px]', toneClasses(typeTone(top.suggestion_type)))}>
                {typeLabels[top.suggestion_type]}
              </Badge>
              <span className="text-xs text-muted-foreground">{Math.round(top.confidence * 100)}% confidence</span>
            </div>
            <p className="mt-2 line-clamp-3 text-sm leading-5">{top.title}</p>
          </div>
        ) : (
          <p className="py-3 text-sm text-muted-foreground">No priority items.</p>
        )}

        {oldest ? (
          <div className="flex items-center gap-2 rounded-lg border bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
            <Clock03Icon className="h-3.5 w-3.5" />
            Oldest pending item: {timeAgo(oldest.created_at)}
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}

export function ReviewFeed() {
  useTitle('CRM Review')
  const navigate = useNavigate()
  const { currentWorkspace } = useWorkspaceStore()
  const wsId = currentWorkspace?.id ?? ''
  const wsSlug = currentWorkspace?.slug ?? ''
  const [activeTab, setActiveTab] = useState<FilterTab>('all')
  const [sortMode, setSortMode] = useState<SortMode>('recommended')
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(1)
  const deferredSearch = useDeferredValue(search.trim())

  const { data, isLoading, isFetching } = usePendingSuggestions(wsId, page)
  const acceptMutation = useAcceptSuggestion(wsId)
  const dismissMutation = useDismissSuggestion(wsId)

  const allSuggestions = useMemo(() => data?.data ?? [], [data?.data])
  const counts = useMemo<Record<CRMSuggestionType, number>>(() => ({
    deal_create: allSuggestions.filter((item) => item.suggestion_type === 'deal_create').length,
    deal_advance: allSuggestions.filter((item) => item.suggestion_type === 'deal_advance').length,
    follow_up: allSuggestions.filter((item) => item.suggestion_type === 'follow_up').length,
    enrichment: allSuggestions.filter((item) => item.suggestion_type === 'enrichment').length,
    risk_alert: allSuggestions.filter((item) => item.suggestion_type === 'risk_alert').length,
  }), [allSuggestions])

  const suggestions = useMemo(() => {
    const filtered = allSuggestions.filter((suggestion) => {
      const tabMatch = activeTab === 'all' || suggestion.suggestion_type === activeTab
      return tabMatch && matchesSearch(suggestion, deferredSearch)
    })
    return sortSuggestions(filtered, sortMode)
  }, [activeTab, allSuggestions, deferredSearch, sortMode])

  const hasQueue = allSuggestions.length > 0
  const totalSuggestions = data?.total ?? 0
  const pageCount = Math.max(1, Math.ceil(totalSuggestions / 20))
  const stats = useMemo(() => {
    const highConfidence = allSuggestions.filter((item) => item.confidence >= 0.9).length
    const riskAlerts = allSuggestions.filter((item) => item.suggestion_type === 'risk_alert').length
    const avgConfidence = allSuggestions.length
      ? Math.round((allSuggestions.reduce((sum, item) => sum + item.confidence, 0) / allSuggestions.length) * 100)
      : 0
    return { highConfidence, riskAlerts, avgConfidence }
  }, [allSuggestions])

  const handleAccept = (id: string) => {
    acceptMutation.mutate({ id }, {
      onSuccess: () => toast.success('Suggestion approved'),
      onError: (err) => toast.error(`Failed to approve: ${err.message}`),
    })
  }

  const handleDismiss = (id: string, reason: CRMSignalDismissalReason) => {
    dismissMutation.mutate({ id, reason }, {
      onSuccess: () => toast.success('Suggestion dismissed'),
      onError: (err) => toast.error(`Failed to dismiss: ${err.message}`),
    })
  }

  const goToInsights = () => {
    if (!wsSlug) return
    void navigate({ to: '/w/$slug/crm/insights', params: { slug: wsSlug } })
  }

  const goToEmailSettings = () => {
    if (!wsSlug) return
    void navigate({ to: '/w/$slug/settings/crm-email', params: { slug: wsSlug } })
  }

  const goToAutonomySettings = () => {
    if (!wsSlug) return
    void navigate({ to: '/w/$slug/settings/crm-autonomy', params: { slug: wsSlug } })
  }

  return (
    <div className="h-full overflow-auto">
      <div className="mx-auto flex w-full max-w-7xl flex-col gap-4 px-4 py-4 md:px-6">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <h1 className="text-xl font-semibold tracking-tight">Review</h1>
            <p className="text-sm text-muted-foreground">Review the evidence and preview the change before CRM records are updated.</p>
          </div>
          <Button variant="outline" size="sm" onClick={goToInsights} disabled={!wsSlug}>
            <SparklesIcon className="h-4 w-4" />
            Open insights
          </Button>
        </div>

        {isLoading || hasQueue ? (
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <StatCard
              label="Pending"
              value={isLoading ? '-' : totalSuggestions}
              detail={totalSuggestions === 1 ? '1 item in review' : `${totalSuggestions} items in review`}
              icon={InboxIcon}
              tone={allSuggestions.length > 0 ? 'accent' : 'neutral'}
            />
            <StatCard
              label="Strong recommendations"
              value={isLoading ? '-' : stats.highConfidence}
              detail="90% or higher on this page"
              icon={SparklesIcon}
              tone={stats.highConfidence > 0 ? 'good' : 'neutral'}
            />
            <StatCard
              label="Risk alerts"
              value={isLoading ? '-' : stats.riskAlerts}
              detail="Needs attention on this page"
              icon={Alert01Icon}
              tone={stats.riskAlerts > 0 ? 'danger' : 'neutral'}
            />
            <StatCard
              label="Avg. recommendation"
              value={isLoading || allSuggestions.length === 0 ? '-' : `${stats.avgConfidence}%`}
              detail="Current page"
              icon={ChartIncreaseIcon}
              tone="good"
            />
          </div>
        ) : null}

        {!isLoading && !hasQueue ? (
          <QueueClearState
            onOpenInsights={goToInsights}
            onOpenEmailSettings={goToEmailSettings}
            onOpenAutonomySettings={goToAutonomySettings}
          />
        ) : (
        <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_340px]">
          <main className="min-w-0 space-y-3">
            <div className="rounded-lg border bg-card p-3">
              <div className="flex flex-wrap items-center gap-2">
                <div className="relative min-w-64 flex-1">
                  <Search01Icon className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    value={search}
                    onChange={(event) => setSearch(event.target.value)}
                    placeholder="Search suggestions..."
                    className="h-8 pl-8 pr-8 text-xs"
                  />
                  {search ? (
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="absolute right-0 top-0 h-8 w-8 text-muted-foreground"
                      onClick={() => setSearch('')}
                    >
                      <Cancel01Icon className="h-3.5 w-3.5" />
                    </Button>
                  ) : null}
                </div>
                <div className="flex rounded-lg border bg-background p-0.5">
                  {sortOptions.map((option) => (
                    <button
                      key={option.key}
                      type="button"
                      onClick={() => setSortMode(option.key)}
                      className={cn(
                        'rounded-md px-2.5 py-1 text-xs font-medium transition-colors',
                        sortMode === option.key
                          ? 'bg-muted text-foreground'
                          : 'text-muted-foreground hover:text-foreground',
                      )}
                    >
                      {option.label}
                    </button>
                  ))}
                </div>
              </div>

              <div className="mt-3 flex gap-1 overflow-x-auto pb-0.5">
                {tabs.map((tab) => {
                  const Icon = tab.icon
                  const count = tab.key === 'all' ? allSuggestions.length : counts[tab.key]
                  const selected = activeTab === tab.key
                  return (
                    <button
                      key={tab.key}
                      type="button"
                      aria-pressed={selected}
                      onClick={() => setActiveTab(tab.key)}
                      className={cn(
                        'inline-flex h-8 shrink-0 items-center gap-1.5 rounded-md px-2.5 text-xs font-medium transition-colors',
                        selected
                          ? 'bg-foreground text-background'
                          : 'text-muted-foreground hover:bg-muted hover:text-foreground',
                      )}
                    >
                      <Icon className="h-3.5 w-3.5" />
                      {tab.label}
                      <span className={cn('tabular-nums', selected ? 'text-background/70' : 'text-muted-foreground/80')}>
                        {count}
                      </span>
                    </button>
                  )
                })}
              </div>
            </div>

            {isLoading ? (
              <div className="rounded-lg border bg-card p-10 text-center text-sm text-muted-foreground">
                Loading suggestions...
              </div>
            ) : suggestions.length === 0 ? (
              <FilterEmptyState
                activeTab={activeTab}
                hasSearch={deferredSearch.length > 0}
                onClearSearch={() => setSearch('')}
              />
            ) : (
              <div className="space-y-2">
                {suggestions.map((suggestion) => (
                  <SuggestionCard
                    key={suggestion.id}
                    suggestion={suggestion}
                    onAccept={handleAccept}
                    onDismiss={handleDismiss}
                    isAccepting={acceptMutation.isPending}
                    isDismissing={dismissMutation.isPending}
                  />
                ))}
                {pageCount > 1 ? (
                  <div className="flex items-center justify-between rounded-lg border bg-card px-3 py-3 text-xs text-muted-foreground">
                    <span>Page {page} of {pageCount}</span>
                    <div className="flex gap-2">
                      <Button variant="outline" size="sm" className="h-8" disabled={page <= 1 || isFetching} onClick={() => setPage((current) => Math.max(1, current - 1))}>Previous</Button>
                      <Button variant="outline" size="sm" className="h-8" disabled={page >= pageCount || isFetching} onClick={() => setPage((current) => Math.min(pageCount, current + 1))}>Next</Button>
                    </div>
                  </div>
                ) : null}
              </div>
            )}
          </main>

          <aside className="space-y-4 xl:sticky xl:top-4 xl:self-start">
            <PrioritySnapshot suggestions={sortSuggestions(allSuggestions, 'recommended')} />
            <QueueBreakdown counts={counts} />
          </aside>
        </div>
        )}
      </div>
    </div>
  )
}
