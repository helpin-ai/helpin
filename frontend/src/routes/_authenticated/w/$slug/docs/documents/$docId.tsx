import { createFileRoute, useParams } from '@tanstack/react-router'
import { DocsDocumentDetail } from '@/pages/docs/DocsDocumentDetail'

type DocsDocumentSearch = {
  from_gap?: string
  from_suggestion?: string
  proposal?: string
}

export const Route = createFileRoute('/_authenticated/w/$slug/docs/documents/$docId')({
  validateSearch: (search: Record<string, unknown>): DocsDocumentSearch => ({
    from_gap: typeof search.from_gap === 'string' ? search.from_gap : undefined,
    from_suggestion: typeof search.from_suggestion === 'string' ? search.from_suggestion : undefined,
    proposal: typeof search.proposal === 'string' ? search.proposal : undefined,
  }),
  component: DocDetailRoute,
})

function DocDetailRoute() {
  const { docId } = useParams({ strict: false }) as { docId: string }
  const { from_gap: fromGapId, from_suggestion: fromSuggestionId } = Route.useSearch()
  return (
    <div className="h-full overflow-hidden" key={docId}>
      <DocsDocumentDetail fromGapId={fromGapId} fromSuggestionId={fromSuggestionId} />
    </div>
  )
}
