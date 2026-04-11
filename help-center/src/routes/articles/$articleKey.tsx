import { createFileRoute, redirect } from '@tanstack/react-router'
import { ArticleRouteView } from '@/components/routes/ArticleRouteView'
import { LoadingState } from '@/components/LoadingState'
import { useDocsContext } from '@/contexts/DocsContext'
import { prefetchArticleRouteData } from '@/lib/routeData'
import { loadRootRouteData } from '@/lib/rootLoader'
import { buildArticleHead } from '@/lib/seo'

export const Route = createFileRoute('/articles/$articleKey')({
  beforeLoad: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)

    if (rootData.multilingualEnabled) {
      throw redirect({
        statusCode: 301,
        href: `/${rootData.config.default_locale}/articles/${params.articleKey}`,
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
    return { rootData, article, alternates: [] }
  },
  head: ({ loaderData }) =>
    loaderData?.article
      ? buildArticleHead(loaderData.rootData, loaderData.article, loaderData.alternates)
      : {},
  component: ArticlePage,
})

function ArticlePage() {
  const { articleKey } = Route.useParams()
  const { locale, multilingualEnabled } = useDocsContext()

  if (multilingualEnabled) {
    return <LoadingState message="Redirecting..." />
  }

  return (
    <ArticleRouteView
      locale={locale}
      articleKey={articleKey}
      multilingualEnabled={false}
    />
  )
}
