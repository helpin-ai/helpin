import { ArrowRight01Icon, InboxIcon } from '@/lib/icons'

export interface UncategorizedSectionProps {
  count: number
  onOpen: () => void
}

/**
 * UncategorizedSection appears at the bottom of the space root view
 * as a single-click entry point into the dedicated uncategorized
 * docs view. Only rendered when the count is greater than zero.
 */
export function UncategorizedSection({ count, onOpen }: UncategorizedSectionProps) {
  if (count === 0) return null
  const label = count === 1 ? '1 document' : `${count} documents`

  return (
    <section className="pt-4">
      <button
        type="button"
        onClick={onOpen}
        className="flex w-full items-center justify-between rounded-lg border border-dashed border-border/60 bg-muted/20 px-4 py-3 text-left transition-colors hover:border-border hover:bg-muted/40"
      >
        <span className="flex items-center gap-2">
          <InboxIcon className="h-4 w-4 text-muted-foreground" />
          <span className="text-sm font-medium">Uncategorized</span>
          <span className="text-xs text-muted-foreground">{label}</span>
        </span>
        <ArrowRight01Icon className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
      </button>
    </section>
  )
}
