import { Drawer } from 'vaul'
import type { ReactNode } from 'react'
import { cn } from '@mobile/lib/cn'

export interface SheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  children: ReactNode
  detents?: number[]
  /** Fill the viewport and let the caller render its own app-style header. */
  fullScreen?: boolean
  className?: string
  /**
   * Accessible name for the sheet, read by screen readers only (visually
   * hidden). vaul's `Drawer.Content` wraps Radix's `Dialog.Content`, which
   * warns ("DialogContent requires a DialogTitle") when no title is
   * present — this satisfies that without changing the visual design.
   */
  title?: string
}

/** Thin wrapper around vaul's Drawer, styled to the app's bottom-sheet look. */
export function Sheet({
  open,
  onOpenChange,
  children,
  detents,
  fullScreen = false,
  className,
  title = 'Sheet',
}: SheetProps) {
  return (
    <Drawer.Root open={open} onOpenChange={onOpenChange} snapPoints={fullScreen ? undefined : detents}>
      <Drawer.Portal>
        <Drawer.Overlay className="fixed inset-0 z-40 bg-black/40" />
        <Drawer.Content
          className={cn(
            'fixed z-50 flex flex-col bg-background outline-none',
            fullScreen
              ? 'inset-0 h-dvh max-h-none rounded-none pb-[var(--safe-bottom)] pt-[var(--safe-top)]'
              : 'inset-x-0 bottom-0 max-h-[90vh] rounded-t-[20px] pb-[max(var(--safe-bottom),16px)]',
            className,
          )}
        >
          <Drawer.Title className="sr-only">{title}</Drawer.Title>
          {/* Radix also warns about a missing Description/aria-describedby once
              a Title is present; a visually-hidden one silences that too. */}
          <Drawer.Description className="sr-only">{title}</Drawer.Description>
          {!fullScreen && (
            <div className="flex shrink-0 justify-center py-2">
              <div className="h-[5px] w-9 rounded-full bg-muted-foreground/30" />
            </div>
          )}
          {children}
        </Drawer.Content>
      </Drawer.Portal>
    </Drawer.Root>
  )
}
