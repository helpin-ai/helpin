import { ChevronLeft } from 'lucide-react'
import { cn } from '@mobile/lib/cn'
import { Skeleton } from '@mobile/ui/skeleton'

/**
 * `pendingComponent` for the lazy-loaded conversation route (Task 22 code
 * split, `src/router.tsx`). Shown while the `conversation-screen` chunk is
 * still downloading — without it, TanStack Router's Match wraps the route in
 * a `SafeFragment` (no Suspense boundary of its own), the lazy component's
 * thrown load promise bubbles to the root, and the ScreenStack slides in a
 * BLANK panel. Worst case is a cold-start push-tap deep link straight into a
 * conversation, where `defaultPreload: 'intent'` never had a chance to
 * prefetch the chunk.
 *
 * Deliberately standalone: this must live in the ENTRY chunk, so it cannot
 * import anything from `src/thread/` (e.g. MessageList's ThreadSkeleton) —
 * a static import from there would drag the thread code back into the
 * initial bundle and undo the split. It instead mirrors that skeleton's
 * shape (3 alternating-width bubbles) plus a TopBar-shaped header with only
 * entry-chunk primitives.
 */
export function ConversationPending() {
  const rows: Array<{ width: string; align: 'justify-start' | 'justify-end' }> = [
    { width: 'w-40', align: 'justify-start' },
    { width: 'w-56', align: 'justify-end' },
    { width: 'w-32', align: 'justify-start' },
  ]
  return (
    <div data-testid="conversation-pending" className="flex h-dvh flex-col bg-background">
      {/* TopBar-shaped header: safe-top padding + 52px row, matching ui/top-bar.tsx's metrics. */}
      <div className="sticky top-0 z-30 bg-background/95 backdrop-blur-sm">
        <div className="pt-[var(--safe-top)]">
          <div className="relative flex h-[52px] items-center px-2">
            <div className="z-10 flex min-w-[44px] items-center justify-center text-muted-foreground">
              <ChevronLeft className="h-6 w-6" aria-hidden />
            </div>
            <div className="pointer-events-none absolute inset-x-12 flex flex-col items-center">
              <Skeleton className="h-4 w-32" />
            </div>
          </div>
        </div>
      </div>

      <div className="flex min-h-0 flex-1 flex-col justify-end gap-3 px-3 pb-4">
        {rows.map((row, index) => (
          <div key={index} className={cn('flex', row.align)}>
            <Skeleton className={cn('h-9 rounded-[16px]', row.width)} />
          </div>
        ))}
      </div>
    </div>
  )
}
