import { useState, useMemo, useCallback, useEffect } from 'react'
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  closestCorners,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragOverEvent,
  type DragStartEvent,
} from '@dnd-kit/core'
import { arrayMove } from '@dnd-kit/sortable'
import {
  Columns2,
  LayoutList,
  SlidersHorizontal,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { STATUS_COLUMNS, type Story, type StoryStatus } from '@/lib/types/pm'
import { MOCK_STORIES } from '@/lib/mock/stories'
import { KanbanColumn } from './kanban-column'
import { StoryCard } from './story-card'
import { CreateStoryDialog } from './create-story-dialog'
import { StoryDetailSheet } from './story-detail-sheet'

const statusColorClass: Record<StoryStatus, string> = {
  backlog: 'text-muted-foreground',
  todo: 'text-amber-600',
  in_progress: 'text-sky-600',
  done: 'text-emerald-600',
}

const isStatusId = (value: string): value is StoryStatus =>
  value === 'backlog' || value === 'todo' || value === 'in_progress' || value === 'done'

export function KanbanBoard() {
  const { currentWorkspace } = useWorkspaceStore()
  const [stories, setStories] = useState<Story[]>(MOCK_STORIES)
  const [activeStory, setActiveStory] = useState<Story | null>(null)
  const [selectedStory, setSelectedStory] = useState<Story | null>(null)
  const [storySheetOpen, setStorySheetOpen] = useState(false)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [dialogDefaultStatus, setDialogDefaultStatus] = useState<StoryStatus>('backlog')

  // Listen for "add-work-item" event from the Header button
  useEffect(() => {
    const handler = () => {
      setDialogDefaultStatus('backlog')
      setDialogOpen(true)
    }
    window.addEventListener('add-work-item', handler)
    return () => window.removeEventListener('add-work-item', handler)
  }, [])

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 8 } }))

  const storiesByStatus = useMemo(() => {
    const grouped: Record<StoryStatus, Story[]> = {
      backlog: [],
      todo: [],
      in_progress: [],
      done: [],
    }
    for (const story of stories) grouped[story.status].push(story)
    for (const key of Object.keys(grouped) as StoryStatus[]) {
      grouped[key].sort((a, b) => a.sort_order - b.sort_order)
    }
    return grouped
  }, [stories])

  const findColumnForStory = useCallback(
    (storyId: string): StoryStatus | null => {
      const story = stories.find((item) => item.id === storyId)
      if (story) return story.status
      if (isStatusId(storyId)) return storyId
      return null
    },
    [stories]
  )

  const handleOpenStory = useCallback((story: Story) => {
    setSelectedStory(story)
    setStorySheetOpen(true)
  }, [])

  const handleDragStart = useCallback(
    (event: DragStartEvent) => {
      const story = stories.find((item) => item.id === String(event.active.id))
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
      const overColumn = findColumnForStory(overId)

      if (!activeColumn || !overColumn || activeColumn === overColumn) return

      setStories((prev) =>
        prev.map((story) =>
          story.id === activeId
            ? {
                ...story,
                status: overColumn,
                sort_order: storiesByStatus[overColumn].length,
              }
            : story
        )
      )
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
      const overColumn = findColumnForStory(overId)
      if (!activeColumn || !overColumn || activeColumn !== overColumn) return

      const columnStories = storiesByStatus[activeColumn]
      const oldIndex = columnStories.findIndex((story) => story.id === activeId)
      const newIndex = columnStories.findIndex((story) => story.id === overId)
      if (oldIndex === -1 || newIndex === -1) return

      const reordered = arrayMove(columnStories, oldIndex, newIndex)
      setStories((prev) => {
        const otherStories = prev.filter((story) => story.status !== activeColumn)
        const updated = reordered.map((story, index) => ({ ...story, sort_order: index }))
        return [...otherStories, ...updated]
      })
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
    const maxNum = stories.reduce((max, story) => {
      const raw = Number.parseInt(story.identifier.split('-')[1] ?? '0', 10)
      return Number.isFinite(raw) && raw > max ? raw : max
    }, 0)
    return `CS-${maxNum + 1}`
  }, [stories])

  return (
    <div className="relative flex h-full min-h-0 flex-col overflow-hidden">
      <div className="flex flex-shrink-0 items-center justify-end gap-1 border-b border-border/70 px-3 py-1.5">
        <Button variant="outline" size="icon" className="h-7 w-7 rounded-sm">
          <LayoutList className="h-3.5 w-3.5" />
        </Button>
        <Button variant="outline" size="icon" className="h-7 w-7 rounded-sm bg-accent/50">
          <Columns2 className="h-3.5 w-3.5" />
        </Button>
        <Button variant="outline" size="icon" className="h-7 w-7 rounded-sm">
          <SlidersHorizontal className="h-3.5 w-3.5" />
        </Button>
        <Button variant="outline" size="sm" className="h-7 rounded-sm text-xs">
          Display
        </Button>
      </div>

      <DndContext
        sensors={sensors}
        collisionDetection={closestCorners}
        onDragStart={handleDragStart}
        onDragOver={handleDragOver}
        onDragEnd={handleDragEnd}
      >
        <div className="min-h-0 flex-1 overflow-x-auto overflow-y-hidden">
          <div className="flex h-full w-max min-w-full gap-0 pt-2">
            {STATUS_COLUMNS.map((col) => (
              <KanbanColumn
                key={col.id}
                id={col.id}
                label={col.label}
                colorClass={statusColorClass[col.id]}
                stories={storiesByStatus[col.id]}
                onAddStory={handleAddStory}
                onOpenStory={handleOpenStory}
              />
            ))}
          </div>
        </div>

        <DragOverlay>
          {activeStory ? <StoryCard story={activeStory} isOverlay /> : null}
        </DragOverlay>
      </DndContext>

      <StoryDetailSheet
        open={storySheetOpen}
        onOpenChange={setStorySheetOpen}
        story={selectedStory}
      />

      <CreateStoryDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        defaultStatus={dialogDefaultStatus}
        nextIdentifier={nextIdentifier}
        workspaceName={currentWorkspace?.name}
        onCreateStory={handleCreateStory}
      />
    </div>
  )
}
