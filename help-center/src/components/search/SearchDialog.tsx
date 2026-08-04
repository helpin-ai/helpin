import { useState, useEffect, useRef } from 'react'
import { Search, Sparkles } from 'lucide-react'
import { useNavigate } from '@tanstack/react-router'
import {
  Dialog,
  DialogContent,
  DialogTitle,
} from '@/components/ui/dialog'
import { useSearchArticles } from '@/hooks/queries'
import { useDocsContext } from '@/contexts/DocsContext'
import { useDebouncedValue } from '@/hooks/useDebouncedValue'
import { buildCanonicalSearchPath } from '@/lib/locale'
import { SearchResultItem } from './SearchResultItem'

interface SearchDialogProps {
  open: boolean
  onClose: () => void
}

export function SearchDialog({ open, onClose }: SearchDialogProps) {
  const { subdomain, locale, multilingualEnabled } = useDocsContext()
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const normalizedQuery = query.trim()
  const debouncedQuery = useDebouncedValue(normalizedQuery, 300)
  const searchQuery =
    open && normalizedQuery.length >= 2 && normalizedQuery === debouncedQuery
      ? debouncedQuery
      : ''
  const isDebouncing =
    open && normalizedQuery.length >= 2 && normalizedQuery !== debouncedQuery
  const { data: results, isLoading } = useSearchArticles(
    subdomain,
    locale,
    searchQuery,
    multilingualEnabled,
  )

  const openFullSearch = () => {
    if (normalizedQuery.length < 2) return
    onClose()
    void navigate({
      to: buildCanonicalSearchPath(multilingualEnabled, locale, normalizedQuery) as never,
    })
  }

  useEffect(() => {
    if (!open) {
      setQuery('')
      return
    }

    setQuery('')
    requestAnimationFrame(() => inputRef.current?.focus())
  }, [open])

  return (
    <Dialog open={open} onOpenChange={(v: boolean) => !v && onClose()}>
      <DialogContent className="sm:max-w-lg p-0 gap-0 overflow-hidden rounded-xl" showCloseButton={false}>
        <DialogTitle className="sr-only">Search documentation</DialogTitle>

        {/* Search input */}
        <div className="flex items-center gap-3 px-4 py-3 border-b border-border">
          <Search size={16} className="text-muted-foreground shrink-0" />
          <input
            ref={inputRef}
            type="text"
            placeholder="Search documentation..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') openFullSearch()
            }}
            className="flex-1 bg-transparent text-sm text-foreground outline-none placeholder:text-muted-foreground"
          />
          <kbd className="text-[10px] font-mono px-1.5 py-0.5 rounded-[4px] bg-muted border border-border text-muted-foreground font-medium">
            ESC
          </kbd>
        </div>

        {/* Results */}
        <div className="max-h-[60vh] overflow-y-auto">
          {normalizedQuery.length < 2 ? (
            <div className="px-4 py-10 text-center text-[13px] text-muted-foreground">
              Type to search documentation...
            </div>
          ) : isDebouncing || isLoading ? (
            <div className="px-4 py-10 text-center">
              <div className="inline-block h-5 w-5 animate-spin rounded-full border-2 border-primary border-t-transparent" />
            </div>
          ) : results && results.length === 0 ? (
            <div className="px-4 py-10 text-center text-[13px] text-muted-foreground">
              No results found for &ldquo;{query}&rdquo;
            </div>
          ) : (
            <div className="py-1">
              {results?.map((result) => (
                <SearchResultItem
                  key={result.id}
                  locale={locale}
                  result={result}
                  variant="compact"
                  multilingualEnabled={multilingualEnabled}
                  onClick={onClose}
                />
              ))}
            </div>
          )}
        </div>

        {/* Full search page + AI answer entry point */}
        {normalizedQuery.length >= 2 && (
          <button
            type="button"
            onClick={openFullSearch}
            className="flex w-full items-center gap-2 border-t border-border px-4 py-2.5 text-left text-[13px] text-muted-foreground transition-colors hover:bg-muted/50 hover:text-foreground"
          >
            <Sparkles size={14} className="shrink-0" />
            <span>
              See all results and get an AI answer for &ldquo;{normalizedQuery}&rdquo;
            </span>
            <kbd className="ml-auto shrink-0 rounded-[4px] border border-border bg-muted px-1.5 py-0.5 font-mono text-[10px] font-medium">
              Enter
            </kbd>
          </button>
        )}
      </DialogContent>
    </Dialog>
  )
}
