import { SearchResultItem } from '@/components/search/SearchResultItem'
import { useDocsContext } from '@/contexts/DocsContext'
import { isMultilingualEnabled } from '@/lib/locale'
import type { SearchResult } from '@/lib/types'

interface SearchResultsProps {
  results: SearchResult[]
  query: string
  locale: string
}

export function SearchResultsList({ results, query, locale }: SearchResultsProps) {
  const { enabledLocales } = useDocsContext()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)

  if (results.length === 0) {
    return (
      <div className="py-12 text-center">
        <p className="text-sm" style={{ color: 'var(--hc-text-secondary)' }}>
          No results found for &ldquo;{query}&rdquo;
        </p>
      </div>
    )
  }

  return (
    <div className="space-y-2">
      {results.map((result) => (
        <SearchResultItem
          key={result.id}
          locale={locale}
          result={result}
          variant="full"
          multilingualEnabled={multilingualEnabled}
        />
      ))}
    </div>
  )
}
