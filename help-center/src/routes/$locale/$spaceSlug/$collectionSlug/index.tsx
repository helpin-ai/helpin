import { createFileRoute } from '@tanstack/react-router'
import { LoadingState } from '@/components/LoadingState'

export const Route = createFileRoute('/$locale/$spaceSlug/$collectionSlug/')({
  component: LegacyLocalizedArticleRoute,
})

function LegacyLocalizedArticleRoute() {
  const { locale, spaceSlug, collectionSlug } = Route.useParams()

  if (typeof window !== 'undefined') {
    window.location.replace(`/${locale}/${spaceSlug}/${collectionSlug}`)
  }

  return <LoadingState message="Redirecting..." />
}
