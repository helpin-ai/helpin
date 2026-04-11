import type { ReactNode } from 'react'
import { Breadcrumbs } from '@/components/navigation/Breadcrumbs'
import { ArticlePager } from '@/components/navigation/ArticlePager'
import { ArticleFeedback } from './ArticleFeedback'
import type { ArticlePagerLink } from '@/lib/navigation'

interface ArticleShellProps {
  locale: string
  title: string
  excerpt?: string | null
  spaceName?: string
  collectionName?: string | null
  collectionSlug?: string | null
  articleSlug: string
  articlePublicId: string
  pager: { prev?: ArticlePagerLink; next?: ArticlePagerLink }
  multilingualEnabled: boolean
  children: ReactNode
}

export function ArticleShell({
  locale,
  title,
  excerpt,
  collectionName,
  collectionSlug,
  articleSlug,
  articlePublicId,
  pager,
  multilingualEnabled,
  children,
}: ArticleShellProps) {
  return (
    <article
      className="mx-auto pt-16 pb-8 px-5 lg:px-6"
      style={{ maxWidth: 'var(--hc-content-max-width)' }}
    >
      {collectionName && (
        <div className="mb-2.5">
          <Breadcrumbs
            locale={locale}
            collectionName={collectionName}
            collectionSlug={collectionSlug}
          />
        </div>
      )}

      <header className="mb-8">
        <h1 className="text-[1.875rem] font-bold leading-tight tracking-tight mb-2">
          {title}
        </h1>
        {excerpt && (
          <p className="text-[15px] leading-relaxed text-muted-foreground">
            {excerpt}
          </p>
        )}
      </header>

      {children}

      <ArticleFeedback
        locale={locale}
        articleSlug={articleSlug}
        articlePublicId={articlePublicId}
        multilingualEnabled={multilingualEnabled}
      />
      <ArticlePager locale={locale} prev={pager.prev} next={pager.next} />
    </article>
  )
}
