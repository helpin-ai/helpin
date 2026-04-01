import { createFileRoute } from '@tanstack/react-router'
import { useMemo } from 'react'
import { useDocsContext } from '@/contexts/DocsContext'
import { usePreviewArticle, useSpaceNavigation } from '@/hooks/queries'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { useScrollSpy } from '@/hooks/useScrollSpy'
import { prefetchPreviewRouteData } from '@/lib/routeData'
import { loadRootRouteData } from '@/lib/rootLoader'
import { buildPreviewHead } from '@/lib/seo'
import { extractTocFromHtml } from '@/lib/toc'
import { ArticleContent } from '@/components/ArticleContent'
import { Sidebar } from '@/components/layout/Sidebar'
import { TableOfContents } from '@/components/layout/TableOfContents'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'

export const Route = createFileRoute('/preview/$docId')({
  validateSearch: (search: Record<string, unknown>) => ({
    token: (search.token as string) || '',
  }),
  loader: async ({ context, location, params }) => {
    const rootData = await loadRootRouteData(context.queryClient, location.pathname)
    const routeSearch = location.search as { token?: string }

    if (!routeSearch.token) {
      return null
    }

    return prefetchPreviewRouteData(
      context.queryClient,
      rootData,
      params.docId,
      routeSearch.token,
    )
  },
  head: ({ matches, loaderData }) => {
    const rootData = matches[0]?.loaderData as
      | import('@/lib/rootLoader').RootRouteData
      | undefined

    return rootData ? buildPreviewHead(rootData, loaderData) : {}
  },
  component: PreviewPage,
})

function PreviewPage() {
  const { docId } = Route.useParams()
  const { token } = Route.useSearch()
  const { subdomain, defaultLocale, multilingualEnabled } = useDocsContext()

  const { data: article, isLoading, error } = usePreviewArticle(subdomain, docId, token)

  // Fetch space navigation for the sidebar once we know the space slug
  const spaceSlug = article?.space_slug ?? ''
  const { data: navigation } = useSpaceNavigation(
    subdomain,
    defaultLocale,
    spaceSlug,
    multilingualEnabled,
  )

  useDocumentTitle(article ? `Preview: ${article.title}` : 'Article Preview')

  const tocItems = useMemo(
    () => (article?.content_html ? extractTocFromHtml(article.content_html) : []),
    [article?.content_html],
  )
  const tocIds = useMemo(() => tocItems.map((item) => item.id), [tocItems])
  const activeHeadingId = useScrollSpy(tocIds)

  if (!token) {
    return (
      <ErrorState
        title="Preview token required"
        message="A valid preview token is needed to view this article preview."
        statusCode={401}
      />
    )
  }

  if (isLoading) return <LoadingState />

  if (error || !article) {
    return (
      <ErrorState
        title="Preview not available"
        message="The preview token may have expired or the document was not found. Go back to the editor and click Preview again."
        statusCode={404}
      />
    )
  }

  return (
    <div className="flex">
        {/* Space sidebar */}
        {navigation && spaceSlug && (
          <Sidebar
            locale={defaultLocale}
            navigation={navigation}
          />
        )}

        <div className="flex-1 min-w-0">
          <article
            className="mx-auto pt-16 pb-8 px-5 lg:px-6"
            style={{ maxWidth: 'var(--hc-content-max-width)' }}
          >
            {article.collection_name && (
              <div className="mb-2.5">
                <nav className="flex items-center gap-1.5 text-[13px] text-muted-foreground">
                  {article.space_name && (
                    <>
                      <span>{article.space_name}</span>
                      <span className="text-muted-foreground/50">/</span>
                    </>
                  )}
                  <span>{article.collection_name}</span>
                </nav>
              </div>
            )}

            <header className="mb-8">
              <h1 className="text-[1.875rem] font-bold leading-tight tracking-tight mb-2">
                {article.icon && <span className="mr-2">{article.icon}</span>}
                {article.title}
              </h1>
              {article.excerpt && (
                <p className="text-[15px] leading-relaxed text-muted-foreground">
                  {article.excerpt}
                </p>
              )}
            </header>

            <ArticleContent html={article.content_html} />

            {/* Feedback section (disabled in preview) */}
            <div className="mt-12 pt-6 border-t border-border">
              <p className="text-[13px] text-muted-foreground mb-3">
                Was this article helpful?
              </p>
              <div className="flex gap-2">
                <button
                  type="button"
                  disabled
                  className="inline-flex items-center gap-1.5 rounded-md border border-border px-3 py-1.5 text-[13px] text-muted-foreground opacity-50 cursor-not-allowed"
                >
                  Yes
                </button>
                <button
                  type="button"
                  disabled
                  className="inline-flex items-center gap-1.5 rounded-md border border-border px-3 py-1.5 text-[13px] text-muted-foreground opacity-50 cursor-not-allowed"
                >
                  No
                </button>
              </div>
              <p className="text-[11px] text-muted-foreground/50 mt-2">(Disabled in preview)</p>
            </div>
          </article>
        </div>

        <TableOfContents items={tocItems} activeId={activeHeadingId} />
    </div>
  )
}
