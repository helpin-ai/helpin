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

export const Route = createFileRoute('/$locale/search')({
  validateSearch: (search: Record<string, unknown>): SearchParams => ({
    q: typeof search.q === 'string' ? search.q : undefined,
    space: typeof search.space === 'string' ? search.space : undefined,
  }),
  component: LocalizedSearchPage,
})

function LocalizedSearchPage() {
  const { q = '', space } = Route.useSearch()
  const { locale, defaultLocale, enabledLocales } = useDocsContext()
  const navigate = useNavigate()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)

  useEffect(() => {
    if (multilingualEnabled) {
      return
    }
    navigate({
      to: buildCanonicalSearchPath(false, defaultLocale, q, space),
      replace: true,
    })
  }, [defaultLocale, multilingualEnabled, navigate, q, space])

  if (!multilingualEnabled) {
    return <LoadingState message="Redirecting..." />
  }

  return <SearchRouteView locale={locale} query={q} space={space} />
}
