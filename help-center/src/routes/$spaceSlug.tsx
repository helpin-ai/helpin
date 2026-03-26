import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { LoadingState } from '@/components/LoadingState'
import { useDocsContext } from '@/contexts/DocsContext'

export const Route = createFileRoute('/$spaceSlug')({
  component: LegacySpaceRedirect,
})

function LegacySpaceRedirect() {
  const { spaceSlug } = Route.useParams()
  const { defaultLocale } = useDocsContext()
  const navigate = useNavigate()

  useEffect(() => {
    navigate({
      to: '/$locale/$spaceSlug',
      params: { locale: defaultLocale, spaceSlug },
      replace: true,
    })
  }, [defaultLocale, navigate, spaceSlug])

  return <LoadingState message="Redirecting..." />
}
