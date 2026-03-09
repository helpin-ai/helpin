import { createFileRoute, Link } from '@tanstack/react-router'
import { useNavigation } from '@/hooks/queries'
import { FileText, ArrowRight } from 'lucide-react'

export const Route = createFileRoute('/')({
  component: HomePage,
})

function HomePage() {
  const subdomain = Route.useRouteContext({ select: (s) => s.subdomain })
  const { data: navigation } = useNavigation(subdomain)

  return (
    <div
      className="mx-auto py-10 px-6"
      style={{ maxWidth: 'var(--hc-content-max-width)' }}
    >
      {/* Hero */}
      <div className="mb-10">
        <h1 className="text-3xl font-bold mb-3">Documentation</h1>
        <p
          className="text-base"
          style={{ color: 'var(--hc-text-secondary)' }}
        >
          Browse our documentation to learn how to get started, explore features,
          and find answers.
        </p>
      </div>

      {/* Collection cards */}
      <div className="grid gap-4 sm:grid-cols-2">
        {navigation?.map((collection) => (
          <Link
            key={collection.id}
            to="/$collectionSlug"
            params={{ collectionSlug: collection.slug }}
            className="group rounded-xl border p-5 transition-all hover:border-[var(--hc-accent)] hover:shadow-sm"
            style={{ borderColor: 'var(--hc-border)' }}
          >
            <div className="flex items-start justify-between mb-3">
              <div className="flex items-center gap-2">
                {collection.icon && (
                  <span className="text-lg">{collection.icon}</span>
                )}
                <h2 className="font-semibold text-sm">{collection.name}</h2>
              </div>
              <ArrowRight
                size={16}
                className="opacity-0 group-hover:opacity-100 transition-opacity"
                style={{ color: 'var(--hc-accent)' }}
              />
            </div>
            <div className="space-y-1">
              {collection.articles.slice(0, 3).map((article) => (
                <div
                  key={article.id}
                  className="flex items-center gap-2 text-sm"
                  style={{ color: 'var(--hc-text-secondary)' }}
                >
                  <FileText size={13} className="shrink-0" />
                  <span className="truncate">{article.title}</span>
                </div>
              ))}
              {collection.articles.length > 3 && (
                <p
                  className="text-xs mt-1"
                  style={{ color: 'var(--hc-text-muted)' }}
                >
                  +{collection.articles.length - 3} more articles
                </p>
              )}
            </div>
          </Link>
        ))}
      </div>
    </div>
  )
}
