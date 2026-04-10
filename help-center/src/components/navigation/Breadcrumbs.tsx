import { DocsLink } from '@/components/DocsLink'
import { useDocsContext } from '@/contexts/DocsContext'
import { buildCanonicalCollectionPath, isMultilingualEnabled } from '@/lib/locale'

interface BreadcrumbsProps {
  locale: string
  collectionName?: string | null
  collectionSlug?: string | null
}

export function Breadcrumbs({
  locale,
  collectionName,
  collectionSlug,
}: BreadcrumbsProps) {
  const { enabledLocales } = useDocsContext()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)

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
      <DocsLink
        to={buildCanonicalCollectionPath(
          multilingualEnabled,
          locale,
          collectionSlug,
        )}
        className="font-medium text-primary transition-colors hover:text-primary/80"
      >
        {collectionName}
      </DocsLink>
    </nav>
  )
}
