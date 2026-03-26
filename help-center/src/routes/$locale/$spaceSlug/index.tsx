import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { useSpaceContext } from '@/contexts/SpaceContext'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'

export const Route = createFileRoute('/$locale/$spaceSlug/')({
  component: LocalizedSpaceIndex,
})

function LocalizedSpaceIndex() {
  const { locale, spaceSlug } = Route.useParams()
  const { space, navigation, isLoading } = useSpaceContext()
  const navigate = useNavigate()

  useDocumentTitle(space?.name)

  useEffect(() => {
    if (navigation.length > 0) {
      const firstCollection = navigation[0]
      if (firstCollection) {
        navigate({
          to: '/$locale/$spaceSlug/$collectionSlug',
          params: {
            locale,
            spaceSlug,
            collectionSlug: firstCollection.slug,
          },
          replace: true,
        })
      }
    }
  }, [navigation, navigate, locale, spaceSlug])

  if (isLoading) return <LoadingState />

  if (navigation.length === 0) {
    return (
      <ErrorState
        title="No articles yet"
        message="This space has no published articles."
      />
    )
  }

  return <LoadingState />
}
