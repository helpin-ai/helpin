import { createFileRoute } from '@tanstack/react-router'
import { LoadingState } from '@/components/LoadingState'

export const Route = createFileRoute('/$spaceSlug/$articleSlug')({
  component: LegacyArticleRoute,
})

function LegacyArticleRoute() {
  const { spaceSlug, articleSlug } = Route.useParams()

  if (typeof window !== 'undefined') {
    window.location.replace(`/${spaceSlug}/${articleSlug}`)
  }

  return <LoadingState message="Redirecting..." />
}
