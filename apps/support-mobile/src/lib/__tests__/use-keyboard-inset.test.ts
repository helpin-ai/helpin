import { renderHook } from '@testing-library/react'
import { useKeyboardInset } from '../use-keyboard-inset'

/**
 * Minimal fake `visualViewport` — a real EventTarget so `addEventListener` /
 * `removeEventListener` behave like the browser API the hook depends on,
 * with a `fire()` helper to simulate a resize/scroll tick.
 */
function createFakeVisualViewport(height: number, offsetTop = 0) {
  const target = new EventTarget()
  return {
    height,
    offsetTop,
    addEventListener: target.addEventListener.bind(target),
    removeEventListener: target.removeEventListener.bind(target),
    fire() {
      target.dispatchEvent(new Event('resize'))
    },
  }
}

const originalInnerHeight = window.innerHeight
const originalVisualViewport = window.visualViewport

afterEach(() => {
  Object.defineProperty(window, 'innerHeight', { value: originalInnerHeight, configurable: true })
  Object.defineProperty(window, 'visualViewport', { value: originalVisualViewport, configurable: true })
  document.documentElement.style.removeProperty('--keyboard-inset')
})

test('sets --keyboard-inset to the covered height when the keyboard opens', () => {
  Object.defineProperty(window, 'innerHeight', { value: 800, configurable: true })
  const vv = createFakeVisualViewport(500)
  Object.defineProperty(window, 'visualViewport', { value: vv, configurable: true })

  renderHook(() => useKeyboardInset())

  expect(document.documentElement.style.getPropertyValue('--keyboard-inset')).toBe('300px')
})

test('updates --keyboard-inset on subsequent resize events', () => {
  Object.defineProperty(window, 'innerHeight', { value: 800, configurable: true })
  const vv = createFakeVisualViewport(800)
  Object.defineProperty(window, 'visualViewport', { value: vv, configurable: true })

  renderHook(() => useKeyboardInset())
  expect(document.documentElement.style.getPropertyValue('--keyboard-inset')).toBe('0px')

  vv.height = 500
  vv.fire()
  expect(document.documentElement.style.getPropertyValue('--keyboard-inset')).toBe('300px')
})

test('resets --keyboard-inset to 0px on unmount', () => {
  Object.defineProperty(window, 'innerHeight', { value: 800, configurable: true })
  const vv = createFakeVisualViewport(500)
  Object.defineProperty(window, 'visualViewport', { value: vv, configurable: true })

  const { unmount } = renderHook(() => useKeyboardInset())
  expect(document.documentElement.style.getPropertyValue('--keyboard-inset')).toBe('300px')

  unmount()
  expect(document.documentElement.style.getPropertyValue('--keyboard-inset')).toBe('0px')
})

test('no-ops when visualViewport is unavailable', () => {
  Object.defineProperty(window, 'visualViewport', { value: undefined, configurable: true })

  expect(() => renderHook(() => useKeyboardInset())).not.toThrow()
  expect(document.documentElement.style.getPropertyValue('--keyboard-inset')).toBe('')
})
