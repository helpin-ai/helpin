import type { DocsSpace } from '@/lib/docsTypes'
import {
  ArrowRight01Icon,
  Delete01Icon,
  LanguageCircleIcon,
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

  // Space-level inline rename (only used at space_root)
  renamingSpace: boolean
  spaceNameValue: string
  onStartRenameSpace: () => void
  onSpaceNameChange: (value: string) => void
  onCommitRenameSpace: () => void
  onCancelRenameSpace: () => void

  // Space-level overflow actions (space_root only)
  onEditSpace: () => void
  onDeleteSpace: () => void
  onOpenTranslations: () => void

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
 *
 * External-capable spaces expose their translations action as a
 * first-class overflow item; all other translation-related dialogs
 * stay with the parent controller.
 */
export function SpaceNodeHeader({
  space,
  view,
  canEdit,
  onNavigateToCollection,
  onCreateDocument,
  onCreateChildCollection,
  renamingSpace,
  spaceNameValue,
  onStartRenameSpace,
  onSpaceNameChange,
  onCommitRenameSpace,
  onCancelRenameSpace,
  onEditSpace,
  onDeleteSpace,
  onOpenTranslations,
  onEditCollection,
  onDeleteCollection,
}: SpaceNodeHeaderProps) {
  const isExternalCapable = space.type === 'external_capable'

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
  const hasOverflowMenu =
    canEdit &&
    ((view.kind === 'space_root') ||
      (view.kind === 'collection' && (onEditCollection || onDeleteCollection)))

  return (
    <header className="flex flex-col gap-2">
      <div className="flex items-start justify-between gap-4">
        <div className="flex min-w-0 items-start gap-3">
          <div className="min-w-0">
            <nav
              aria-label="Breadcrumb"
              className="group flex min-w-0 items-center gap-2"
            >
              {ancestors.map((ancestor) => (
                <span key={ancestor.id} className="flex shrink-0 items-center gap-2">
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <button
                        type="button"
                        onClick={ancestor.onClick}
                        className="max-w-[200px] truncate rounded text-xl font-normal text-muted-foreground transition-colors hover:text-foreground"
                      >
                        {ancestor.label}
                      </button>
                    </TooltipTrigger>
                    <TooltipContent>{ancestor.label}</TooltipContent>
                  </Tooltip>
                  <ArrowRight01Icon
                    className="h-4 w-4 shrink-0 text-muted-foreground/40"
                    aria-hidden="true"
                  />
                </span>
              ))}
              {view.kind === 'space_root' && renamingSpace ? (
                <input
                  key="space-rename-input"
                  autoFocus
                  value={spaceNameValue}
                  onChange={(event) => onSpaceNameChange(event.target.value)}
                  onFocus={(event) => event.target.select()}
                  onBlur={onCommitRenameSpace}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') onCommitRenameSpace()
                    if (event.key === 'Escape') onCancelRenameSpace()
                  }}
                  className="min-w-0 border-b border-primary/40 bg-transparent px-0 py-0 text-xl font-semibold outline-none"
                />
              ) : (
                <h1 className="truncate text-xl font-semibold">{titleLabel}</h1>
              )}
              {view.kind === 'space_root' && canEdit && !renamingSpace && (
                <button
                  type="button"
                  onClick={onStartRenameSpace}
                  className="rounded p-1 text-muted-foreground/60 opacity-0 transition-opacity hover:bg-muted hover:text-foreground group-hover:opacity-100 focus:opacity-100"
                  aria-label="Rename space"
                >
                  <PencilEdit01Icon className="h-3.5 w-3.5" />
                </button>
              )}
            </nav>
            {description && (
              <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{description}</p>
            )}
          </div>
        </div>

        {canEdit && view.kind !== 'loading' && (
          <div className="flex shrink-0 items-center gap-2">
            {onCreateChildCollection && (
              <button
                type="button"
                onClick={onCreateChildCollection}
                className="inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-background px-2.5 py-1.5 text-xs font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
              >
                <PlusSignIcon className="h-3.5 w-3.5" />
                {view.kind === 'collection' ? 'Sub-collection' : 'Collection'}
              </button>
            )}
            <button
              type="button"
              onClick={onCreateDocument}
              className="inline-flex items-center gap-1.5 rounded-md bg-primary px-2.5 py-1.5 text-xs font-medium text-primary-foreground shadow-sm transition-colors hover:bg-primary/90"
            >
              <PlusSignIcon className="h-3.5 w-3.5" />
              Document
            </button>

            {hasOverflowMenu && (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <button
                    type="button"
                    className="flex h-8 w-8 items-center justify-center rounded-md transition-colors hover:bg-muted"
                    aria-label="More options"
                  >
                    <MoreHorizontalIcon className="h-4 w-4" />
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-48">
                  {view.kind === 'space_root' && (
                    <>
                      <DropdownMenuItem onClick={onEditSpace}>
                        <PencilEdit01Icon className="h-3.5 w-3.5" />
                        Edit space
                      </DropdownMenuItem>
                      {isExternalCapable && (
                        <DropdownMenuItem onClick={onOpenTranslations}>
                          <LanguageCircleIcon className="h-3.5 w-3.5" />
                          Translations
                        </DropdownMenuItem>
                      )}
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
