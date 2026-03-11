interface BreadcrumbsProps {
  spaceSlug: string
  collectionName?: string | null
}

export function Breadcrumbs({
  collectionName,
}: BreadcrumbsProps) {
  if (!collectionName) return null

  return (
    <nav className="flex items-center text-[14px]">
      <span className="font-medium text-primary">{collectionName}</span>
    </nav>
  )
}
