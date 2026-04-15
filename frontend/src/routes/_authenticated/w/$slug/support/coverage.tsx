import { createFileRoute } from '@tanstack/react-router'
import { SupportCoveragePage } from '@/pages/support/coverage/SupportCoveragePage'

export const Route = createFileRoute('/_authenticated/w/$slug/support/coverage')({
  component: SupportCoveragePage,
})
