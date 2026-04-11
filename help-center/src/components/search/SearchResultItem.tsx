import { FileText, ArrowRight } from 'lucide-react'
import { DocsLink } from '@/components/DocsLink'
import {
  buildCanonicalArticlePath,
} from '@/lib/locale'
import type { SearchResult } from '@/lib/types'

interface SearchResultItemProps {
  locale: string
  result: SearchResult
  variant?: 'compact' | 'full'
  multilingualEnabled?: boolean
  onClick?: () => void
}

export function SearchResultItem({
  locale,
  result,
  variant = 'full',
  multilingualEnabled = true,
  onClick,
}: SearchResultItemProps) {
  const isCompact = variant === 'compact'
  const targetLocale = result.locale || locale
  const collectionSlug = result.collection_slug

  const content = (
    <>
      <FileText
        size={isCompact ? 15 : 18}
        className="shrink-0 text-primary/70"
      />
      <div className="min-w-0 flex-1">
        <h3 className={`font-medium ${isCompact ? 'text-[13px]' : 'text-sm mb-1'}`}>
          {result.title}
        </h3>
        {(result.space_name || result.collection_name || result.collection_ancestor_path) && (
          <div className={`flex items-center gap-1.5 text-[11px] text-muted-foreground/60 ${isCompact ? 'mt-0.5' : 'mb-1'}`}>
            {result.space_name && <span>{result.space_name}</span>}
            {/* Prefer the full ancestor path when the backend supplied
                one ("Parent / Middle / Current") — it already includes
                the active collection name. Fall back to the flat
                collection_name for top-level collections. */}
            {(result.collection_ancestor_path || result.collection_name) && result.space_name && (
              <span>/</span>
            )}
            {result.collection_ancestor_path ? (
              <span>{result.collection_ancestor_path}</span>
            ) : (
              result.collection_name && <span>{result.collection_name}</span>
            )}
          </div>
        )}
        {!isCompact && result.excerpt && (
          <p className="text-muted-foreground text-[13px] mt-1 line-clamp-2">
            {result.excerpt}
          </p>
        )}
      </div>
      {isCompact && (
        <ArrowRight
          size={13}
          className="shrink-0 opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground/50"
        />
      )}
    </>
  )

  const className = isCompact
    ? 'flex items-center gap-3 px-4 py-2.5 mx-1 rounded-md transition-colors hover:bg-muted/60 group'
    : 'block rounded-lg border border-border/70 p-4 transition-colors hover:border-primary/30 hover:bg-primary/[0.02] group'

  if (!collectionSlug) {
    return (
      <DocsLink
        to={buildCanonicalArticlePath(
          multilingualEnabled,
          targetLocale,
          result.slug,
          result.public_id,
        )}
        onClick={onClick}
        className={className}
      >
        {content}
      </DocsLink>
    )
  }

  return (
    <DocsLink
      to={buildCanonicalArticlePath(
        multilingualEnabled,
        targetLocale,
        result.slug,
        result.public_id,
      )}
      onClick={onClick}
      className={className}
    >
      {content}
    </DocsLink>
  )
}
