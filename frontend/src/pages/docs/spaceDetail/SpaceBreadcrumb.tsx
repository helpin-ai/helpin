import type { ReactNode } from 'react'
import { ArrowRight01Icon } from '@/lib/icons'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

export interface SpaceBreadcrumbItem {
  id: string
  label: string
  icon?: ReactNode
  /** When omitted, the segment is rendered as static (non-clickable) text. */
  onClick?: () => void
}

export interface SpaceBreadcrumbProps {
  items: SpaceBreadcrumbItem[]
}

/**
 * SpaceBreadcrumb renders an ancestor chain above the page title on
 * the docs space detail page. Each clickable segment jumps the URL
 * back up the tree. Individual labels truncate with a tooltip when
 * they overflow. The max depth is Space ▸ L0 ▸ L1 ▸ L2, so four
 * segments is the worst case — no middle-ellipsis is needed.
 */
export function SpaceBreadcrumb({ items }: SpaceBreadcrumbProps) {
  if (items.length === 0) return null

  return (
    <nav
      aria-label="Breadcrumb"
      className="flex items-center gap-1 text-xs text-muted-foreground"
    >
      {items.map((item, index) => {
        const isLast = index === items.length - 1
        const body = (
          <span className="flex min-w-0 max-w-[180px] items-center gap-1">
            {item.icon}
            <span className="truncate">{item.label}</span>
          </span>
        )
        const segment =
          isLast || !item.onClick ? (
            <span
              className={isLast ? 'font-medium text-foreground' : undefined}
              aria-current={isLast ? 'page' : undefined}
            >
              {body}
            </span>
          ) : (
            <button
              type="button"
              onClick={item.onClick}
              className="rounded transition-colors hover:text-foreground"
            >
              {body}
            </button>
          )

        return (
          <span key={item.id} className="flex items-center gap-1">
            <Tooltip>
              <TooltipTrigger asChild>{segment}</TooltipTrigger>
              <TooltipContent>{item.label}</TooltipContent>
            </Tooltip>
            {!isLast && (
              <ArrowRight01Icon className="h-3 w-3 shrink-0 opacity-40" aria-hidden="true" />
            )}
          </span>
        )
      })}
    </nav>
  )
}
