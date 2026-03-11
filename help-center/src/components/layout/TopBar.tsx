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
  const { theme, toggleTheme, canToggle } = useTheme(config.theme_mode)

  const sortedLinks = [...(config.header_links ?? [])].sort(
    (a, b) => (a.position ?? 0) - (b.position ?? 0),
  )

  return (
    <header className="sticky top-0 z-30 grid grid-cols-[1fr_auto_1fr] items-center border-b border-border bg-background/95 backdrop-blur-sm px-5 h-[var(--hc-header-height)]">
      {/* Left: Brand + Space tabs */}
      <div className="flex items-center min-w-0">
        <Link to="/" className="flex items-center gap-2.5 shrink-0">
          {config.brand_logo_url ? (
            <>
              <img
                src={config.brand_logo_url}
                alt={config.brand_name}
                className={cn(
                  'h-8 w-auto object-contain',
                  config.brand_logo_dark_url && 'dark:hidden',
                )}
              />
              {config.brand_logo_dark_url && (
                <img
                  src={config.brand_logo_dark_url}
                  alt={config.brand_name}
                  className="h-8 w-auto object-contain hidden dark:block"
                />
              )}
            </>
          ) : (
            <span className="font-semibold text-[15px] tracking-tight text-foreground">
              {config.brand_name || 'Docs'}
            </span>
          )}
        </Link>

        {spaces.length > 1 && (
          <>
          <div className="h-5 w-px bg-border ml-5 mr-1.5 shrink-0" />
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
          </>
        )}
      </div>

      {/* Center: Search */}
      <button
        onClick={onSearchClick}
        className="flex items-center gap-2 pl-3 pr-2 h-8 rounded-lg border border-border bg-muted/40 hover:bg-muted/70 transition-colors cursor-text w-[280px]"
      >
        <Search size={14} className="text-muted-foreground shrink-0" />
        <span className="flex-1 text-left text-[13px] text-muted-foreground">
          {config.search_placeholder || 'Search...'}
        </span>
        <kbd className="hidden sm:inline-flex items-center rounded-[4px] px-1.5 py-0.5 text-[10px] font-mono font-medium bg-background border border-border text-muted-foreground">
          ⌘K
        </kbd>
      </button>

      {/* Right: Header links + Theme toggle */}
      <div className="flex items-center justify-end gap-1">
        {sortedLinks.length > 0 && (
          <nav className="hidden md:flex items-center gap-1">
            {sortedLinks.map((link, i) =>
              link.style === 'button' ? (
                <a
                  key={i}
                  href={link.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center px-3 py-1.5 text-[13px] font-medium rounded-md transition-colors text-primary-foreground"
                  style={{ backgroundColor: config.brand_color || 'var(--color-primary)' }}
                >
                  {link.label}
                </a>
              ) : (
                <a
                  key={i}
                  href={link.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="px-3 py-1.5 text-[13px] font-medium text-muted-foreground hover:text-foreground transition-colors rounded-md"
                >
                  {link.label}
                </a>
              ),
            )}
          </nav>
        )}

        {canToggle && (
          <div className="relative group ml-2">
            <button
              onClick={toggleTheme}
              className="inline-flex items-center justify-center h-8 w-8 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-colors"
              aria-label={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
            >
              {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
            </button>
            <span className="pointer-events-none absolute left-1/2 -translate-x-1/2 top-full mt-1.5 px-2 py-1 rounded-md bg-foreground text-background text-[11px] font-medium whitespace-nowrap opacity-0 group-hover:opacity-100 transition-opacity duration-150">
              {theme === 'dark' ? 'Light mode' : 'Dark mode'}
            </span>
          </div>
        )}
      </div>
    </header>
  )
}
