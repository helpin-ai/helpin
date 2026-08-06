import { useCallback, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import type { Editor } from '@tiptap/core'
import { Message01Icon } from '@/lib/icons'
import { QuickTooltip } from '@/components/ui/quick-tooltip'

interface BlockCommentTriggerProps {
  editor: Editor | null
  onComment: (anchor: { block_id: string }) => void
}

const BLOCK_SELECTOR = 'p, li, h1, h2, h3, h4, h5, h6, blockquote, pre, [data-block-id]'

/**
 * Right-side hover affordance that shows a "Comment" icon at the right edge
 * of the block under the cursor — but only when the cursor is in the right
 * half of the block (so it doesn't compete with the editor selection).
 */
export function BlockCommentTrigger({ editor, onComment }: BlockCommentTriggerProps) {
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null)
  const [host, setHost] = useState<HTMLElement | null>(null)
  const blockIdRef = useRef<string | null>(null)
  const hideTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    if (!editor || editor.isDestroyed) return
    const wrapper = editor.view.dom.closest('.docs-editor-wrapper') as HTMLElement | null
    setHost(wrapper)
  }, [editor])

  const closestBlock = useCallback(
    (target: Element | null): HTMLElement | null => {
      if (!editor || editor.isDestroyed || !target) return null
      const editorDOM = editor.view.dom
      let el: Element | null = target
      while (el && editorDOM.contains(el) && el !== editorDOM) {
        if (el instanceof HTMLElement && el.matches(BLOCK_SELECTOR) && el.dataset.blockId) {
          return el
        }
        el = el.parentElement
      }
      return null
    },
    [editor],
  )

  useEffect(() => {
    if (!editor || editor.isDestroyed) return
    const editorDOM = editor.view.dom
    const wrapper = editorDOM.closest('.docs-editor-wrapper') as HTMLElement | null
    if (!wrapper) return

    const cancelHide = () => {
      if (hideTimerRef.current) {
        clearTimeout(hideTimerRef.current)
        hideTimerRef.current = null
      }
    }

    const onMove = (event: MouseEvent) => {
      const target = document.elementFromPoint(event.clientX, event.clientY) as Element | null
      // If the cursor is over the trigger button itself, keep it visible.
      if (target?.closest('[data-block-comment-trigger]')) {
        cancelHide()
        return
      }
      const block = closestBlock(target)
      if (!block) {
        if (!hideTimerRef.current) {
          hideTimerRef.current = setTimeout(() => {
            setPos(null)
            blockIdRef.current = null
            hideTimerRef.current = null
          }, 200)
        }
        return
      }
      cancelHide()
      const rect = block.getBoundingClientRect()
      // Only show when the cursor is in the right half of the block, so we
      // don't fight the existing left-side block hover handle.
      const rightHalfStart = rect.left + rect.width * 0.5
      if (event.clientX < rightHalfStart) {
        setPos(null)
        blockIdRef.current = null
        return
      }
      const wrapperRect = wrapper.getBoundingClientRect()
      // Match the vertical offset the left-side drag-handle uses so the
      // comment trigger aligns with the + / ⋮⋮ icons on the same row.
      const compStyle = window.getComputedStyle(block)
      const parsedLineHeight = parseInt(compStyle.lineHeight, 10)
      const lineHeight = isNaN(parsedLineHeight)
        ? parseInt(compStyle.fontSize, 10) * 1.2
        : parsedLineHeight
      const paddingTop = parseInt(compStyle.paddingTop, 10) || 0
      const HANDLE_HEIGHT = 28 // matches package + our trigger size
      const topOffset = (lineHeight - HANDLE_HEIGHT) / 2 + paddingTop
      const top = rect.top - wrapperRect.top + wrapper.scrollTop + topOffset
      const left = rect.right - wrapperRect.left + wrapper.scrollLeft + 8
      setPos({ top, left })
      blockIdRef.current = block.dataset.blockId ?? null
    }

    const onLeave = () => {
      if (!hideTimerRef.current) {
        hideTimerRef.current = setTimeout(() => {
          setPos(null)
          blockIdRef.current = null
          hideTimerRef.current = null
        }, 200)
      }
    }

    wrapper.addEventListener('mousemove', onMove)
    wrapper.addEventListener('mouseleave', onLeave)
    return () => {
      wrapper.removeEventListener('mousemove', onMove)
      wrapper.removeEventListener('mouseleave', onLeave)
      cancelHide()
    }
  }, [editor, closestBlock])

  if (!editor || !editor.isEditable || !pos || !host) return null

  return createPortal(
    <QuickTooltip label="Comment on this block">
      <button
        type="button"
        data-block-comment-trigger
        onMouseEnter={() => {
          if (hideTimerRef.current) {
            clearTimeout(hideTimerRef.current)
            hideTimerRef.current = null
          }
        }}
        onClick={() => {
          const blockId = blockIdRef.current
          if (!blockId) return
          onComment({ block_id: blockId })
        }}
        className="absolute z-10 flex h-7 w-5 cursor-pointer items-center justify-center rounded text-muted-foreground/70 transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
        style={{ top: pos.top, left: pos.left }}
        aria-label="Comment on this block"
      >
        <Message01Icon className="h-4 w-4" />
      </button>
    </QuickTooltip>,
    host,
  )
}
