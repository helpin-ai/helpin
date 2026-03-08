import { createFileRoute } from '@tanstack/react-router'
import { DocsDocumentDetail } from '@/pages/docs/DocsDocumentDetail'

export const Route = createFileRoute('/_authenticated/w/$slug/docs/documents/$docId')({
  component: () => (
    <div className="h-full overflow-hidden">
      <DocsDocumentDetail />
    </div>
  ),
})
