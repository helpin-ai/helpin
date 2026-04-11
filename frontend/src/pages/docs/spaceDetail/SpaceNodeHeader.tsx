import type { DocsSpace } from '@/lib/docsTypes'
import { StoredIcon } from '@/components/ui/icon-picker'
import {
  ArrowLeft02Icon,
  Delete01Icon,
  Folder01Icon,
  FolderOpenIcon,
  GlobeIcon,
  InboxIcon,
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
import { QuickTooltip } from '@/components/ui/quick-tooltip'
import type { NodeView } from './nodeSelection'
import { SpaceBreadcrumb, type SpaceBreadcrumbItem } from './SpaceBreadcrumb'

export interface SpaceNodeHeaderProps {
  space: DocsSpace
  view: NodeView
  /** Pre-computed by the controller, respects the table's filter. */
  docCount: number
  /** Direct child-collection count of the current node. */
  subCount: number
  canEdit: boolean

  // Navigation
  onNavigateToDocs: () => void
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
  docCount,
  subCount,
  canEdit,
  onNavigateToDocs,
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

  // ── Breadcrumb ────────────────────────────────────────────────────
  const breadcrumbItems: SpaceBreadcrumbItem[] = (() => {
    if (view.kind === 'space_root' || view.kind === 'loading') return []
    const items: SpaceBreadcrumbItem[] = [
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
      items.push({
        id: view.node.collection.id,
        label: view.node.collection.name,
      })
    } else if (view.kind === 'uncategorized') {
      items.push({ id: 'uncategorized', label: 'Uncategorized' })
    }
    return items
  })()

  // ── Title content ────────────────────────────────────────────────
  const titleIcon =
    view.kind === 'collection' ? (
      <StoredIcon
        name={view.node.collection.icon}
        className="h-5 w-5 shrink-0 text-muted-foreground"
        textClassName="text-xl"
        fallback={<FolderOpenIcon className="h-5 w-5 shrink-0 text-muted-foreground" />}
      />
    ) : view.kind === 'uncategorized' ? (
      <InboxIcon className="h-5 w-5 shrink-0 text-muted-foreground" />
    ) : (
      <StoredIcon
        name={space.icon}
        className="h-5 w-5 shrink-0 text-muted-foreground"
        textClassName="text-xl"
        fallback={<Folder01Icon className="h-5 w-5 shrink-0 text-muted-foreground" />}
      />
    )

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

  // ── Count line ────────────────────────────────────────────────────
  // At space root the child buckets are top-level *collections*;
  // inside a collection they are *sub-collections*. Using the right
  // word at the right level matches the card-grid section heading.
  const docPart = docCount === 1 ? '1 document' : `${docCount} documents`
  const subPart = (() => {
    if (subCount === 0) return null
    if (view.kind === 'space_root') {
      return subCount === 1 ? '1 collection' : `${subCount} collections`
    }
    return subCount === 1 ? '1 sub-collection' : `${subCount} sub-collections`
  })()
  const countLine = subPart ? `${docPart} · ${subPart}` : docPart

  // ── Overflow menu content ─────────────────────────────────────────
  const hasOverflowMenu =
    canEdit &&
    ((view.kind === 'space_root') ||
      (view.kind === 'collection' && (onEditCollection || onDeleteCollection)))

  return (
    <header className="flex flex-col gap-2">
      {breadcrumbItems.length > 0 && <SpaceBreadcrumb items={breadcrumbItems} />}

      <div className="flex items-start justify-between gap-4">
        <div className="flex min-w-0 items-start gap-3">
          {view.kind === 'space_root' && (
            <button
              type="button"
              onClick={onNavigateToDocs}
              className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-md transition-colors hover:bg-muted"
              aria-label="Back to docs"
            >
              <ArrowLeft02Icon className="h-4 w-4" />
            </button>
          )}
          <div className="min-w-0">
            <div className="group flex items-center gap-2">
              {titleIcon}
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
              {view.kind === 'space_root' && isExternalCapable && (
                <QuickTooltip label="Published to public help center">
                  <GlobeIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
                </QuickTooltip>
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
            </div>
            {description && (
              <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{description}</p>
            )}
            <p className="mt-1 text-xs text-muted-foreground">{countLine}</p>
          </div>
        </div>

        {canEdit && view.kind !== 'loading' && (
          <div className="flex shrink-0 items-center gap-2">
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button
                  type="button"
                  className="inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-background px-2.5 py-1.5 text-xs font-medium transition-colors hover:bg-muted"
                >
                  <PlusSignIcon className="h-3.5 w-3.5" />
                  Add
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-48">
                <DropdownMenuItem onClick={onCreateDocument}>
                  <PlusSignIcon className="h-3.5 w-3.5" />
                  Document
                </DropdownMenuItem>
                {onCreateChildCollection && (
                  <DropdownMenuItem onClick={onCreateChildCollection}>
                    <PlusSignIcon className="h-3.5 w-3.5" />
                    {view.kind === 'collection' ? 'Sub-collection' : 'Collection'}
                  </DropdownMenuItem>
                )}
              </DropdownMenuContent>
            </DropdownMenu>

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
