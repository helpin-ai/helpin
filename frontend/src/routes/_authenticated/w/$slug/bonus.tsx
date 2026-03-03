import { createFileRoute } from '@tanstack/react-router'
import BonusDashboard from '@/pages/BonusDashboard'

export const Route = createFileRoute('/_authenticated/w/$slug/bonus')({
  component: BonusDashboard,
})
