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
import { Button } from '@/components/ui/button'

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

  // Cards with zero docs dim slightly so non-empty areas are easier
  // to spot in a dense grid. Hover + focus reset the opacity so the
  // card still feels interactive.
  const isEmpty = docCount === 0 && subCount === 0

  return (
    <div className="group/card relative">
      <button
        type="button"
        onClick={onOpen}
        aria-label={ariaLabel}
        className={`flex w-full items-start gap-2.5 rounded-lg border border-border/60 bg-card p-3 text-left transition-all hover:border-border hover:bg-muted/40 hover:opacity-100 focus-visible:opacity-100 ${
          isEmpty ? 'opacity-70' : ''
        }`}
      >
        <StoredIcon
          name={collection.icon}
          className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground"
          textClassName="text-base"
          fallback={<FolderOpenIcon className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />}
        />
        <div className="min-w-0 flex-1 pr-5">
          <div className="line-clamp-2 text-sm font-medium leading-snug">{collection.name}</div>
          {collection.description && (
            <p className="mt-0.5 line-clamp-1 text-xs text-muted-foreground">
              {collection.description}
            </p>
          )}
          <div className="mt-1.5 flex gap-2 text-[11px] text-muted-foreground/80">
            <span>{isEmpty ? 'Empty' : docLabel}</span>
            {subCount > 0 && <span>· {subLabel}</span>}
          </div>
        </div>
      </button>

      {canEdit && (
        <div className="absolute right-2 top-2 opacity-0 transition-opacity group-hover/card:opacity-100 group-focus-within/card:opacity-100">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                onClick={(event) => event.stopPropagation()}
                aria-label={`More options for ${collection.name}`}
              >
                <MoreHorizontalIcon className="h-3.5 w-3.5 text-muted-foreground" />
              </Button>
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
