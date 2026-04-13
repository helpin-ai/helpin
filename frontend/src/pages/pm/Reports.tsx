import { useMemo, useState } from 'react'
import { useTitle } from '@/hooks/useTitle'
import { useSprintCloseouts } from '@/hooks/queries'
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { SprintCloseoutTable } from '@/components/pm/reports/SprintCloseoutTable'

const SUMMARY_TITLES = [
  { key: 'committed', label: 'Committed', countKey: 'committed_count', pointsKey: 'committed_points' },
  { key: 'completed', label: 'Completed', countKey: 'completed_count', pointsKey: 'completed_points' },
  { key: 'unfinished', label: 'Unfinished', countKey: 'unfinished_count', pointsKey: 'unfinished_points' },
  { key: 'rolled', label: 'Rolled over', countKey: 'rolled_over_count', pointsKey: 'rolled_over_points' },
] as const

export function ReportsPage() {
  useTitle('Reports')

  const workspace = useWorkspaceStore((state) => state.currentWorkspace)
  const workspaceId = workspace?.id ?? ''
  const { teams } = useAccessibleTeams(workspaceId)
  const [teamId, setTeamId] = useState('')

  const closeoutsQuery = useSprintCloseouts(workspaceId, { team_id: teamId || undefined })
  const items = closeoutsQuery.data?.items ?? []

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
    return items.reduce((acc, item) => ({
      committed_count: acc.committed_count + item.committed_count,
      committed_points: acc.committed_points + item.committed_points,
      completed_count: acc.completed_count + item.completed_count,
      completed_points: acc.completed_points + item.completed_points,
      unfinished_count: acc.unfinished_count + item.unfinished_count,
      unfinished_points: acc.unfinished_points + item.unfinished_points,
      rolled_over_count: acc.rolled_over_count + item.rolled_over_count,
      rolled_over_points: acc.rolled_over_points + item.rolled_over_points,
    }), base)
  }, [items])

  return (
    <div className="mx-auto flex h-full w-full max-w-7xl flex-col gap-6 px-4 py-6 md:px-6">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Sprint reports</h1>
          <p className="text-sm text-muted-foreground">
            Frozen sprint closeouts preserve completed, unfinished, and rolled-over work after rollover runs.
          </p>
        </div>
        <label className="flex flex-col gap-1 text-sm">
          <span className="text-muted-foreground">Team</span>
          <select
            value={teamId}
            onChange={(event) => setTeamId(event.target.value)}
            className="h-9 rounded-md border border-border bg-background px-3 text-sm"
          >
            <option value="">All accessible teams</option>
            {teams.map((team) => (
              <option key={team.id} value={team.id}>{team.name}</option>
            ))}
          </select>
        </label>
      </div>

      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        {SUMMARY_TITLES.map((metric) => (
          <Card key={metric.key} size="sm" className="border-border/60">
            <CardHeader className="pb-0">
              <CardTitle className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {metric.label}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-1">
              <div className="text-2xl font-semibold">{summary[metric.countKey]}</div>
              <p className="text-xs text-muted-foreground">{summary[metric.pointsKey]} pts</p>
            </CardContent>
          </Card>
        ))}
      </div>

      {closeoutsQuery.isLoading ? (
        <div className="rounded-xl border border-dashed border-border/70 px-6 py-10 text-center text-sm text-muted-foreground">
          Loading sprint closeouts...
        </div>
      ) : (
        <SprintCloseoutTable items={items} />
      )}
    </div>
  )
}
