import { useId } from 'react'
import { Link } from '@tanstack/react-router'
import type { CustomerPortalArticleSuggestion } from '@/lib/services/customerPortalService'
import { cn } from '@/lib/utils'

/**
 * PortalArticleSuggestions lists help center articles that may answer the
 * request being written. Articles open in a new tab so the draft stays put:
 * public ones on the help center, ones for signed-in customers in the portal.
 * The live region announces changes; nothing renders while there are none.
 */
export function PortalArticleSuggestions({ slug, articles, onOpen, className }: {
  slug: string
  articles: CustomerPortalArticleSuggestion[]
  onOpen?: (article: CustomerPortalArticleSuggestion) => void
  className?: string
}) {
  const headingId = useId()
  const count = articles.length
  return (
    <aside className={cn('min-w-0', className)} aria-labelledby={count ? headingId : undefined}>
      <p className="sr-only" aria-live="polite">
        {count === 0 ? '' : count === 1 ? '1 related article' : `${count} related articles`}
      </p>
      {count > 0 ? (
        <>
          <h2 id={headingId} className="text-[12px] font-semibold uppercase tracking-[0.06em] text-quiet-muted">Related articles</h2>
          <p className="mt-1 text-[12.5px] leading-5 text-quiet-text-tertiary">These might answer your question. They open in a new tab.</p>
          <ul className="mt-3 divide-y divide-quiet-divider-light border-y border-quiet-divider-light">
            {articles.map((article) => (
              <li key={article.id}>
                {article.portal_article_id ? (
                  <Link
                    to="/portal/$slug/help/$articleId"
                    params={{ slug, articleId: article.portal_article_id }}
                    target="_blank"
                    onClick={() => onOpen?.(article)}
                    className={linkClassName}
                  >
                    <SuggestionText article={article} />
                  </Link>
                ) : (
                  <a href={article.url} target="_blank" rel="noopener noreferrer" onClick={() => onOpen?.(article)} className={linkClassName}>
                    <SuggestionText article={article} />
                  </a>
                )}
              </li>
            ))}
          </ul>
        </>
      ) : null}
    </aside>
  )
}

const linkClassName = 'block rounded-sm py-3 hover:bg-quiet-row-hover focus-visible:outline-2 focus-visible:outline-ring'

function SuggestionText({ article }: { article: CustomerPortalArticleSuggestion }) {
  return (
    <>
      <span className="block text-[13.5px] font-semibold leading-5 tracking-[-0.008em] text-quiet-text-primary">{article.title}</span>
      {article.space_name ? <span className="mt-0.5 block text-[11.5px] text-quiet-muted">{article.space_name}</span> : null}
      {article.snippet ? <span className="mt-1 line-clamp-2 text-[12.5px] leading-5 text-quiet-text-tertiary">{article.snippet}</span> : null}
    </>
  )
}
