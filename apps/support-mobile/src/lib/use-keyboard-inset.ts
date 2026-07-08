import { useEffect } from 'react'

/**
 * Syncs `--keyboard-inset` (defined in index.css, read by screens that pin
 * content above the on-screen keyboard) to `window.visualViewport`. The
 * visual viewport shrinks by the keyboard's height when it opens on mobile
 * Safari/Chrome/WebView — the delta between `innerHeight` and the viewport's
 * height (plus its `offsetTop`, which accounts for viewport pan-and-scroll)
 * is exactly how much of the layout viewport the keyboard is covering.
 */
export function useKeyboardInset(): void {
  useEffect(() => {
    const vv = window.visualViewport
    if (!vv) return
    const update = () => {
      const inset = Math.max(0, window.innerHeight - vv.height - vv.offsetTop)
      document.documentElement.style.setProperty('--keyboard-inset', `${inset}px`)
    }
    vv.addEventListener('resize', update)
    vv.addEventListener('scroll', update)
    update()
    return () => {
      vv.removeEventListener('resize', update)
      vv.removeEventListener('scroll', update)
      document.documentElement.style.setProperty('--keyboard-inset', '0px')
    }
  }, [])
}
