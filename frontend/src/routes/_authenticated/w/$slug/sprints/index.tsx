import { createFileRoute } from '@tanstack/react-router'
import { SprintsPage as Sprints } from '@/pages/pm/Sprints'

export const Route = createFileRoute('/_authenticated/w/$slug/sprints/')({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <Sprints />
    </div>
  ),
})
