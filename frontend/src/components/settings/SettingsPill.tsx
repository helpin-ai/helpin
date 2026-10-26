import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

/** A toggleable choice in a settings form, shown as a pill. */
export function SettingsPill({ selected, onClick, children }: { selected: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      aria-pressed={selected}
      onClick={onClick}
      className={cn(
        'rounded-full border px-2.5 py-1 text-xs transition-colors focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary',
        selected ? 'border-foreground bg-foreground text-background' : 'border-border text-muted-foreground hover:bg-muted',
      )}
    >
      {children}
    </button>
  )
}
