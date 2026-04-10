import { createContext, useContext, type ReactNode } from 'react'
import type { HelpCenterConfig, Space } from '@/lib/types'

interface DocsContextValue {
  basepath: string
  subdomain: string
  locale: string
  defaultLocale: string
  enabledLocales: string[]
  multilingualEnabled: boolean
  config: HelpCenterConfig
  spaces: Space[]
}

const DocsContext = createContext<DocsContextValue | null>(null)

export function DocsProvider({
  children,
  basepath,
  subdomain,
  locale,
  defaultLocale,
  enabledLocales,
  multilingualEnabled,
  config,
  spaces,
}: DocsContextValue & { children: ReactNode }) {
  return (
    <DocsContext.Provider
      value={{
        basepath,
        subdomain,
        locale,
        defaultLocale,
        enabledLocales,
        multilingualEnabled,
        config,
        spaces,
      }}
    >
      {children}
    </DocsContext.Provider>
  )
}

export function useDocsContext() {
  const ctx = useContext(DocsContext)
  if (!ctx) throw new Error('useDocsContext must be used within DocsProvider')
  return ctx
}
