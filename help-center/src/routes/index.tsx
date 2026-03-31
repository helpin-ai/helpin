import { createFileRoute, redirect } from '@tanstack/react-router'
import { LoadingState } from '@/components/LoadingState'
import { LocalizedHomePage } from '@/components/home/LocalizedHomePage'
import { useDocsContext } from '@/contexts/DocsContext'
import { loadRootRouteData } from '@/lib/rootLoader'
import { prefetchHomeRouteData } from '@/lib/routeData'
import { buildHomeHead } from '@/lib/seo'
import { buildCanonicalHomePath, isMultilingualEnabled } from '@/lib/locale'

export const Route = createFileRoute('/')({
  beforeLoad: async ({ context, location }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)

    if (rootData.multilingualEnabled) {
      throw redirect({
        statusCode: 301,
        to: buildCanonicalHomePath(true, rootData.config.default_locale),
      })
    }
  },
  loader: async ({ context, location }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    await prefetchHomeRouteData(context.queryClient, rootData)
    return rootData
  },
  head: ({ loaderData }) =>
    loaderData ? buildHomeHead(loaderData) : {},
  component: RootLocaleRedirect,
})

function RootLocaleRedirect() {
  const { enabledLocales } = useDocsContext()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)

  if (!multilingualEnabled) {
    return <LocalizedHomePage />
  }

  return <LoadingState message="Redirecting..." />
}
