import type { ReactNode } from 'react'
import { Breadcrumbs } from '@/components/navigation/Breadcrumbs'
import { ArticlePager } from '@/components/navigation/ArticlePager'
import { ArticleFeedback } from './ArticleFeedback'
import type { ArticlePagerLink } from '@/lib/navigation'

interface ArticleShellProps {
  title: string
  excerpt?: string | null
  spaceSlug: string
  spaceName?: string
  collectionName?: string | null
  articleSlug: string
  pager: { prev?: ArticlePagerLink; next?: ArticlePagerLink }
  children: ReactNode
}

export function ArticleShell({
  title,
  excerpt,
  spaceSlug,
  spaceName: _spaceName,
  collectionName,
  articleSlug,
  pager,
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
            spaceSlug={spaceSlug}
            collectionName={collectionName}
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

      <ArticleFeedback spaceSlug={spaceSlug} articleSlug={articleSlug} />
      <ArticlePager spaceSlug={spaceSlug} prev={pager.prev} next={pager.next} />
    </article>
  )
}
