import { useEffect, useRef, useState } from 'react'
import { cn } from '@/lib/utils'

/**
 * StickyFormFooter pins its children to the top of the scroll
 * viewport when the page is scrolled. At rest (not scrolled) it
 * renders as a plain row. When scrolled it gains card styling.
 * Hidden entirely when `visible` is false (no unsaved changes).
 *
 * Uses a scroll listener on the nearest scroll ancestor to detect
 * when the page has been scrolled. No sentinel elements, no
 * wrapper divs — renders as a single direct child of the form
 * so sticky positioning has the full form height to work with.
 */
export function StickyFormFooter({
  children,
  className,
  visible = true,
}: {
  children: React.ReactNode
  className?: string
  /** When false, the footer hides entirely. */
  visible?: boolean
}) {
  const ref = useRef<HTMLDivElement>(null)
  const [isStuck, setIsStuck] = useState(false)

  useEffect(() => {
    const el = ref.current
    if (!el) return

    // Walk up to the nearest scroll container.
    let scrollEl: HTMLElement | null = el.parentElement
    while (scrollEl) {
      const style = getComputedStyle(scrollEl)
      if (
        style.overflow === 'auto' ||
        style.overflow === 'scroll' ||
        style.overflowY === 'auto' ||
        style.overflowY === 'scroll'
      ) {
        break
      }
      scrollEl = scrollEl.parentElement
    }
    if (!scrollEl) return

    const target = scrollEl
    const onScroll = () => setIsStuck(target.scrollTop > 10)
    target.addEventListener('scroll', onScroll, { passive: true })
    onScroll()
    return () => target.removeEventListener('scroll', onScroll)
  }, [])

  return (
    <div
      ref={ref}
      className={cn(
        'sticky -top-4 md:-top-6 z-10 transition-[background-color,border-color,box-shadow,padding] duration-200',
        !visible && 'invisible h-0 overflow-hidden p-0',
        visible && isStuck && 'rounded-lg border border-border/60 bg-card/95 px-4 py-2.5 shadow-sm backdrop-blur-sm',
        visible && !isStuck && 'py-1',
        className,
      )}
    >
      <div className={cn('flex items-center justify-end gap-2', !visible && 'invisible')}>
        {children}
      </div>
    </div>
  )
}
