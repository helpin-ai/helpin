import { Link } from '@tanstack/react-router'

interface BreadcrumbsProps {
  locale: string
  spaceSlug: string
  collectionName?: string | null
  collectionSlug?: string | null
}

export function Breadcrumbs({
  locale,
  spaceSlug,
  collectionName,
  collectionSlug,
}: BreadcrumbsProps) {
  if (!collectionName) return null

  if (!collectionSlug) {
    return (
      <nav className="flex items-center text-[14px]">
        <span className="font-medium text-primary">{collectionName}</span>
      </nav>
    )
  }

  return (
    <nav className="flex items-center text-[14px]">
      <Link
        to="/$locale/$spaceSlug/$collectionSlug"
        params={{ locale, spaceSlug, collectionSlug }}
        className="font-medium text-primary transition-colors hover:text-primary/80"
      >
        {collectionName}
      </Link>
    </nav>
  )
}
