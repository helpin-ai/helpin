import { createFileRoute, redirect } from '@tanstack/react-router'
import { ArticleRouteView } from '@/components/routes/ArticleRouteView'
import { LoadingState } from '@/components/LoadingState'
import { useDocsContext } from '@/contexts/DocsContext'
import { prefetchArticleRouteData } from '@/lib/routeData'
import { loadAlternateLinks } from '@/lib/alternateLinks'
import { loadRootRouteData } from '@/lib/rootLoader'
import { buildArticleHead } from '@/lib/seo'

export const Route = createFileRoute('/$locale/articles/$articleKey')({
  beforeLoad: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    if (!rootData.multilingualEnabled) {
      throw redirect({
        statusCode: 301,
        href: `/articles/${params.articleKey}`,
      })
    }
  },
  loader: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    const article = await prefetchArticleRouteData(
      context.queryClient,
      rootData,
      params.articleKey,
    )
    const alternates = article
      ? await loadAlternateLinks(context.queryClient, rootData, {
          kind: 'article',
          spaceId: rootData.spaces.find(
            (space) => space.slug === article.space_slug,
          )?.id,
          collectionId: article.collection_id ?? undefined,
          articleId: article.id,
        })
      : []

    return { rootData, article, alternates }
  },
  head: ({ loaderData }) =>
    loaderData?.article
      ? buildArticleHead(loaderData.rootData, loaderData.article, loaderData.alternates)
      : {},
  component: LocalizedArticlePage,
})

function LocalizedArticlePage() {
  const { articleKey } = Route.useParams()
  const { locale, multilingualEnabled } = useDocsContext()

  if (!multilingualEnabled) {
    return <LoadingState message="Redirecting..." />
  }

  return (
    <ArticleRouteView
      locale={locale}
      articleKey={articleKey}
      multilingualEnabled
    />
  )
}
