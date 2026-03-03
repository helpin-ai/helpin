import { format } from 'date-fns'
import {
  ArrowUpRight,
  CircleCheck,
  CircleDashed,
  CircleDot,
  MinusCircle,
  UserRound,
} from 'lucide-react'
import type { Story } from '@/lib/types/pm'
import { PRIORITY_CONFIG } from '@/lib/types/pm'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { cn } from '@/lib/utils'

interface StoryDetailSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  story: Story | null
}

const statusConfig: Record<
  Story['status'],
  { icon: typeof CircleDashed; label: string; className: string }
> = {
  backlog: {
    icon: CircleDashed,
    label: 'Backlog',
    className: 'text-muted-foreground',
  },
  todo: {
    icon: CircleDot,
    label: 'Todo',
    className: 'text-amber-600',
  },
  in_progress: {
    icon: MinusCircle,
    label: 'In Progress',
    className: 'text-sky-600',
  },
  done: {
    icon: CircleCheck,
    label: 'Done',
    className: 'text-emerald-600',
  },
}

export function StoryDetailSheet({
  open,
  onOpenChange,
  story,
}: StoryDetailSheetProps) {
  if (!story) return null

  const status = statusConfig[story.status]
  const StatusIcon = status.icon
  const priority = PRIORITY_CONFIG[story.priority]

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-[86vw] gap-0 p-0 sm:max-w-[760px]">
        <SheetHeader className="gap-3 border-b border-border/70 px-5 pt-4 pb-3">
          <div className="flex items-center justify-between pr-10">
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium text-muted-foreground">{story.identifier}</span>
              <Badge variant="outline" className="h-6 rounded-sm px-2 text-xs font-medium">
                <StatusIcon className={cn('h-3.5 w-3.5', status.className)} />
                {status.label}
              </Badge>
            </div>
            <Button variant="ghost" size="icon-sm" className="text-muted-foreground">
              <ArrowUpRight className="h-4 w-4" />
            </Button>
          </div>
          <SheetTitle className="text-3xl font-semibold leading-tight">
            {story.title}
          </SheetTitle>
          <SheetDescription className="text-base">
            {story.description?.trim() || 'No description yet.'}
          </SheetDescription>
        </SheetHeader>

        <div className="flex-1 overflow-y-auto px-5 py-4">
          <div className="grid grid-cols-2 gap-x-6 gap-y-3 border-b border-border/70 pb-4 text-sm">
            <div>
              <p className="text-xs uppercase tracking-wide text-muted-foreground">Status</p>
              <p className="mt-1 font-medium">{status.label}</p>
            </div>
            <div>
              <p className="text-xs uppercase tracking-wide text-muted-foreground">Priority</p>
              <div className="mt-1 flex items-center gap-2">
                <span
                  className="h-2 w-2 rounded-full"
                  style={{ backgroundColor: priority.color }}
                />
                <span className="font-medium">{priority.label}</span>
              </div>
            </div>
            <div>
              <p className="text-xs uppercase tracking-wide text-muted-foreground">Assignee</p>
              <p className="mt-1 font-medium">{story.assignee?.name || 'Unassigned'}</p>
            </div>
            <div>
              <p className="text-xs uppercase tracking-wide text-muted-foreground">Created</p>
              <p className="mt-1 font-medium">{format(new Date(story.created_at), 'MMM d, yyyy')}</p>
            </div>
          </div>

          <section className="pt-4">
            <h3 className="mb-3 text-base font-semibold">Labels</h3>
            {story.labels.length === 0 ? (
              <p className="text-sm text-muted-foreground">No labels added.</p>
            ) : (
              <div className="flex flex-wrap gap-2">
                {story.labels.map((label) => (
                  <Badge key={label} variant="outline" className="rounded-sm px-2 py-1 text-xs">
                    {label}
                  </Badge>
                ))}
              </div>
            )}
          </section>

          <section className="pt-6">
            <h3 className="mb-3 text-base font-semibold">Activity</h3>
            <div className="rounded-sm border border-border/70 px-3 py-3 text-sm text-muted-foreground">
              <div className="flex items-start gap-2">
                <UserRound className="mt-0.5 h-4 w-4 shrink-0" />
                <p>
                  Activity stream will appear here after backend story events are connected.
                </p>
              </div>
            </div>
          </section>
        </div>
      </SheetContent>
    </Sheet>
  )
}
