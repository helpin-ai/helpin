import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useSpaceContext } from '@/contexts/SpaceContext'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'
import { useEffect } from 'react'

export const Route = createFileRoute('/$spaceSlug/')({
  component: SpaceIndex,
})

function SpaceIndex() {
  const { spaceSlug, space, navigation, isLoading } = useSpaceContext()
  const navigate = useNavigate()

  useDocumentTitle(space?.name)

  useEffect(() => {
    if (navigation.length > 0) {
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
