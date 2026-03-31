import { createFileRoute, redirect } from '@tanstack/react-router'
import { useDocsContext } from '@/contexts/DocsContext'
import { LoadingState } from '@/components/LoadingState'
import { ArticleRouteView } from '@/components/routes/ArticleRouteView'
import { prefetchArticleRouteData } from '@/lib/routeData'
import { loadRootRouteData } from '@/lib/rootLoader'
import { buildArticleHead } from '@/lib/seo'
import {
  buildCanonicalArticlePath,
  buildCanonicalCollectionPath,
  isMultilingualEnabled,
} from '@/lib/locale'

export const Route = createFileRoute('/$spaceSlug/$articleSlug')({
  beforeLoad: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
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
      return { article, rootData, alternates: [] }
    }
  },
  head: ({ loaderData, params }) =>
    loaderData?.article
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
  const { defaultLocale, enabledLocales } = useDocsContext()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)
  const normalizedSpaceSlug = spaceSlug.trim().toLowerCase()
  const isKnownLocaleSlug = enabledLocales.some(
    (locale) => locale.toLowerCase() === normalizedSpaceSlug,
  )

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
