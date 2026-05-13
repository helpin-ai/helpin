import { useCallback, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import type { Editor } from '@tiptap/core'
import { toast } from 'sonner'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { PlusSignIcon, Copy01Icon, Delete01Icon } from '@/lib/icons'
import {
  TURN_INTO_OPTIONS,
  copyBlockLink,
  deleteBlock,
  duplicateBlock,
  insertParagraphAndOpenSlash,
  turnInto,
} from './blockActions'

interface BlockHoverHandleProps {
  editor: Editor
}

function GripIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 16 16"
      fill="none"
      stroke="none"
      className={className}
      aria-hidden="true"
    >
      <circle cx="6" cy="4" r="1.25" fill="currentColor" />
      <circle cx="10" cy="4" r="1.25" fill="currentColor" />
      <circle cx="6" cy="8" r="1.25" fill="currentColor" />
      <circle cx="10" cy="8" r="1.25" fill="currentColor" />
      <circle cx="6" cy="12" r="1.25" fill="currentColor" />
      <circle cx="10" cy="12" r="1.25" fill="currentColor" />
    </svg>
  )
}

export function BlockHoverHandle({ editor }: BlockHoverHandleProps) {
  // The tiptap-extension-global-drag-handle plugin creates a `.drag-handle`
  // div inside `editor.view.dom.parentElement` and toggles its position +
  // visibility via inline styles. We portal our React UI into that div.
  const [host, setHost] = useState<HTMLElement | null>(null)
  const blockPosRef = useRef<number | null>(null)
  const [menuOpen, setMenuOpen] = useState(false)

  useEffect(() => {
    const dom = editor.view?.dom
    const parent = dom?.parentElement
    if (!dom || !parent) return
    const findHandle = () =>
      parent.querySelector<HTMLElement>(':scope > .drag-handle')

    // Pin the handle to a fixed left position aligned with the editor's
    // left edge so it doesn't slide horizontally per-block (lists, quotes,
    // headings can otherwise have different left edges).
    const HANDLE_GAP = 8
    const HANDLE_WIDTH = 40
    const pinLeft = (el: HTMLElement) => {
      const editorRect = editor.view.dom.getBoundingClientRect()
      const desired = `${Math.round(editorRect.left - HANDLE_WIDTH - HANDLE_GAP)}px`
      if (el.style.left !== desired) el.style.left = desired
    }

    const setupHost = (el: HTMLElement) => {
      el.style.display = 'flex'
      el.style.alignItems = 'center'
      el.style.gap = '0px'
      pinLeft(el)
      // Watch the package's style updates and override left after each.
      const styleObserver = new MutationObserver(() => pinLeft(el))
      styleObserver.observe(el, { attributes: true, attributeFilter: ['style'] })
      // The package hides the handle when the mouse leaves the editor's
      // parent, but since our handle sits OUTSIDE that parent (fixed left,
      // pinned to the editor's left margin), leaving the handle to the left
      // doesn't fire mouseout on the editor parent. Hide manually here, with
      // a small delay so the handle doesn't disappear if the cursor briefly
      // exits and returns.
      let hideTimer: ReturnType<typeof setTimeout> | null = null
      const cancelHide = () => {
        if (hideTimer) {
          clearTimeout(hideTimer)
          hideTimer = null
        }
      }
      const onLeave = (event: MouseEvent) => {
        const related = event.relatedTarget as Element | null
        const stillInEditor =
          related?.classList?.contains('tiptap') ||
          related?.closest?.('.tiptap') ||
          related?.closest?.('.drag-handle')
        if (stillInEditor) return
        cancelHide()
        hideTimer = setTimeout(() => {
          el.classList.add('hide')
          hideTimer = null
        }, 350)
      }
      const onEnter = () => cancelHide()
      el.addEventListener('mouseleave', onLeave)
      el.addEventListener('mouseenter', onEnter)

      // The package only listens to the legacy `mousewheel` event to hide
      // on scroll, which doesn't fire for trackpads, scrollbars, or modern
      // wheel events. Hide on every scroll inside the editor (and on the
      // window) so the handle doesn't linger at a stale position.
      const hide = () => el.classList.add('hide')
      const wrapper = editor.view.dom.closest('.docs-editor-wrapper')
      const scrollTargets: (HTMLElement | Window)[] = [window]
      if (wrapper instanceof HTMLElement) scrollTargets.push(wrapper)
      const scrollOpts: AddEventListenerOptions = { passive: true, capture: true }
      scrollTargets.forEach((t) => t.addEventListener('scroll', hide, scrollOpts))

      // When the cursor is over a parent list item that wraps a nested list,
      // the package can briefly anchor to the parent's full bounding box
      // (which includes the children gap area). Hide the handle when the
      // cursor is in the parent's nested-list area but not on a child item,
      // so the parent doesn't flash while moving between children.
      const editorDOM = editor.view.dom
      const onEditorMove = (event: MouseEvent) => {
        const target = document.elementFromPoint(event.clientX, event.clientY)
        if (!(target instanceof Element) || !editorDOM.contains(target)) return
        // If we're directly inside a leaf li (no nested list), nothing to do.
        const directLi = target.closest('li')
        if (!directLi) return
        const isParentLi =
          directLi.querySelector(':scope > ul') ||
          directLi.querySelector(':scope > ol')
        if (!isParentLi) return
        // We're inside a parent li. If the cursor is within the nested
        // list's vertical range AND not on a leaf child li, the package
        // would anchor to the parent — hide instead.
        const nested = directLi.querySelector(':scope > ul, :scope > ol') as HTMLElement | null
        if (!nested) return
        const nestedRect = nested.getBoundingClientRect()
        const inNestedRange =
          event.clientY >= nestedRect.top && event.clientY <= nestedRect.bottom
        if (!inNestedRange) return
        // Cursor is in the nested-list area. Make sure we're on an actual
        // leaf child li; otherwise hide.
        const innerLi = target.closest('li')
        const isLeafChild =
          innerLi &&
          innerLi !== directLi &&
          !innerLi.querySelector(':scope > ul') &&
          !innerLi.querySelector(':scope > ol')
        if (!isLeafChild) {
          el.classList.add('hide')
        }
      }
      editorDOM.addEventListener('mousemove', onEditorMove)

      return {
        styleObserver,
        cleanup: () => {
          cancelHide()
          el.removeEventListener('mouseleave', onLeave)
          el.removeEventListener('mouseenter', onEnter)
          editorDOM.removeEventListener('mousemove', onEditorMove)
          scrollTargets.forEach((t) => t.removeEventListener('scroll', hide, scrollOpts))
        },
      }
    }

    const existing = findHandle()
    if (existing) {
      const { styleObserver, cleanup } = setupHost(existing)
      setHost(existing)
      return () => {
        styleObserver.disconnect()
        cleanup()
      }
    }

    let setup: { styleObserver: MutationObserver; cleanup: () => void } | null = null
    const observer = new MutationObserver(() => {
      const next = findHandle()
      if (next) {
        setup = setupHost(next)
        setHost(next)
        observer.disconnect()
      }
    })
    observer.observe(parent, { childList: true })
    return () => {
      observer.disconnect()
      setup?.styleObserver.disconnect()
      setup?.cleanup()
    }
  }, [editor])

  // Resolve the block under the handle's current screen position.
  const resolveHoveredPos = useCallback((): number | null => {
    if (!host) return null
    const rect = host.getBoundingClientRect()
    const probeX = rect.right + 50
    const probeY = rect.top + rect.height / 2
    const view = editor.view
    const result = view.posAtCoords({ left: probeX, top: probeY })
    if (!result) return null
    const $pos = view.state.doc.resolve(result.inside >= 0 ? result.inside : result.pos)
    if ($pos.depth === 0) return $pos.pos
    return $pos.before(1)
  }, [editor, host])

  // Track the hovered block continuously.
  useEffect(() => {
    if (!host) return
    const dom = editor.view.dom
    const onMove = () => {
      blockPosRef.current = resolveHoveredPos()
    }
    dom.addEventListener('mousemove', onMove)
    return () => dom.removeEventListener('mousemove', onMove)
  }, [editor, host, resolveHoveredPos])

  const handlePlus = useCallback(() => {
    const pos = blockPosRef.current ?? resolveHoveredPos()
    if (pos == null) return
    insertParagraphAndOpenSlash(editor, pos)
  }, [editor, resolveHoveredPos])

  const onTurnInto = useCallback((kind: typeof TURN_INTO_OPTIONS[number]['kind']) => {
    const pos = blockPosRef.current ?? resolveHoveredPos()
    if (pos == null) return
    turnInto(editor, pos, kind)
    setMenuOpen(false)
  }, [editor, resolveHoveredPos])

  const onCopyLink = useCallback(async () => {
    const pos = blockPosRef.current ?? resolveHoveredPos()
    if (pos == null) return
    const ok = await copyBlockLink(editor, pos)
    setMenuOpen(false)
    toast[ok ? 'success' : 'error'](ok ? 'Block link copied' : 'No link available for this block')
  }, [editor, resolveHoveredPos])

  const onDuplicate = useCallback(() => {
    const pos = blockPosRef.current ?? resolveHoveredPos()
    if (pos == null) return
    duplicateBlock(editor, pos)
    setMenuOpen(false)
  }, [editor, resolveHoveredPos])

  const onDelete = useCallback(() => {
    const pos = blockPosRef.current ?? resolveHoveredPos()
    if (pos == null) return
    deleteBlock(editor, pos)
    setMenuOpen(false)
  }, [editor, resolveHoveredPos])

  if (!editor.isEditable) return null
  if (!host) return null

  const ui = (
    <>
      <button
        type="button"
        aria-label="Insert block below"
        title="Click to add a block below"
        // The + must NOT initiate a block drag.
        draggable={false}
        onMouseDown={(e) => {
          e.preventDefault()
          e.stopPropagation()
        }}
        onClick={handlePlus}
        className="flex h-7 w-5 items-center justify-center rounded text-muted-foreground/70 hover:bg-muted hover:text-foreground transition-colors cursor-pointer"
      >
        <PlusSignIcon className="h-4 w-4" />
      </button>
      <button
        type="button"
        aria-label="Block actions (drag to reorder)"
        title="Drag to reorder, click for actions"
        // Let drag start; only open menu on a real click.
        onClick={(e) => {
          e.stopPropagation()
          setMenuOpen(true)
        }}
        className="flex h-7 w-5 cursor-grab items-center justify-center rounded text-muted-foreground/70 hover:bg-muted hover:text-foreground transition-colors active:cursor-grabbing"
      >
        <GripIcon className="h-4 w-4" />
      </button>
      <DropdownMenu open={menuOpen} onOpenChange={setMenuOpen}>
        {/* Hidden anchor; the grip button is a regular button so drag events
           bubble to the package's listeners on the .drag-handle parent. */}
        <DropdownMenuTrigger asChild>
          <span aria-hidden="true" style={{ position: 'absolute', width: 0, height: 0, opacity: 0, pointerEvents: 'none' }} />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" side="bottom" className="w-48">
          <DropdownMenuSub>
            <DropdownMenuSubTrigger>Turn into</DropdownMenuSubTrigger>
            <DropdownMenuSubContent className="w-44">
              {TURN_INTO_OPTIONS.map((opt) => (
                <DropdownMenuItem key={opt.kind} onSelect={() => onTurnInto(opt.kind)}>
                  {opt.label}
                </DropdownMenuItem>
              ))}
            </DropdownMenuSubContent>
          </DropdownMenuSub>
          <DropdownMenuSeparator />
          <DropdownMenuItem onSelect={() => void onCopyLink()}>
            <Copy01Icon className="h-3.5 w-3.5" />
            Copy link to block
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={onDuplicate}>
            <Copy01Icon className="h-3.5 w-3.5" />
            Duplicate
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onSelect={onDelete}
            className="text-destructive focus:text-destructive"
          >
            <Delete01Icon className="h-3.5 w-3.5" />
            Delete
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </>
  )

  return createPortal(ui, host)
}
