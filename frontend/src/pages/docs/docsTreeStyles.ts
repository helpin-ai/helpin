/** Shared style classes and helpers for collection/article tree rendering
 *  across the normal docs view and arrange mode. */

export const COLLECTION_ROW_CLASS =
  'text-sm font-medium text-foreground'

export const ARTICLE_ROW_CLASS =
  'text-sm font-medium text-muted-foreground'

export const DOC_ICON_CLASS =
  'h-4 w-4 shrink-0 text-muted-foreground'

export const COLLECTION_ICON_CLASS =
  'h-3.5 w-3.5 shrink-0'

export const STATUS_BADGE_CLASS =
  'shrink-0 text-xs font-medium'

export const UPDATED_TEXT_CLASS =
  'shrink-0 text-[11px] text-muted-foreground'

export const COUNT_BADGE_CLASS =
  'rounded-full bg-muted px-1.5 text-[10px] tabular-nums text-muted-foreground'

export function statusColor(status: string): string {
  switch (status) {
    case 'published': return 'text-emerald-600 dark:text-emerald-400'
    case 'archived': return 'text-muted-foreground/60'
    default: return 'text-amber-600 dark:text-amber-400'
  }
}
