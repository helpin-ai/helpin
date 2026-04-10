import { useCallback, useEffect, useMemo, useState } from 'react'
import { Menu } from 'lucide-react'
import { useArticle, useSpaceNavigation } from '@/hooks/queries'
import { useDocsContext } from '@/contexts/DocsContext'
import { useScrollSpy } from '@/hooks/useScrollSpy'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { extractTocFromHtml } from '@/lib/toc'
import { buildCanonicalCollectionPath } from '@/lib/locale'
import { prefixBasepath } from '@/lib/pathUtils'
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
  collectionSlug: string
  articleSlug: string
  multilingualEnabled: boolean
}

export function ArticleRouteView({
  locale,
  collectionSlug,
  articleSlug,
  multilingualEnabled,
}: ArticleRouteViewProps) {
  const { subdomain, spaces, basepath } = useDocsContext()
  const [mobileNavOpen, setMobileNavOpen] = useState(false)
  const closeMobileNav = useCallback(() => setMobileNavOpen(false), [])
  const matchingSpace = spaces.find((space) => space.slug === collectionSlug)
  const { data: article, isLoading, error } = useArticle(
    subdomain,
    locale,
    collectionSlug,
    articleSlug,
    multilingualEnabled,
  )
  const resolvedSpaceSlug = article?.space_slug ?? ''
  const { data: navigation = [], isLoading: navigationLoading } =
    useSpaceNavigation(
      subdomain,
      locale,
      resolvedSpaceSlug,
      multilingualEnabled,
    )

  useEffect(() => {
    if (article || isLoading || !matchingSpace) {
      return
    }

    window.location.replace(
      prefixBasepath(
        basepath,
        buildCanonicalCollectionPath(multilingualEnabled, locale, articleSlug),
      ),
    )
  }, [article, articleSlug, basepath, isLoading, locale, matchingSpace, multilingualEnabled])

  const tocItems = useMemo(
    () => (article?.content_html ? extractTocFromHtml(article.content_html) : []),
    [article?.content_html],
  )
  const tocIds = useMemo(() => tocItems.map((item) => item.id), [tocItems])
  const activeHeadingId = useScrollSpy(tocIds)
  const pager = useMemo(() => getArticlePager(navigation, articleSlug), [navigation, articleSlug])

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

  if (navigationLoading && resolvedSpaceSlug) {
    return <LoadingState />
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
          spaceSlug={article.space_slug}
          articleSlug={articleSlug}
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
