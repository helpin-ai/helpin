import { useRouterState } from '@tanstack/react-router'
import { DocsLink } from '@/components/DocsLink'
import { useDocsContext } from '@/contexts/DocsContext'
import { buildCanonicalArticlePath, isMultilingualEnabled } from '@/lib/locale'
import { prefixBasepath } from '@/lib/pathUtils'
import { cn } from '@/lib/utils'
import { PhIcon } from '@/components/PhIcon'
import type { NavItem } from '@/lib/types'

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
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)

  return (
    <nav className="py-4 px-3">
      {navigation.map((collection, idx) => (
        <CollectionGroup
          key={collection.id}
          locale={locale}
          collection={collection}
          multilingualEnabled={multilingualEnabled}
          basepath={basepath}
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
  multilingualEnabled,
  basepath,
  onArticleClick,
  isFirst,
}: {
  collection: NavItem
  locale: string
  multilingualEnabled: boolean
  basepath: string
  onArticleClick?: () => void
  isFirst: boolean
}) {
  const pathname = useRouterState({ select: (state) => state.location.pathname })

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
          const href = buildCanonicalArticlePath(
            multilingualEnabled,
            locale,
            collection.slug,
            article.slug,
          )
          const isActive = pathname === prefixBasepath(basepath, href)

          return (
            <DocsLink
              key={article.id}
              to={href}
              onClick={onArticleClick}
              className={cn(
                'block rounded-lg px-3 py-[7px] text-[13px] transition-colors',
                isActive
                  ? 'bg-sidebar-active text-sidebar-active-foreground font-medium'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              {article.title}
            </DocsLink>
          )
        })}
      </div>
    </div>
  )
}
