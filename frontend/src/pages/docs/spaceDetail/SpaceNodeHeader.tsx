import type { DocsSpace } from '@/lib/docsTypes'
import { StoredIcon } from '@/components/ui/icon-picker'
import {
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
import type { NodeView } from './nodeSelection'
import {
  QuietBreadcrumbs,
  QuietDetailAction,
  QuietDetailHeader,
  QuietIconAction,
} from '@/components/design-system/quiet'

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
  onNavigateToSpaces: () => void

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
  onNavigateToSpaces,
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
  // compact and keeps the page hierarchy visible.
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

  const breadcrumbItems = [
    { id: 'all-docs', label: 'All Docs', onClick: onNavigateToSpaces },
    ...ancestors,
  ]

  return (
    <QuietDetailHeader
      breadcrumbs={(
        <QuietBreadcrumbs
          items={breadcrumbItems}
          onBack={breadcrumbItems[breadcrumbItems.length - 1].onClick}
          backLabel={breadcrumbItems.length === 1 ? 'Back to All Docs' : `Back to ${breadcrumbItems[breadcrumbItems.length - 1].label}`}
        />
      )}
      title={(
        <span className="flex min-w-0 items-center gap-2">
          {titleIcon}
          <span className="truncate">{titleLabel}</span>
        </span>
      )}
      meta={description ? <span className="block truncate text-[12.5px] text-quiet-text-tertiary">{description}</span> : null}
      actions={canEdit && view.kind !== 'loading' ? (
        <>
          {onCreateChildCollection && (
            <QuietDetailAction
              onClick={onCreateChildCollection}
              icon={<PlusSignIcon className="h-3.5 w-3.5" />}
              label={view.kind === 'collection' ? 'Sub-collection' : 'Collection'}
            />
          )}
          <QuietDetailAction
            tone="primary"
            onClick={onCreateDocument}
            icon={<PlusSignIcon className="h-3.5 w-3.5" />}
            label="Document"
          />

          {hasOverflowMenu && (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <QuietIconAction type="button" aria-label="More options">
                  <MoreHorizontalIcon className="h-[15px] w-[15px]" />
                </QuietIconAction>
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
        </>
      ) : null}
    />
  )
}
