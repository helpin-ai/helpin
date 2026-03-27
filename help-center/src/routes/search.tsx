import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { useDocsContext } from '@/contexts/DocsContext'
import { LoadingState } from '@/components/LoadingState'
import { SearchRouteView } from '@/components/routes/SearchRouteView'
import { buildCanonicalSearchPath, isMultilingualEnabled } from '@/lib/locale'

interface SearchParams {
  q?: string
  space?: string
}

export const Route = createFileRoute('/search')({
  validateSearch: (search: Record<string, unknown>): SearchParams => ({
    q: typeof search.q === 'string' ? search.q : undefined,
    space: typeof search.space === 'string' ? search.space : undefined,
  }),
  component: SearchRoute,
})

function SearchRoute() {
  const { q = '', space } = Route.useSearch()
  const { defaultLocale, enabledLocales } = useDocsContext()
  const navigate = useNavigate()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)

  useEffect(() => {
    if (!multilingualEnabled) {
      return
    }
    navigate({
      to: buildCanonicalSearchPath(true, defaultLocale, q, space),
      replace: true,
    })
  }, [defaultLocale, multilingualEnabled, navigate, q, space])

  if (!multilingualEnabled) {
    return <SearchRouteView locale={defaultLocale} query={q} space={space} />
  }

  return <LoadingState message="Redirecting..." />
}
