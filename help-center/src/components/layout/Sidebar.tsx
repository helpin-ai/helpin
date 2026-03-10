import { ScrollArea } from '@/components/ui/scroll-area'
import { NavTree } from '@/components/navigation/NavTree'
import type { NavItem } from '@/lib/types'

interface SidebarProps {
  navigation: NavItem[]
  spaceSlug: string
}

export function Sidebar({ navigation, spaceSlug }: SidebarProps) {
  return (
    <aside
      className="sticky top-[var(--hc-header-height)] hidden lg:block shrink-0 border-r border-border/70 dark:bg-card"
      style={{
        width: 'var(--hc-sidebar-width)',
        height: 'calc(100vh - var(--hc-header-height))',
      }}
    >
      <ScrollArea className="h-full">
        <NavTree navigation={navigation} spaceSlug={spaceSlug} />
      </ScrollArea>
    </aside>
  )
}
