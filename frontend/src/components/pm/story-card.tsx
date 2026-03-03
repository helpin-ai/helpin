import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { CircleCheck, CircleDashed, CircleDot, MinusCircle } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { PRIORITY_CONFIG, type Story } from '@/lib/types/pm'
import { cn } from '@/lib/utils'

interface StoryCardProps {
  story: Story
  isOverlay?: boolean
  onOpen?: (story: Story) => void
}

const statusVisual: Record<
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

export function StoryCard({ story, isOverlay, onOpen }: StoryCardProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: story.id })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  }

  const priority = PRIORITY_CONFIG[story.priority]
  const status = statusVisual[story.status]
  const StatusIcon = status.icon
  const initials = story.assignee
    ? story.assignee.name
        .split(' ')
        .map((n) => n[0])
        .join('')
    : null

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...attributes}
      {...listeners}
      role="button"
      tabIndex={0}
      onClick={() => onOpen?.(story)}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault()
          onOpen?.(story)
        }
      }}
      className={cn(
        'rounded-lg border border-border/80 bg-background p-3 shadow-sm transition-all cursor-grab active:cursor-grabbing',
        isDragging && 'opacity-50',
        isOverlay && 'ring-1 ring-primary/40 shadow-md',
        'hover:border-border hover:shadow-md'
      )}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="text-[11px] font-medium text-muted-foreground">
          {story.identifier}
        </span>
        {story.priority !== 'none' && (
          <span
            className="inline-block h-2 w-2 rounded-full"
            style={{ backgroundColor: priority.color }}
            title={priority.label}
          />
        )}
      </div>

      <p className="mt-1 text-[13px] font-medium leading-snug line-clamp-2">
        {story.title}
      </p>

      <div className="mt-2 flex items-center gap-1.5">
        <Badge variant="outline" className="h-6 rounded-sm px-2 text-[11px] font-medium">
          <StatusIcon className={cn('h-3.5 w-3.5', status.className)} />
          {status.label}
        </Badge>
        {story.labels.slice(0, 1).map((label) => (
          <Badge
            key={label}
            variant="outline"
            className="h-6 rounded-sm px-2 text-[11px] text-muted-foreground"
          >
            {label}
          </Badge>
        ))}
        {initials && (
          <span className="ml-auto inline-flex h-5 w-5 items-center justify-center rounded-full border border-border/80 bg-muted/40 text-[10px] font-semibold text-muted-foreground">
            {initials}
          </span>
        )}
      </div>
    </div>
  )
}
