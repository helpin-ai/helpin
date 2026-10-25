import type { ReactNode } from 'react'
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip'

interface QuickTooltipProps {
  label: string
  side?: 'top' | 'right' | 'bottom' | 'left'
  contentClassName?: string
  children: ReactNode
}

export function QuickTooltip({ label, side = 'top', children, contentClassName }: QuickTooltipProps) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>{children}</TooltipTrigger>
      <TooltipContent side={side} className={contentClassName}>{label}</TooltipContent>
    </Tooltip>
  )
}
