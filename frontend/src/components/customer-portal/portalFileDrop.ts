import { useCallback, useRef, useState, type ClipboardEvent, type DragEvent } from 'react'

/** pastedFiles returns files from a paste; the caller keeps the pasted text otherwise. */
export function pastedFiles(event: ClipboardEvent) {
  return Array.from(event.clipboardData?.files ?? [])
}

/**
 * usePortalFileDrop makes an element accept dropped files. `dragging` drives
 * the visible drop cue; the attach button remains the keyboard path.
 */
export function usePortalFileDrop(addFiles: (files: File[]) => void, enabled: boolean) {
  const [dragging, setDragging] = useState(false)
  const depth = useRef(0)
  const hasFiles = (event: DragEvent) => Array.from(event.dataTransfer?.types ?? []).includes('Files')

  const onDragEnter = useCallback((event: DragEvent) => {
    if (!enabled || !hasFiles(event)) return
    event.preventDefault()
    depth.current += 1
    setDragging(true)
  }, [enabled])
  const onDragOver = useCallback((event: DragEvent) => {
    if (!enabled || !hasFiles(event)) return
    event.preventDefault()
    event.dataTransfer.dropEffect = 'copy'
  }, [enabled])
  const onDragLeave = useCallback((event: DragEvent) => {
    if (!enabled || !hasFiles(event)) return
    depth.current = Math.max(0, depth.current - 1)
    if (depth.current === 0) setDragging(false)
  }, [enabled])
  const onDrop = useCallback((event: DragEvent) => {
    if (!enabled || !hasFiles(event)) return
    event.preventDefault()
    depth.current = 0
    setDragging(false)
    addFiles(Array.from(event.dataTransfer.files))
  }, [addFiles, enabled])

  return { dragging, dropProps: { onDragEnter, onDragOver, onDragLeave, onDrop } }
}
