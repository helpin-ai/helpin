import { Link, useRouterState } from '@tanstack/react-router'
import { useDocsContext } from '@/contexts/DocsContext'
import { buildCanonicalArticlePath, isMultilingualEnabled } from '@/lib/locale'
import { cn } from '@/lib/utils'
import { PhIcon } from '@/components/PhIcon'
import { buildNavTree } from '@/lib/navigation'
import type { NavItem, NavTreeNode } from '@/lib/types'

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
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)
  const tree = buildNavTree(navigation)

  return (
    <nav className="py-4 px-3">
      {tree.map((node, idx) => (
        <CollectionGroup
          key={node.item.id}
          locale={locale}
          node={node}
          multilingualEnabled={multilingualEnabled}
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
  onArticleClick,
  isFirst,
  level,
}: {
  node: NavTreeNode
  locale: string
  multilingualEnabled: boolean
  onArticleClick?: () => void
  isFirst: boolean
  level: number
}) {
  const pathname = useRouterState({ select: (state) => state.location.pathname })

  // Top-level sections are spaced out for visual grouping. Nested
  // sub-collections sit flush under their parent header and use their
  // parent's spacing; the indentation comes from the `level` offset.
  const spacing = level === 0 && !isFirst ? 'mt-5' : level > 0 ? 'mt-1' : ''
  // Each depth level indents children by a small amount so the tree is
  // visually readable without overwhelming narrower sidebars.
  const indent = level === 0 ? 'px-3' : level === 1 ? 'pl-5 pr-3' : 'pl-7 pr-3'
  const headingClass = level === 0
    ? 'text-[14px] font-medium text-foreground'
    : 'text-[13px] font-medium text-muted-foreground'

  return (
    <div className={cn(spacing)}>
      <div className={cn('flex items-center gap-2 py-[7px]', indent, headingClass)}>
        {node.item.icon && (
          <PhIcon name={node.item.icon} size={16} weight="regular" className="shrink-0 text-muted-foreground" />
        )}
        <span className="truncate">{node.item.name}</span>
      </div>

      <div className="mt-0.5">
        {node.item.articles.map((article) => {
          const href = buildCanonicalArticlePath(
            multilingualEnabled,
            locale,
            node.item.slug,
            article.slug,
          )
          const isActive = pathname === href

          return (
            <Link
              key={article.id}
              to={href}
              onClick={onArticleClick}
              className={cn(
                'block rounded-lg py-[7px] text-[13px] transition-colors',
                indent,
                isActive
                  ? 'bg-sidebar-active text-sidebar-active-foreground font-medium'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              {article.title}
            </Link>
          )
        })}

        {node.children.map((child, idx) => (
          <CollectionGroup
            key={child.item.id}
            locale={locale}
            node={child}
            multilingualEnabled={multilingualEnabled}
            onArticleClick={onArticleClick}
            isFirst={idx === 0}
            level={level + 1}
          />
        ))}
      </div>
    </div>
  )
}
