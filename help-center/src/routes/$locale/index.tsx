import { createFileRoute, redirect } from '@tanstack/react-router'
import { LocalizedHomePage } from '@/components/home/LocalizedHomePage'
import { LoadingState } from '@/components/LoadingState'
import { useDocsContext } from '@/contexts/DocsContext'
import { prefetchHomeRouteData } from '@/lib/routeData'
import { loadRootRouteData } from '@/lib/rootLoader'
import { buildHomeHead } from '@/lib/seo'
import { isMultilingualEnabled } from '@/lib/locale'

export const Route = createFileRoute('/$locale/')({
  beforeLoad: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    const isValidLocaleParam = rootData.config.enabled_locales.some(
      (enabledLocale) => enabledLocale.toLowerCase() === params.locale.toLowerCase(),
    )

    if (!isValidLocaleParam && rootData.multilingualEnabled) {
      throw redirect({
        statusCode: 301,
        href: `/${rootData.config.default_locale}`,
      })
    }

    if (isValidLocaleParam && !rootData.multilingualEnabled) {
      throw redirect({
        statusCode: 301,
        to: '/',
      })
    }

    if (!isValidLocaleParam && !rootData.multilingualEnabled) {
      throw redirect({
        statusCode: 301,
        href: `/c/${params.locale}`,
      })
    }
  },
  loader: async ({ context, location }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    await prefetchHomeRouteData(context.queryClient, rootData)
    return rootData
  },
  head: ({ loaderData }) => (loaderData ? buildHomeHead(loaderData) : {}),
  component: LocalizedHomeRoute,
})

function LocalizedHomeRoute() {
  const { enabledLocales } = useDocsContext()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)

  if (!multilingualEnabled) {
    return <LoadingState message="Redirecting..." />
  }

  return <LocalizedHomePage />
}
