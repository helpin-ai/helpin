import type { ReactNode } from 'react'
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip'

interface QuickTooltipProps {
  label: string
  side?: 'top' | 'right' | 'bottom' | 'left'
  children: ReactNode
}

export function QuickTooltip({ label, side = 'top', children }: QuickTooltipProps) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>{children}</TooltipTrigger>
      <TooltipContent side={side}>{label}</TooltipContent>
    </Tooltip>
  )
}
