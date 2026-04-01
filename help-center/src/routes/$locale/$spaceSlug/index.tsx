import { createFileRoute, redirect } from '@tanstack/react-router'
import { ArticleRouteView } from '@/components/routes/ArticleRouteView'
import { CollectionRouteView } from '@/components/routes/CollectionRouteView'
import { useDocsContext } from '@/contexts/DocsContext'
import { LoadingState } from '@/components/LoadingState'
import {
  prefetchArticleRouteData,
  prefetchCollectionRouteData,
} from '@/lib/routeData'
import { loadAlternateLinks } from '@/lib/alternateLinks'
import { loadRootRouteData } from '@/lib/rootLoader'
import { buildArticleHead, buildCollectionHead } from '@/lib/seo'
import {
  buildCanonicalArticlePath,
  buildCanonicalCollectionPath,
  isMultilingualEnabled,
} from '@/lib/locale'

export const Route = createFileRoute('/$locale/$spaceSlug/')({
  beforeLoad: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    const isValidLocaleParam = rootData.config.enabled_locales.some(
      (enabledLocale) => enabledLocale.toLowerCase() === params.locale.toLowerCase(),
    )

    if (!isValidLocaleParam && rootData.multilingualEnabled) {
      throw redirect({
        statusCode: 301,
        to: buildCanonicalArticlePath(
          true,
          rootData.config.default_locale,
          params.locale,
          params.spaceSlug,
        ),
      })
    }

    if (isValidLocaleParam && !rootData.multilingualEnabled) {
      throw redirect({
        statusCode: 301,
        to: buildCanonicalCollectionPath(
          false,
          rootData.config.default_locale,
          params.spaceSlug,
        ),
      })
    }
  },
  loader: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    const isValidLocaleParam = rootData.config.enabled_locales.some(
      (enabledLocale) => enabledLocale.toLowerCase() === params.locale.toLowerCase(),
    )

    if (!isValidLocaleParam && !rootData.multilingualEnabled) {
      const article = await prefetchArticleRouteData(
        context.queryClient,
        rootData,
        params.locale,
        params.spaceSlug,
      )
      return {
        kind: 'article' as const,
        article,
        collection: null,
        rootData,
        alternates: [],
      }
    }

    if (isValidLocaleParam && rootData.multilingualEnabled) {
      const collection = await prefetchCollectionRouteData(
        context.queryClient,
        rootData,
        params.spaceSlug,
      )
      const alternates = collection
        ? await loadAlternateLinks(context.queryClient, rootData, {
            kind: 'collection',
            spaceId: rootData.spaces.find(
              (space) => space.slug === (collection.space_slug || params.spaceSlug),
            )?.id,
            collectionId: collection.collection.id,
          })
        : []

      return {
        kind: 'collection' as const,
        article: null,
        collection,
        rootData,
        alternates,
      }
    }
  },
  head: ({ loaderData, params }) => {
    if (!loaderData) {
      return {}
    }

    if (loaderData.kind === 'article' && loaderData.article) {
      return buildArticleHead(
        loaderData.rootData,
        loaderData.article,
        params.locale,
        params.spaceSlug,
        loaderData.alternates,
      )
    }

    if (loaderData.kind === 'collection' && loaderData.collection) {
      return buildCollectionHead(
        loaderData.rootData,
        loaderData.collection,
        params.spaceSlug,
        loaderData.alternates,
      )
    }

    return {}
  },
  component: LocalizedSpaceIndex,
})

function LocalizedSpaceIndex() {
  const { locale: localeParam, spaceSlug } = Route.useParams()
  const { defaultLocale, enabledLocales } = useDocsContext()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)
  const isValidLocaleParam = enabledLocales.some(
    (enabledLocale) => enabledLocale.toLowerCase() === localeParam.toLowerCase(),
  )

  if (!isValidLocaleParam && !multilingualEnabled) {
    return (
      <ArticleRouteView
        locale={defaultLocale}
        collectionSlug={localeParam}
        articleSlug={spaceSlug}
        multilingualEnabled={false}
      />
    )
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
