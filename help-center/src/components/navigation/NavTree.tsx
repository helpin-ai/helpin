import { useRouterState } from '@tanstack/react-router'
import { DocsLink } from '@/components/DocsLink'
import { PhIcon } from '@/components/PhIcon'
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
import { prefixBasepath } from '@/lib/pathUtils'
import type { NavItem, NavTreeNode } from '@/lib/types'
import { cn } from '@/lib/utils'

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
  const { enabledLocales, basepath } = useDocsContext()
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)
  const tree = buildNavTree(navigation)

  return (
    <nav key={pathname} className="px-3 py-4">
      {tree.map((node, idx) => (
        <CollectionGroup
          key={node.item.id}
          locale={locale}
          node={node}
          multilingualEnabled={multilingualEnabled}
          basepath={basepath}
          pathname={pathname}
          onArticleClick={onArticleClick}
          isFirst={idx === 0}
          level={0}
        />
      ))}
    </nav>
  )
}

function CollectionGroup({
  node,
  locale,
  multilingualEnabled,
  basepath,
  pathname,
  onArticleClick,
  isFirst,
  level,
}: {
  node: NavTreeNode
  locale: string
  multilingualEnabled: boolean
  basepath: string
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
  )
  const isActiveCollection =
    pathname === prefixBasepath(basepath, collectionHref)

  return (
    <div className={cn(spacing)}>
      <div
        className={cn(
          'flex items-center gap-2 rounded-lg py-[7px]',
          indent,
          headingClass,
          isActiveCollection && 'bg-sidebar-active text-sidebar-active-foreground',
        )}
      >
        {node.item.icon && level === 0 ? (
          <PhIcon
            name={node.item.icon}
            size={16}
            weight="regular"
            className={cn(
              'shrink-0',
              isActiveCollection
                ? 'text-sidebar-active-foreground'
                : 'text-muted-foreground',
            )}
          />
        ) : null}
        <span className="truncate">{node.item.name}</span>
      </div>

      <div className="mt-0.5">
        {node.item.articles.map((article) => (
          <ArticleLink
            key={article.id}
            locale={locale}
            articleSlug={article.slug}
            publicId={article.public_id}
            title={article.title}
            multilingualEnabled={multilingualEnabled}
            basepath={basepath}
            pathname={pathname}
            onArticleClick={onArticleClick}
            level={level}
          />
        ))}

        {node.children.length > 0 ? (
          <NestedCollectionAccordion
            locale={locale}
            nodes={node.children}
            multilingualEnabled={multilingualEnabled}
            basepath={basepath}
            pathname={pathname}
            onArticleClick={onArticleClick}
            level={level + 1}
          />
        ) : null}
      </div>
    </div>
  )
}

function NestedCollectionAccordion({
  nodes,
  locale,
  multilingualEnabled,
  basepath,
  pathname,
  onArticleClick,
  level,
}: {
  nodes: NavTreeNode[]
  locale: string
  multilingualEnabled: boolean
  basepath: string
  pathname: string
  onArticleClick?: () => void
  level: number
}) {
  const defaultValue = nodes
    .filter((node) =>
      nodeContainsActivePath(node, {
        locale,
        multilingualEnabled,
        basepath,
        pathname,
      }),
    )
    .map((node) => node.item.id)

  return (
    <Accordion type="multiple" defaultValue={defaultValue} className="mt-1">
      {nodes.map((node, idx) => (
        <NestedCollectionItem
          key={node.item.id}
          locale={locale}
          node={node}
          multilingualEnabled={multilingualEnabled}
          basepath={basepath}
          pathname={pathname}
          onArticleClick={onArticleClick}
          isFirst={idx === 0}
          level={level}
        />
      ))}
    </Accordion>
  )
}

function NestedCollectionItem({
  node,
  locale,
  multilingualEnabled,
  basepath,
  pathname,
  onArticleClick,
  isFirst,
  level,
}: {
  node: NavTreeNode
  locale: string
  multilingualEnabled: boolean
  basepath: string
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
  )
  const isActiveCollection =
    pathname === prefixBasepath(basepath, collectionHref)

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
            : 'text-muted-foreground hover:text-foreground',
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
          'py-[7px] text-[13px] font-medium hover:bg-transparent hover:no-underline',
          indent,
          isActiveCollection
            ? 'bg-sidebar-active text-sidebar-active-foreground'
            : 'text-muted-foreground hover:text-foreground',
        )}
      >
        <span className="truncate">{node.item.name}</span>
      </AccordionTrigger>

      <AccordionContent className="pb-0">
        <div className="mt-0.5">
          {node.item.articles.map((article) => (
            <ArticleLink
              key={article.id}
              locale={locale}
              articleSlug={article.slug}
              publicId={article.public_id}
              title={article.title}
              multilingualEnabled={multilingualEnabled}
              basepath={basepath}
              pathname={pathname}
              onArticleClick={onArticleClick}
              level={level}
            />
          ))}

          {node.children.length > 0 ? (
            <NestedCollectionAccordion
              locale={locale}
              nodes={node.children}
              multilingualEnabled={multilingualEnabled}
              basepath={basepath}
              pathname={pathname}
              onArticleClick={onArticleClick}
              level={level + 1}
            />
          ) : null}
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
  basepath,
  pathname,
  onArticleClick,
  level,
}: {
  locale: string
  articleSlug: string
  publicId: string
  title: string
  multilingualEnabled: boolean
  basepath: string
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
  const isActive = pathname === prefixBasepath(basepath, href)
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
          : 'text-muted-foreground hover:text-foreground',
      )}
    >
      {title}
    </DocsLink>
  )
}

function collectionIndent(level: number) {
  if (level <= 0) return 'px-3'
  if (level === 1) return 'pl-5 pr-3'
  if (level === 2) return 'pl-7 pr-3'
  return 'pl-9 pr-3'
}

function articleIndent(level: number) {
  if (level <= 0) return 'px-3'
  if (level === 1) return 'pl-7 pr-3'
  if (level === 2) return 'pl-9 pr-3'
  return 'pl-11 pr-3'
}

function nodeContainsActivePath(
  node: NavTreeNode,
  {
    locale,
    multilingualEnabled,
    basepath,
    pathname,
  }: {
    locale: string
    multilingualEnabled: boolean
    basepath: string
    pathname: string
  },
): boolean {
  const collectionPath = prefixBasepath(
    basepath,
    buildCanonicalCollectionPath(multilingualEnabled, locale, node.item.slug),
  )

  if (pathname === collectionPath) return true

  for (const article of node.item.articles) {
    const articlePath = prefixBasepath(
      basepath,
      buildCanonicalArticlePath(
        multilingualEnabled,
        locale,
        article.slug,
        article.public_id,
      ),
    )
    if (pathname === articlePath) return true
  }

  return node.children.some((child) =>
    nodeContainsActivePath(child, {
      locale,
      multilingualEnabled,
      basepath,
      pathname,
    }),
  )
}
