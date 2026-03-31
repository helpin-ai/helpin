import { createFileRoute, redirect } from '@tanstack/react-router'
import { LocalizedHomePage } from '@/components/home/LocalizedHomePage'
import { CollectionRouteView } from '@/components/routes/CollectionRouteView'
import { LoadingState } from '@/components/LoadingState'
import { useDocsContext } from '@/contexts/DocsContext'
import { prefetchCollectionRouteData } from '@/lib/routeData'
import { loadRootRouteData } from '@/lib/rootLoader'
import { buildCollectionHead, buildHomeHead } from '@/lib/seo'
import {
  buildCanonicalCollectionPath,
  buildCanonicalHomePath,
  isMultilingualEnabled,
} from '@/lib/locale'

export const Route = createFileRoute('/$locale/')({
  beforeLoad: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    const isValidLocaleParam = rootData.config.enabled_locales.some(
      (enabledLocale) => enabledLocale.toLowerCase() === params.locale.toLowerCase(),
    )

    if (!isValidLocaleParam && rootData.multilingualEnabled) {
      throw redirect({
        statusCode: 301,
        to: buildCanonicalCollectionPath(
          true,
          rootData.config.default_locale,
          params.locale,
        ),
      })
    }

    if (isValidLocaleParam && !rootData.multilingualEnabled) {
      throw redirect({
        statusCode: 301,
        to: buildCanonicalHomePath(false, rootData.config.default_locale),
      })
    }
  },
  loader: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    const isValidLocaleParam = rootData.config.enabled_locales.some(
      (enabledLocale) => enabledLocale.toLowerCase() === params.locale.toLowerCase(),
    )

    if (!isValidLocaleParam && !rootData.multilingualEnabled) {
      const collection = await prefetchCollectionRouteData(
        context.queryClient,
        rootData,
        params.locale,
      )
      return { kind: 'collection' as const, collection, rootData, alternates: [] }
    }

    return { kind: 'home' as const, collection: null, rootData, alternates: [] }
  },
  head: ({ loaderData, params }) => {
    if (!loaderData) {
      return {}
    }

    return loaderData.kind === 'collection' && loaderData.collection
      ? buildCollectionHead(
          loaderData.rootData,
          loaderData.collection,
          params.locale,
          loaderData.alternates,
        )
      : buildHomeHead(loaderData.rootData)
  },
  component: LocalizedHomeRoute,
})

function LocalizedHomeRoute() {
  const { defaultLocale, enabledLocales } = useDocsContext()
  const { locale: localeParam } = Route.useParams()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)
  const isValidLocaleParam = enabledLocales.some(
    (enabledLocale) => enabledLocale.toLowerCase() === localeParam.toLowerCase(),
  )

  if (!isValidLocaleParam && !multilingualEnabled) {
    return (
      <CollectionRouteView
        locale={defaultLocale}
        collectionOrSpaceSlug={localeParam}
        multilingualEnabled={false}
      />
    )
  }

  if (!isValidLocaleParam || !multilingualEnabled) {
    return <LoadingState message="Redirecting..." />
  }

  return <LocalizedHomePage />
}
