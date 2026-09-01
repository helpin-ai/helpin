import type { ReactNode } from 'react'
import { QuietPageViewport } from '@/components/design-system/quiet'

export function DocsRouteViewport({ children }: { children: ReactNode }) {
  return <QuietPageViewport>{children}</QuietPageViewport>
}
