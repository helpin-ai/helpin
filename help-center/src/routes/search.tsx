import { createFileRoute } from '@tanstack/react-router'
import { useSearchArticles } from '@/hooks/queries'
import { SearchResultsList } from '@/components/SearchResults'
import { LoadingState } from '@/components/LoadingState'

interface SearchParams {
  q?: string
}

export const Route = createFileRoute('/search')({
  validateSearch: (search: Record<string, unknown>): SearchParams => ({
    q: typeof search.q === 'string' ? search.q : undefined,
  }),
  component: SearchPage,
})

function SearchPage() {
  const { q = '' } = Route.useSearch()
  const subdomain = Route.useRouteContext({ select: (s) => s.subdomain })
  const { data: results, isLoading } = useSearchArticles(subdomain, q)

  return (
    <div
      className="mx-auto py-10 px-6"
      style={{ maxWidth: 'var(--hc-content-max-width)' }}
    >
      <header className="mb-6">
        <h1 className="text-2xl font-bold mb-1">Search results</h1>
        {q && (
          <p
            className="text-sm"
            style={{ color: 'var(--hc-text-secondary)' }}
          >
            Showing results for &ldquo;{q}&rdquo;
          </p>
        )}
      </header>

      {isLoading ? (
        <LoadingState message="Searching..." />
      ) : (
        <SearchResultsList results={results ?? []} query={q} />
      )}
    </div>
  )
}
