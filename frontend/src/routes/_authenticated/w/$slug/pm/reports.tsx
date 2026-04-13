import { createFileRoute } from '@tanstack/react-router'
import { ReportsPage } from '@/pages/pm/Reports'

export const Route = createFileRoute('/_authenticated/w/$slug/pm/reports')({
  component: ReportsPage,
})
