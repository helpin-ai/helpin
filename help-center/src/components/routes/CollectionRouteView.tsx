import { useCallback, useMemo, useState } from 'react'
import { ArrowRight, Braces, Folder, Menu } from 'lucide-react'
import { DocsLink } from '@/components/DocsLink'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { useAPIReferences, useCollection, useSpaceNavigation } from '@/hooks/queries'
import { useDocsContext } from '@/contexts/DocsContext'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import {
  buildCanonicalAPIReferencePath,
  buildCanonicalArticlePath,
  buildCanonicalCollectionPath,
} from '@/lib/locale'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'
import { Sidebar, SidebarSkeleton } from '@/components/layout/Sidebar'
import { Footer } from '@/components/layout/Footer'
import { MobileNav } from '@/components/navigation/MobileNav'
import { Breadcrumbs, type BreadcrumbEntry } from '@/components/navigation/Breadcrumbs'
import {
  buildNavTree,
  navAncestorChain,
} from '@/lib/navigation'
import type { NavTreeNode } from '@/lib/types'
import { cn } from '@/lib/utils'

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
  const { data: apiReferences = [], isLoading: apiReferencesLoading } =
    useAPIReferences(
      subdomain,
      locale,
      matchingSpace?.slug ?? '',
      multilingualEnabled,
    )
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
  const { data: collectionNavigation = [] } =
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

        {matchingSpace && (
          <>
            <div className="fixed left-0 right-0 top-[var(--hc-header-height)] z-20 flex items-center gap-2 border-b border-border bg-background px-4 py-2 lg:hidden">
              <button
                onClick={() => setMobileNavOpen(true)}
                disabled={spaceNavigation.length === 0}
                className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
                aria-label="Open navigation"
              >
                <Menu size={18} />
              </button>
              <span className="truncate text-[13px] font-medium">
                {activeHeading}
              </span>
            </div>
            {spaceNavigation.length > 0 || apiReferences.length > 0 ? (
              <Sidebar
                locale={locale}
                navigation={spaceNavigation}
                spaceSlug={matchingSpace.slug}
              />
            ) : spaceNavigationLoading ? (
              <SidebarSkeleton />
            ) : null}
          </>
        )}

        <main className="min-w-0 flex-1 pt-[41px] lg:pl-8 lg:pt-0">
          <section
            className="mx-auto px-5 pb-10 pt-16 lg:px-8"
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

            {spaceNavigationLoading || apiReferencesLoading ? (
              <LoadingState message="Loading space..." />
            ) : topLevelNodes.length === 0 && apiReferences.length === 0 ? (
              <ErrorState
                title="No articles yet"
                message="This space has no published articles."
              />
            ) : (
              <>
                {topLevelNodes.length > 0 && (
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
                {apiReferences.length > 0 && (
                  <div className="mt-10">
                    <h2 className="mb-3 text-[13px] font-semibold uppercase tracking-wide text-muted-foreground">
                      API Reference
                    </h2>
                    <div className="grid gap-3 sm:grid-cols-2">
                      {apiReferences.map((reference) => (
                        <DocsLink
                          key={reference.id}
                          to={buildCanonicalAPIReferencePath(
                            multilingualEnabled,
                            locale,
                            matchingSpace.slug,
                            reference.slug,
                          )}
                          className="group flex items-center gap-3 rounded-2xl border border-border/70 px-4 py-4 transition-colors hover:border-primary/30 hover:bg-primary/[0.02]"
                        >
                          <div className="rounded-xl bg-primary/10 p-2 text-primary">
                            <Braces size={18} />
                          </div>
                          <div className="min-w-0 flex-1">
                            <div className="truncate text-sm font-medium text-foreground">
                              {reference.name}
                            </div>
                            <div className="mt-0.5 text-xs text-muted-foreground">
                              {reference.api_version ? `Version ${reference.api_version} · ` : ''}
                              {reference.operation_count} operations
                            </div>
                          </div>
                          <ArrowRight
                            size={16}
                            className="shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5"
                          />
                        </DocsLink>
                      ))}
                    </div>
                  </div>
                )}
              </>
            )}
            <Footer />
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

      {collectionSpaceSlug && (
        <>
          <div className="fixed left-0 right-0 top-[var(--hc-header-height)] z-20 flex items-center gap-2 border-b border-border bg-background px-4 py-2 lg:hidden">
            <button
              disabled={activeNavigation.length === 0}
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
          {activeNavigation.length > 0 ? (
            <Sidebar
              locale={locale}
              navigation={activeNavigation}
              spaceSlug={collectionSpaceSlug}
            />
          ) : (
            <SidebarSkeleton />
          )}
        </>
      )}

      <main className="min-w-0 flex-1 pt-[41px] lg:pl-8 lg:pt-0">
        <section
          className="mx-auto px-5 pb-10 pt-16 lg:px-8"
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

          {/* Direct articles stay as the simplest view: just article rows.
              When mixed with sub-collections they come first, so the
              current collection's own content is immediately visible. */}
          {hasArticles && (
            <div className={hasChildren ? 'mt-8 space-y-3' : 'mt-8 space-y-3'}>
              {hasChildren && (
                <h2 className="text-[13px] font-semibold uppercase tracking-wide text-muted-foreground">
                  Articles In This Section
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

          {/* Nested collections are rendered as accordion rows instead of
              cards so visitors can preview the structure inline without
              drilling into each child page one by one. */}
          {hasChildren && (
            <div className={hasArticles ? 'mt-10 space-y-4' : 'mt-8 space-y-4'}>
              {hasArticles && (
                <h2 className="text-[13px] font-semibold uppercase tracking-wide text-muted-foreground">
                  Sub-collections
                </h2>
              )}
              <CollectionAccordionList
                nodes={childNodes}
                locale={locale}
                multilingualEnabled={multilingualEnabled}
              />
            </div>
          )}

          {!hasChildren && !hasArticles && (
            <div className="py-10 text-sm text-muted-foreground">
              No articles are published in this collection yet.
            </div>
          )}
          <Footer />
        </section>
      </main>
    </div>
  )
}

function CollectionAccordionList({
  nodes,
  locale,
  multilingualEnabled,
  level = 0,
}: {
  nodes: NavTreeNode[]
  locale: string
  multilingualEnabled: boolean
  level?: number
}) {
  if (nodes.length === 0) return null

  return (
    <Accordion
      type="single"
      collapsible
      className={cn(level === 0 ? 'rounded-2xl border border-border/70 bg-background' : 'mt-3')}
    >
      {nodes.map((node) => (
        <CollectionAccordionItem
          key={node.item.id}
          node={node}
          locale={locale}
          multilingualEnabled={multilingualEnabled}
          level={level}
        />
      ))}
    </Accordion>
  )
}

function CollectionAccordionItem({
  node,
  locale,
  multilingualEnabled,
  level,
}: {
  node: NavTreeNode
  locale: string
  multilingualEnabled: boolean
  level: number
}) {
  const directArticles = node.item.articles
  const childNodes = node.children
  const hasArticles = directArticles.length > 0
  const hasChildren = childNodes.length > 0
  const hasExpandableContent = hasArticles || hasChildren
  const totalArticles = countDescendantArticles(node)
  const summary = buildCollectionCardSummary(
    directArticles.length,
    childNodes.length,
    totalArticles,
  )

  if (!hasExpandableContent) {
    return (
      <DocsLink
        to={buildCanonicalCollectionPath(
          multilingualEnabled,
          locale,
          node.item.slug,
          node.item.public_id,
        )}
        className={cn(
          'group flex items-center justify-between rounded-2xl border border-border/70 px-4 py-4 transition-colors hover:border-primary/30 hover:bg-primary/[0.02]',
          level > 0 && 'rounded-xl',
        )}
      >
        <div className="min-w-0">
          <div className="text-sm font-medium text-foreground">{node.item.name}</div>
          {summary && (
            <div className="mt-1 text-[12px] text-muted-foreground">{summary}</div>
          )}
        </div>
        <ArrowRight
          size={16}
          className="shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5"
        />
      </DocsLink>
    )
  }

  return (
    <AccordionItem
      value={node.item.id}
      className={cn(
        'border-b border-border/70 px-2 last:border-b-0',
        level > 0 && 'rounded-xl border border-border/70 px-0 last:border-b',
      )}
    >
      <AccordionTrigger
        className={cn(
          'rounded-xl px-3 py-4 hover:bg-primary/[0.02] hover:no-underline',
          level > 0 && 'px-4',
        )}
      >
        <div className="flex min-w-0 flex-1 items-center justify-between gap-4 text-left">
          <div className="min-w-0">
            <div className="truncate text-sm font-medium text-foreground">
              {node.item.name}
            </div>
            {summary && (
              <div className="mt-1 text-[12px] text-muted-foreground">{summary}</div>
            )}
          </div>
        </div>
      </AccordionTrigger>

      <AccordionContent className={cn('px-3 pb-4', level > 0 && 'px-4')}>
        <div className="border-l border-border/70 pl-4">
          {hasArticles ? (
            <div className="space-y-2">
              {directArticles.map((article) => (
                <DocsLink
                  key={article.id}
                  to={buildCanonicalArticlePath(
                    multilingualEnabled,
                    locale,
                    article.slug,
                    article.public_id,
                  )}
                  className="group flex items-center justify-between rounded-xl px-3 py-3 transition-colors hover:bg-primary/[0.02]"
                >
                  <div className="min-w-0">
                    <div className="text-sm text-foreground">{article.title}</div>
                  </div>
                  <ArrowRight
                    size={16}
                    className="shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5"
                  />
                </DocsLink>
              ))}
            </div>
          ) : null}

          {hasChildren ? (
            <CollectionAccordionList
              nodes={childNodes}
              locale={locale}
              multilingualEnabled={multilingualEnabled}
              level={level + 1}
            />
          ) : null}
        </div>
      </AccordionContent>
    </AccordionItem>
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
  const totalArticles = countDescendantArticles(node)
  const description = buildCollectionCardSummary(articleCount, childCount, totalArticles)

  return (
    <DocsLink
      to={buildCanonicalCollectionPath(
        multilingualEnabled,
        locale,
        node.item.slug,
        node.item.public_id,
      )}
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

function countDescendantArticles(node: NavTreeNode): number {
  let total = node.item.articles.length
  for (const child of node.children) {
    total += countDescendantArticles(child)
  }
  return total
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
