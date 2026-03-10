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
    <nav className="flex items-center gap-1.5 text-[13px] text-muted-foreground">
      <Link
        to="/$spaceSlug"
        params={{ spaceSlug }}
        className="hover:text-foreground transition-colors"
      >
        {spaceName}
      </Link>
      {collectionName && (
        <>
          <ChevronRight size={12} className="text-muted-foreground/40" />
          <span className="text-muted-foreground/70">{collectionName}</span>
        </>
      )}
    </nav>
  )
}
