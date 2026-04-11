import { FolderOpenIcon, PlusSignIcon } from '@/lib/icons'
import type { NodeView } from './nodeSelection'

export interface EmptyNodeStateProps {
  view: NodeView
  canEdit: boolean
  /**
   * When false, the "Add sub-collection" button is hidden. Used for
   * depth-2 collections (where nesting further would exceed the
   * 3-tier cap) and for the uncategorized view.
   */
  canAddChildCollection: boolean
  onCreateDocument: () => void
  onCreateChildCollection: () => void
}

/**
 * EmptyNodeState renders the main-area empty state when the current
 * node has no documents and no child collections to show. Copy and
 * CTAs adjust per view kind so users at the space root see "Create
 * your first collection" while leaf-collection visitors see
 * "Nothing here yet" with contextual create buttons.
 */
export function EmptyNodeState({
  view,
  canEdit,
  canAddChildCollection,
  onCreateDocument,
  onCreateChildCollection,
}: EmptyNodeStateProps) {
  const headline =
    view.kind === 'space_root'
      ? 'This space is empty'
      : view.kind === 'collection'
        ? 'Nothing here yet'
        : view.kind === 'uncategorized'
          ? 'No uncategorized documents'
          : ''

  const helperText =
    view.kind === 'space_root'
      ? 'Collections help you organize documents into groups — like topics, categories, or projects. Create your first collection to start adding documents.'
      : null

  const childCollectionLabel =
    view.kind === 'space_root' ? 'Add collection' : 'Add sub-collection'

  return (
    <div className="flex flex-col items-center justify-center gap-3 px-4 py-12 text-center max-w-md mx-auto">
      <FolderOpenIcon className="h-10 w-10 text-muted-foreground/30" />
      <p className="text-sm font-medium">{headline}</p>
      {helperText && (
        <p className="text-xs leading-relaxed text-muted-foreground">{helperText}</p>
      )}
      {canEdit && (
        <div className="mt-1 flex flex-wrap justify-center gap-2">
          <button
            type="button"
            onClick={onCreateDocument}
            className="inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-background px-3 py-1.5 text-sm font-medium text-foreground shadow-sm transition-colors hover:bg-muted"
          >
            <PlusSignIcon className="h-3.5 w-3.5" />
            Add document
          </button>
          {canAddChildCollection && (
            <button
              type="button"
              onClick={onCreateChildCollection}
              className="inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-background px-3 py-1.5 text-sm font-medium text-foreground shadow-sm transition-colors hover:bg-muted"
            >
              <PlusSignIcon className="h-3.5 w-3.5" />
              {childCollectionLabel}
            </button>
          )}
        </div>
      )}
    </div>
  )
}
