import { createFileRoute } from '@tanstack/react-router'
import BonusDashboard from '@/pages/BonusDashboard'

export const Route = createFileRoute('/_authenticated/w/$slug/bonus')({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <BonusDashboard />
    </div>
  ),
})
