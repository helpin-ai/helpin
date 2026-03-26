import { Link, useMatchRoute } from '@tanstack/react-router'
import { cn } from '@/lib/utils'
import { PhIcon } from '@/components/PhIcon'
import type { NavItem } from '@/lib/types'

interface NavTreeProps {
  locale: string
  navigation: NavItem[]
  spaceSlug: string
  onArticleClick?: () => void
}

export function NavTree({
  locale,
  navigation,
  spaceSlug,
  onArticleClick,
}: NavTreeProps) {
  return (
    <nav className="py-4 px-3">
      {navigation.map((collection, idx) => (
        <CollectionGroup
          key={collection.id}
          locale={locale}
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
  locale,
  spaceSlug,
  onArticleClick,
  isFirst,
}: {
  collection: NavItem
  locale: string
  spaceSlug: string
  onArticleClick?: () => void
  isFirst: boolean
}) {
  const matchRoute = useMatchRoute()

  return (
    <div className={cn(!isFirst && 'mt-5')}>
      <div className="flex items-center gap-2 px-3 py-[7px] text-[14px] font-medium text-foreground">
        {collection.icon && (
          <PhIcon name={collection.icon} size={16} weight="regular" className="shrink-0 text-muted-foreground" />
        )}
        <span className="truncate">{collection.name}</span>
      </div>

      <div className="mt-0.5">
        {collection.articles.map((article) => {
          const isActive = !!matchRoute({
            to: '/$locale/$spaceSlug/$collectionSlug/$articleSlug',
            params: {
              locale,
              spaceSlug,
              collectionSlug: collection.slug,
              articleSlug: article.slug,
            },
          })

          return (
            <Link
              key={article.id}
              to="/$locale/$spaceSlug/$collectionSlug/$articleSlug"
              params={{
                locale,
                spaceSlug,
                collectionSlug: collection.slug,
                articleSlug: article.slug,
              }}
              onClick={onArticleClick}
              className={cn(
                'block rounded-lg px-3 py-[7px] text-[13px] transition-colors',
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
