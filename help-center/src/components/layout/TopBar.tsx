import { Search, Moon, Sun } from 'lucide-react'
import { Link, useParams } from '@tanstack/react-router'
import { cn } from '@/lib/utils'
import { useDocsContext } from '@/contexts/DocsContext'
import { useTheme } from '@/hooks/useTheme'

interface TopBarProps {
  onSearchClick: () => void
}

export function TopBar({ onSearchClick }: TopBarProps) {
  const { config, spaces } = useDocsContext()
  const params = useParams({ strict: false }) as { spaceSlug?: string }
  const activeSpaceSlug = params.spaceSlug
  const { theme, toggleTheme } = useTheme()

  return (
    <header className="sticky top-0 z-30 flex items-center border-b border-border bg-background/95 backdrop-blur-sm px-5 h-[var(--hc-header-height)]">
      {/* Brand */}
      <Link to="/" className="flex items-center gap-2.5 shrink-0 mr-8">
        {config.brand_logo_url && (
          <img
            src={config.brand_logo_url}
            alt={config.brand_name}
            className="h-6 w-auto object-contain"
          />
        )}
        <span className="font-semibold text-[15px] tracking-tight text-foreground">
          {config.brand_name || 'Docs'}
        </span>
      </Link>

      {/* Space tabs */}
      {spaces.length > 1 && (
        <nav className="hidden sm:flex items-center gap-1">
          {spaces.map((space) => {
            const isActive = activeSpaceSlug === space.slug
            return (
              <Link
                key={space.id}
                to="/$spaceSlug"
                params={{ spaceSlug: space.slug }}
                className={cn(
                  'px-3 py-1.5 text-[13.5px] rounded-md transition-colors',
                  isActive
                    ? 'text-foreground font-semibold'
                    : 'text-muted-foreground font-medium hover:text-foreground',
                )}
              >
                {space.icon && <span className="mr-1.5">{space.icon}</span>}
                {space.name}
              </Link>
            )
          })}
        </nav>
      )}

      {/* Search trigger */}
      <button
        onClick={onSearchClick}
        className="ml-auto flex items-center gap-2 pl-3 pr-2 h-8 rounded-lg border border-border bg-muted/40 hover:bg-muted/70 transition-colors cursor-text max-w-[240px] w-full"
      >
        <Search size={14} className="text-muted-foreground shrink-0" />
        <span className="flex-1 text-left text-[13px] text-muted-foreground">
          Search...
        </span>
        <kbd className="hidden sm:inline-flex items-center rounded-[4px] px-1.5 py-0.5 text-[10px] font-mono font-medium bg-background border border-border text-muted-foreground">
          ⌘K
        </kbd>
      </button>

      {/* Theme toggle */}
      <button
        onClick={toggleTheme}
        className="ml-3 inline-flex items-center justify-center h-8 w-8 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-colors"
        aria-label="Toggle dark mode"
      >
        {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
      </button>
    </header>
  )
}
