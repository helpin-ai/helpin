import type { ReactNode } from 'react'
import { ArrowUpDownIcon, Tick01Icon } from '@/lib/icons'
import { cn, timeAgo } from '@/lib/utils'
import type { DocStatus } from '@/lib/docsTypes'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

export type DocsLibrarySortField = 'position' | 'updated_at' | 'title'
export type DocsLibrarySortDirection = 'asc' | 'desc'

export interface DocsLibrarySortValue {
  field: DocsLibrarySortField
  direction: DocsLibrarySortDirection
}

const SORT_OPTIONS: Array<DocsLibrarySortValue & { label: string }> = [
  { field: 'position', direction: 'asc', label: 'Manual order' },
  { field: 'updated_at', direction: 'desc', label: 'Recently updated' },
  { field: 'updated_at', direction: 'asc', label: 'Oldest updated' },
  { field: 'title', direction: 'asc', label: 'Title A–Z' },
  { field: 'title', direction: 'desc', label: 'Title Z–A' },
]

function sortOptionKey(value: DocsLibrarySortValue) {
  return `${value.field}:${value.direction}`
}

export function DocsLibrarySortMenu({
  value,
  onChange,
  includeManualOrder = false,
}: {
  value: DocsLibrarySortValue
  onChange: (value: DocsLibrarySortValue) => void
  includeManualOrder?: boolean
}) {
  const options = includeManualOrder
    ? SORT_OPTIONS
    : SORT_OPTIONS.filter((option) => option.field !== 'position')
  const selected = options.find((option) => sortOptionKey(option) === sortOptionKey(value)) ?? options[0]

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="text-xs text-muted-foreground"
          aria-label={`Sort documents: ${selected.label}`}
        >
          <ArrowUpDownIcon className="h-3.5 w-3.5" />
          <span className="hidden sm:inline">{selected.label}</span>
          <span className="sm:hidden">Sort</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        {options.map((option) => {
          const active = sortOptionKey(option) === sortOptionKey(selected)
          return (
            <DropdownMenuItem
              key={sortOptionKey(option)}
              onClick={() => onChange({ field: option.field, direction: option.direction })}
              className={active ? 'font-medium' : ''}
            >
              {option.label}
              {active && <Tick01Icon className="ml-auto h-3.5 w-3.5" />}
            </DropdownMenuItem>
          )
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function ExceptionalStatus({ status }: { status: DocStatus }) {
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
  compact = false,
}: {
  title: string
  status: DocStatus
  updatedAt: string
  location?: string | null
  proposalBadge?: ReactNode
  onOpen: () => void
  actions?: ReactNode
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
            <ExceptionalStatus status={status} />
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
