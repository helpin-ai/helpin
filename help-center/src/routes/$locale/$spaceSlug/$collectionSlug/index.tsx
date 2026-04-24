import { createFileRoute } from '@tanstack/react-router'
import { LoadingState } from '@/components/LoadingState'
import { useDocsContext } from '@/contexts/DocsContext'
import { prefixBasepath } from '@/lib/pathUtils'

export const Route = createFileRoute('/$locale/$spaceSlug/$collectionSlug/')({
  component: LegacyLocalizedArticleRoute,
})

function LegacyLocalizedArticleRoute() {
  const { locale, spaceSlug, collectionSlug } = Route.useParams()
  const { basepath } = useDocsContext()

  if (typeof window !== 'undefined') {
    window.location.replace(
      prefixBasepath(basepath, `/${locale}/${spaceSlug}/${collectionSlug}`),
    )
  }

  return <LoadingState message="Redirecting..." />
}
