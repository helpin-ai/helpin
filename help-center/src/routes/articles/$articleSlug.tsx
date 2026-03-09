import { createFileRoute } from '@tanstack/react-router'
import { useArticle } from '@/hooks/queries'
import { ArticleContent } from '@/components/ArticleContent'
import { TableOfContents, type TocItem } from '@/components/layout/TableOfContents'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'
import { useMemo } from 'react'

export const Route = createFileRoute('/articles/$articleSlug')({
  component: ArticlePage,
})

function ArticlePage() {
  const { articleSlug } = Route.useParams()
  const subdomain = Route.useRouteContext({ select: (s) => s.subdomain })
  const { data: article, isLoading, error } = useArticle(subdomain, articleSlug)

  // Extract TOC headings from HTML content
  const tocItems = useMemo<TocItem[]>(() => {
    if (!article?.content_html) return []
    const matches = article.content_html.matchAll(
      /<h([23])\s+id="([^"]+)"[^>]*>(.*?)<\/h[23]>/gi,
    )
    return Array.from(matches).map((m) => ({
      level: parseInt(m[1]!, 10),
      id: m[2]!,
      text: m[3]!.replace(/<[^>]+>/g, ''),
    }))
  }, [article?.content_html])

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
        <ArticleContent article={article} />
      </div>
      <TableOfContents items={tocItems} />
    </div>
  )
}
