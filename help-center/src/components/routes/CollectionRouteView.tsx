import { useCallback, useMemo, useState } from 'react'
import { ArrowRight, Folder, Menu } from 'lucide-react'
import { DocsLink } from '@/components/DocsLink'
import { useCollection, useSpaceNavigation } from '@/hooks/queries'
import { useDocsContext } from '@/contexts/DocsContext'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { buildCanonicalArticlePath, buildCanonicalCollectionPath } from '@/lib/locale'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'
import { Sidebar } from '@/components/layout/Sidebar'
import { MobileNav } from '@/components/navigation/MobileNav'
import { Breadcrumbs, type BreadcrumbEntry } from '@/components/navigation/Breadcrumbs'
import {
  buildNavTree,
  navAncestorChain,
} from '@/lib/navigation'
import type { NavTreeNode } from '@/lib/types'

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
  const directArticles = collectionData?.articles ?? []
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

  // Fold the flat navigation into a tree so we can find direct children
  // of the active collection and render them as navigable subgroups.
  const navTree = useMemo<NavTreeNode[]>(
    () => buildNavTree(activeNavigation),
    [activeNavigation],
  )

  // Locate the active collection's node in the tree so we can surface
  // its direct children and its ancestor breadcrumb chain. Falls back to
  // null when the nav request is still in flight or the collection is
  // not yet present in the flat list.
  const activeNode = useMemo<NavTreeNode | null>(() => {
    if (!collection) return null
    const walk = (nodes: NavTreeNode[]): NavTreeNode | null => {
      for (const node of nodes) {
        if (node.item.id === collection.id) return node
        const hit = walk(node.children)
        if (hit) return hit
      }
      return null
    }
    return walk(navTree)
  }, [collection, navTree])

  // Breadcrumb chain: root -> ... -> parent -> current. Rendered only
  // when the active collection has at least one ancestor; top-level
  // collections get no breadcrumb so the page header is clean.
  const breadcrumbEntries = useMemo<BreadcrumbEntry[]>(() => {
    if (!collection) return []
    const ancestors = navAncestorChain(navTree, collection.id)
    const entries: BreadcrumbEntry[] = ancestors.map((node) => ({
      id: node.item.id,
      name: node.item.name,
      slug: node.item.slug,
    }))
    entries.push({ id: collection.id, name: collection.name, slug: null })
    return entries
  }, [collection, navTree])

  useDocumentTitle(collection?.name ?? matchingSpace?.name)

  // Space-level page: when the URL resolves to a space rather than a
  // collection, render the space's top-level collections as a landing
  // grid instead of silently redirecting to the first one. Users can
  // drill in from here and the URL stays stable.
  if (!collection && matchingSpace) {
    const topLevelNodes = buildNavTree(spaceNavigation)
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
            <header className="border-b border-border/70 pb-6">
              <h1 className="mt-3 text-[2rem] font-bold tracking-tight text-foreground">
                {matchingSpace.name}
              </h1>
              {matchingSpace.description && (
                <p className="mt-2 text-[15px] text-muted-foreground">
                  {matchingSpace.description}
                </p>
              )}
            </header>

            {spaceNavigationLoading ? (
              <LoadingState message="Loading space..." />
            ) : topLevelNodes.length === 0 ? (
              <ErrorState
                title="No articles yet"
                message="This space has no published articles."
              />
            ) : (
              <div className="mt-8 grid gap-3 sm:grid-cols-2">
                {topLevelNodes.map((node) => (
                  <CollectionCard
                    key={node.item.id}
                    node={node}
                    locale={locale}
                    multilingualEnabled={multilingualEnabled}
                  />
                ))}
              </div>
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

  const childNodes = activeNode?.children ?? []
  const hasChildren = childNodes.length > 0
  const hasArticles = directArticles.length > 0

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
            {breadcrumbEntries.length > 1 && (
              <div className="mb-3">
                <Breadcrumbs locale={locale} ancestors={breadcrumbEntries} />
              </div>
            )}
            <h1 className="mt-3 text-[2rem] font-bold tracking-tight text-foreground">
              {collection.name}
            </h1>
          </header>

          {/* Child collections first — they act as further drilldowns
              before the direct article list. */}
          {hasChildren && (
            <div className="mt-8 grid gap-3 sm:grid-cols-2">
              {childNodes.map((child) => (
                <CollectionCard
                  key={child.item.id}
                  node={child}
                  locale={locale}
                  multilingualEnabled={multilingualEnabled}
                />
              ))}
            </div>
          )}

          {/* Then direct articles of this collection. */}
          {hasArticles && (
            <div className={hasChildren ? 'mt-10 space-y-3' : 'mt-8 space-y-3'}>
              {hasChildren && (
                <h2 className="text-[13px] font-semibold uppercase tracking-wide text-muted-foreground">
                  Articles
                </h2>
              )}
              {directArticles.map((article) => (
                <DocsLink
                  key={article.id}
                  to={buildCanonicalArticlePath(
                    multilingualEnabled,
                    locale,
                    article.slug,
                    article.public_id,
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
                </DocsLink>
              ))}
            </div>
          )}

          {!hasChildren && !hasArticles && (
            <div className="py-10 text-sm text-muted-foreground">
              No articles are published in this collection yet.
            </div>
          )}
        </section>
      </main>
    </div>
  )
}

// CollectionCard renders a single navigable collection tile showing
// the collection name and a quick count of direct articles + child
// collections. It links to the collection's canonical URL regardless
// of whether the card lives on a space page or on a parent collection.
function CollectionCard({
  node,
  locale,
  multilingualEnabled,
}: {
  node: NavTreeNode
  locale: string
  multilingualEnabled: boolean
}) {
  const articleCount = node.item.articles.length
  const childCount = node.children.length
  const flattenDescendantArticles = (nodes: NavTreeNode[]): number => {
    let total = 0
    for (const n of nodes) {
      total += n.item.articles.length
      total += flattenDescendantArticles(n.children)
    }
    return total
  }
  const totalArticles = articleCount + flattenDescendantArticles(node.children)
  const description = buildCollectionCardSummary(articleCount, childCount, totalArticles)

  return (
    <DocsLink
      to={buildCanonicalCollectionPath(multilingualEnabled, locale, node.item.slug)}
      className="group flex items-start gap-3 rounded-2xl border border-border/70 px-4 py-4 transition-colors hover:border-primary/30 hover:bg-primary/[0.02]"
    >
      <Folder size={18} className="mt-0.5 shrink-0 text-muted-foreground" />
      <div className="min-w-0 flex-1">
        <div className="text-sm font-semibold text-foreground">{node.item.name}</div>
        {description && (
          <div className="mt-1 text-[12px] text-muted-foreground">{description}</div>
        )}
      </div>
      <ArrowRight
        size={16}
        className="mt-0.5 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5"
      />
    </DocsLink>
  )
}

/**
 * Builds the small summary line under a CollectionCard heading.
 * "3 articles · 2 sub-collections" when both are present; a bare
 * article count or sub-collection count when only one is non-zero.
 */
function buildCollectionCardSummary(directArticles: number, childCount: number, totalArticles: number): string {
  const parts: string[] = []
  if (totalArticles > 0) {
    parts.push(`${totalArticles} ${totalArticles === 1 ? 'article' : 'articles'}`)
  } else if (directArticles === 0) {
    parts.push('No articles yet')
  }
  if (childCount > 0) {
    parts.push(`${childCount} ${childCount === 1 ? 'sub-collection' : 'sub-collections'}`)
  }
  return parts.join(' · ')
}
