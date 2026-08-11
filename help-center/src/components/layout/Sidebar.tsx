import { ScrollArea } from '@/components/ui/scroll-area'
import { NavTree } from '@/components/navigation/NavTree'
import type { NavItem } from '@/lib/types'

interface SidebarProps {
  locale: string
  navigation: NavItem[]
}

export function Sidebar({ locale, navigation }: SidebarProps) {
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
