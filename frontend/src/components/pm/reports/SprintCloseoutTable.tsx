import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { SprintCloseoutListItem } from '@/lib/pmTypes'

function formatPercent(rate: number) {
  return `${Math.round(rate * 100)}%`
}

function formatDate(value?: string) {
  if (!value) return '—'
  return new Date(value).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
}

function formatCount(count: number, points: number, hasPoints: boolean) {
  if (!hasPoints) return String(count)
  return `${count} · ${points} pts`
}

export function SprintCloseoutTable({
  items,
  hasPoints = true,
  onSelectSprint,
  selectedSprintId,
}: {
  items: SprintCloseoutListItem[]
  hasPoints?: boolean
  onSelectSprint?: (sprintId: string) => void
  selectedSprintId?: string
}) {
  if (items.length === 0) {
    return (
      <div className="rounded-xl border border-dashed border-border/70 px-6 py-10 text-center text-sm text-muted-foreground">
        No sprint closeouts yet. Closeout data is captured when a sprint ends and rollover runs.
      </div>
    )
  }

  return (
    <div className="rounded-xl border border-border/70 bg-card">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Sprint</TableHead>
            <TableHead>Team</TableHead>
            <TableHead>Committed</TableHead>
            <TableHead>Completed</TableHead>
            <TableHead>Rolled over</TableHead>
            <TableHead>Completion</TableHead>
            <TableHead>Rolled to</TableHead>
            <TableHead>Closed</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((item) => {
            const isSelected = selectedSprintId === item.sprint_id
            return (
              <TableRow
                key={item.closeout_id}
                className={`${onSelectSprint ? 'cursor-pointer' : ''} ${isSelected ? 'bg-primary/5' : ''}`}
                onClick={() => onSelectSprint?.(item.sprint_id)}
              >
                <TableCell className="font-medium">{item.sprint_name}</TableCell>
                <TableCell>{item.team_name ?? '—'}</TableCell>
                <TableCell>{formatCount(item.committed_count, item.committed_points, hasPoints)}</TableCell>
                <TableCell>{formatCount(item.completed_count, item.completed_points, hasPoints)}</TableCell>
                <TableCell>{formatCount(item.rolled_over_count, item.rolled_over_points, hasPoints)}</TableCell>
                <TableCell>{formatPercent(item.completion_rate)}</TableCell>
                <TableCell>{item.rolled_to_sprint_name ?? '—'}</TableCell>
                <TableCell>{formatDate(item.closed_at)}</TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
