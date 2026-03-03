import { createFileRoute } from '@tanstack/react-router'
import TeamGoals from '@/pages/TeamGoals'

export const Route = createFileRoute('/_authenticated/w/$slug/team-goals')({
  component: TeamGoals,
})
