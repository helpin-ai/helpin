import { createFileRoute } from '@tanstack/react-router'
import { DocsHome } from '@/pages/docs/DocsHome'

export const Route = createFileRoute('/_authenticated/w/$slug/docs/')({
  component: DocsHome,
})
