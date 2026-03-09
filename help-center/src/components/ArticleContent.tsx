import type { ArticleDetail } from '@/lib/types'

interface ArticleContentProps {
  article: ArticleDetail
}

export function ArticleContent({ article }: ArticleContentProps) {
  return (
    <article
      className="mx-auto py-8 px-6"
      style={{ maxWidth: 'var(--hc-content-max-width)' }}
    >
      {/* Header */}
      <header className="mb-8">
        {article.collection_name && (
          <p
            className="text-xs font-medium uppercase tracking-wider mb-2"
            style={{ color: 'var(--hc-accent)' }}
          >
            {article.collection_name}
          </p>
        )}
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

      {/* Content — renders HTML from backend or placeholder */}
      {article.content_html ? (
        <div
          className="prose prose-sm max-w-none
            prose-headings:font-semibold prose-headings:tracking-tight
            prose-h2:text-xl prose-h2:mt-10 prose-h2:mb-4
            prose-h3:text-lg prose-h3:mt-8 prose-h3:mb-3
            prose-p:leading-relaxed prose-p:text-[var(--hc-text-secondary)]
            prose-a:text-[var(--hc-accent)] prose-a:no-underline hover:prose-a:underline
            prose-code:text-sm prose-code:bg-[var(--hc-bg-tertiary)] prose-code:px-1.5 prose-code:py-0.5 prose-code:rounded
            prose-pre:bg-[var(--hc-bg-secondary)] prose-pre:border prose-pre:border-[var(--hc-border)] prose-pre:rounded-lg
            prose-img:rounded-lg
            prose-li:text-[var(--hc-text-secondary)]"
          dangerouslySetInnerHTML={{ __html: article.content_html }}
        />
      ) : (
        <div
          className="rounded-lg border p-8 text-center text-sm"
          style={{
            borderColor: 'var(--hc-border)',
            color: 'var(--hc-text-muted)',
          }}
        >
          No content available for this article.
        </div>
      )}

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
    </article>
  )
}
