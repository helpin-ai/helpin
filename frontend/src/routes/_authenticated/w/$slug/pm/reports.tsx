import { createFileRoute } from '@tanstack/react-router'
import { ReportsPage } from '@/pages/pm/Reports'

export const Route = createFileRoute('/_authenticated/w/$slug/pm/reports')({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <ReportsPage />
    </div>
  ),
})
