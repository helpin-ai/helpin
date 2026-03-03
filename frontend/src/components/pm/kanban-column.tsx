import { useDroppable } from '@dnd-kit/core'
import {
  SortableContext,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { Plus } from 'lucide-react'
import type { Story, StoryStatus } from '@/lib/types/pm'
import { StoryCard } from './story-card'

interface KanbanColumnProps {
  id: StoryStatus
  label: string
  color: string
  stories: Story[]
  onAddStory: (status: StoryStatus) => void
}

export function KanbanColumn({
  id,
  label,
  color,
  stories,
  onAddStory,
}: KanbanColumnProps) {
  const { setNodeRef, isOver } = useDroppable({ id })

  const sortableIds = stories.map((s) => s.id)

  return (
    <div className="flex min-w-[280px] w-[280px] flex-col rounded-lg bg-muted/30">
      {/* Column header */}
      <div className="flex items-center justify-between px-3 py-2.5">
        <div className="flex items-center gap-2">
          <span
            className="inline-block h-2.5 w-2.5 rounded-full"
            style={{ backgroundColor: color }}
          />
          <span className="text-sm font-medium">{label}</span>
          <span className="text-xs text-muted-foreground">
            {stories.length}
          </span>
        </div>
        <button
          onClick={() => onAddStory(id)}
          className="rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
        >
          <Plus className="h-4 w-4" />
        </button>
      </div>

      {/* Droppable card list */}
      <SortableContext items={sortableIds} strategy={verticalListSortingStrategy}>
        <div
          ref={setNodeRef}
          className={`flex-1 space-y-2 overflow-y-auto px-2 pb-2 min-h-[80px] rounded-b-lg transition-colors ${
            isOver ? 'bg-primary/5' : ''
          }`}
        >
          {stories.map((story) => (
            <StoryCard key={story.id} story={story} />
          ))}
        </div>
      </SortableContext>
    </div>
  )
}
