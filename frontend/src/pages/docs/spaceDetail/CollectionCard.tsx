import type { CollectionTreeNode } from '@/components/docs/docsCollectionTree'
import { StoredIcon } from '@/components/ui/icon-picker'
import {
  Delete01Icon,
  FolderOpenIcon,
  MoreHorizontalIcon,
  PencilEdit01Icon,
} from '@/lib/icons'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

export interface CollectionCardProps {
  node: CollectionTreeNode
  /** Recursive doc count across the collection's subtree. */
  docCount: number
  /** Direct child-collection count (immediate children only). */
  subCount: number
  canEdit: boolean
  onOpen: () => void
  onEdit: () => void
  onDelete: () => void
}

/**
 * CollectionCard is a clickable card summarising a collection node
 * in the main area of the space detail page. Body click drills into
 * the collection; the overflow menu exposes edit / delete actions
 * (permission-gated). Appears in the space root "Collections" grid
 * and the collection view's "Sub-collections" grid.
 */
export function CollectionCard({
  node,
  docCount,
  subCount,
  canEdit,
  onOpen,
  onEdit,
  onDelete,
}: CollectionCardProps) {
  const { collection } = node
  const docLabel = docCount === 1 ? '1 doc' : `${docCount} docs`
  const subLabel = subCount === 1 ? '1 sub-collection' : `${subCount} sub-collections`
  const ariaLabel =
    subCount > 0
      ? `Open ${collection.name}, ${docLabel}, ${subLabel}`
      : `Open ${collection.name}, ${docLabel}`

  return (
    <div className="group/card relative">
      <button
        type="button"
        onClick={onOpen}
        aria-label={ariaLabel}
        className="flex w-full items-start gap-3 rounded-lg border border-border/60 bg-card p-4 text-left transition-colors hover:border-border hover:bg-muted/40"
      >
        <StoredIcon
          name={collection.icon}
          className="h-5 w-5 shrink-0 text-muted-foreground"
          textClassName="text-lg"
          fallback={<FolderOpenIcon className="h-5 w-5 shrink-0 text-muted-foreground" />}
        />
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm font-medium">{collection.name}</div>
          {collection.description && (
            <p className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">
              {collection.description}
            </p>
          )}
          <div className="mt-2 flex gap-3 text-[11px] text-muted-foreground/80">
            <span>{docLabel}</span>
            {subCount > 0 && <span>{subLabel}</span>}
          </div>
        </div>
      </button>

      {canEdit && (
        <div className="absolute right-2 top-2 opacity-0 transition-opacity group-hover/card:opacity-100 group-focus-within/card:opacity-100">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                onClick={(event) => event.stopPropagation()}
                className="flex h-6 w-6 items-center justify-center rounded hover:bg-muted"
                aria-label={`More options for ${collection.name}`}
              >
                <MoreHorizontalIcon className="h-3.5 w-3.5 text-muted-foreground" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-40">
              <DropdownMenuItem onClick={onEdit}>
                <PencilEdit01Icon className="h-3.5 w-3.5" />
                Edit collection
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={onDelete}
                className="text-destructive focus:text-destructive"
              >
                <Delete01Icon className="h-3.5 w-3.5" />
                Delete collection
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      )}
    </div>
  )
}
