import { useDroppable } from '@dnd-kit/core'
import {
  SortableContext,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CircleDot, Plus } from 'lucide-react'
import type { Story, StoryStatus } from '@/lib/types/pm'
import { StoryCard } from './story-card'

interface KanbanColumnProps {
  id: StoryStatus
  label: string
  colorClass: string
  stories: Story[]
  onAddStory: (status: StoryStatus) => void
  onOpenStory: (story: Story) => void
}

export function KanbanColumn({
  id,
  label,
  colorClass,
  stories,
  onAddStory,
  onOpenStory,
}: KanbanColumnProps) {
  const { setNodeRef, isOver } = useDroppable({ id })

  const sortableIds = stories.map((s) => s.id)

  return (
    <div className="flex w-[350px] flex-shrink-0 flex-col px-1.5">
      <div className="sticky top-0 z-[2] mb-1 flex items-center justify-between py-1">
        <div className="flex items-center gap-1.5">
          <CircleDot className={`h-3.5 w-3.5 ${colorClass}`} />
          <h3 className="text-sm font-medium leading-none">{label}</h3>
          <span className="pl-1 text-xs font-medium leading-none text-muted-foreground">{stories.length}</span>
        </div>
        <button
          type="button"
          aria-label={`Add story to ${label}`}
          onClick={() => onAddStory(id)}
          className="rounded-sm p-0.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
        >
          <Plus className="h-3.5 w-3.5" />
        </button>
      </div>

      <SortableContext items={sortableIds} strategy={verticalListSortingStrategy}>
        <div
          ref={setNodeRef}
          className={`flex-1 space-y-2 overflow-y-auto pb-3 transition-colors ${
            isOver ? 'rounded-sm bg-primary/5' : ''
          }`}
        >
          {stories.map((story) => (
            <StoryCard key={story.id} story={story} onOpen={onOpenStory} />
          ))}
          <button
            type="button"
            onClick={() => onAddStory(id)}
            className="mt-1 flex w-full items-center gap-1.5 rounded-sm px-2 py-1.5 text-left text-xs text-muted-foreground transition-colors hover:bg-muted/50 hover:text-foreground"
          >
            <Plus className="h-3.5 w-3.5" />
            <span>New work item</span>
          </button>
        </div>
      </SortableContext>
    </div>
  )
}
