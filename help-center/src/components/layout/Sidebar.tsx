import { Link, useMatchRoute } from '@tanstack/react-router'
import { ChevronRight, FileText } from 'lucide-react'
import { useState } from 'react'
import { cn } from '@/lib/utils'
import type { NavItem } from '@/lib/types'

interface SidebarProps {
  navigation: NavItem[]
}

export function Sidebar({ navigation }: SidebarProps) {
  return (
    <aside
      className="sticky top-[var(--hc-header-height)] hidden lg:block shrink-0 overflow-y-auto border-r"
      style={{
        width: 'var(--hc-sidebar-width)',
        height: 'calc(100vh - var(--hc-header-height))',
        borderColor: 'var(--hc-border)',
      }}
    >
      <nav className="py-4 px-3">
        {navigation.map((collection) => (
          <CollectionGroup key={collection.id} collection={collection} />
        ))}
      </nav>
    </aside>
  )
}

function CollectionGroup({ collection }: { collection: NavItem }) {
  const [isOpen, setIsOpen] = useState(true)
  const matchRoute = useMatchRoute()

  const hasActiveArticle = collection.articles.some((a) =>
    matchRoute({ to: '/articles/$articleSlug', params: { articleSlug: a.slug } }),
  )

  return (
    <div className="mb-2">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className={cn(
          'flex w-full items-center gap-1.5 rounded-md px-2 py-1.5 text-xs font-semibold uppercase tracking-wider transition-colors',
          hasActiveArticle
            ? 'text-[var(--hc-accent)]'
            : 'text-[var(--hc-text-secondary)] hover:text-[var(--hc-text)]',
        )}
      >
        <ChevronRight
          size={14}
          className={cn(
            'transition-transform shrink-0',
            isOpen && 'rotate-90',
          )}
        />
        {collection.icon && <span className="shrink-0">{collection.icon}</span>}
        <span className="truncate">{collection.name}</span>
      </button>

      {isOpen && (
        <div className="ml-2 mt-0.5 border-l" style={{ borderColor: 'var(--hc-border-light)' }}>
          {collection.articles.map((article) => {
            const isActive = !!matchRoute({
              to: '/articles/$articleSlug',
              params: { articleSlug: article.slug },
            })

            return (
              <Link
                key={article.id}
                to="/articles/$articleSlug"
                params={{ articleSlug: article.slug }}
                className={cn(
                  'flex items-center gap-2 rounded-md ml-1 px-2 py-1.5 text-sm transition-colors',
                  isActive
                    ? 'bg-[var(--hc-accent-light)] text-[var(--hc-accent)] font-medium'
                    : 'text-[var(--hc-text-secondary)] hover:text-[var(--hc-text)] hover:bg-[var(--hc-bg-secondary)]',
                )}
              >
                <FileText size={14} className="shrink-0" />
                <span className="truncate">{article.title}</span>
              </Link>
            )
          })}
        </div>
      )}
    </div>
  )
}
