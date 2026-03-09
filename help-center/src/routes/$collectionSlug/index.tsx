import { createFileRoute, Link } from '@tanstack/react-router'
import { useCollection } from '@/hooks/queries'
import { FileText } from 'lucide-react'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'

export const Route = createFileRoute('/$collectionSlug/')({
  component: CollectionPage,
})

function CollectionPage() {
  const { collectionSlug } = Route.useParams()
  const subdomain = Route.useRouteContext({ select: (s) => s.subdomain })
  const { data, isLoading, error } = useCollection(subdomain, collectionSlug)

  if (isLoading) return <LoadingState />
  if (error || !data) {
    return (
      <ErrorState
        title="Collection not found"
        message="This collection does not exist or has no published articles."
        statusCode={404}
      />
    )
  }

  const { collection, articles } = data

  return (
    <div
      className="mx-auto py-10 px-6"
      style={{ maxWidth: 'var(--hc-content-max-width)' }}
    >
      <header className="mb-8">
        <div className="flex items-center gap-2 mb-2">
          {collection.icon && (
            <span className="text-xl">{collection.icon}</span>
          )}
          <h1 className="text-2xl font-bold">{collection.name}</h1>
        </div>
        {collection.description && (
          <p
            className="text-sm"
            style={{ color: 'var(--hc-text-secondary)' }}
          >
            {collection.description}
          </p>
        )}
      </header>

      <div className="space-y-1">
        {articles.map((article) => (
          <Link
            key={article.id}
            to="/articles/$articleSlug"
            params={{ articleSlug: article.slug }}
            className="flex items-center gap-3 rounded-lg px-3 py-3 transition-colors hover:bg-[var(--hc-bg-secondary)] group"
          >
            <FileText
              size={16}
              className="shrink-0"
              style={{ color: 'var(--hc-text-muted)' }}
            />
            <div className="min-w-0">
              <p className="text-sm font-medium group-hover:text-[var(--hc-accent)] transition-colors">
                {article.title}
              </p>
              {article.excerpt && (
                <p
                  className="text-xs truncate mt-0.5"
                  style={{ color: 'var(--hc-text-muted)' }}
                >
                  {article.excerpt}
                </p>
              )}
            </div>
          </Link>
        ))}
        {articles.length === 0 && (
          <p
            className="text-sm py-8 text-center"
            style={{ color: 'var(--hc-text-muted)' }}
          >
            No published articles in this collection.
          </p>
        )}
      </div>
    </div>
  )
}
