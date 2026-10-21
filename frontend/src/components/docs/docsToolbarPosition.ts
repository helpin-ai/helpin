type Rect = { left: number; right: number; top: number; bottom: number }
type Size = { width: number; height: number }

/** Convert viewport selection coordinates to editor-local, visible placement. */
export function docsToolbarPosition(anchor: Rect, container: Rect, size: Size, viewport: Size, below = false) {
  const gap = 8
  const leftEdge = Math.max(0, container.left) + gap
  const rightEdge = Math.min(viewport.width, container.right) - gap
  const topEdge = Math.max(0, container.top) + gap
  const bottomEdge = Math.min(viewport.height, container.bottom) - gap
  const maxWidth = Math.max(0, rightEdge - leftEdge)
  const maxHeight = Math.max(0, bottomEdge - topEdge)
  const width = Math.min(size.width, maxWidth)
  const height = Math.min(size.height, maxHeight)
  const centered = (anchor.left + anchor.right - width) / 2
  const above = anchor.top - height - gap
  const preferredTop = below || above < topEdge ? anchor.bottom + gap : above
  return {
    left: Math.max(leftEdge, Math.min(centered, rightEdge - width)) - container.left,
    top: Math.max(topEdge, Math.min(preferredTop, bottomEdge - height)) - container.top,
    maxWidth,
    maxHeight,
  }
}
