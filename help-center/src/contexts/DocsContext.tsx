import { createContext, useContext, type ReactNode } from 'react'
import type { HelpCenterConfig, Space } from '@/lib/types'

interface DocsContextValue {
  subdomain: string
  config: HelpCenterConfig
  spaces: Space[]
}

const DocsContext = createContext<DocsContextValue | null>(null)

export function DocsProvider({
  children,
  subdomain,
  config,
  spaces,
}: DocsContextValue & { children: ReactNode }) {
  return (
    <DocsContext.Provider value={{ subdomain, config, spaces }}>
      {children}
    </DocsContext.Provider>
  )
}

export function useDocsContext() {
  const ctx = useContext(DocsContext)
  if (!ctx) throw new Error('useDocsContext must be used within DocsProvider')
  return ctx
}
