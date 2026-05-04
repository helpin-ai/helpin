import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

export interface DocumentOutlineItem {
  index: number
  level: number
  text: string
}

interface DocsOutlineMinimapProps {
  items: DocumentOutlineItem[]
  scrollContainer: HTMLElement | null
  onSelect: (index: number) => void
}

// Dash widths per heading level (compact)
function dashWidth(level: number): number {
  if (level <= 2) return 16
  if (level === 3) return 12
  if (level === 4) return 8
  return 6
}

// Threshold below the wrapper's top edge that defines "current section":
// the last heading whose top is ≤ this offset is the active one.
const ACTIVE_THRESHOLD = 96

export function DocsOutlineMinimap({ items, scrollContainer, onSelect }: DocsOutlineMinimapProps) {
  const [activeIndex, setActiveIndex] = useState(0)
  const [hovered, setHovered] = useState(false)
  const hideTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // The scroll container we receive is the editor shell. The actual scroller
  // is `.docs-editor-wrapper` (which has `overflow-y-auto`) inside it.
  const scroller = useMemo<HTMLElement | null>(() => {
    if (!scrollContainer) return null
    return (scrollContainer.querySelector('.docs-editor-wrapper') as HTMLElement | null) ?? scrollContainer
  }, [scrollContainer])

  const computeActive = useCallback(() => {
    if (!scroller || items.length === 0) return
    const headings = scroller.querySelectorAll<HTMLElement>(
      '.docs-editor-prose h1, .docs-editor-prose h2, .docs-editor-prose h3, .docs-editor-prose h4, .docs-editor-prose h5, .docs-editor-prose h6',
    )
    if (headings.length === 0) return
    const wrapperTop = scroller.getBoundingClientRect().top
    let next = 0
    for (let i = 0; i < headings.length; i++) {
      const top = headings[i].getBoundingClientRect().top - wrapperTop
      if (top <= ACTIVE_THRESHOLD) next = i
      else break
    }
    setActiveIndex((prev) => (prev === next ? prev : next))
  }, [items, scroller])

  // rAF-throttled scroll listener.
  useEffect(() => {
    if (!scroller) return
    let pending = false
    const onScroll = () => {
      if (pending) return
      pending = true
      requestAnimationFrame(() => {
        pending = false
        computeActive()
      })
    }
    computeActive()
    scroller.addEventListener('scroll', onScroll, { passive: true })
    window.addEventListener('resize', onScroll)
    return () => {
      scroller.removeEventListener('scroll', onScroll)
      window.removeEventListener('resize', onScroll)
    }
  }, [scroller, computeActive])

  // Recompute when items change (typing inserts/removes headings).
  useEffect(() => {
    computeActive()
  }, [items, computeActive])

  const enter = useCallback(() => {
    if (hideTimerRef.current) {
      clearTimeout(hideTimerRef.current)
      hideTimerRef.current = null
    }
    setHovered(true)
  }, [])
  const leave = useCallback(() => {
    if (hideTimerRef.current) clearTimeout(hideTimerRef.current)
    hideTimerRef.current = setTimeout(() => setHovered(false), 120)
  }, [])

  const handleClick = useCallback(
    (index: number) => {
      onSelect(index)
      setActiveIndex(index)
    },
    [onSelect],
  )

  const minIndentLevel = useMemo(
    () => items.reduce((min, it) => Math.min(min, it.level), 6),
    [items],
  )

  if (items.length < 3) return null

  return (
    <div className="pointer-events-none absolute right-4 top-24 z-20 hidden max-h-[60vh] lg:flex">
      {/* Expanded labeled panel (sits to the LEFT of the strip so it doesn't
          collide with the right rail). Hover on the panel itself keeps it open. */}
      <div
        onMouseEnter={enter}
        onMouseLeave={leave}
        className={`pointer-events-auto mr-2 max-h-[60vh] w-64 max-w-64 overflow-y-auto rounded-md border border-border/50 bg-popover py-2 shadow-md transition-opacity duration-150 ${
          hovered ? 'opacity-100' : 'pointer-events-none opacity-0'
        }`}
      >
        {items.map((item) => {
          const isActive = item.index === activeIndex
          return (
            <button
              key={`${item.index}:${item.text}`}
              type="button"
              onClick={() => handleClick(item.index)}
              className={`block w-full truncate px-3 py-1 text-left text-sm transition-colors ${
                isActive ? 'font-medium text-foreground' : 'text-muted-foreground hover:text-foreground'
              }`}
              style={{ paddingLeft: `${12 + Math.max(0, item.level - minIndentLevel) * 10}px` }}
            >
              {item.text}
            </button>
          )
        })}
      </div>
      {/* Always-visible dash strip */}
      <div
        onMouseEnter={enter}
        onMouseLeave={leave}
        className="pointer-events-auto flex flex-col items-end gap-1.5 py-1.5 pl-2 pr-0.5"
      >
        {items.map((item) => {
          const isActive = item.index === activeIndex
          return (
            <button
              key={`dash:${item.index}:${item.text}`}
              type="button"
              aria-label={item.text}
              onClick={() => handleClick(item.index)}
              className="group/dash flex h-2.5 items-center"
            >
              <span
                className={`inline-block h-0.5 rounded-full transition-colors ${
                  isActive
                    ? 'bg-foreground'
                    : 'bg-border group-hover/dash:bg-muted-foreground'
                }`}
                style={{ width: dashWidth(item.level) }}
              />
            </button>
          )
        })}
      </div>
    </div>
  )
}
