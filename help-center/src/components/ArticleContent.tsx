interface ArticleContentProps {
  html?: string
}

export function ArticleContent({ html }: ArticleContentProps) {
  if (!html) {
    return (
      <div
        className="rounded-lg border p-8 text-center text-sm"
        style={{
          borderColor: 'var(--hc-border)',
          color: 'var(--hc-text-muted)',
        }}
      >
        No content available for this article.
      </div>
    )
  }

  return (
    <div
      className="hc-prose"
      dangerouslySetInnerHTML={{ __html: html }}
    />
  )
}
