import { useRouterState } from '@tanstack/react-router'
import { Braces } from 'lucide-react'
import { DocsLink } from '@/components/DocsLink'
import { ScrollArea } from '@/components/ui/scroll-area'
import { NavTree } from '@/components/navigation/NavTree'
import { useDocsContext } from '@/contexts/DocsContext'
import { useAPIReferences } from '@/hooks/queries'
import { buildCanonicalAPIReferencePath } from '@/lib/locale'
import type { NavItem } from '@/lib/types'
import { cn } from '@/lib/utils'

interface SidebarProps {
  locale: string
  navigation: NavItem[]
  spaceSlug?: string
}

export function Sidebar({ locale, navigation, spaceSlug = '' }: SidebarProps) {
  const { subdomain, multilingualEnabled } = useDocsContext()
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const { data: apiReferences = [] } = useAPIReferences(
    subdomain,
    locale,
    spaceSlug,
    multilingualEnabled,
  )

  return (
    <aside
      className="sticky top-[var(--hc-header-height)] hidden lg:block shrink-0 border-r border-border/70 dark:bg-card"
      style={{
        width: 'var(--hc-sidebar-width)',
        height: 'calc(100vh - var(--hc-header-height))',
      }}
      >
      <ScrollArea className="h-full">
        <NavTree locale={locale} navigation={navigation} />
        {apiReferences.length > 0 && (
          <nav
            aria-label="API references"
            className={cn(
              'mx-3 pb-5 pt-4',
              navigation.length > 0 && 'border-t border-border/70',
            )}
          >
            <p className="mb-2 px-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground/70">
              API Reference
            </p>
            <div className="space-y-1">
              {apiReferences.map((reference) => {
                const target = buildCanonicalAPIReferencePath(
                  multilingualEnabled,
                  locale,
                  spaceSlug,
                  reference.slug,
                )
                const active = pathname === target
                return (
                  <DocsLink
                    key={reference.id}
                    to={target}
                    aria-current={active ? 'page' : undefined}
                    className={cn(
                      'flex items-center gap-2 rounded-md px-2 py-1.5 text-[12.5px] font-medium transition-colors',
                      active
                        ? 'bg-sidebar-active text-sidebar-active-foreground'
                        : 'text-muted-foreground hover:bg-muted/60 hover:text-foreground',
                    )}
                  >
                    <Braces size={13} className="shrink-0" />
                    <span className="min-w-0 truncate">{reference.name}</span>
                  </DocsLink>
                )
              })}
            </div>
          </nav>
        )}
      </ScrollArea>
    </aside>
  )
}

export function SidebarSkeleton() {
  return (
    <aside
      aria-hidden="true"
      className="sticky top-[var(--hc-header-height)] hidden shrink-0 border-r border-border/70 px-4 py-5 lg:block"
      style={{
        width: 'var(--hc-sidebar-width)',
        height: 'calc(100vh - var(--hc-header-height))',
      }}
    >
      <div className="h-4 w-2/3 animate-pulse rounded bg-muted" />
      <div className="mt-5 h-3 w-5/6 animate-pulse rounded bg-muted" />
      <div className="mt-3 h-3 w-3/4 animate-pulse rounded bg-muted" />
      <div className="mt-3 h-3 w-4/5 animate-pulse rounded bg-muted" />
    </aside>
  )
}
