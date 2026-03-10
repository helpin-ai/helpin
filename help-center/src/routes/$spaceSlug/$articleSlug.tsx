import { createFileRoute } from '@tanstack/react-router'
import { useMemo } from 'react'
import { useArticle, useSpaceNavigation } from '@/hooks/queries'
import { useDocsContext } from '@/contexts/DocsContext'
import { useScrollSpy } from '@/hooks/useScrollSpy'
import { extractTocFromHtml } from '@/lib/toc'
import { getArticlePager } from '@/lib/navigation'
import { ArticleContent } from '@/components/ArticleContent'
import { TableOfContents } from '@/components/layout/TableOfContents'
import { Breadcrumbs } from '@/components/navigation/Breadcrumbs'
import { ArticlePager } from '@/components/navigation/ArticlePager'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'

export const Route = createFileRoute('/$spaceSlug/$articleSlug')({
  component: ArticlePage,
})

function ArticlePage() {
  const { spaceSlug, articleSlug } = Route.useParams()
  const { subdomain, spaces } = useDocsContext()

  const {
    data: article,
    isLoading,
    error,
  } = useArticle(subdomain, spaceSlug, articleSlug)

  const { data: navigation } = useSpaceNavigation(subdomain, spaceSlug)

  // Extract TOC headings from HTML content
  const tocItems = useMemo(
    () => (article?.content_html ? extractTocFromHtml(article.content_html) : []),
    [article?.content_html],
  )

  // Scroll spy for active TOC heading
  const tocIds = useMemo(() => tocItems.map((item) => item.id), [tocItems])
  const activeHeadingId = useScrollSpy(tocIds)

  // Prev/next article pager
  const pager = useMemo(
    () => (navigation ? getArticlePager(navigation, articleSlug) : {}),
    [navigation, articleSlug],
  )

  // Current space info for breadcrumbs
  const currentSpace = spaces.find((s) => s.slug === spaceSlug)

  if (isLoading) return <LoadingState />
  if (error || !article) {
    return (
      <ErrorState
        title="Article not found"
        message="This article does not exist or is not published."
        statusCode={404}
      />
    )
  }

  return (
    <div className="flex">
      <div className="flex-1 min-w-0">
        <article
          className="mx-auto py-8 px-6"
          style={{ maxWidth: 'var(--hc-content-max-width)' }}
        >
          {/* Breadcrumbs */}
          {currentSpace && (
            <div className="mb-6">
              <Breadcrumbs
                spaceSlug={spaceSlug}
                spaceName={currentSpace.name}
                collectionName={article.collection_name}
              />
            </div>
          )}

          {/* Article header */}
          <header className="mb-8">
            <h1 className="text-3xl font-bold leading-tight mb-3">
              {article.title}
            </h1>
            {article.excerpt && (
              <p
                className="text-base leading-relaxed"
                style={{ color: 'var(--hc-text-secondary)' }}
              >
                {article.excerpt}
              </p>
            )}
          </header>

          {/* Article content */}
          <ArticleContent html={article.content_html} />

          {/* Feedback */}
          <footer
            className="mt-12 pt-6 border-t"
            style={{ borderColor: 'var(--hc-border)' }}
          >
            <p
              className="text-sm mb-3"
              style={{ color: 'var(--hc-text-secondary)' }}
            >
              Was this article helpful?
            </p>
            <div className="flex gap-2">
              <button
                className="rounded-lg border px-4 py-2 text-sm transition-colors hover:bg-[var(--hc-bg-secondary)]"
                style={{ borderColor: 'var(--hc-border)' }}
              >
                Yes
              </button>
              <button
                className="rounded-lg border px-4 py-2 text-sm transition-colors hover:bg-[var(--hc-bg-secondary)]"
                style={{ borderColor: 'var(--hc-border)' }}
              >
                No
              </button>
            </div>
          </footer>

          {/* Prev/Next navigation */}
          <ArticlePager
            spaceSlug={spaceSlug}
            prev={pager.prev}
            next={pager.next}
          />
        </article>
      </div>

      <TableOfContents items={tocItems} activeId={activeHeadingId} />
    </div>
  )
}
