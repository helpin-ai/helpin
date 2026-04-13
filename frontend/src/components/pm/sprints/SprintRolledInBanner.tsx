import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import type { SprintInboundRolloverSummary } from '@/lib/pmTypes'

export function SprintRolledInBanner({
  items,
  onOpenSprint,
}: {
  items: SprintInboundRolloverSummary[]
  onOpenSprint?: (sprintId: string) => void
}) {
  const totalTasks = items.reduce((sum, item) => sum + item.rolled_over_count, 0)
  const totalPoints = items.reduce((sum, item) => sum + item.rolled_over_points, 0)

  return (
    <div className="rounded-xl border border-blue-200/70 bg-blue-50/70 px-4 py-3 text-sm text-blue-950">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="space-y-1">
          <p className="font-medium">
            {totalTasks} tasks rolled into this sprint
            <span className="ml-2 text-blue-800/80">{totalPoints} pts</span>
          </p>
          <div className="flex flex-wrap gap-2">
            {items.map((item) => (
              onOpenSprint ? (
                <Button
                  key={item.source_sprint_id}
                  type="button"
                  variant="outline"
                  size="sm"
                  className="h-7 border-blue-200 bg-white/80 px-2 text-xs text-blue-900 hover:bg-white"
                  onClick={() => onOpenSprint(item.source_sprint_id)}
                >
                  {item.source_sprint_name}
                  <Badge variant="outline" className="ml-1 border-blue-200 bg-blue-100 text-blue-900">
                    {item.rolled_over_count}
                  </Badge>
                </Button>
              ) : (
                <span key={item.source_sprint_id} className="inline-flex items-center gap-1 rounded-full border border-blue-200 bg-white/80 px-2 py-1 text-xs">
                  {item.source_sprint_name}
                  <Badge variant="outline" className="border-blue-200 bg-blue-100 text-blue-900">
                    {item.rolled_over_count}
                  </Badge>
                </span>
              )
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
