import { createFileRoute, redirect } from '@tanstack/react-router'
import { LoadingState } from '@/components/LoadingState'
import { LocalizedHomePage } from '@/components/home/LocalizedHomePage'
import { CollectionRouteView } from '@/components/routes/CollectionRouteView'
import { useDocsContext } from '@/contexts/DocsContext'
import { prefetchCollectionRouteData } from '@/lib/routeData'
import { loadRootRouteData } from '@/lib/rootLoader'
import { buildCollectionHead, buildHomeHead } from '@/lib/seo'
import {
  buildCanonicalCollectionPath,
  buildCanonicalHomePath,
  isMultilingualEnabled,
} from '@/lib/locale'

export const Route = createFileRoute('/$spaceSlug')({
  beforeLoad: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    const normalizedSlug = params.spaceSlug.trim().toLowerCase()
    const isKnownLocaleSlug = rootData.config.enabled_locales.some(
      (locale) => locale.toLowerCase() === normalizedSlug,
    )

    if (rootData.multilingualEnabled && !isKnownLocaleSlug) {
      throw redirect({
        statusCode: 301,
        to: buildCanonicalCollectionPath(
          true,
          rootData.config.default_locale,
          params.spaceSlug,
        ),
      })
    }

    if (!rootData.multilingualEnabled && isKnownLocaleSlug) {
      throw redirect({
        statusCode: 301,
        to: buildCanonicalHomePath(false, rootData.config.default_locale),
      })
    }
  },
  loader: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    const normalizedSlug = params.spaceSlug.trim().toLowerCase()
    const isKnownLocaleSlug = rootData.config.enabled_locales.some(
      (locale) => locale.toLowerCase() === normalizedSlug,
    )

    if (!rootData.multilingualEnabled && !isKnownLocaleSlug) {
      const collection = await prefetchCollectionRouteData(
        context.queryClient,
        rootData,
        params.spaceSlug,
      )
      return { kind: 'collection' as const, rootData, collection }
    }

    return { kind: 'home' as const, rootData, collection: null }
  },
  head: ({ loaderData, params }) => {
    if (!loaderData) {
      return {}
    }

    return loaderData.kind === 'collection' && loaderData.collection
      ? buildCollectionHead(
          loaderData.rootData,
          loaderData.collection,
          params.spaceSlug,
        )
      : buildHomeHead(loaderData.rootData)
  },
  component: LegacySpaceRedirect,
})

function LegacySpaceRedirect() {
  const { spaceSlug } = Route.useParams()
  const { defaultLocale, enabledLocales } = useDocsContext()
  const normalizedSlug = spaceSlug.trim().toLowerCase()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)
  const isKnownLocaleSlug = enabledLocales.some(
    (locale) => locale.toLowerCase() === normalizedSlug,
  )

  if (multilingualEnabled && isKnownLocaleSlug) {
    return <LocalizedHomePage />
  }

  if (!multilingualEnabled && !isKnownLocaleSlug) {
    return (
      <CollectionRouteView
        locale={defaultLocale}
        collectionOrSpaceSlug={spaceSlug}
        multilingualEnabled={false}
      />
    )
  }

  return <LoadingState message="Redirecting..." />
}
