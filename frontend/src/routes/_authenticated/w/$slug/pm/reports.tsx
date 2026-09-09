import { createFileRoute } from '@tanstack/react-router'
import { ReportsPage } from '@/pages/pm/Reports'

export const Route = createFileRoute('/_authenticated/w/$slug/pm/reports')({
  component: () => (
    <div className="h-full min-h-0 overflow-hidden">
      <ReportsPage />
    </div>
  ),
})
