import { createFileRoute, redirect } from '@tanstack/react-router'
import { useDocsContext } from '@/contexts/DocsContext'
import { LoadingState } from '@/components/LoadingState'
import { SearchRouteView } from '@/components/routes/SearchRouteView'
import { prefetchSearchRouteData } from '@/lib/routeData'
import { loadRootRouteData } from '@/lib/rootLoader'
import { buildSearchHead } from '@/lib/seo'
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
  beforeLoad: async ({ context, location }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    const routeSearch = location.search as SearchParams

    if (rootData.multilingualEnabled) {
      throw redirect({
        statusCode: 301,
        to: buildCanonicalSearchPath(
          true,
          rootData.config.default_locale,
          routeSearch.q,
          routeSearch.space,
        ),
      })
    }
  },
  loader: async ({ context, location }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    const routeSearch = location.search as SearchParams
    await prefetchSearchRouteData(
      context.queryClient,
      rootData,
      routeSearch.q ?? '',
      routeSearch.space,
    )
    return { rootData, search: routeSearch }
  },
  head: ({ loaderData }) =>
    loaderData
      ? buildSearchHead(
          loaderData.rootData,
          loaderData.search.q,
          loaderData.search.space,
        )
      : {},
  component: SearchRoute,
})

function SearchRoute() {
  const { q = '', space } = Route.useSearch()
  const { locale, enabledLocales } = useDocsContext()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)

  if (!multilingualEnabled) {
    return <SearchRouteView locale={locale} query={q} space={space} />
  }

  return <LoadingState message="Redirecting..." />
}
