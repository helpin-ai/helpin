import { useMemo, useState } from 'react'
import { useTitle } from '@/hooks/useTitle'
import { useSprintCloseouts } from '@/hooks/queries'
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { ArrowLeft02Icon } from '@/lib/icons'
import { SprintCloseoutTable } from '@/components/pm/reports/SprintCloseoutTable'

type ReportView = 'landing' | 'sprint-closeouts'
type TimeRange = 'last5' | 'last10' | 'all'

const REPORT_CARDS = [
  {
    id: 'sprint-closeouts' as const,
    title: 'Sprint Closeouts',
    description: 'Committed vs completed vs rolled over per sprint. Frozen at sprint end so numbers never drift.',
    icon: '📊',
    available: true,
  },
  {
    id: 'velocity' as const,
    title: 'Team Velocity',
    description: 'Points completed per sprint across teams. Track trends over time.',
    icon: '🔄',
    available: false,
  },
  {
    id: 'cycle-time' as const,
    title: 'Cycle Time',
    description: 'Average time from start to completion per task type.',
    icon: '📈',
    available: false,
  },
]

const SUMMARY_TITLES = [
  { key: 'committed', label: 'Committed', countKey: 'committed_count', pointsKey: 'committed_points' },
  { key: 'completed', label: 'Completed', countKey: 'completed_count', pointsKey: 'completed_points' },
  { key: 'unfinished', label: 'Unfinished', countKey: 'unfinished_count', pointsKey: 'unfinished_points' },
  { key: 'rolled', label: 'Rolled over', countKey: 'rolled_over_count', pointsKey: 'rolled_over_points' },
] as const

const TIME_RANGES: { value: TimeRange; label: string }[] = [
  { value: 'last5', label: 'Last 5 sprints' },
  { value: 'last10', label: 'Last 10 sprints' },
  { value: 'all', label: 'All sprints' },
]

export function ReportsPage() {
  useTitle('Reports')
  const [view, setView] = useState<ReportView>('landing')

  if (view === 'sprint-closeouts') {
    return <SprintCloseoutsReport onBack={() => setView('landing')} />
  }

  return (
    <div className="max-w-7xl mx-auto flex w-full flex-col gap-6">
      <div>
        <h1 className="text-lg font-semibold">Reports</h1>
        <p className="text-sm text-muted-foreground">
          Track team performance, sprint outcomes, and delivery trends.
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {REPORT_CARDS.map((card) => (
          <button
            key={card.id}
            type="button"
            disabled={!card.available}
            onClick={() => card.available && setView(card.id as ReportView)}
            className={`group rounded-xl border p-5 text-left transition-all ${
              card.available
                ? 'border-border/60 bg-card hover:border-border hover:shadow-sm cursor-pointer'
                : 'border-border/40 bg-muted/20 opacity-60 cursor-default'
            }`}
          >
            <div className="text-2xl mb-3">{card.icon}</div>
            <h3 className="text-sm font-semibold">{card.title}</h3>
            <p className="mt-1 text-xs text-muted-foreground leading-relaxed">
              {card.description}
            </p>
            {!card.available && (
              <span className="mt-3 inline-block rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground">
                Coming soon
              </span>
            )}
          </button>
        ))}
      </div>
    </div>
  )
}

function SprintCloseoutsReport({ onBack }: { onBack: () => void }) {
  const workspace = useWorkspaceStore((state) => state.currentWorkspace)
  const workspaceId = workspace?.id ?? ''
  const { teams } = useAccessibleTeams(workspaceId)
  const [teamId, setTeamId] = useState('')
  const [timeRange, setTimeRange] = useState<TimeRange>('last5')
  const [selectedSprintId, setSelectedSprintId] = useState('')

  const closeoutsQuery = useSprintCloseouts(workspaceId, { team_id: teamId || undefined })
  const allItems = closeoutsQuery.data?.items ?? []

  // Apply time range filter (items are assumed sorted by closed_at desc)
  const rangedItems = useMemo(() => {
    if (timeRange === 'all') return allItems
    const limit = timeRange === 'last5' ? 5 : 10
    return allItems.slice(0, limit)
  }, [allItems, timeRange])

  // If a specific sprint is selected, show only that one
  const displayItems = selectedSprintId
    ? rangedItems.filter((item) => item.sprint_id === selectedSprintId)
    : rangedItems

  // Summary aggregates from the displayed items
  const summary = useMemo(() => {
    const base = {
      committed_count: 0,
      committed_points: 0,
      completed_count: 0,
      completed_points: 0,
      unfinished_count: 0,
      unfinished_points: 0,
      rolled_over_count: 0,
      rolled_over_points: 0,
    }
    return displayItems.reduce((acc, item) => ({
      committed_count: acc.committed_count + item.committed_count,
      committed_points: acc.committed_points + item.committed_points,
      completed_count: acc.completed_count + item.completed_count,
      completed_points: acc.completed_points + item.completed_points,
      unfinished_count: acc.unfinished_count + item.unfinished_count,
      unfinished_points: acc.unfinished_points + item.unfinished_points,
      rolled_over_count: acc.rolled_over_count + item.rolled_over_count,
      rolled_over_points: acc.rolled_over_points + item.rolled_over_points,
    }), base)
  }, [displayItems])

  // Hide points entirely when no tasks have estimates
  const hasPoints = summary.committed_points > 0

  // Completion rate for the selected view
  const completionRate = summary.committed_count > 0
    ? Math.round((summary.completed_count / summary.committed_count) * 100)
    : 0

  return (
    <div className="max-w-7xl mx-auto flex w-full flex-col gap-6">
      {/* Header */}
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0" onClick={onBack}>
            <ArrowLeft02Icon className="h-4 w-4" />
          </Button>
          <div>
            <h1 className="text-lg font-semibold">Sprint Closeouts</h1>
            <p className="text-sm text-muted-foreground">
              {selectedSprintId
                ? `Showing ${displayItems[0]?.sprint_name ?? 'selected sprint'}`
                : `${displayItems.length} sprint${displayItems.length === 1 ? '' : 's'} · ${completionRate}% overall completion`}
            </p>
          </div>
        </div>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-3">
        <label className="flex flex-col gap-1 text-sm">
          <span className="text-xs text-muted-foreground">Team</span>
          <select
            value={teamId}
            onChange={(event) => { setTeamId(event.target.value); setSelectedSprintId(''); }}
            className="h-8 rounded-md border border-border bg-background px-2.5 text-xs"
          >
            <option value="">All teams</option>
            {teams.map((team) => (
              <option key={team.id} value={team.id}>{team.name}</option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1 text-sm">
          <span className="text-xs text-muted-foreground">Period</span>
          <select
            value={timeRange}
            onChange={(event) => { setTimeRange(event.target.value as TimeRange); setSelectedSprintId(''); }}
            className="h-8 rounded-md border border-border bg-background px-2.5 text-xs"
          >
            {TIME_RANGES.map((range) => (
              <option key={range.value} value={range.value}>{range.label}</option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1 text-sm">
          <span className="text-xs text-muted-foreground">Sprint</span>
          <select
            value={selectedSprintId}
            onChange={(event) => setSelectedSprintId(event.target.value)}
            className="h-8 rounded-md border border-border bg-background px-2.5 text-xs"
          >
            <option value="">All in range</option>
            {rangedItems.map((item) => (
              <option key={item.sprint_id} value={item.sprint_id}>{item.sprint_name}</option>
            ))}
          </select>
        </label>

        {selectedSprintId && (
          <button
            type="button"
            onClick={() => setSelectedSprintId('')}
            className="self-end mb-0.5 text-xs text-primary hover:underline"
          >
            Clear sprint filter
          </button>
        )}
      </div>

      {/* Summary cards */}
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        {SUMMARY_TITLES.map((metric) => (
          <Card key={metric.key} size="sm" className="border-border/60">
            <CardHeader className="pb-0">
              <CardTitle className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {metric.label}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-1">
              <div className="text-2xl font-semibold">
                {summary[metric.countKey]}
                <span className="text-sm font-normal text-muted-foreground ml-1">
                  {summary[metric.countKey] === 1 ? 'task' : 'tasks'}
                </span>
              </div>
              {hasPoints && (
                <p className="text-xs text-muted-foreground">{summary[metric.pointsKey]} pts</p>
              )}
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Table */}
      {closeoutsQuery.isLoading ? (
        <div className="rounded-xl border border-dashed border-border/70 px-6 py-10 text-center text-sm text-muted-foreground">
          Loading sprint closeouts...
        </div>
      ) : (
        <SprintCloseoutTable
          items={displayItems}
          hasPoints={hasPoints}
          onSelectSprint={(sprintId) => setSelectedSprintId(sprintId === selectedSprintId ? '' : sprintId)}
          selectedSprintId={selectedSprintId}
        />
      )}
    </div>
  )
}
