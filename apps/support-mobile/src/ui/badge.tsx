import type { ReactNode } from 'react'
import { cn } from '@mobile/lib/cn'

export type BadgeTone = 'neutral' | 'primary' | 'warning' | 'success' | 'destructive'

export interface BadgeProps {
  tone: BadgeTone
  children: ReactNode
  className?: string
}

const toneClasses: Record<BadgeTone, string> = {
  neutral: 'bg-muted text-muted-foreground',
  primary: 'bg-primary/10 text-primary',
  warning: 'bg-amber-500/15 text-amber-600 dark:text-amber-400',
  success: 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400',
  destructive: 'bg-destructive/15 text-destructive',
}

export function Badge({ tone, children, className }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-caption',
        toneClasses[tone],
        className,
      )}
    >
      {children}
    </span>
  )
}
