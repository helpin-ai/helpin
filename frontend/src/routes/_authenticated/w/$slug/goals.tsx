import { createFileRoute } from '@tanstack/react-router'
import CompanyGoals from '@/pages/CompanyGoals'

export const Route = createFileRoute('/_authenticated/w/$slug/goals')({
  component: CompanyGoals,
})
