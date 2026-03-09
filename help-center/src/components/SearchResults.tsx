import { Link } from '@tanstack/react-router'
import { FileText } from 'lucide-react'
import type { SearchResult } from '@/lib/types'

interface SearchResultsProps {
  results: SearchResult[]
  query: string
}

export function SearchResultsList({ results, query }: SearchResultsProps) {
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
        <Link
          key={result.id}
          to="/articles/$articleSlug"
          params={{ articleSlug: result.slug }}
          className="block rounded-lg border p-4 transition-colors hover:bg-[var(--hc-bg-secondary)]"
          style={{ borderColor: 'var(--hc-border)' }}
        >
          <div className="flex items-start gap-3">
            <FileText
              size={18}
              className="mt-0.5 shrink-0"
              style={{ color: 'var(--hc-accent)' }}
            />
            <div className="min-w-0">
              <h3 className="font-medium text-sm mb-1">{result.title}</h3>
              {result.collection_name && (
                <p
                  className="text-xs mb-1"
                  style={{ color: 'var(--hc-text-muted)' }}
                >
                  {result.collection_name}
                </p>
              )}
              {result.excerpt && (
                <p
                  className="text-sm line-clamp-2"
                  style={{ color: 'var(--hc-text-secondary)' }}
                >
                  {result.excerpt}
                </p>
              )}
            </div>
          </div>
        </Link>
      ))}
    </div>
  )
}
