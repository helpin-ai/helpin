import { cn } from '@mobile/lib/cn'

export interface SkeletonProps {
  className?: string
}

/**
 * `bg-muted` block with a shimmering highlight sweeping across it every 1.4s.
 * The `skeleton-shimmer` keyframes are defined in `src/index.css` (kept there,
 * not inline, since Tailwind's `animate-[...]` arbitrary value needs a
 * `@keyframes` rule visible in the stylesheet to reference by name).
 */
export function Skeleton({ className }: SkeletonProps) {
  return (
    <div className={cn('relative isolate overflow-hidden rounded-md bg-muted', className)}>
      <div className="absolute inset-0 -translate-x-full animate-[skeleton-shimmer_1.4s_ease-in-out_infinite] bg-gradient-to-r from-transparent via-foreground/10 to-transparent" />
    </div>
  )
}
