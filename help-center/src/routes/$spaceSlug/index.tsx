import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useSpaceNavigation } from '@/hooks/queries'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'
import { useEffect } from 'react'

export const Route = createFileRoute('/$spaceSlug/')({
  component: SpaceIndex,
})

function SpaceIndex() {
  const { spaceSlug } = Route.useParams()
  const subdomain = Route.useRouteContext({ select: (s) => s.subdomain })
  const { data: navigation } = useSpaceNavigation(subdomain, spaceSlug)
  const navigate = useNavigate()

  useEffect(() => {
    if (navigation && navigation.length > 0) {
      const firstArticle = navigation[0]?.articles[0]
      if (firstArticle) {
        navigate({
          to: '/$spaceSlug/$articleSlug',
          params: { spaceSlug, articleSlug: firstArticle.slug },
          replace: true,
        })
      }
    }
  }, [navigation, navigate, spaceSlug])

  if (navigation && navigation.length === 0) {
    return (
      <ErrorState
        title="No articles yet"
        message="This space has no published articles."
      />
    )
  }

  return <LoadingState />
}
