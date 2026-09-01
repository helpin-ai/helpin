import type { ComponentProps } from 'react'

import { cn } from '@/lib/utils'

export function SupportInboxPanelHeader({ className, ...props }: ComponentProps<'div'>) {
  return (
    <div
      data-slot="support-inbox-panel-header"
      className={cn(
        'relative z-10 flex h-11 shrink-0 items-center border-b border-border/60 bg-background dark:border-sidebar-border dark:bg-sidebar',
        className,
      )}
      {...props}
    />
  )
}
