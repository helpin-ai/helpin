import { useEffect, useMemo, useState } from 'react'
import { Link } from '@tanstack/react-router'
import DOMPurify from 'dompurify'
import { QuietEmptyState, QuietMetaLine, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet'
import {
  CustomerPortalApiError,
  customerPortalService,
  type CustomerPortalHelpArticle,
  type CustomerPortalHelpArticleDetail,
} from '@/lib/services/customerPortalService'

const SEARCH_DELAY_MS = 300
const SEARCH_MIN_CHARS = 2

type Loaded<T> = { data: T; error: null } | { data: null; error: string }

/**
 * PortalHelpList lists the help articles for signed-in customers by space and
 * collection, and searches them by meaning as the customer types.
 */
export function PortalHelpList({ slug, onSessionError }: { slug: string; onSessionError: (error: unknown) => boolean }) {
  const [list, setList] = useState<Loaded<CustomerPortalHelpArticle[]> | null>(null)
  const [reload, setReload] = useState(0)
  const [query, setQuery] = useState('')
  const [search, setSearch] = useState<{ query: string; result: Loaded<CustomerPortalHelpArticle[]> } | null>(null)
  const trimmed = query.trim()

  useEffect(() => {
    let active = true
    customerPortalService.helpArticles(slug)
      .then((articles) => { if (active) setList({ data: articles, error: null }) })
      .catch((error) => {
        if (active && !onSessionError(error)) setList({ data: null, error: 'Help articles could not be loaded.' })
      })
    return () => { active = false }
  }, [onSessionError, reload, slug])

  useEffect(() => {
    if (trimmed.length < SEARCH_MIN_CHARS) return
    const controller = new AbortController()
    const timer = setTimeout(() => {
      customerPortalService.searchHelpArticles(slug, trimmed, controller.signal)
        .then((articles) => setSearch({ query: trimmed, result: { data: articles, error: null } }))
        .catch((error) => {
          if (controller.signal.aborted || onSessionError(error)) return
          setSearch({ query: trimmed, result: { data: null, error: 'Search failed. Try again.' } })
        })
    }, SEARCH_DELAY_MS)
    return () => {
      clearTimeout(timer)
      controller.abort()
    }
  }, [onSessionError, slug, trimmed])

  const searching = trimmed.length >= SEARCH_MIN_CHARS
  const searchResult = searching && search?.query === trimmed ? search.result : null

  if (!list) return <p role="status" className="mt-6 text-sm text-quiet-text-tertiary">Loading help articles…</p>
  if (list.error !== null) {
    return (
      <QuietEmptyState
        className="mt-8"
        title={list.error}
        description="Check your connection and try again."
        action={<QuietTextAction onClick={() => setReload((current) => current + 1)}>Try again</QuietTextAction>}
      />
    )
  }
  if (list.data.length === 0) {
    return <QuietEmptyState className="mt-8" title="No help articles yet" description="Articles your support team shares with customers will appear here." />
  }

  return (
    <div className="mt-6">
      <QuietUnderlineInput
        type="search"
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        placeholder="Search help articles"
        aria-label="Search help articles"
      />
      {searching ? (
        <section className="mt-6" aria-label="Search results" aria-busy={!searchResult}>
          {!searchResult ? (
            <p role="status" className="text-sm text-quiet-text-tertiary">Searching…</p>
          ) : searchResult.error !== null ? (
            <p role="alert" className="text-sm text-quiet-text-tertiary">{searchResult.error}</p>
          ) : searchResult.data.length === 0 ? (
            <p role="status" className="text-sm text-quiet-text-tertiary">No articles match “{trimmed}”.</p>
          ) : (
            <HelpArticleLinks slug={slug} articles={searchResult.data} />
          )}
        </section>
      ) : (
        <HelpArticleGroups slug={slug} articles={list.data} />
      )}
    </div>
  )
}

/** HelpArticleGroups lists articles under their space and collection. */
function HelpArticleGroups({ slug, articles }: { slug: string; articles: CustomerPortalHelpArticle[] }) {
  const groups = useMemo(() => {
    const bySection = new Map<string, { title: string; articles: CustomerPortalHelpArticle[] }>()
    for (const article of articles) {
      const title = article.collection_name ? `${article.space_name} · ${article.collection_name}` : article.space_name
      const group = bySection.get(title) ?? { title, articles: [] }
      group.articles.push(article)
      bySection.set(title, group)
    }
    return [...bySection.values()]
  }, [articles])
  return (
    <div className="mt-6 space-y-8">
      {groups.map((group) => (
        <section key={group.title} aria-label={group.title}>
          <h2 className="text-[12px] font-semibold uppercase tracking-[0.06em] text-quiet-muted">{group.title}</h2>
          <HelpArticleLinks slug={slug} articles={group.articles} showSpace={false} />
        </section>
      ))}
    </div>
  )
}

function HelpArticleLinks({ slug, articles, showSpace = true }: { slug: string; articles: CustomerPortalHelpArticle[]; showSpace?: boolean }) {
  return (
    <ul className="mt-2 divide-y divide-quiet-divider-light border-y border-quiet-divider-light">
      {articles.map((article) => {
        const summary = article.snippet || article.excerpt
        return (
          <li key={article.id}>
            <Link
              to="/portal/$slug/help/$articleId"
              params={{ slug, articleId: article.id }}
              className="block rounded-sm px-2 py-3 hover:bg-quiet-row-hover focus-visible:outline-2 focus-visible:outline-ring"
            >
              <span className="block text-[14px] font-semibold leading-5 tracking-[-0.008em] text-quiet-text-primary">{article.title}</span>
              {showSpace ? <span className="mt-0.5 block text-[11.5px] text-quiet-muted">{article.space_name}</span> : null}
              {summary ? <span className="mt-1 line-clamp-2 text-[13px] leading-5 text-quiet-text-tertiary">{summary}</span> : null}
            </Link>
          </li>
        )
      })}
    </ul>
  )
}

/**
 * PortalHelpArticleView shows one help article for signed-in customers. The
 * server renders it like a public help center article; it is sanitized again
 * here before it reaches the page.
 */
export function PortalHelpArticleView({ slug, articleId, onSessionError, onTitle }: {
  slug: string
  articleId: string
  onSessionError: (error: unknown) => boolean
  onTitle?: (title: string) => void
}) {
  const [state, setState] = useState<{ id: string; result: Loaded<CustomerPortalHelpArticleDetail> } | null>(null)
  const [reload, setReload] = useState(0)

  useEffect(() => {
    let active = true
    customerPortalService.helpArticle(slug, articleId)
      .then((article) => {
        if (!active) return
        setState({ id: articleId, result: { data: article, error: null } })
        onTitle?.(article.title)
      })
      .catch((error) => {
        if (!active || onSessionError(error)) return
        const missing = error instanceof CustomerPortalApiError && error.status === 404
        setState({ id: articleId, result: { data: null, error: missing ? 'This article is not available.' : 'The article could not be loaded.' } })
      })
    return () => { active = false }
  }, [articleId, onSessionError, onTitle, reload, slug])

  const current = state?.id === articleId ? state.result : null
  const html = useMemo(() => (current?.data ? DOMPurify.sanitize(current.data.content_html) : ''), [current])

  if (!current) return <p role="status" className="mt-6 text-sm text-quiet-text-tertiary">Loading article…</p>
  if (current.error !== null) {
    return (
      <QuietEmptyState
        className="mt-8"
        title={current.error}
        description="It may have been moved or unpublished."
        action={<QuietTextAction onClick={() => setReload((value) => value + 1)}>Try again</QuietTextAction>}
      />
    )
  }
  const article = current.data
  return (
    <article className="mt-2">
      <QuietMetaLine items={[article.space_name, article.collection_name]} />
      <div className="prose prose-sm mt-6 max-w-none dark:prose-invert" dangerouslySetInnerHTML={{ __html: html }} />
    </article>
  )
}
