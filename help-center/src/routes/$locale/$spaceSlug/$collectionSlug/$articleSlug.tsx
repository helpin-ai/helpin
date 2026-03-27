import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { LoadingState } from '@/components/LoadingState'

export const Route = createFileRoute(
  '/$locale/$spaceSlug/$collectionSlug/$articleSlug',
)({
  component: LegacyLocalizedArticleRedirect,
})

function LegacyLocalizedArticleRedirect() {
  const { locale, collectionSlug, articleSlug } = Route.useParams()
  const navigate = useNavigate()

  useEffect(() => {
    navigate({
      to: '/$locale/$spaceSlug/$collectionSlug',
      params: { locale, spaceSlug: collectionSlug, collectionSlug: articleSlug },
      replace: true,
    })
  }, [articleSlug, collectionSlug, locale, navigate])

  return <LoadingState message="Redirecting..." />
}
