import { ChevronLeft, ChevronRight } from 'lucide-react'
import { DocsLink } from '@/components/DocsLink'
import { useDocsContext } from '@/contexts/DocsContext'
import { buildCanonicalArticlePath, isMultilingualEnabled } from '@/lib/locale'
import type { ArticlePagerLink } from '@/lib/navigation'

interface ArticlePagerProps {
  locale: string
  prev?: ArticlePagerLink
  next?: ArticlePagerLink
}

export function ArticlePager({ locale, prev, next }: ArticlePagerProps) {
  const { enabledLocales } = useDocsContext()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)

  if (!prev && !next) return null

  return (
    <nav className="flex items-stretch gap-4 mt-12 pt-8 border-t border-border">
      {prev ? (
        <DocsLink
          to={buildCanonicalArticlePath(
            multilingualEnabled,
            locale,
            prev.collectionSlug,
            prev.slug,
          )}
          className="flex-1 group rounded-lg border border-border/70 px-4 py-3.5 transition-colors hover:border-primary/30 hover:bg-primary/[0.02]"
        >
          <div className="flex items-center gap-1 text-xs text-muted-foreground mb-1">
            <ChevronLeft size={12} />
            Previous
          </div>
          <div className="font-medium text-sm text-foreground group-hover:text-primary transition-colors">
            {prev.title}
          </div>
        </DocsLink>
      ) : (
        <div className="flex-1" />
      )}
      {next ? (
        <DocsLink
          to={buildCanonicalArticlePath(
            multilingualEnabled,
            locale,
            next.collectionSlug,
            next.slug,
          )}
          className="flex-1 group rounded-lg border border-border/70 px-4 py-3.5 text-right transition-colors hover:border-primary/30 hover:bg-primary/[0.02]"
        >
          <div className="flex items-center justify-end gap-1 text-xs text-muted-foreground mb-1">
            Next
            <ChevronRight size={12} />
          </div>
          <div className="font-medium text-sm text-foreground group-hover:text-primary transition-colors">
            {next.title}
          </div>
        </DocsLink>
      ) : (
        <div className="flex-1" />
      )}
    </nav>
  )
}
