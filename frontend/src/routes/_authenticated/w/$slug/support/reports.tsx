import { createFileRoute } from '@tanstack/react-router'
import { SupportReportsPage } from '@/components/support/reports/SupportReportsPage'

function RouteComponent() {
  return (
    <div className="h-full overflow-auto">
      <SupportReportsPage />
    </div>
  )
}

export const Route = createFileRoute('/_authenticated/w/$slug/support/reports')({
  component: RouteComponent,
})
