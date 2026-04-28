import { createFileRoute, useParams } from '@tanstack/react-router'
import { DocsDocumentDetail } from '@/pages/docs/DocsDocumentDetail'

export const Route = createFileRoute('/_authenticated/w/$slug/docs/documents/$docId')({
  component: DocDetailRoute,
})

function DocDetailRoute() {
  const { docId } = useParams({ strict: false }) as { docId: string }
  const search = new URLSearchParams(window.location.search)
  const fromGapId = search.get('from_gap') ?? undefined
  const fromSuggestionId = search.get('from_suggestion') ?? undefined
  return (
    <div className="h-full overflow-hidden" key={docId}>
      <DocsDocumentDetail fromGapId={fromGapId} fromSuggestionId={fromSuggestionId} />
    </div>
  )
}
