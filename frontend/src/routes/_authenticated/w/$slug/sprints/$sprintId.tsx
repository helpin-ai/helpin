import { createFileRoute } from '@tanstack/react-router'
import SprintDetail from '@/pages/SprintDetail'

export const Route = createFileRoute(
  '/_authenticated/w/$slug/sprints/$sprintId',
)({
  component: SprintDetail,
})
