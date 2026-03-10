import { Link, useMatchRoute } from '@tanstack/react-router'
import { cn } from '@/lib/utils'
import type { NavItem } from '@/lib/types'

interface NavTreeProps {
  navigation: NavItem[]
  spaceSlug: string
  onArticleClick?: () => void
}

export function NavTree({
  navigation,
  spaceSlug,
  onArticleClick,
}: NavTreeProps) {
  return (
    <nav className="py-4 px-3">
      {navigation.map((collection, idx) => (
        <CollectionGroup
          key={collection.id}
          collection={collection}
          spaceSlug={spaceSlug}
          onArticleClick={onArticleClick}
          isFirst={idx === 0}
        />
      ))}
    </nav>
  )
}

function CollectionGroup({
  collection,
  spaceSlug,
  onArticleClick,
  isFirst,
}: {
  collection: NavItem
  spaceSlug: string
  onArticleClick?: () => void
  isFirst: boolean
}) {
  const matchRoute = useMatchRoute()

  return (
    <div className={cn(!isFirst && 'mt-5')}>
      <div className="flex items-center gap-2 px-3 py-[7px] text-[14px] font-medium text-foreground">
        {collection.icon && (
          <span className="shrink-0 text-[15px]">{collection.icon}</span>
        )}
        <span className="truncate">{collection.name}</span>
      </div>

      <div className="mt-0.5">
        {collection.articles.map((article) => {
          const isActive = !!matchRoute({
            to: '/$spaceSlug/$articleSlug',
            params: { spaceSlug, articleSlug: article.slug },
          })

          return (
            <Link
              key={article.id}
              to="/$spaceSlug/$articleSlug"
              params={{ spaceSlug, articleSlug: article.slug }}
              onClick={onArticleClick}
              className={cn(
                'block rounded-lg px-3 py-[7px] text-[14px] transition-colors',
                isActive
                  ? 'bg-sidebar-active text-sidebar-active-foreground font-medium'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              {article.title}
            </Link>
          )
        })}
      </div>
    </div>
  )
}
