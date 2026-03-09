import { Outlet } from '@tanstack/react-router'
import { TopBar } from './TopBar'
import { Sidebar } from './Sidebar'
import type { HelpCenterConfig, NavItem } from '@/lib/types'

interface AppShellProps {
  config: HelpCenterConfig | undefined
  navigation: NavItem[]
}

export function AppShell({ config, navigation }: AppShellProps) {
  return (
    <div className="min-h-screen" style={{ backgroundColor: 'var(--hc-bg)' }}>
      <TopBar config={config} />
      <div className="flex">
        <Sidebar navigation={navigation} />
        <main className="flex-1 min-w-0">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
