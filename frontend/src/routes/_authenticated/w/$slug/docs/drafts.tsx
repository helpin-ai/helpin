import { createFileRoute } from '@tanstack/react-router'
import { DocsRouteViewport } from '@/components/docs/DocsRouteViewport'
import { DocsDocumentList } from '@/pages/docs/DocsDocumentList'

export const Route = createFileRoute('/_authenticated/w/$slug/docs/drafts')({
  component: () => (
    <DocsRouteViewport>
      <DocsDocumentList
        title="Drafts"
        description="Documents that haven't been published yet."
        filterMode="drafts"
      />
    </DocsRouteViewport>
  ),
})
