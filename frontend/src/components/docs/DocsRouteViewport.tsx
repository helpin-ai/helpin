import type { ReactNode } from 'react'

export function DocsRouteViewport({ children }: { children: ReactNode }) {
  return (
    <div className="h-full overflow-auto p-4 pb-20 [scrollbar-gutter:stable] md:p-6 md:pb-24">
      <div className="mx-auto max-w-7xl">
        {children}
      </div>
    </div>
  )
}
