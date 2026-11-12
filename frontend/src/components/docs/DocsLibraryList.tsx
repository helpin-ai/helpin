import type { ReactNode } from 'react'
import { cn, timeAgo } from '@/lib/utils'
import type { DocStatus } from '@/lib/docsTypes'

/** Marks drafts and archived documents next to a title; published shows nothing. */
export function DocsExceptionalStatus({ status }: { status: DocStatus }) {
  if (status === 'published') return null

  return (
    <span
      className={cn(
        'shrink-0 text-[11px] font-medium',
        status === 'draft'
          ? 'text-amber-600 dark:text-amber-400'
          : 'text-muted-foreground/70',
      )}
    >
      {status === 'draft' ? 'Draft' : 'Archived'}
    </span>
  )
}

export function DocsLibraryList({
  children,
  className,
  ariaLabel = 'Documents',
}: {
  children: ReactNode
  className?: string
  ariaLabel?: string
}) {
  return (
    <div
      role="list"
      aria-label={ariaLabel}
      data-slot="docs-library-list"
      className={cn('[&>[data-slot=docs-library-row]:last-child]:border-b-0', className)}
    >
      {children}
    </div>
  )
}

export function DocsLibraryRow({
  title,
  status,
  updatedAt,
  location,
  proposalBadge,
  onOpen,
  actions,
  selection,
  compact = false,
}: {
  title: string
  status: DocStatus
  updatedAt: string
  location?: string | null
  proposalBadge?: ReactNode
  onOpen: () => void
  actions?: ReactNode
  selection?: ReactNode
  compact?: boolean
}) {
  const updatedLabel = timeAgo(updatedAt)

  return (
    <div
      role="listitem"
      data-slot="docs-library-row"
      className={cn(
        'group/library-row flex items-stretch border-b border-border/45 transition-colors hover:bg-muted/35 focus-within:bg-muted/35',
        compact ? 'min-h-10' : 'min-h-[54px]',
      )}
    >
      {selection && <div className="flex shrink-0 items-center pl-3 pr-1">{selection}</div>}
      <button
        type="button"
        onClick={onOpen}
        className={cn(
          'flex min-w-0 flex-1 items-center gap-4 px-3 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring',
          compact ? 'py-2' : 'py-2.5',
        )}
      >
        <span className="min-w-0 flex-1">
          <span className="flex min-w-0 items-center gap-2">
            <span className={cn('truncate font-medium text-foreground', compact ? 'text-[13px]' : 'text-sm')}>
              {title || 'Untitled'}
            </span>
            <DocsExceptionalStatus status={status} />
            {proposalBadge}
          </span>
          {(location || updatedLabel) && (
            <span className="mt-0.5 flex min-w-0 items-center gap-1.5 text-[11px] text-muted-foreground/75">
              {location && <span className="truncate">{location}</span>}
              {location && <span className="sm:hidden" aria-hidden="true">·</span>}
              <span className="shrink-0 sm:hidden">Updated {updatedLabel}</span>
            </span>
          )}
        </span>
        <span
          className="hidden shrink-0 text-xs text-muted-foreground sm:block"
          title={new Date(updatedAt).toLocaleString()}
        >
          {updatedLabel}
        </span>
      </button>
      {actions && (
        <div className="flex w-9 shrink-0 items-center justify-center opacity-100 transition-opacity md:opacity-0 md:group-hover/library-row:opacity-100 md:group-focus-within/library-row:opacity-100">
          {actions}
        </div>
      )}
    </div>
  )
}
