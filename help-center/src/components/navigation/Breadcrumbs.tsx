import { Fragment } from 'react'
import { Link } from '@tanstack/react-router'
import { useDocsContext } from '@/contexts/DocsContext'
import { buildCanonicalCollectionPath, isMultilingualEnabled } from '@/lib/locale'

export interface BreadcrumbEntry {
  id: string
  name: string
  slug: string | null
}

interface BreadcrumbsProps {
  locale: string
  /**
   * Ancestor chain ordered top-down (root first, current last). When
   * provided, every ancestor is rendered as a separate link with a
   * chevron separator. The entry with a non-null slug becomes a link;
   * entries with a null slug render as plain text (uncategorized /
   * legacy rows without a canonical URL).
   */
  ancestors?: BreadcrumbEntry[]
  /**
   * Legacy single-collection mode. When `ancestors` is omitted the
   * component falls back to rendering just this one collection. This
   * keeps existing routes that don't yet wire the full breadcrumb
   * chain working unchanged.
   */
  collectionName?: string | null
  collectionSlug?: string | null
}

export function Breadcrumbs({
  locale,
  ancestors,
  collectionName,
  collectionSlug,
}: BreadcrumbsProps) {
  const { enabledLocales } = useDocsContext()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)

  // Prefer the explicit ancestor chain when provided; otherwise build
  // a single-entry chain from the legacy (collectionName, collectionSlug)
  // props so existing call sites keep rendering exactly as before.
  const entries: BreadcrumbEntry[] = (() => {
    if (ancestors && ancestors.length > 0) return ancestors
    if (collectionName) {
      return [{ id: 'legacy', name: collectionName, slug: collectionSlug ?? null }]
    }
    return []
  })()

  if (entries.length === 0) return null

  return (
    <nav className="flex items-center gap-1.5 text-[14px]">
      {entries.map((entry, idx) => {
        const isLast = idx === entries.length - 1
        const content =
          entry.slug != null ? (
            <Link
              to={buildCanonicalCollectionPath(multilingualEnabled, locale, entry.slug)}
              className="font-medium text-primary transition-colors hover:text-primary/80"
            >
              {entry.name}
            </Link>
          ) : (
            <span className="font-medium text-primary">{entry.name}</span>
          )
        return (
          <Fragment key={entry.id}>
            {content}
            {!isLast && <span className="text-muted-foreground">/</span>}
          </Fragment>
        )
      })}
    </nav>
  )
}
