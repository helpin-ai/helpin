import { useCallback, useEffect, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { ArrowRight, Menu } from 'lucide-react'
import { useCollection, useSpaceNavigation } from '@/hooks/queries'
import { useDocsContext } from '@/contexts/DocsContext'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { buildCanonicalArticlePath, buildCanonicalCollectionPath } from '@/lib/locale'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'
import { Sidebar } from '@/components/layout/Sidebar'
import { MobileNav } from '@/components/navigation/MobileNav'

interface CollectionRouteViewProps {
  locale: string
  collectionOrSpaceSlug: string
  multilingualEnabled: boolean
}

export function CollectionRouteView({
  locale,
  collectionOrSpaceSlug,
  multilingualEnabled,
}: CollectionRouteViewProps) {
  const { subdomain, spaces } = useDocsContext()
  const [mobileNavOpen, setMobileNavOpen] = useState(false)
  const closeMobileNav = useCallback(() => setMobileNavOpen(false), [])
  const matchingSpace = spaces.find((space) => space.slug === collectionOrSpaceSlug)
  const { data: spaceNavigation = [], isLoading: spaceNavigationLoading } =
    useSpaceNavigation(
      subdomain,
      locale,
      matchingSpace?.slug ?? '',
      multilingualEnabled,
    )
  const { data: collectionData, isLoading: collectionLoading, error: collectionError } =
    useCollection(subdomain, locale, collectionOrSpaceSlug, multilingualEnabled)
  const collection = collectionData?.collection
  const articles = collectionData?.articles ?? []
  const collectionSpaceSlug = collectionData?.space_slug ?? ''
  const { data: collectionNavigation = [], isLoading: collectionNavigationLoading } =
    useSpaceNavigation(
      subdomain,
      locale,
      collectionSpaceSlug,
      multilingualEnabled,
    )
  const activeNavigation = collection ? collectionNavigation : spaceNavigation
  const activeHeading = collection?.name ?? matchingSpace?.name ?? ''

  useDocumentTitle(collection?.name ?? matchingSpace?.name)

  useEffect(() => {
    if (collection || collectionLoading || !matchingSpace || spaceNavigation.length === 0) {
      return
    }

    const firstCollection = spaceNavigation[0]
    if (!firstCollection) {
      return
    }

    window.location.replace(
      buildCanonicalCollectionPath(multilingualEnabled, locale, firstCollection.slug),
    )
  }, [
    collection,
    collectionLoading,
    locale,
    matchingSpace,
    multilingualEnabled,
    spaceNavigation,
  ])

  if (!collection && matchingSpace) {
    return (
      <div className="flex">
        {mobileNavOpen && spaceNavigation.length > 0 && (
          <MobileNav
            locale={locale}
            navigation={spaceNavigation}
            onClose={closeMobileNav}
          />
        )}

        {spaceNavigation.length > 0 && (
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
                {activeHeading}
              </span>
            </div>
            <Sidebar locale={locale} navigation={spaceNavigation} />
          </>
        )}

        <main className="min-w-0 flex-1 pt-[41px] lg:pt-0">
          <section
            className="mx-auto px-5 pb-10 pt-16 lg:px-6"
            style={{ maxWidth: 'var(--hc-content-max-width)' }}
          >
            {spaceNavigationLoading ? (
              <LoadingState message="Loading collection..." />
            ) : spaceNavigation.length === 0 ? (
              <ErrorState
                title="No articles yet"
                message="This space has no published articles."
              />
            ) : (
              <LoadingState message="Opening collection..." />
            )}
          </section>
        </main>
      </div>
    )
  }

  if (collectionLoading) return <LoadingState message="Loading collection..." />
  if (collectionError || !collection) {
    return (
      <ErrorState
        title="Collection not found"
        message="This collection does not exist or has no published articles."
        statusCode={404}
      />
    )
  }

  if (collectionNavigationLoading && collectionSpaceSlug) {
    return <LoadingState message="Loading collection..." />
  }

  return (
    <div className="flex">
      {mobileNavOpen && activeNavigation.length > 0 && (
        <MobileNav
          locale={locale}
          navigation={activeNavigation}
          onClose={closeMobileNav}
        />
      )}

      {activeNavigation.length > 0 && (
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
              {activeHeading}
            </span>
          </div>
          <Sidebar locale={locale} navigation={activeNavigation} />
        </>
      )}

      <main className="min-w-0 flex-1 pt-[41px] lg:pt-0">
        <section
          className="mx-auto px-5 pb-10 pt-16 lg:px-6"
          style={{ maxWidth: 'var(--hc-content-max-width)' }}
        >
          <header className="border-b border-border/70 pb-6">
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
                  to={buildCanonicalArticlePath(
                    multilingualEnabled,
                    locale,
                    collection.slug,
                    article.slug,
                  )}
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
      </main>
    </div>
  )
}
