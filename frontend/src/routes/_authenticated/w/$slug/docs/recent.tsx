import { createFileRoute } from '@tanstack/react-router'
import { DocsRouteViewport } from '@/components/docs/DocsRouteViewport'
import { DocsDocumentList } from '@/pages/docs/DocsDocumentList'

export const Route = createFileRoute('/_authenticated/w/$slug/docs/recent')({
  component: () => (
    <DocsRouteViewport>
      <DocsDocumentList
        title="Recent Documents"
        description="Recently updated documents across all spaces."
        filterMode="recent"
      />
    </DocsRouteViewport>
  ),
})
