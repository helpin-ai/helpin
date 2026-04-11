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
 * SpaceBreadcrumb renders the ancestor chain above the page title.
 * The current node is NOT part of the items — it is already the
 * page title below, so including it would double up the name.
 * Every segment is clickable and navigates up the tree. Labels
 * truncate with a tooltip when they overflow. Max chain in
 * practice is three ancestors (Space ▸ L0 ▸ L1) above a depth-2
 * collection view.
 */
export function SpaceBreadcrumb({ items }: SpaceBreadcrumbProps) {
  if (items.length === 0) return null

  return (
    <nav
      aria-label="Breadcrumb"
      className="flex items-center gap-1 text-sm text-muted-foreground"
    >
      {items.map((item, index) => {
        const isLast = index === items.length - 1
        const body = (
          <span className="flex min-w-0 max-w-[180px] items-center gap-1">
            {item.icon}
            <span className="truncate">{item.label}</span>
          </span>
        )
        const segment = item.onClick ? (
          <button
            type="button"
            onClick={item.onClick}
            className="rounded transition-colors hover:text-foreground"
          >
            {body}
          </button>
        ) : (
          <span>{body}</span>
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
