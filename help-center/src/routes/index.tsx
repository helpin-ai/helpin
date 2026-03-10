import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useDocsContext } from '@/contexts/DocsContext'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'
import { useEffect } from 'react'

export const Route = createFileRoute('/')({
  component: HomePage,
})

function HomePage() {
  const { spaces } = useDocsContext()
  const navigate = useNavigate()

  useEffect(() => {
    if (spaces.length > 0) {
      navigate({
        to: '/$spaceSlug',
        params: { spaceSlug: spaces[0]!.slug },
        replace: true,
      })
    }
  }, [spaces, navigate])

  if (spaces.length === 0) {
    return (
      <ErrorState
        title="No documentation available"
        message="This help center has no published spaces yet."
      />
    )
  }

  return <LoadingState message="Redirecting..." />
}
