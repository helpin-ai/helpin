import { createFileRoute, redirect } from '@tanstack/react-router'
import { useDocsContext } from '@/contexts/DocsContext'
import { LoadingState } from '@/components/LoadingState'
import { ArticleRouteView } from '@/components/routes/ArticleRouteView'
import { CollectionRouteView } from '@/components/routes/CollectionRouteView'
import { prefetchArticleRouteData, prefetchCollectionRouteData } from '@/lib/routeData'
import { loadRootRouteData } from '@/lib/rootLoader'
import { buildArticleHead, buildCollectionHead } from '@/lib/seo'
import {
  buildCanonicalArticlePath,
  buildCanonicalCollectionPath,
  isMultilingualEnabled,
} from '@/lib/locale'
import { stripBasepath } from '@/lib/pathUtils'

export const Route = createFileRoute('/$spaceSlug/$articleSlug')({
  beforeLoad: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    const internalPath = stripBasepath(location.pathname, rootData.basepath)
    const internalSegments = internalPath.split('/').filter(Boolean)

    if (!rootData.multilingualEnabled && internalSegments.length === 1) {
      return
    }

    const normalizedSpaceSlug = params.spaceSlug.trim().toLowerCase()
    const isKnownLocaleSlug = rootData.config.enabled_locales.some(
      (locale) => locale.toLowerCase() === normalizedSpaceSlug,
    )

    if (rootData.multilingualEnabled) {
      throw redirect({
        statusCode: 301,
        to: buildCanonicalArticlePath(
          true,
          rootData.config.default_locale,
          params.spaceSlug,
          params.articleSlug,
        ),
      })
    }

    if (isKnownLocaleSlug) {
      throw redirect({
        statusCode: 301,
        to: buildCanonicalCollectionPath(
          false,
          rootData.config.default_locale,
          params.articleSlug,
        ),
      })
    }
  },
  loader: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    const internalPath = stripBasepath(location.pathname, rootData.basepath)
    const internalSegments = internalPath.split('/').filter(Boolean)
    if (!rootData.multilingualEnabled && internalSegments.length === 1) {
      const collection = await prefetchCollectionRouteData(
        context.queryClient,
        rootData,
        params.articleSlug,
      )
      return { kind: 'collection' as const, collection, rootData, alternates: [] }
    }

    const normalizedSpaceSlug = params.spaceSlug.trim().toLowerCase()
    const isKnownLocaleSlug = rootData.config.enabled_locales.some(
      (locale) => locale.toLowerCase() === normalizedSpaceSlug,
    )

    if (!rootData.multilingualEnabled && !isKnownLocaleSlug) {
      const article = await prefetchArticleRouteData(
        context.queryClient,
        rootData,
        params.spaceSlug,
        params.articleSlug,
      )
      return { kind: 'article' as const, article, rootData, alternates: [] }
    }
  },
  head: ({ loaderData, params }) =>
    loaderData?.kind === 'collection' && loaderData.collection
      ? buildCollectionHead(
          loaderData.rootData,
          loaderData.collection,
          params.articleSlug,
          loaderData.alternates,
        )
      : loaderData?.kind === 'article' && loaderData.article
      ? buildArticleHead(
          loaderData.rootData,
          loaderData.article,
          params.spaceSlug,
          params.articleSlug,
          loaderData.alternates,
        )
      : {},
  component: LegacyArticleRedirect,
})

function LegacyArticleRedirect() {
  const { spaceSlug, articleSlug } = Route.useParams()
  const { basepath, defaultLocale, enabledLocales } = useDocsContext()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)
  const normalizedSpaceSlug = spaceSlug.trim().toLowerCase()
  const isKnownLocaleSlug = enabledLocales.some(
    (locale) => locale.toLowerCase() === normalizedSpaceSlug,
  )

  if (!multilingualEnabled && basepath === `/${spaceSlug}`) {
    return (
      <CollectionRouteView
        locale={defaultLocale}
        collectionOrSpaceSlug={articleSlug}
        multilingualEnabled={false}
      />
    )
  }

  if (!multilingualEnabled && !isKnownLocaleSlug) {
    return (
      <ArticleRouteView
        locale={defaultLocale}
        collectionSlug={spaceSlug}
        articleSlug={articleSlug}
        multilingualEnabled={false}
      />
    )
  }

  return <LoadingState message="Redirecting..." />
}
