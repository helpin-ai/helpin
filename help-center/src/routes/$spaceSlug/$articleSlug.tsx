import { createFileRoute } from '@tanstack/react-router'
import { useMemo } from 'react'
import { useArticle } from '@/hooks/queries'
import { useDocsContext } from '@/contexts/DocsContext'
import { useSpaceContext } from '@/contexts/SpaceContext'
import { useScrollSpy } from '@/hooks/useScrollSpy'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { extractTocFromHtml } from '@/lib/toc'
import { ArticleShell } from '@/components/article/ArticleShell'
import { ArticleContent } from '@/components/ArticleContent'
import { TableOfContents } from '@/components/layout/TableOfContents'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'

export const Route = createFileRoute('/$spaceSlug/$articleSlug')({
  component: ArticlePage,
})

function ArticlePage() {
  const { spaceSlug, articleSlug } = Route.useParams()
  const { subdomain } = useDocsContext()
  const { space, getPager, getCollectionName } = useSpaceContext()

  const { data: article, isLoading, error } = useArticle(subdomain, spaceSlug, articleSlug)

  // Document title
  useDocumentTitle(article?.title)

  // Extract TOC headings from HTML content
  const tocItems = useMemo(
    () => (article?.content_html ? extractTocFromHtml(article.content_html) : []),
    [article?.content_html],
  )

  // Scroll spy for active TOC heading
  const tocIds = useMemo(() => tocItems.map((item) => item.id), [tocItems])
  const activeHeadingId = useScrollSpy(tocIds)

  // Prev/next from space context (no duplicate query)
  const pager = useMemo(() => getPager(articleSlug), [getPager, articleSlug])

  // Collection name from space context (fallback to article data)
  const collectionName = getCollectionName(articleSlug) ?? article?.collection_name

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
        <ArticleShell
          title={article.seo_title || article.title}
          excerpt={article.excerpt}
          spaceSlug={spaceSlug}
          spaceName={space?.name}
          collectionName={collectionName}
          articleSlug={articleSlug}
          pager={pager}
        >
          <ArticleContent html={article.content_html} />
        </ArticleShell>
      </div>

      <TableOfContents items={tocItems} activeId={activeHeadingId} />
    </div>
  )
}
