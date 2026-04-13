import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { SprintCloseoutListItem } from '@/lib/pmTypes'

function formatPercent(rate: number) {
  return `${Math.round(rate * 100)}%`
}

function formatDate(value?: string) {
  if (!value) return '—'
  return new Date(value).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
}

export function SprintCloseoutTable({ items }: { items: SprintCloseoutListItem[] }) {
  if (items.length === 0) {
    return (
      <div className="rounded-xl border border-dashed border-border/70 px-6 py-10 text-center text-sm text-muted-foreground">
        No sprint closeouts yet.
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
          {items.map((item) => (
            <TableRow key={item.closeout_id}>
              <TableCell className="font-medium">{item.sprint_name}</TableCell>
              <TableCell>{item.team_name ?? '—'}</TableCell>
              <TableCell>{item.committed_count} · {item.committed_points} pts</TableCell>
              <TableCell>{item.completed_count} · {item.completed_points} pts</TableCell>
              <TableCell>{item.rolled_over_count} · {item.rolled_over_points} pts</TableCell>
              <TableCell>{formatPercent(item.completion_rate)}</TableCell>
              <TableCell>{item.rolled_to_sprint_name ?? '—'}</TableCell>
              <TableCell>{formatDate(item.closed_at)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
