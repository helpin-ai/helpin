import type { SprintCloseout } from '@/lib/pmTypes'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

const METRICS = [
  {
    key: 'committed',
    label: 'Committed',
    countKey: 'committed_count',
    pointsKey: 'committed_points',
    tone: 'text-foreground',
  },
  {
    key: 'completed',
    label: 'Completed',
    countKey: 'completed_count',
    pointsKey: 'completed_points',
    tone: 'text-emerald-600',
  },
  {
    key: 'unfinished',
    label: 'Unfinished',
    countKey: 'unfinished_count',
    pointsKey: 'unfinished_points',
    tone: 'text-amber-600',
  },
  {
    key: 'rolled',
    label: 'Rolled over',
    countKey: 'rolled_over_count',
    pointsKey: 'rolled_over_points',
    tone: 'text-blue-600',
  },
] as const

export function SprintCloseoutSummary({ closeout }: { closeout: SprintCloseout }) {
  return (
    <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
      {METRICS.map((metric) => (
        <Card key={metric.key} size="sm" className="border-border/60">
          <CardHeader className="pb-0">
            <CardTitle className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              {metric.label}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-1">
            <div className={`text-2xl font-semibold ${metric.tone}`}>
              {closeout[metric.countKey]}
            </div>
            <p className="text-xs text-muted-foreground">{closeout[metric.pointsKey]} pts</p>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
