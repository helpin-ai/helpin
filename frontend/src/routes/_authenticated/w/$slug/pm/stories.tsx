import { createFileRoute } from '@tanstack/react-router'
import { KanbanBoard } from '@/components/pm/kanban-board'

export const Route = createFileRoute('/_authenticated/w/$slug/pm/stories')({
  component: Stories,
})

function Stories() {
  return (
    <div className="flex h-full flex-col">
      <KanbanBoard />
    </div>
  )
}
