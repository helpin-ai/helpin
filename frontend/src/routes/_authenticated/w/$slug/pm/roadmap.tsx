import { createFileRoute } from '@tanstack/react-router'
import { RoadmapPage } from '@/pages/pm/Roadmap'

export const Route = createFileRoute('/_authenticated/w/$slug/pm/roadmap')({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <RoadmapPage />
    </div>
  ),
})
