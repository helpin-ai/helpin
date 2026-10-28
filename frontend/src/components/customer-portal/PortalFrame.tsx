import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

export type PortalFrameWidth = 'prose' | 'form' | 'wide'

/**
 * PortalFrame is a public customer page: a hairline header with the
 * workspace name and account actions, then one centered column at the
 * Quiet prose measure. It uses no backdrop card; these pages live outside
 * the authenticated app shell.
 */
export function PortalFrame({
  name,
  logoUrl,
  color,
  account,
  width = 'prose',
  children,
}: {
  name: string
  logoUrl?: string | null
  color?: string | null
  account?: ReactNode
  width?: PortalFrameWidth
  children: ReactNode
}) {
  // The request table uses the wide column; threads and forms keep a reading measure.
  const columnClassName = width === 'form' ? 'max-w-[480px]' : width === 'wide' ? 'max-w-[1080px]' : 'max-w-[760px]'
  return (
    <div className="min-h-svh bg-background text-quiet-text-primary">
      <header className="border-b border-quiet-divider-strong">
        <div className={cn('mx-auto flex h-14 w-full items-center justify-between gap-4 px-4 sm:px-6', columnClassName)}>
          <div className="flex min-w-0 items-center gap-2.5">
            <PortalBrandMark name={name} logoUrl={logoUrl} color={color} />
            <span className="truncate text-sm font-semibold tracking-[-0.008em]">{name}</span>
          </div>
          {account}
        </div>
      </header>
      <main className={cn('mx-auto w-full px-4 pb-20 pt-8 sm:px-6 sm:pt-10', columnClassName)}>
        {children}
      </main>
    </div>
  )
}

/** PortalBrandMark shows the workspace logo, or its initial in the brand colour. */
export function PortalBrandMark({ name, logoUrl, color }: { name: string; logoUrl?: string | null; color?: string | null }) {
  if (logoUrl) return <img src={logoUrl} alt="" className="size-7 shrink-0 rounded-md object-contain" />
  return (
    <span
      aria-hidden="true"
      className="flex size-7 shrink-0 items-center justify-center rounded-md bg-quiet-action text-[12px] font-semibold text-quiet-action-ink"
      style={color ? { backgroundColor: color, color: '#fff' } : undefined}
    >
      {name.trim().charAt(0).toUpperCase() || 'S'}
    </span>
  )
}
