import { createFileRoute, Link } from '@tanstack/react-router'
import { ArrowRight } from 'lucide-react'
import { useCollection } from '@/hooks/queries'
import { useDocsContext } from '@/contexts/DocsContext'
import { useSpaceContext } from '@/contexts/SpaceContext'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'

export const Route = createFileRoute('/$locale/$spaceSlug/$collectionSlug/')({
  component: CollectionPage,
})

function CollectionPage() {
  const { locale, spaceSlug, collectionSlug } = Route.useParams()
  const { subdomain } = useDocsContext()
  const { space } = useSpaceContext()
  const { data, isLoading, error } = useCollection(
    subdomain,
    locale,
    spaceSlug,
    collectionSlug,
  )

  const collection = data?.collection
  const articles = data?.articles ?? []

  useDocumentTitle(collection?.name)

  if (isLoading) return <LoadingState message="Loading collection..." />
  if (error || !collection) {
    return (
      <ErrorState
        title="Collection not found"
        message="This collection does not exist or has no published articles."
        statusCode={404}
      />
    )
  }

  return (
    <section
      className="mx-auto px-5 pb-10 pt-16 lg:px-6"
      style={{ maxWidth: 'var(--hc-content-max-width)' }}
    >
      <header className="border-b border-border/70 pb-6">
        <div className="text-[12px] font-medium uppercase tracking-[0.2em] text-muted-foreground/70">
          {space?.name}
        </div>
        <h1 className="mt-3 text-[2rem] font-bold tracking-tight text-foreground">
          {collection.name}
        </h1>
      </header>

      {articles.length === 0 ? (
        <div className="py-10 text-sm text-muted-foreground">
          No articles are published in this collection yet.
        </div>
      ) : (
        <div className="mt-8 space-y-3">
          {articles.map((article) => (
            <Link
              key={article.id}
              to="/$locale/$spaceSlug/$collectionSlug/$articleSlug"
              params={{
                locale,
                spaceSlug,
                collectionSlug: collection.slug,
                articleSlug: article.slug,
              }}
              className="group flex items-center justify-between rounded-2xl border border-border/70 px-4 py-4 transition-colors hover:border-primary/30 hover:bg-primary/[0.02]"
            >
              <div className="min-w-0">
                <div className="text-sm font-medium text-foreground">
                  {article.title}
                </div>
              </div>
              <ArrowRight
                size={16}
                className="shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5"
              />
            </Link>
          ))}
        </div>
      )}
    </section>
  )
}
