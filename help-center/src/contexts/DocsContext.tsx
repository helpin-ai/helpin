import { createContext, useContext, type ReactNode } from 'react'
import type { HelpCenterConfig, Space } from '@/lib/types'

interface DocsContextValue {
  subdomain: string
  locale: string
  defaultLocale: string
  enabledLocales: string[]
  config: HelpCenterConfig
  spaces: Space[]
}

const DocsContext = createContext<DocsContextValue | null>(null)

export function DocsProvider({
  children,
  subdomain,
  locale,
  defaultLocale,
  enabledLocales,
  config,
  spaces,
}: DocsContextValue & { children: ReactNode }) {
  return (
    <DocsContext.Provider
      value={{ subdomain, locale, defaultLocale, enabledLocales, config, spaces }}
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
