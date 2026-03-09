import { Search } from 'lucide-react'
import { useState, useCallback } from 'react'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { cn } from '@/lib/utils'
import type { HelpCenterConfig, Space } from '@/lib/types'

interface TopBarProps {
  config: HelpCenterConfig | undefined
  spaces: Space[]
}

export function TopBar({ config, spaces }: TopBarProps) {
  const [query, setQuery] = useState('')
  const navigate = useNavigate()
  const params = useParams({ strict: false }) as { spaceSlug?: string }
  const activeSpaceSlug = params.spaceSlug

  const handleSearch = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault()
      const trimmed = query.trim()
      if (trimmed.length >= 2) {
        navigate({
          to: '/search',
          search: { q: trimmed, space: activeSpaceSlug },
        })
      }
    },
    [query, navigate, activeSpaceSlug],
  )

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
        {config?.brand_logo_url && (
          <img
            src={config.brand_logo_url}
            alt={config.brand_name}
            className="h-6 w-6 rounded object-contain"
          />
        )}
        <span className="font-semibold text-[15px] tracking-tight">
          {config?.brand_name || 'Docs'}
        </span>
      </Link>

      {/* Space tabs — inline in header like Mintlify */}
      {spaces.length > 1 && (
        <nav className="flex items-center gap-0.5">
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

      {/* Search — right aligned */}
      <form onSubmit={handleSearch} className="ml-auto max-w-xs w-full">
        <div
          className="flex items-center gap-2 rounded-lg border px-3 py-1.5 text-sm transition-colors focus-within:border-[var(--hc-accent)]"
          style={{
            backgroundColor: 'var(--hc-bg-secondary)',
            borderColor: 'var(--hc-border)',
          }}
        >
          <Search size={14} style={{ color: 'var(--hc-text-muted)' }} />
          <input
            type="text"
            placeholder="Search..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="flex-1 bg-transparent outline-none text-[13px] placeholder:text-[var(--hc-text-muted)]"
          />
          <kbd
            className="hidden sm:inline-flex items-center rounded px-1.5 py-0.5 text-[11px] font-mono"
            style={{
              backgroundColor: 'var(--hc-bg-tertiary)',
              color: 'var(--hc-text-muted)',
            }}
          >
            ⌘K
          </kbd>
        </div>
      </form>
    </header>
  )
}
