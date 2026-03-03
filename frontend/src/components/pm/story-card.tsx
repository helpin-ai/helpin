import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { PRIORITY_CONFIG, type Story } from '@/lib/types/pm'
import { cn } from '@/lib/utils'

interface StoryCardProps {
  story: Story
  isOverlay?: boolean
}

export function StoryCard({ story, isOverlay }: StoryCardProps) {
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
      className={cn(
        'rounded-lg border bg-background p-3 shadow-sm transition-shadow cursor-grab active:cursor-grabbing',
        isDragging && 'opacity-50',
        isOverlay && 'ring-2 ring-primary shadow-md',
        'hover:shadow-md'
      )}
    >
      {/* Top row: identifier + priority */}
      <div className="flex items-center justify-between mb-1">
        <span className="text-xs font-medium text-muted-foreground">
          {story.identifier}
        </span>
        {story.priority !== 'none' && (
          <span
            className="inline-block h-2.5 w-2.5 rounded-full"
            style={{ backgroundColor: priority.color }}
            title={priority.label}
          />
        )}
      </div>

      {/* Title */}
      <p className="text-sm font-medium leading-snug line-clamp-2">
        {story.title}
      </p>

      {/* Labels */}
      {story.labels.length > 0 && (
        <div className="flex flex-wrap gap-1 mt-2">
          {story.labels.map((label) => (
            <span
              key={label}
              className="inline-flex items-center rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground"
            >
              {label}
            </span>
          ))}
        </div>
      )}

      {/* Assignee */}
      {initials && (
        <div className="flex items-center mt-2">
          <span className="inline-flex h-5 w-5 items-center justify-center rounded-full bg-primary/10 text-[10px] font-semibold text-primary">
            {initials}
          </span>
          <span className="ml-1.5 text-xs text-muted-foreground">
            {story.assignee!.name}
          </span>
        </div>
      )}
    </div>
  )
}
