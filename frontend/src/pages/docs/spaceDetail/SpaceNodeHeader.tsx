import type { DocsSpace } from '@/lib/docsTypes'
import { StoredIcon } from '@/components/ui/icon-picker'
import {
  ArrowLeft02Icon,
  ArrowRight01Icon,
  Delete01Icon,
  FolderOpenIcon,
  InboxIcon,
  MoreHorizontalIcon,
  PencilEdit01Icon,
  PlusSignIcon,
} from '@/lib/icons'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Button } from '@/components/ui/button'
import type { NodeView } from './nodeSelection'

interface AncestorLink {
  id: string
  label: string
  onClick: () => void
}

export interface SpaceNodeHeaderProps {
  space: DocsSpace
  view: NodeView
  canEdit: boolean

  // Navigation
  /** Passed the target collection id, or null to reset to space root. */
  onNavigateToCollection: (collectionId: string | null) => void

  // Create actions
  onCreateDocument: () => void
  /**
   * null when the current view cannot accept a new child collection
   * (uncategorized view, or a depth-2 collection that has already
   * reached the 3-tier cap).
   */
  onCreateChildCollection: (() => void) | null

  // Space-level overflow actions (space_root only)
  onEditSpace: () => void
  onDeleteSpace: () => void

  // Collection-level overflow actions (collection view only)
  onEditCollection: (() => void) | null
  onDeleteCollection: (() => void) | null
}

/**
 * SpaceNodeHeader is the top-of-page header for the docs space
 * detail. It renders the breadcrumb (when drilled), the title row
 * with the appropriate icon/name for the current node, a count
 * line, and the [+] / [⋯] action menus. All three view kinds
 * (space_root / collection / uncategorized) flow through this
 * single component so the layout stays consistent.
 */
export function SpaceNodeHeader({
  space,
  view,
  canEdit,
  onNavigateToCollection,
  onCreateDocument,
  onCreateChildCollection,
  onEditSpace,
  onDeleteSpace,
  onEditCollection,
  onDeleteCollection,
}: SpaceNodeHeaderProps) {
  // ── Ancestors (rendered inline, left of the title) ─────────────────
  // The current node is NOT in this list — it's the big title to the
  // right of the last arrow. That keeps the "you are here" signal
  // compact and matches the GitHub/Notion/Linear pattern.
  const ancestors: AncestorLink[] = (() => {
    if (view.kind === 'space_root' || view.kind === 'loading') return []
    const items: AncestorLink[] = [
      {
        id: 'space',
        label: space.name,
        onClick: () => onNavigateToCollection(null),
      },
    ]
    if (view.kind === 'collection') {
      for (const ancestor of view.ancestors) {
        items.push({
          id: ancestor.collection.id,
          label: ancestor.collection.name,
          onClick: () => onNavigateToCollection(ancestor.collection.id),
        })
      }
    }
    return items
  })()

  // ── Title content ────────────────────────────────────────────────
  // Icon renders on the current collection or uncategorized bucket
  // so admins see exactly what the public help center will display
  // next to this node. Spaces themselves have no icon-picker UI so
  // there's nothing meaningful to show — skip the fallback folder.
  const titleIcon =
    view.kind === 'collection' ? (
      <StoredIcon
        name={view.node.collection.icon}
        className="h-[18px] w-[18px] shrink-0 text-muted-foreground"
        textClassName="text-lg"
        fallback={<FolderOpenIcon className="h-[18px] w-[18px] shrink-0 text-muted-foreground" />}
      />
    ) : view.kind === 'uncategorized' ? (
      <InboxIcon className="h-[18px] w-[18px] shrink-0 text-muted-foreground" />
    ) : null

  const titleLabel =
    view.kind === 'collection'
      ? view.node.collection.name
      : view.kind === 'uncategorized'
        ? 'Uncategorized'
        : space.name

  const description =
    view.kind === 'collection'
      ? view.node.collection.description ?? null
      : view.kind === 'uncategorized'
        ? 'Documents not assigned to any collection.'
        : null

  // ── Overflow menu content ─────────────────────────────────────────
  // Translations are managed from Settings > Help Center > Languages & Translation.
  // Do not re-add a translations action here — see commit history for why this was removed.
  const hasOverflowMenu =
    canEdit &&
    ((view.kind === 'space_root') ||
      (view.kind === 'collection' && (onEditCollection || onDeleteCollection)))

  return (
    <header className="flex flex-col gap-2">
      <div className="flex flex-col items-stretch gap-3 sm:flex-row sm:items-start sm:justify-between sm:gap-4">
        <div className="flex min-w-0 items-start gap-3">
          <div className="min-w-0">
            <nav
              aria-label="Breadcrumb"
              className="group flex min-w-0 items-center gap-2"
            >
              {ancestors.length > 0 && (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  onClick={ancestors[ancestors.length - 1].onClick}
                  className="shrink-0 text-muted-foreground"
                  aria-label="Go back"
                >
                  <ArrowLeft02Icon className="h-4 w-4" />
                </Button>
              )}
              {ancestors.map((ancestor) => (
                <span key={ancestor.id} className="flex shrink-0 items-center gap-1.5">
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <button
                        type="button"
                        onClick={ancestor.onClick}
                        className="max-w-[200px] truncate rounded text-lg font-normal text-muted-foreground transition-colors hover:text-foreground"
                      >
                        {ancestor.label}
                      </button>
                    </TooltipTrigger>
                    <TooltipContent>{ancestor.label}</TooltipContent>
                  </Tooltip>
                  <ArrowRight01Icon
                    className="h-3.5 w-3.5 shrink-0 text-muted-foreground"
                    aria-hidden="true"
                  />
                </span>
              ))}
              {titleIcon}
              <h1 className="truncate text-lg font-medium">{titleLabel}</h1>
            </nav>
            {description && (
              <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{description}</p>
            )}
          </div>
        </div>

        {canEdit && view.kind !== 'loading' && (
          <div className="flex shrink-0 flex-wrap items-center justify-end gap-2 sm:flex-nowrap">
            {onCreateChildCollection && (
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={onCreateChildCollection}
                className="text-xs text-muted-foreground"
              >
                <PlusSignIcon className="h-3.5 w-3.5" />
                {view.kind === 'collection' ? 'Sub-collection' : 'Collection'}
              </Button>
            )}
            <Button
              type="button"
              size="sm"
              onClick={onCreateDocument}
              className="text-xs"
            >
              <PlusSignIcon className="h-3.5 w-3.5" />
              Document
            </Button>

            {hasOverflowMenu && (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    aria-label="More options"
                  >
                    <MoreHorizontalIcon className="h-4 w-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-48">
                  {view.kind === 'space_root' && (
                    <>
                      <DropdownMenuItem onClick={onEditSpace}>
                        <PencilEdit01Icon className="h-3.5 w-3.5" />
                        Edit space
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem
                        onClick={onDeleteSpace}
                        className="text-destructive focus:text-destructive"
                      >
                        <Delete01Icon className="h-3.5 w-3.5" />
                        Delete space
                      </DropdownMenuItem>
                    </>
                  )}
                  {view.kind === 'collection' && (
                    <>
                      {onEditCollection && (
                        <DropdownMenuItem onClick={onEditCollection}>
                          <PencilEdit01Icon className="h-3.5 w-3.5" />
                          Edit collection
                        </DropdownMenuItem>
                      )}
                      {onDeleteCollection && (
                        <>
                          {onEditCollection && <DropdownMenuSeparator />}
                          <DropdownMenuItem
                            onClick={onDeleteCollection}
                            className="text-destructive focus:text-destructive"
                          >
                            <Delete01Icon className="h-3.5 w-3.5" />
                            Delete collection
                          </DropdownMenuItem>
                        </>
                      )}
                    </>
                  )}
                </DropdownMenuContent>
              </DropdownMenu>
            )}
          </div>
        )}
      </div>
    </header>
  )
}
