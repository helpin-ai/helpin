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
      <CollectionAccordionList
        locale={locale}
        nodes={tree}
        multilingualEnabled={multilingualEnabled}
        basepath={basepath}
        pathname={pathname}
        onArticleClick={onArticleClick}
        level={0}
      />
    </nav>
  )
}

function CollectionAccordionList({
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
  if (nodes.length === 0) return null

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
    <Accordion
      type="multiple"
      defaultValue={defaultValue}
      className={cn(level === 0 ? 'flex flex-col gap-2' : 'mt-1')}
    >
      {nodes.map((node) => (
        <CollectionAccordionItem
          key={node.item.id}
          locale={locale}
          node={node}
          multilingualEnabled={multilingualEnabled}
          basepath={basepath}
          pathname={pathname}
          onArticleClick={onArticleClick}
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
  basepath,
  pathname,
  onArticleClick,
  level,
}: {
  node: NavTreeNode
  locale: string
  multilingualEnabled: boolean
  basepath: string
  pathname: string
  onArticleClick?: () => void
  level: number
}) {
  const hasArticles = node.item.articles.length > 0
  const hasChildren = node.children.length > 0
  const hasExpandableContent = hasArticles || hasChildren
  const collectionHref = buildCanonicalCollectionPath(
    multilingualEnabled,
    locale,
    node.item.slug,
  )
  const isActiveCollection =
    pathname === prefixBasepath(basepath, collectionHref)
  const triggerIndent =
    level === 0 ? 'px-3' : level === 1 ? 'pl-4 pr-2' : 'pl-5 pr-2'

  if (!hasExpandableContent) {
    return (
      <DocsLink
        to={collectionHref}
        onClick={onArticleClick}
        className={cn(
          'flex items-center gap-2 rounded-lg py-2 text-[13px] transition-colors',
          triggerIndent,
          isActiveCollection
            ? 'bg-sidebar-active font-medium text-sidebar-active-foreground'
            : 'text-muted-foreground hover:bg-muted/60 hover:text-foreground',
        )}
      >
        {level === 0 && node.item.icon ? (
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
      </DocsLink>
    )
  }

  return (
    <AccordionItem
      value={node.item.id}
      className={cn(
        'border-none',
        level === 0 ? 'rounded-xl bg-background/60' : 'rounded-lg',
      )}
    >
      <AccordionTrigger
        className={cn(
          'hover:no-underline',
          triggerIndent,
          level === 0 ? 'text-[14px]' : 'text-[13px] text-muted-foreground',
          isActiveCollection
            ? 'bg-sidebar-active text-sidebar-active-foreground'
            : 'hover:bg-muted/60',
        )}
      >
        <span className="flex min-w-0 items-center gap-2">
          {level === 0 && node.item.icon ? (
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
        </span>
      </AccordionTrigger>

      <AccordionContent className={cn(level === 0 ? 'px-2' : 'px-0')}>
        <div
          className={cn(
            'ml-4 border-l border-border/70 pl-2',
            level === 0 ? 'pb-1' : 'pb-0',
          )}
        >
          {hasArticles ? (
            <div className="space-y-1">
              {node.item.articles.map((article) => {
                const href = buildCanonicalArticlePath(
                  multilingualEnabled,
                  locale,
                  article.slug,
                  article.public_id,
                )
                const isActiveArticle =
                  pathname === prefixBasepath(basepath, href)

                return (
                  <DocsLink
                    key={article.id}
                    to={href}
                    onClick={onArticleClick}
                    className={cn(
                      'block rounded-lg px-3 py-2 text-[13px] transition-colors',
                      isActiveArticle
                        ? 'bg-sidebar-active font-medium text-sidebar-active-foreground'
                        : 'text-muted-foreground hover:bg-muted/60 hover:text-foreground',
                    )}
                  >
                    {article.title}
                  </DocsLink>
                )
              })}
            </div>
          ) : null}

          {hasChildren ? (
            <CollectionAccordionList
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
