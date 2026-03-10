import { useState, useEffect, useRef } from 'react'
import { Link } from '@tanstack/react-router'
import { Search, FileText, ArrowRight } from 'lucide-react'
import { useSearchArticles } from '@/hooks/queries'
import { useDocsContext } from '@/contexts/DocsContext'

interface SearchDialogProps {
  open: boolean
  onClose: () => void
}

export function SearchDialog({ open, onClose }: SearchDialogProps) {
  const { subdomain } = useDocsContext()
  const [query, setQuery] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const { data: results, isLoading } = useSearchArticles(subdomain, query)

  // Focus input and reset query on open
  useEffect(() => {
    if (open) {
      setQuery('')
      requestAnimationFrame(() => inputRef.current?.focus())
    }
  }, [open])

  // Close on Escape
  useEffect(() => {
    if (!open) return
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [open, onClose])

  // Prevent body scroll
  useEffect(() => {
    if (!open) return
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = ''
    }
  }, [open])

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50">
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-black/50 backdrop-blur-sm"
        onClick={onClose}
      />

      {/* Dialog */}
      <div className="fixed top-[15%] left-1/2 -translate-x-1/2 w-[calc(100%-2rem)] max-w-lg">
        <div
          className="rounded-xl border shadow-2xl overflow-hidden"
          style={{
            backgroundColor: 'var(--hc-bg)',
            borderColor: 'var(--hc-border)',
          }}
        >
          {/* Search input */}
          <div
            className="flex items-center gap-3 px-4 py-3 border-b"
            style={{ borderColor: 'var(--hc-border)' }}
          >
            <Search size={16} style={{ color: 'var(--hc-text-muted)' }} />
            <input
              ref={inputRef}
              type="text"
              placeholder="Search documentation..."
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="flex-1 bg-transparent outline-none text-sm"
            />
            <kbd
              className="text-[11px] font-mono px-1.5 py-0.5 rounded"
              style={{
                backgroundColor: 'var(--hc-bg-tertiary)',
                color: 'var(--hc-text-muted)',
              }}
            >
              ESC
            </kbd>
          </div>

          {/* Results */}
          <div className="max-h-[60vh] overflow-y-auto">
            {query.length < 2 ? (
              <div
                className="px-4 py-8 text-center text-sm"
                style={{ color: 'var(--hc-text-muted)' }}
              >
                Type to search documentation...
              </div>
            ) : isLoading ? (
              <div className="px-4 py-8 text-center">
                <div
                  className="inline-block h-5 w-5 animate-spin rounded-full border-2 border-current border-t-transparent"
                  style={{ color: 'var(--hc-accent)' }}
                />
              </div>
            ) : results && results.length === 0 ? (
              <div
                className="px-4 py-8 text-center text-sm"
                style={{ color: 'var(--hc-text-secondary)' }}
              >
                No results found for &ldquo;{query}&rdquo;
              </div>
            ) : (
              <div className="py-2">
                {results?.map((result) => (
                  <Link
                    key={result.id}
                    to="/$spaceSlug/$articleSlug"
                    params={{
                      spaceSlug: result.space_slug,
                      articleSlug: result.slug,
                    }}
                    onClick={onClose}
                    className="flex items-start gap-3 px-4 py-3 transition-colors hover:bg-[var(--hc-bg-secondary)] group"
                  >
                    <FileText
                      size={16}
                      className="mt-0.5 shrink-0"
                      style={{ color: 'var(--hc-accent)' }}
                    />
                    <div className="flex-1 min-w-0">
                      <div className="font-medium text-sm">{result.title}</div>
                      {(result.space_name || result.collection_name) && (
                        <div
                          className="flex items-center gap-1.5 text-xs mt-0.5"
                          style={{ color: 'var(--hc-text-muted)' }}
                        >
                          {result.space_name && (
                            <span>{result.space_name}</span>
                          )}
                          {result.space_name && result.collection_name && (
                            <span>/</span>
                          )}
                          {result.collection_name && (
                            <span>{result.collection_name}</span>
                          )}
                        </div>
                      )}
                      {result.excerpt && (
                        <p
                          className="text-xs mt-1 line-clamp-1"
                          style={{ color: 'var(--hc-text-secondary)' }}
                        >
                          {result.excerpt}
                        </p>
                      )}
                    </div>
                    <ArrowRight
                      size={14}
                      className="mt-1 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity"
                      style={{ color: 'var(--hc-text-muted)' }}
                    />
                  </Link>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
