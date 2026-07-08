import { Drawer } from 'vaul'
import type { ReactNode } from 'react'
import { cn } from '@mobile/lib/cn'

export interface SheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  children: ReactNode
  detents?: number[]
  className?: string
}

/** Thin wrapper around vaul's Drawer, styled to the app's bottom-sheet look. */
export function Sheet({ open, onOpenChange, children, detents, className }: SheetProps) {
  return (
    <Drawer.Root open={open} onOpenChange={onOpenChange} snapPoints={detents}>
      <Drawer.Portal>
        <Drawer.Overlay className="fixed inset-0 z-40 bg-black/40" />
        <Drawer.Content
          className={cn(
            'fixed inset-x-0 bottom-0 z-50 flex max-h-[90vh] flex-col rounded-t-[20px] bg-background pb-[max(var(--safe-bottom),16px)] outline-none',
            className,
          )}
        >
          <div className="flex shrink-0 justify-center py-2">
            <div className="h-[5px] w-9 rounded-full bg-muted-foreground/30" />
          </div>
          {children}
        </Drawer.Content>
      </Drawer.Portal>
    </Drawer.Root>
  )
}
