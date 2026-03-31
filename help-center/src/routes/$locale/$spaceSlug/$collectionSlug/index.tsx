import { createFileRoute, redirect } from '@tanstack/react-router'
import { ArticleRouteView } from '@/components/routes/ArticleRouteView'
import { useDocsContext } from '@/contexts/DocsContext'
import { LoadingState } from '@/components/LoadingState'
import { prefetchArticleRouteData } from '@/lib/routeData'
import { loadRootRouteData } from '@/lib/rootLoader'
import { buildArticleHead } from '@/lib/seo'
import { buildCanonicalArticlePath, isMultilingualEnabled } from '@/lib/locale'

export const Route = createFileRoute('/$locale/$spaceSlug/$collectionSlug/')({
  beforeLoad: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)

    if (!rootData.multilingualEnabled) {
      throw redirect({
        statusCode: 301,
        to: buildCanonicalArticlePath(
          false,
          rootData.config.default_locale,
          params.spaceSlug,
          params.collectionSlug,
        ),
      })
    }
  },
  loader: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)

    if (rootData.multilingualEnabled) {
      const article = await prefetchArticleRouteData(
        context.queryClient,
        rootData,
        params.spaceSlug,
        params.collectionSlug,
      )
      return { article, rootData }
    }
  },
  head: ({ loaderData, params }) =>
    loaderData?.article
      ? buildArticleHead(
          loaderData.rootData,
          loaderData.article,
          params.spaceSlug,
          params.collectionSlug,
        )
      : {},
  component: LocalizedArticleOrLegacyCollectionRoute,
})

function LocalizedArticleOrLegacyCollectionRoute() {
  const { locale, spaceSlug, collectionSlug } = Route.useParams()
  const { enabledLocales } = useDocsContext()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)

  if (!multilingualEnabled) {
    return <LoadingState message="Redirecting..." />
  }

  return (
    <ArticleRouteView
      locale={locale}
      collectionSlug={spaceSlug}
      articleSlug={collectionSlug}
      multilingualEnabled
    />
  )
}
