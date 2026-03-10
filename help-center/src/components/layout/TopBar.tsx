import { Search } from 'lucide-react'
import { Link, useParams } from '@tanstack/react-router'
import { cn } from '@/lib/utils'
import { useDocsContext } from '@/contexts/DocsContext'

interface TopBarProps {
  onSearchClick: () => void
}

export function TopBar({ onSearchClick }: TopBarProps) {
  const { config, spaces } = useDocsContext()
  const params = useParams({ strict: false }) as { spaceSlug?: string }
  const activeSpaceSlug = params.spaceSlug

  return (
    <header
      className="sticky top-0 z-30 flex items-center border-b px-5"
      style={{
        height: 'var(--hc-header-height)',
        backgroundColor: 'var(--hc-bg)',
        borderColor: 'var(--hc-border)',
      }}
    >
      {/* Brand */}
      <Link to="/" className="flex items-center gap-2 shrink-0 mr-6">
        {config.brand_logo_url && (
          <img
            src={config.brand_logo_url}
            alt={config.brand_name}
            className="h-6 w-6 rounded object-contain"
          />
        )}
        <span className="font-semibold text-[15px] tracking-tight">
          {config.brand_name || 'Docs'}
        </span>
      </Link>

      {/* Space tabs — hidden on small screens */}
      {spaces.length > 1 && (
        <nav className="hidden sm:flex items-center gap-0.5">
          {spaces.map((space) => {
            const isActive = activeSpaceSlug === space.slug
            return (
              <Link
                key={space.id}
                to="/$spaceSlug"
                params={{ spaceSlug: space.slug }}
                className={cn(
                  'relative px-3 py-1.5 text-[13px] font-medium rounded-md transition-colors',
                  isActive
                    ? 'text-[var(--hc-accent)]'
                    : 'text-[var(--hc-text-secondary)] hover:text-[var(--hc-text)] hover:bg-[var(--hc-bg-secondary)]',
                )}
              >
                {space.icon && <span className="mr-1">{space.icon}</span>}
                {space.name}
              </Link>
            )
          })}
        </nav>
      )}

      {/* Search trigger — opens ⌘K dialog */}
      <button
        onClick={onSearchClick}
        className="ml-auto flex items-center gap-2 rounded-lg border px-3 py-1.5 text-sm transition-colors hover:border-[var(--hc-accent)] max-w-xs w-full cursor-text"
        style={{
          backgroundColor: 'var(--hc-bg-secondary)',
          borderColor: 'var(--hc-border)',
        }}
      >
        <Search size={14} style={{ color: 'var(--hc-text-muted)' }} />
        <span
          className="flex-1 text-left text-[13px]"
          style={{ color: 'var(--hc-text-muted)' }}
        >
          Search...
        </span>
        <kbd
          className="hidden sm:inline-flex items-center rounded px-1.5 py-0.5 text-[11px] font-mono"
          style={{
            backgroundColor: 'var(--hc-bg-tertiary)',
            color: 'var(--hc-text-muted)',
          }}
        >
          ⌘K
        </kbd>
      </button>
    </header>
  )
}
