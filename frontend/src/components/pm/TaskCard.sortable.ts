import { CSS, type Transform } from '@dnd-kit/utilities'
import { defaultAnimateLayoutChanges, type AnimateLayoutChanges } from '@dnd-kit/sortable'

const SHIFT_TRANSITION = 'transform 200ms cubic-bezier(0.25, 1, 0.5, 1)'

export function getSortableTaskCardStyle({
  transform,
  transition,
  isDragging,
}: {
  transform: Transform | null
  transition?: string
  isDragging: boolean
}) {
  return {
    transform: CSS.Transform.toString(transform),
    // Dragged card follows the pointer instantly; sibling cards animate smoothly
    transition: isDragging ? undefined : (transition || SHIFT_TRANSITION),
  }
}

/**
 * Allow layout animations when items are added/removed from the sortable
 * context (cross-column drag previews), not just when reordered in-place.
 */
export const animateCardLayoutChanges: AnimateLayoutChanges = (args) => {
  const { isSorting, wasDragging } = args
  if (isSorting || wasDragging) {
    return defaultAnimateLayoutChanges(args)
  }
  return true
}
