import { CSS, type Transform } from '@dnd-kit/utilities'
import { defaultAnimateLayoutChanges, type AnimateLayoutChanges } from '@dnd-kit/sortable'

const SHIFT_TRANSITION = 'transform 120ms cubic-bezier(0.2, 0, 0, 1)'
const CARD_DRAG_BLOCK_SELECTOR = [
  '[data-no-task-card-drag="true"]',
  'button',
  'a[href]',
  'input',
  'textarea',
  'select',
  'summary',
  '[contenteditable="true"]',
].join(',')

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
    transform: isDragging ? undefined : CSS.Transform.toString(transform),
    transition: isDragging ? undefined : (transition || SHIFT_TRANSITION),
  }
}

export function shouldIgnoreTaskCardDrag(target: EventTarget | null, currentTarget: EventTarget | null): boolean {
  if (!(target instanceof Element) || !(currentTarget instanceof Element)) {
    return false
  }
  if (!currentTarget.contains(target)) {
    return true
  }
  const blockedElement = target.closest(CARD_DRAG_BLOCK_SELECTOR)
  return !!blockedElement && blockedElement !== currentTarget
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
