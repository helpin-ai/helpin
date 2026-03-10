import { useEffect, useState, useCallback } from 'react'

export function useScrollSpy(ids: string[]): string {
  const [activeId, setActiveId] = useState('')

  const handleScroll = useCallback(() => {
    if (ids.length === 0) return

    // Offset from top to account for sticky header
    const offset = 100

    // Find the heading closest to (but above) the scroll position
    let current = ''
    for (const id of ids) {
      const el = document.getElementById(id)
      if (!el) continue
      const top = el.getBoundingClientRect().top
      if (top <= offset) {
        current = id
      } else {
        break
      }
    }

    // If we haven't scrolled past any heading yet, highlight the first one
    // only if we're near the top of the page
    if (!current && ids.length > 0) {
      const firstEl = document.getElementById(ids[0]!)
      if (firstEl && firstEl.getBoundingClientRect().top <= offset + 200) {
        current = ids[0]!
      }
    }

    setActiveId(current)
  }, [ids])

  useEffect(() => {
    if (ids.length === 0) return

    // Run once on mount
    handleScroll()

    window.addEventListener('scroll', handleScroll, { passive: true })
    return () => window.removeEventListener('scroll', handleScroll)
  }, [ids, handleScroll])

  return activeId
}
