import { useRouterState } from '@tanstack/react-router'
import { DocsLink } from '@/components/DocsLink'
import { PublicIcon } from '@/components/PublicIcon'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { useDocsContext } from '@/contexts/DocsContext'
import { buildNavTree } from '@/lib/navigation'
import {
  buildCanonicalArticlePath,
  buildCanonicalCollectionPath,
  isMultilingualEnabled,
} from '@/lib/locale'
import type { NavArticle, NavItem, NavTreeNode } from '@/lib/types'
import { cn } from '@/lib/utils'

// Merge direct articles and sub-collections of a node into a single
// position-sorted list so the rendered order matches the author's
// intent (and matches the import order from Nextra _meta files).
type MergedChild =
  | { kind: 'article'; position: number; article: NavArticle }
  | { kind: 'collection'; position: number; node: NavTreeNode }

function buildMergedChildren(node: NavTreeNode): MergedChild[] {
  const items: MergedChild[] = []
  for (const article of node.item.articles) {
    items.push({ kind: 'article', position: article.position, article })
  }
  for (const child of node.children) {
    items.push({ kind: 'collection', position: child.item.position, node: child })
  }
  // Primary: position. Tie-break: collections before articles (stable for
  // legacy imports where both types used independent 0..N sequences),
  // then by id for full determinism.
  items.sort((a, b) => {
    if (a.position !== b.position) return a.position - b.position
    if (a.kind !== b.kind) return a.kind === 'collection' ? -1 : 1
    const aId = a.kind === 'article' ? a.article.id : a.node.item.id
    const bId = b.kind === 'article' ? b.article.id : b.node.item.id
    return aId.localeCompare(bId)
  })
  return items
}

interface NavTreeProps {
  locale: string
  navigation: NavItem[]
  onArticleClick?: () => void
}

export function NavTree({
  locale,
  navigation,
  onArticleClick,
}: NavTreeProps) {
  const { enabledLocales } = useDocsContext()
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)
  const tree = buildNavTree(navigation)
  const defaultOpenRoots = tree
    .filter((node, index) => index === 0 || nodeContainsActivePath(node, {
      locale,
      multilingualEnabled,
      pathname,
    }))
    .map((node) => node.item.id)

  return (
    <nav key={pathname} className="px-3 py-4">
      <Accordion type="multiple" defaultValue={defaultOpenRoots}>
        {tree.map((node, idx) => (
          <CollectionGroup
            key={node.item.id}
            locale={locale}
            node={node}
            multilingualEnabled={multilingualEnabled}
            pathname={pathname}
            onArticleClick={onArticleClick}
            isFirst={idx === 0}
            level={0}
          />
        ))}
      </Accordion>
    </nav>
  )
}

function CollectionGroup({
  node,
  locale,
  multilingualEnabled,
  pathname,
  onArticleClick,
  isFirst,
  level,
}: {
  node: NavTreeNode
  locale: string
  multilingualEnabled: boolean
  pathname: string
  onArticleClick?: () => void
  isFirst: boolean
  level: number
}) {
  const spacing = level === 0 && !isFirst ? 'mt-5' : level > 0 ? 'mt-1' : ''
  const indent = collectionIndent(level)
  const headingClass = level === 0
    ? 'text-[14px] font-medium text-foreground'
    : 'text-[13px] font-medium text-muted-foreground'
  const collectionHref = buildCanonicalCollectionPath(
    multilingualEnabled,
    locale,
    node.item.slug,
    node.item.public_id,
  )
  const isActiveCollection = pathname === collectionHref
  const children = buildMergedChildren(node)
  const hasExpandableContent = children.length > 0
  const label = (
    <span className="flex min-w-0 items-center gap-2">
      {node.item.icon && level === 0 ? (
        <PublicIcon
          name={node.item.icon}
          size={16}
          className={cn(
            'shrink-0',
            isActiveCollection
              ? 'text-sidebar-active-foreground'
              : 'text-muted-foreground',
          )}
        />
      ) : null}
      <span className="truncate">{node.item.name}</span>
    </span>
  )

  if (!hasExpandableContent) {
    return (
      <div
        className={cn(
          'flex items-center gap-2 rounded-lg py-[7px]',
          spacing,
          indent,
          headingClass,
          isActiveCollection && 'bg-sidebar-active text-sidebar-active-foreground',
        )}
      >
        {label}
      </div>
    )
  }

  return (
    <AccordionItem value={node.item.id} className={cn('border-none', spacing)}>
      <AccordionTrigger
        className={cn(
          'cursor-pointer rounded-lg py-[7px] hover:no-underline',
          indent,
          headingClass,
          isActiveCollection
            ? 'bg-sidebar-active text-sidebar-active-foreground'
            : 'hover:bg-muted/50',
        )}
      >
        {label}
      </AccordionTrigger>
      <AccordionContent className="pb-0">
        <div className="mt-0.5">
          <MergedChildren
            children={children}
            locale={locale}
            multilingualEnabled={multilingualEnabled}
            pathname={pathname}
            onArticleClick={onArticleClick}
            level={level}
          />
        </div>
      </AccordionContent>
    </AccordionItem>
  )
}

function MergedChildren({
  children,
  locale,
  multilingualEnabled,
  pathname,
  onArticleClick,
  level,
}: {
  children: MergedChild[]
  locale: string
  multilingualEnabled: boolean
  pathname: string
  onArticleClick?: () => void
  level: number
}) {
  // For each collection child, compute if it contains the active path
  // so we can pre-expand it.
  const defaultOpen = children
    .filter(
      (c) =>
        c.kind === 'collection' &&
        nodeContainsActivePath(c.node, {
          locale,
          multilingualEnabled,
          pathname,
        }),
    )
    .map((c) => (c.kind === 'collection' ? c.node.item.id : ''))

  return (
    <Accordion type="multiple" defaultValue={defaultOpen} className="mt-1">
      {children.map((child, idx) => {
        if (child.kind === 'article') {
          return (
            <ArticleLink
              key={child.article.id}
              locale={locale}
              articleSlug={child.article.slug}
              publicId={child.article.public_id}
              title={child.article.title}
              multilingualEnabled={multilingualEnabled}
              pathname={pathname}
              onArticleClick={onArticleClick}
              level={level}
            />
          )
        }
        return (
          <NestedCollectionItem
            key={child.node.item.id}
            locale={locale}
            node={child.node}
            multilingualEnabled={multilingualEnabled}
            pathname={pathname}
            onArticleClick={onArticleClick}
            isFirst={idx === 0}
            level={level + 1}
          />
        )
      })}
    </Accordion>
  )
}

function NestedCollectionItem({
  node,
  locale,
  multilingualEnabled,
  pathname,
  onArticleClick,
  isFirst,
  level,
}: {
  node: NavTreeNode
  locale: string
  multilingualEnabled: boolean
  pathname: string
  onArticleClick?: () => void
  isFirst: boolean
  level: number
}) {
  const hasArticles = node.item.articles.length > 0
  const hasChildren = node.children.length > 0
  const hasExpandableContent = hasArticles || hasChildren
  const spacing = !isFirst ? 'mt-1' : ''
  const indent = collectionIndent(level)
  const collectionHref = buildCanonicalCollectionPath(
    multilingualEnabled,
    locale,
    node.item.slug,
    node.item.public_id,
  )
  const isActiveCollection = pathname === collectionHref

  if (!hasExpandableContent) {
    return (
      <DocsLink
        to={collectionHref}
        onClick={onArticleClick}
        className={cn(
          'block rounded-lg py-[7px] text-[13px] transition-colors',
          spacing,
          indent,
          isActiveCollection
            ? 'bg-sidebar-active font-medium text-sidebar-active-foreground'
            : 'text-muted-foreground hover:bg-muted/50 hover:text-foreground',
        )}
      >
        {node.item.name}
      </DocsLink>
    )
  }

  return (
    <AccordionItem value={node.item.id} className={cn('border-none', spacing)}>
      <AccordionTrigger
        className={cn(
          'cursor-pointer py-[7px] text-[13px] font-medium hover:no-underline',
          indent,
          isActiveCollection
            ? 'bg-sidebar-active text-sidebar-active-foreground'
            : 'text-muted-foreground hover:bg-muted/50 hover:text-foreground',
        )}
      >
        <span className="truncate">{node.item.name}</span>
      </AccordionTrigger>

      <AccordionContent className="pb-0">
        <div className="mt-0.5">
          <MergedChildren
            children={buildMergedChildren(node)}
            locale={locale}
            multilingualEnabled={multilingualEnabled}
            pathname={pathname}
            onArticleClick={onArticleClick}
            level={level}
          />
        </div>
      </AccordionContent>
    </AccordionItem>
  )
}

function ArticleLink({
  locale,
  articleSlug,
  publicId,
  title,
  multilingualEnabled,
  pathname,
  onArticleClick,
  level,
}: {
  locale: string
  articleSlug: string
  publicId: string
  title: string
  multilingualEnabled: boolean
  pathname: string
  onArticleClick?: () => void
  level: number
}) {
  const href = buildCanonicalArticlePath(
    multilingualEnabled,
    locale,
    articleSlug,
    publicId,
  )
  const isActive = pathname === href
  const indent = articleIndent(level)

  return (
    <DocsLink
      to={href}
      onClick={onArticleClick}
      className={cn(
        'block rounded-lg py-[7px] text-[13px] transition-colors',
        indent,
        isActive
          ? 'bg-sidebar-active font-medium text-sidebar-active-foreground'
          : 'text-muted-foreground hover:bg-muted/50 hover:text-foreground',
      )}
    >
      {title}
    </DocsLink>
  )
}

function collectionIndent(level: number) {
  if (level <= 0) return 'px-3'
  // Subcollection headers sit at the same indent as their sibling articles
  if (level === 1) return 'pl-5 pr-3'
  if (level === 2) return 'pl-8 pr-3'
  return 'pl-11 pr-3'
}

function articleIndent(level: number) {
  if (level <= 0) return 'pl-5 pr-3'
  // Articles inside subcollections are indented from their collection header
  if (level === 1) return 'pl-8 pr-3'
  if (level === 2) return 'pl-11 pr-3'
  return 'pl-14 pr-3'
}

function nodeContainsActivePath(
  node: NavTreeNode,
  {
    locale,
    multilingualEnabled,
    pathname,
  }: {
    locale: string
    multilingualEnabled: boolean
    pathname: string
  },
): boolean {
  const collectionPath = buildCanonicalCollectionPath(
    multilingualEnabled,
    locale,
    node.item.slug,
    node.item.public_id,
  )

  if (pathname === collectionPath) return true

  for (const article of node.item.articles) {
    const articlePath = buildCanonicalArticlePath(
      multilingualEnabled,
      locale,
      article.slug,
      article.public_id,
    )
    if (pathname === articlePath) return true
  }

  return node.children.some((child) =>
    nodeContainsActivePath(child, {
      locale,
      multilingualEnabled,
      pathname,
    }),
  )
}
