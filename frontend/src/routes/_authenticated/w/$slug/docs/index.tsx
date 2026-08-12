import { createFileRoute } from '@tanstack/react-router'
import { DocsRouteViewport } from '@/components/docs/DocsRouteViewport'
import { DocsHome } from '@/pages/docs/DocsHome'

export const Route = createFileRoute('/_authenticated/w/$slug/docs/')({
  component: () => (
    <DocsRouteViewport>
      <DocsHome />
    </DocsRouteViewport>
  ),
})
