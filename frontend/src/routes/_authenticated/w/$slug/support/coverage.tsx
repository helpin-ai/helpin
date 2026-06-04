import { createFileRoute } from '@tanstack/react-router'
import { SupportCoveragePage } from '@/pages/support/coverage/SupportCoveragePage'

function RouteComponent() {
  return (
    <div className="h-full overflow-auto">
      <SupportCoveragePage />
    </div>
  )
}

export const Route = createFileRoute('/_authenticated/w/$slug/support/coverage')({
  component: RouteComponent,
})
