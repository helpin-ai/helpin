import type { ReactNode } from 'react'
import { cn } from '@mobile/lib/cn'

export interface EmptyStateProps {
  icon: ReactNode
  title: string
  body?: string
  action?: ReactNode
  className?: string
}

export function EmptyState({ icon, title, body, action, className }: EmptyStateProps) {
  return (
    <div className={cn('flex flex-col items-center justify-center gap-3 px-6 py-12 text-center', className)}>
      <div className="flex h-14 w-14 items-center justify-center rounded-full bg-muted text-muted-foreground">
        {icon}
      </div>
      <div className="flex flex-col gap-1">
        <p className="text-headline">{title}</p>
        {body && <p className="text-footnote text-muted-foreground">{body}</p>}
      </div>
      {action && <div className="mt-2">{action}</div>}
    </div>
  )
}
