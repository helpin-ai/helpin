import { createFileRoute } from '@tanstack/react-router'
import { useSearchArticles } from '@/hooks/queries'
import { useDocsContext } from '@/contexts/DocsContext'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { SearchResultsList } from '@/components/SearchResults'
import { LoadingState } from '@/components/LoadingState'

interface SearchParams {
  q?: string
  space?: string
}

export const Route = createFileRoute('/$locale/search')({
  validateSearch: (search: Record<string, unknown>): SearchParams => ({
    q: typeof search.q === 'string' ? search.q : undefined,
    space: typeof search.space === 'string' ? search.space : undefined,
  }),
  component: LocalizedSearchPage,
})

function LocalizedSearchPage() {
  const { q = '', space } = Route.useSearch()
  const { subdomain, locale } = useDocsContext()
  const { data: results, isLoading } = useSearchArticles(
    subdomain,
    locale,
    q,
    space,
  )

  useDocumentTitle(q ? `Search: ${q}` : 'Search')

  return (
    <div
      className="mx-auto px-6 py-10"
      style={{ maxWidth: 'var(--hc-content-max-width)' }}
    >
      <header className="mb-6">
        <h1 className="mb-1 text-2xl font-bold">Search results</h1>
        {q && (
          <p className="text-sm" style={{ color: 'var(--hc-text-secondary)' }}>
            Showing results for &ldquo;{q}&rdquo;
          </p>
        )}
      </header>

      {isLoading ? (
        <LoadingState message="Searching..." />
      ) : (
        <SearchResultsList results={results ?? []} query={q} locale={locale} />
      )}
    </div>
  )
}
