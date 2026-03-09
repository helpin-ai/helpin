import { Outlet } from '@tanstack/react-router'
import { TopBar } from './TopBar'
import type { HelpCenterConfig, Space } from '@/lib/types'

interface AppShellProps {
  config: HelpCenterConfig | undefined
  spaces: Space[]
}

export function AppShell({ config, spaces }: AppShellProps) {
  return (
    <div className="min-h-screen" style={{ backgroundColor: 'var(--hc-bg)' }}>
      <TopBar config={config} spaces={spaces} />
      <Outlet />
    </div>
  )
}
