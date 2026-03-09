import { createFileRoute, useParams } from '@tanstack/react-router'
import { DocsDocumentDetail } from '@/pages/docs/DocsDocumentDetail'

export const Route = createFileRoute('/_authenticated/w/$slug/docs/documents/$docId')({
  component: DocDetailRoute,
})

function DocDetailRoute() {
  const { docId } = useParams({ strict: false }) as { docId: string }
  return (
    <div className="h-full overflow-hidden" key={docId}>
      <DocsDocumentDetail />
    </div>
  )
}
