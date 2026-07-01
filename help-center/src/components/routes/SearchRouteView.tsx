import { useSearchArticles } from '@/hooks/queries'
import { useDocsContext } from '@/contexts/DocsContext'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { SearchResultsList } from '@/components/SearchResults'
import { LoadingState } from '@/components/LoadingState'
import { Footer } from '@/components/layout/Footer'

interface SearchRouteViewProps {
  locale: string
  query: string
  space?: string
}

export function SearchRouteView({
  locale,
  query,
  space,
}: SearchRouteViewProps) {
  const { subdomain, multilingualEnabled } = useDocsContext()
  const { data: results, isLoading } = useSearchArticles(
    subdomain,
    locale,
    query,
    multilingualEnabled,
    space,
  )

  useDocumentTitle(query ? `Search: ${query}` : 'Search')

  return (
    <div
      className="mx-auto px-6 py-10"
      style={{ maxWidth: 'var(--hc-content-max-width)' }}
    >
      <header className="mb-6">
        <h1 className="mb-1 text-2xl font-bold">Search results</h1>
        {query && (
          <p className="text-sm" style={{ color: 'var(--hc-text-secondary)' }}>
            Showing results for &ldquo;{query}&rdquo;
          </p>
        )}
      </header>

      {isLoading ? (
        <LoadingState message="Searching..." />
      ) : (
        <SearchResultsList results={results ?? []} query={query} locale={locale} />
      )}
      <Footer />
    </div>
  )
}
