import { createFileRoute } from '@tanstack/react-router'
import Tasks from '@/pages/Tasks'

export const Route = createFileRoute('/_authenticated/w/$slug/tasks')({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <Tasks />
    </div>
  ),
})
