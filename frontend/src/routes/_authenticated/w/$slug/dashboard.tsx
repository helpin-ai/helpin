import { createFileRoute } from '@tanstack/react-router'
import Dashboard from '@/pages/Dashboard'

export const Route = createFileRoute('/_authenticated/w/$slug/dashboard')({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <Dashboard />
    </div>
  ),
})
