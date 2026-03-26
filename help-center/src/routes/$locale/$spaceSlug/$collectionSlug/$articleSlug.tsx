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

export const Route = createFileRoute(
  '/$locale/$spaceSlug/$collectionSlug/$articleSlug',
)({
  component: LocalizedArticlePage,
})

function LocalizedArticlePage() {
  const { locale, spaceSlug, collectionSlug, articleSlug } = Route.useParams()
  const { subdomain } = useDocsContext()
  const { space, getPager, getCollectionName } = useSpaceContext()

  const { data: article, isLoading, error } = useArticle(
    subdomain,
    locale,
    spaceSlug,
    collectionSlug,
    articleSlug,
  )

  useDocumentTitle(article?.title)

  const tocItems = useMemo(
    () => (article?.content_html ? extractTocFromHtml(article.content_html) : []),
    [article?.content_html],
  )

  const tocIds = useMemo(() => tocItems.map((item) => item.id), [tocItems])
  const activeHeadingId = useScrollSpy(tocIds)

  const pager = useMemo(() => getPager(articleSlug), [getPager, articleSlug])
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
      <div className="min-w-0 flex-1">
        <ArticleShell
          locale={locale}
          title={article.seo_title || article.title}
          excerpt={article.excerpt}
          spaceSlug={spaceSlug}
          spaceName={space?.name}
          collectionName={collectionName}
          collectionSlug={collectionSlug}
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
