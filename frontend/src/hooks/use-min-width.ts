import { useEffect, useState } from 'react'

export function isAtLeastWidth(viewportWidth: number, minWidth: number) {
  return viewportWidth >= minWidth
}

export function useMinWidth(minWidth: number) {
  const [matches, setMatches] = useState(() => (
    typeof window !== 'undefined' && window.matchMedia(`(min-width: ${minWidth}px)`).matches
  ))

  useEffect(() => {
    const query = window.matchMedia(`(min-width: ${minWidth}px)`)
    const update = () => setMatches(query.matches)
    update()
    query.addEventListener('change', update)
    return () => query.removeEventListener('change', update)
  }, [minWidth])

  return matches
}
