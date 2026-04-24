import { useCallback, useMemo, useState } from 'react'
import { Menu } from 'lucide-react'
import { useArticle, useSpaceNavigation } from '@/hooks/queries'
import { useDocsContext } from '@/contexts/DocsContext'
import { useScrollSpy } from '@/hooks/useScrollSpy'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { extractTocFromHtml } from '@/lib/toc'
import { ArticleShell } from '@/components/article/ArticleShell'
import { ArticleContent } from '@/components/ArticleContent'
import { TableOfContents } from '@/components/layout/TableOfContents'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'
import { Sidebar } from '@/components/layout/Sidebar'
import { MobileNav } from '@/components/navigation/MobileNav'
import { getArticlePager } from '@/lib/navigation'

interface ArticleRouteViewProps {
  locale: string
  articleKey: string
  multilingualEnabled: boolean
}

export function ArticleRouteView({
  locale,
  articleKey,
  multilingualEnabled,
}: ArticleRouteViewProps) {
  const { subdomain } = useDocsContext()
  const [mobileNavOpen, setMobileNavOpen] = useState(false)
  const closeMobileNav = useCallback(() => setMobileNavOpen(false), [])
  const { data: article, isLoading, error } = useArticle(
    subdomain,
    locale,
    articleKey,
    multilingualEnabled,
  )
  const resolvedSpaceSlug = article?.space_slug ?? ''
  const { data: navigation = [] } =
    useSpaceNavigation(
      subdomain,
      locale,
      resolvedSpaceSlug,
      multilingualEnabled,
    )

  const tocItems = useMemo(
    () => (article?.content_html ? extractTocFromHtml(article.content_html) : []),
    [article?.content_html],
  )
  const tocIds = useMemo(() => tocItems.map((item) => item.id), [tocItems])
  const activeHeadingId = useScrollSpy(tocIds)
  const pager = useMemo(
    () => getArticlePager(navigation, article?.public_id ?? ''),
    [navigation, article?.public_id],
  )

  useDocumentTitle(article?.title)

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
      {mobileNavOpen && navigation.length > 0 && (
        <MobileNav
          locale={locale}
          navigation={navigation}
          onClose={closeMobileNav}
        />
      )}

      {navigation.length > 0 && (
        <>
          <div className="fixed left-0 right-0 top-[var(--hc-header-height)] z-20 flex items-center gap-2 border-b border-border bg-background px-4 py-2 lg:hidden">
            <button
              onClick={() => setMobileNavOpen(true)}
              className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
              aria-label="Open navigation"
            >
              <Menu size={18} />
            </button>
            <span className="truncate text-[13px] font-medium">
              {article.title}
            </span>
          </div>
          <Sidebar locale={locale} navigation={navigation} />
        </>
      )}

      <div className="min-w-0 flex-1">
        <ArticleShell
          locale={locale}
          title={article.seo_title || article.title}
          excerpt={article.excerpt}
          collectionName={article.collection_name}
          collectionSlug={article.collection_slug}
          collectionPublicId={article.collection_public_id}
          articleSlug={article.slug}
          articlePublicId={article.public_id}
          pager={pager}
          multilingualEnabled={multilingualEnabled}
        >
          <ArticleContent html={article.content_html} />
        </ArticleShell>
      </div>

      <TableOfContents items={tocItems} activeId={activeHeadingId} />
    </div>
  )
}
