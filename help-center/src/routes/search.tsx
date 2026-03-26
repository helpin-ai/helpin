import { createFileRoute } from '@tanstack/react-router'
import { useDocsContext } from '@/contexts/DocsContext'
import { LoadingState } from '@/components/LoadingState'
import { buildLocaleSearchPath } from '@/lib/locale'
import { useEffect } from 'react'
import { useNavigate } from '@tanstack/react-router'

interface SearchParams {
  q?: string
  space?: string
}

export const Route = createFileRoute('/search')({
  validateSearch: (search: Record<string, unknown>): SearchParams => ({
    q: typeof search.q === 'string' ? search.q : undefined,
    space: typeof search.space === 'string' ? search.space : undefined,
  }),
  component: LegacySearchRedirect,
})

function LegacySearchRedirect() {
  const { q = '', space } = Route.useSearch()
  const { defaultLocale } = useDocsContext()
  const navigate = useNavigate()

  useEffect(() => {
    navigate({
      to: buildLocaleSearchPath(defaultLocale, q, space),
      replace: true,
    })
  }, [defaultLocale, navigate, q, space])

  return <LoadingState message="Redirecting..." />
}
