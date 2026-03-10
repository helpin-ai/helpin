import { Link } from '@tanstack/react-router'
import { ChevronRight } from 'lucide-react'

interface BreadcrumbsProps {
  spaceSlug: string
  spaceName: string
  collectionName?: string | null
}

export function Breadcrumbs({
  spaceSlug,
  spaceName,
  collectionName,
}: BreadcrumbsProps) {
  return (
    <nav
      aria-label="Breadcrumb"
      className="flex items-center gap-1.5 text-[13px]"
    >
      <Link
        to="/$spaceSlug"
        params={{ spaceSlug }}
        className="transition-colors hover:text-[var(--hc-text)]"
        style={{ color: 'var(--hc-text-secondary)' }}
      >
        {spaceName}
      </Link>
      {collectionName && (
        <>
          <ChevronRight
            size={12}
            className="shrink-0"
            style={{ color: 'var(--hc-text-muted)' }}
          />
          <span style={{ color: 'var(--hc-text-secondary)' }}>
            {collectionName}
          </span>
        </>
      )}
    </nav>
  )
}
