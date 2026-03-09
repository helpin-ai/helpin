import { Link, useMatchRoute } from '@tanstack/react-router'
import { ChevronRight, FileText } from 'lucide-react'
import { useState } from 'react'
import { cn } from '@/lib/utils'
import type { NavItem } from '@/lib/types'

interface SidebarProps {
  navigation: NavItem[]
  spaceSlug: string
}

export function Sidebar({ navigation, spaceSlug }: SidebarProps) {
  return (
    <aside
      className="sticky top-[var(--hc-header-height)] hidden lg:block shrink-0 overflow-y-auto border-r"
      style={{
        width: 'var(--hc-sidebar-width)',
        height: 'calc(100vh - var(--hc-header-height))',
        borderColor: 'var(--hc-border)',
      }}
    >
      <nav className="py-5 px-3">
        {navigation.map((collection) => (
          <CollectionGroup
            key={collection.id}
            collection={collection}
            spaceSlug={spaceSlug}
          />
        ))}
      </nav>
    </aside>
  )
}

function CollectionGroup({
  collection,
  spaceSlug,
}: {
  collection: NavItem
  spaceSlug: string
}) {
  const [isOpen, setIsOpen] = useState(true)
  const matchRoute = useMatchRoute()

  const hasActiveArticle = collection.articles.some((a) =>
    matchRoute({
      to: '/$spaceSlug/$articleSlug',
      params: { spaceSlug, articleSlug: a.slug },
    }),
  )

  return (
    <div className="mb-1">
      {/* Collection header */}
      <button
        onClick={() => setIsOpen(!isOpen)}
        className={cn(
          'flex w-full items-center gap-1.5 rounded-md px-2 py-[7px] text-[13px] font-semibold transition-colors',
          hasActiveArticle
            ? 'text-[var(--hc-text)]'
            : 'text-[var(--hc-text-secondary)] hover:text-[var(--hc-text)]',
        )}
      >
        <ChevronRight
          size={14}
          className={cn(
            'transition-transform shrink-0 text-[var(--hc-text-muted)]',
            isOpen && 'rotate-90',
          )}
        />
        {collection.icon && <span className="shrink-0 text-sm">{collection.icon}</span>}
        <span className="truncate">{collection.name}</span>
      </button>

      {/* Article list */}
      {isOpen && (
        <div className="ml-[11px] border-l border-[var(--hc-border-light)]">
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
                className={cn(
                  'flex items-center gap-2 px-2.5 py-[6px] text-[13px] transition-colors border-l-2 -ml-px',
                  isActive
                    ? 'border-[var(--hc-accent)] text-[var(--hc-accent)] font-medium bg-[var(--hc-accent-light)]'
                    : 'border-transparent text-[var(--hc-text-secondary)] hover:text-[var(--hc-text)] hover:border-[var(--hc-border)]',
                )}
              >
                <FileText size={14} className="shrink-0 opacity-60" />
                <span className="truncate">{article.title}</span>
              </Link>
            )
          })}
        </div>
      )}
    </div>
  )
}
