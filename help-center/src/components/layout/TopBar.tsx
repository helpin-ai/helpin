import { Search } from 'lucide-react'
import { useState, useCallback } from 'react'
import { useNavigate } from '@tanstack/react-router'
import type { HelpCenterConfig } from '@/lib/types'

interface TopBarProps {
  config: HelpCenterConfig | undefined
}

export function TopBar({ config }: TopBarProps) {
  const [query, setQuery] = useState('')
  const navigate = useNavigate()

  const handleSearch = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault()
      const trimmed = query.trim()
      if (trimmed.length >= 2) {
        navigate({ to: '/search', search: { q: trimmed } })
      }
    },
    [query, navigate],
  )

  return (
    <header
      className="sticky top-0 z-30 flex items-center gap-4 border-b px-6"
      style={{
        height: 'var(--hc-header-height)',
        backgroundColor: 'var(--hc-bg)',
        borderColor: 'var(--hc-border)',
      }}
    >
      {/* Brand */}
      <div className="flex items-center gap-2.5 shrink-0">
        {config?.brand_logo_url && (
          <img
            src={config.brand_logo_url}
            alt={config.brand_name}
            className="h-7 w-7 rounded object-contain"
          />
        )}
        <span className="font-semibold text-sm">{config?.brand_name || 'Help Center'}</span>
      </div>

      {/* Search */}
      <form onSubmit={handleSearch} className="flex-1 max-w-md ml-auto">
        <div
          className="flex items-center gap-2 rounded-lg border px-3 py-1.5 text-sm transition-colors focus-within:border-[var(--hc-accent)]"
          style={{
            backgroundColor: 'var(--hc-bg-secondary)',
            borderColor: 'var(--hc-border)',
          }}
        >
          <Search size={15} style={{ color: 'var(--hc-text-muted)' }} />
          <input
            type="text"
            placeholder="Search documentation..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="flex-1 bg-transparent outline-none placeholder:text-[var(--hc-text-muted)]"
          />
          <kbd
            className="hidden sm:inline-flex items-center rounded px-1.5 py-0.5 text-xs font-mono"
            style={{
              backgroundColor: 'var(--hc-bg-tertiary)',
              color: 'var(--hc-text-muted)',
            }}
          >
            /
          </kbd>
        </div>
      </form>
    </header>
  )
}
