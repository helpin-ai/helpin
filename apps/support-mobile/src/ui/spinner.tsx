import { cn } from '@mobile/lib/cn'

export interface SpinnerProps {
  size?: number
  className?: string
}

export function Spinner({ size = 20, className }: SpinnerProps) {
  return (
    <span
      role="status"
      aria-label="Loading"
      className={cn(
        'inline-block animate-spin rounded-full border-2 border-current border-t-transparent text-muted-foreground',
        className,
      )}
      style={{ width: size, height: size }}
    />
  )
}
