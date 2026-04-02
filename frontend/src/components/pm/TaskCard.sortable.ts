import { CSS, type Transform } from '@dnd-kit/utilities'

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
    transition: isDragging ? undefined : transition,
  }
}
