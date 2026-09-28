import { createFileRoute, redirect } from '@tanstack/react-router'
import { CollectionRouteView } from '@/components/routes/CollectionRouteView'
import { LoadingState } from '@/components/LoadingState'
import { useDocsContext } from '@/contexts/DocsContext'
import { prefixBasepath } from '@/lib/pathUtils'
import { prefetchSpaceRouteData } from '@/lib/routeData'
import { loadRootRouteData } from '@/lib/rootLoader'

export const Route = createFileRoute('/$locale/$spaceSlug/')({
  beforeLoad: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    const isValidLocaleParam = rootData.config.enabled_locales.some(
      (enabledLocale) => enabledLocale.toLowerCase() === params.locale.toLowerCase(),
    )

    if (!isValidLocaleParam && !rootData.multilingualEnabled) {
      return
    }

    if (!isValidLocaleParam && rootData.multilingualEnabled) {
      throw redirect({
        statusCode: 301,
        href: `/${rootData.config.default_locale}/${params.spaceSlug}`,
      })
    }

    if (!rootData.multilingualEnabled) {
      throw redirect({
        statusCode: 301,
        href: `/${params.spaceSlug}`,
      })
    }
  },
  loader: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    await prefetchSpaceRouteData(context.queryClient, rootData, params.spaceSlug)
  },
  component: LocalizedSpaceIndex,
})

function LocalizedSpaceIndex() {
  const { locale: localeParam, spaceSlug } = Route.useParams()
  const { basepath, enabledLocales, multilingualEnabled } = useDocsContext()
  const isValidLocaleParam = enabledLocales.some(
    (enabledLocale) => enabledLocale.toLowerCase() === localeParam.toLowerCase(),
  )

  if (!isValidLocaleParam && !multilingualEnabled) {
    if (typeof window !== 'undefined') {
      window.location.replace(prefixBasepath(basepath, `/${localeParam}/${spaceSlug}`))
    }
    return <LoadingState message="Redirecting..." />
  }

  if (!isValidLocaleParam || !multilingualEnabled) {
    return <LoadingState message="Redirecting..." />
  }

  return (
    <CollectionRouteView
      locale={localeParam}
      collectionOrSpaceSlug={spaceSlug}
      multilingualEnabled
    />
  )
}
