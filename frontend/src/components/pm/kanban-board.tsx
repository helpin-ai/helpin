import { useState, useMemo, useCallback } from 'react'
import {
  DndContext,
  DragOverlay,
  closestCorners,
  PointerSensor,
  useSensor,
  useSensors,
  type DragStartEvent,
  type DragEndEvent,
  type DragOverEvent,
} from '@dnd-kit/core'
import { arrayMove } from '@dnd-kit/sortable'
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { STATUS_COLUMNS, type Story, type StoryStatus } from '@/lib/types/pm'
import { MOCK_STORIES } from '@/lib/mock/stories'
import { KanbanColumn } from './kanban-column'
import { StoryCard } from './story-card'
import { CreateStoryDialog } from './create-story-dialog'

export function KanbanBoard() {
  const [stories, setStories] = useState<Story[]>(MOCK_STORIES)
  const [activeStory, setActiveStory] = useState<Story | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [dialogDefaultStatus, setDialogDefaultStatus] =
    useState<StoryStatus>('backlog')

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } })
  )

  const storiesByStatus = useMemo(() => {
    const grouped: Record<StoryStatus, Story[]> = {
      backlog: [],
      todo: [],
      in_progress: [],
      done: [],
    }
    for (const story of stories) {
      grouped[story.status].push(story)
    }
    // Sort within each group by sort_order
    for (const key of Object.keys(grouped) as StoryStatus[]) {
      grouped[key].sort((a, b) => a.sort_order - b.sort_order)
    }
    return grouped
  }, [stories])

  const findColumnForStory = useCallback(
    (storyId: string): StoryStatus | null => {
      const story = stories.find((s) => s.id === storyId)
      if (story) return story.status
      // Check if the id is a column id
      if (
        ['backlog', 'todo', 'in_progress', 'done'].includes(
          storyId as StoryStatus
        )
      ) {
        return storyId as StoryStatus
      }
      return null
    },
    [stories]
  )

  const handleDragStart = useCallback(
    (event: DragStartEvent) => {
      const story = stories.find((s) => s.id === String(event.active.id))
      setActiveStory(story ?? null)
    },
    [stories]
  )

  const handleDragOver = useCallback(
    (event: DragOverEvent) => {
      const { active, over } = event
      if (!over) return

      const activeId = String(active.id)
      const overId = String(over.id)

      const activeColumn = findColumnForStory(activeId)
      let overColumn = findColumnForStory(overId)

      // If hovering over a column directly
      if (
        ['backlog', 'todo', 'in_progress', 'done'].includes(
          overId as StoryStatus
        )
      ) {
        overColumn = overId as StoryStatus
      }

      if (!activeColumn || !overColumn || activeColumn === overColumn) return

      setStories((prev) => {
        const updated = prev.map((s) =>
          s.id === activeId
            ? {
                ...s,
                status: overColumn,
                sort_order: storiesByStatus[overColumn].length,
              }
            : s
        )
        return updated
      })
    },
    [findColumnForStory, storiesByStatus]
  )

  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      const { active, over } = event
      setActiveStory(null)

      if (!over) return

      const activeId = String(active.id)
      const overId = String(over.id)

      if (activeId === overId) return

      const activeColumn = findColumnForStory(activeId)
      let overColumn = findColumnForStory(overId)

      if (
        ['backlog', 'todo', 'in_progress', 'done'].includes(
          overId as StoryStatus
        )
      ) {
        overColumn = overId as StoryStatus
      }

      if (!activeColumn || !overColumn) return

      // Same column reorder
      if (activeColumn === overColumn) {
        const columnStories = storiesByStatus[activeColumn]
        const oldIndex = columnStories.findIndex((s) => s.id === activeId)
        const newIndex = columnStories.findIndex((s) => s.id === overId)
        if (oldIndex === -1 || newIndex === -1) return

        const reordered = arrayMove(columnStories, oldIndex, newIndex)
        setStories((prev) => {
          const otherStories = prev.filter((s) => s.status !== activeColumn)
          const updated = reordered.map((s, i) => ({ ...s, sort_order: i }))
          return [...otherStories, ...updated]
        })
      }
    },
    [findColumnForStory, storiesByStatus]
  )

  const handleAddStory = useCallback((status: StoryStatus) => {
    setDialogDefaultStatus(status)
    setDialogOpen(true)
  }, [])

  const handleCreateStory = useCallback(
    (story: Story) => {
      const maxOrder = storiesByStatus[story.status].length
      setStories((prev) => [...prev, { ...story, sort_order: maxOrder }])
    },
    [storiesByStatus]
  )

  const nextIdentifier = useMemo(() => {
    const maxNum = stories.reduce((max, s) => {
      const num = parseInt(s.identifier.split('-')[1], 10)
      return num > max ? num : max
    }, 0)
    return `CS-${maxNum + 1}`
  }, [stories])

  return (
    <div className="flex h-full flex-col">
      {/* Toolbar */}
      <div className="flex items-center gap-3 pb-4">
        <Button size="sm" onClick={() => handleAddStory('backlog')}>
          <Plus className="mr-1 h-4 w-4" />
          Add Story
        </Button>
      </div>

      {/* Board */}
      <DndContext
        sensors={sensors}
        collisionDetection={closestCorners}
        onDragStart={handleDragStart}
        onDragOver={handleDragOver}
        onDragEnd={handleDragEnd}
      >
        <div className="flex flex-1 gap-4 overflow-x-auto pb-4">
          {STATUS_COLUMNS.map((col) => (
            <KanbanColumn
              key={col.id}
              id={col.id}
              label={col.label}
              color={col.color}
              stories={storiesByStatus[col.id]}
              onAddStory={handleAddStory}
            />
          ))}
        </div>

        <DragOverlay>
          {activeStory ? (
            <StoryCard story={activeStory} isOverlay />
          ) : null}
        </DragOverlay>
      </DndContext>

      {/* Create Story Dialog */}
      <CreateStoryDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        defaultStatus={dialogDefaultStatus}
        nextIdentifier={nextIdentifier}
        onCreateStory={handleCreateStory}
      />
    </div>
  )
}
