import { createFileRoute } from '@tanstack/react-router'
import Sprints from '@/pages/Sprints'

export const Route = createFileRoute('/_authenticated/w/$slug/sprints/')({
  component: Sprints,
})
