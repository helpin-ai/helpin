import { Link } from '@tanstack/react-router'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import type { ArticlePagerLink } from '@/lib/navigation'

interface ArticlePagerProps {
  spaceSlug: string
  prev?: ArticlePagerLink
  next?: ArticlePagerLink
}

export function ArticlePager({ spaceSlug, prev, next }: ArticlePagerProps) {
  if (!prev && !next) return null

  return (
    <nav
      className="flex items-stretch gap-4 mt-12 pt-6 border-t"
      style={{ borderColor: 'var(--hc-border)' }}
    >
      {prev ? (
        <Link
          to="/$spaceSlug/$articleSlug"
          params={{ spaceSlug, articleSlug: prev.slug }}
          className="flex-1 group rounded-lg border p-4 transition-colors hover:bg-[var(--hc-bg-secondary)]"
          style={{ borderColor: 'var(--hc-border)' }}
        >
          <div
            className="flex items-center gap-1 text-xs mb-1"
            style={{ color: 'var(--hc-text-muted)' }}
          >
            <ChevronLeft size={12} />
            Previous
          </div>
          <div className="font-medium text-sm group-hover:text-[var(--hc-accent)]">
            {prev.title}
          </div>
        </Link>
      ) : (
        <div className="flex-1" />
      )}
      {next ? (
        <Link
          to="/$spaceSlug/$articleSlug"
          params={{ spaceSlug, articleSlug: next.slug }}
          className="flex-1 group rounded-lg border p-4 text-right transition-colors hover:bg-[var(--hc-bg-secondary)]"
          style={{ borderColor: 'var(--hc-border)' }}
        >
          <div
            className="flex items-center justify-end gap-1 text-xs mb-1"
            style={{ color: 'var(--hc-text-muted)' }}
          >
            Next
            <ChevronRight size={12} />
          </div>
          <div className="font-medium text-sm group-hover:text-[var(--hc-accent)]">
            {next.title}
          </div>
        </Link>
      ) : (
        <div className="flex-1" />
      )}
    </nav>
  )
}
