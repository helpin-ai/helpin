import { Link01Icon, PlusSignIcon } from '@/lib/icons'
import { Button } from '@/components/ui/button'

interface SprintPlanningColumnActionsProps {
  linkTasksDisabledReason?: string | null
  onLinkTasks: () => void
  onCreateTask: () => void
}

export function SprintPlanningColumnActions({
  linkTasksDisabledReason,
  onLinkTasks,
  onCreateTask,
}: SprintPlanningColumnActionsProps) {
  return (
    <div className="mt-auto grid grid-cols-2 gap-1">
      <Button
        variant="ghost"
        size="sm"
        className="gap-1.5 px-2 text-muted-foreground"
        disabled={Boolean(linkTasksDisabledReason)}
        title={linkTasksDisabledReason ?? undefined}
        onClick={onLinkTasks}
      >
        <Link01Icon className="h-4 w-4" />
        Link tasks
      </Button>
      <Button
        variant="ghost"
        size="sm"
        className="gap-1.5 px-2 text-muted-foreground"
        onClick={onCreateTask}
      >
        <PlusSignIcon className="h-4 w-4" />
        Create task
      </Button>
    </div>
  )
}
