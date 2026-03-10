import { useState, useEffect, useRef } from 'react'
import { Search } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogTitle,
} from '@/components/ui/dialog'
import { useSearchArticles } from '@/hooks/queries'
import { useDocsContext } from '@/contexts/DocsContext'
import { SearchResultItem } from './SearchResultItem'

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

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
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
            className="flex-1 bg-transparent text-sm text-foreground outline-none placeholder:text-muted-foreground"
          />
          <kbd className="text-[10px] font-mono px-1.5 py-0.5 rounded-[4px] bg-muted border border-border text-muted-foreground font-medium">
            ESC
          </kbd>
        </div>

        {/* Results */}
        <div className="max-h-[60vh] overflow-y-auto">
          {query.length < 2 ? (
            <div className="px-4 py-10 text-center text-[13px] text-muted-foreground">
              Type to search documentation...
            </div>
          ) : isLoading ? (
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
                  result={result}
                  variant="compact"
                  onClick={onClose}
                />
              ))}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
