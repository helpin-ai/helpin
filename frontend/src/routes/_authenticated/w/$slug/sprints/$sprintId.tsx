import { createFileRoute } from '@tanstack/react-router'
import { SprintDetailPage as SprintDetail } from '@/pages/pm/SprintDetail'

export const Route = createFileRoute(
  '/_authenticated/w/$slug/sprints/$sprintId',
)({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <SprintDetail />
    </div>
  ),
})
