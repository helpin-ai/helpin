import { createFileRoute } from '@tanstack/react-router'
import { DocsRouteViewport } from '@/components/docs/DocsRouteViewport'
import { DocsDocumentList } from '@/pages/docs/DocsDocumentList'

export const Route = createFileRoute('/_authenticated/w/$slug/docs/my')({
  component: () => (
    <DocsRouteViewport>
      <DocsDocumentList
        title="My Documents"
        description="All documents you own."
        filterMode="my"
      />
    </DocsRouteViewport>
  ),
})
