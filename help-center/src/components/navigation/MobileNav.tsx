import { useEffect } from 'react'
import { Link, useMatchRoute } from '@tanstack/react-router'
import { X, FileText } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { NavItem } from '@/lib/types'

interface MobileNavProps {
  navigation: NavItem[]
  spaceSlug: string
  onClose: () => void
}

export function MobileNav({ navigation, spaceSlug, onClose }: MobileNavProps) {
  const matchRoute = useMatchRoute()

  // Prevent body scroll while open
  useEffect(() => {
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = ''
    }
  }, [])

  return (
    <div className="fixed inset-0 z-40 lg:hidden">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />

      {/* Drawer */}
      <aside
        className="absolute left-0 top-0 bottom-0 overflow-y-auto"
        style={{
          width: 'min(280px, 85vw)',
          backgroundColor: 'var(--hc-bg)',
          borderRight: '1px solid var(--hc-border)',
        }}
      >
        <div
          className="flex items-center justify-between px-4 py-3 border-b"
          style={{ borderColor: 'var(--hc-border)' }}
        >
          <span className="text-sm font-semibold">Navigation</span>
          <button
            onClick={onClose}
            className="p-1 rounded-md transition-colors hover:bg-[var(--hc-bg-secondary)]"
            aria-label="Close navigation"
          >
            <X size={16} style={{ color: 'var(--hc-text-secondary)' }} />
          </button>
        </div>
        <nav className="py-3 px-3">
          {navigation.map((collection) => (
            <div key={collection.id} className="mb-2">
              <div
                className="flex items-center gap-1.5 px-2 py-[7px] text-[13px] font-semibold"
                style={{ color: 'var(--hc-text)' }}
              >
                {collection.icon && (
                  <span className="text-sm">{collection.icon}</span>
                )}
                <span>{collection.name}</span>
              </div>
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
                      onClick={onClose}
                      className={cn(
                        'flex items-center gap-2 px-2.5 py-[7px] text-[13px] border-l-2 -ml-px transition-colors',
                        isActive
                          ? 'border-[var(--hc-accent)] text-[var(--hc-accent)] font-medium bg-[var(--hc-accent-light)]'
                          : 'border-transparent text-[var(--hc-text-secondary)] hover:text-[var(--hc-text)]',
                      )}
                    >
                      <FileText size={14} className="shrink-0 opacity-60" />
                      <span className="truncate">{article.title}</span>
                    </Link>
                  )
                })}
              </div>
            </div>
          ))}
        </nav>
      </aside>
    </div>
  )
}
