import { createFileRoute } from '@tanstack/react-router'
import CompanyGoals from '@/pages/CompanyGoals'

export const Route = createFileRoute('/_authenticated/w/$slug/goals')({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <CompanyGoals />
    </div>
  ),
})
