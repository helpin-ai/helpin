import { createFileRoute } from '@tanstack/react-router'
import TeamGoals from '@/pages/TeamGoals'

export const Route = createFileRoute('/_authenticated/w/$slug/team-goals')({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <TeamGoals />
    </div>
  ),
})
