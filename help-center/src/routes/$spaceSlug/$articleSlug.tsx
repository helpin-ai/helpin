import { createFileRoute } from '@tanstack/react-router'
import { LoadingState } from '@/components/LoadingState'
import { useDocsContext } from '@/contexts/DocsContext'
import { prefixBasepath } from '@/lib/pathUtils'

export const Route = createFileRoute('/$spaceSlug/$articleSlug')({
  component: LegacyArticleRoute,
})

function LegacyArticleRoute() {
  const { spaceSlug, articleSlug } = Route.useParams()
  const { basepath } = useDocsContext()

  if (typeof window !== 'undefined') {
    window.location.replace(prefixBasepath(basepath, `/${spaceSlug}/${articleSlug}`))
  }

  return <LoadingState message="Redirecting..." />
}
