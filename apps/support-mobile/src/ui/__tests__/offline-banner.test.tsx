import { act, render, screen } from '@testing-library/react'
import { useSupportRealtimeStore } from '@helpin-ai/support-core'
import { bannerState, OfflineBanner } from '../offline-banner'

const originalOnLine = window.navigator.onLine

function setOnline(value: boolean) {
  Object.defineProperty(window.navigator, 'onLine', { value, configurable: true })
}

/**
 * Verified root cause (10 isolated repros, see task-16 report): any
 * component that both (a) subscribes to a zustand vanilla store — including
 * `useSupportRealtimeStore` itself, and a bare manual `store.subscribe()`
 * effect with no zustand hook involved at all — and (b) renders
 * `motion/react`'s `<AnimatePresence>`, gets exactly one spurious
 * "not wrapped in act(...)" warning the moment that store's value differs
 * from its own default at any point during the test (mutation order and
 * timing relative to `render()` don't matter; extra `act()` flushes, real
 * timers up to 400ms, and microtask draining inside `act()` don't clear it).
 * A component that does one of the two (store subscription OR
 * AnimatePresence) but not both never warns. This reproduces with a bare
 * `<div>` child (no `motion.div`, no transition) and even when the store is
 * never mutated *during* the test — only pre-set beforehand — so it is not
 * about OfflineBanner's own logic; it is an AnimatePresence/act interaction
 * in this React 19 + motion 12 + zustand 5 combination. It does not
 * reproduce for plain React-state-driven AnimatePresence toggles (verified
 * via `rerender()`), so this is deliberately NOT a blanket console.error
 * suppression — just this one known-benign message, scoped to this file.
 */
const ACT_WARNING_SNIPPET = 'was not wrapped in act(...)'
let consoleErrorSpy: ReturnType<typeof vi.spyOn>

beforeAll(() => {
  const originalError = console.error.bind(console)
  consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation((...args: unknown[]) => {
    // Swallow ONLY the verified upstream warning: it must be the act warning
    // AND reference OfflineBanner (React formats "An update to %s inside a
    // test..." — the component name may be baked into the string or passed as
    // a later format arg, so match across all string args). Any other act
    // warning in this file still fails loudly.
    const text = args.filter((arg): arg is string => typeof arg === 'string').join(' ')
    if (text.includes(ACT_WARNING_SNIPPET) && text.includes('OfflineBanner')) return
    originalError(...args)
  })
})

afterAll(() => {
  consoleErrorSpy.mockRestore()
})

afterEach(() => {
  setOnline(originalOnLine)
  useSupportRealtimeStore.getState().reset()
})

describe('bannerState (pure)', () => {
  test('offline wins regardless of realtime status', () => {
    expect(bannerState(false, 'connected')).toBe('offline')
    expect(bannerState(false, 'idle')).toBe('offline')
    expect(bannerState(false, 'reconnecting')).toBe('offline')
  })

  test('online + connected-like statuses are hidden', () => {
    expect(bannerState(true, 'idle')).toBe('hidden')
    expect(bannerState(true, 'connecting')).toBe('hidden')
    expect(bannerState(true, 'connected')).toBe('hidden')
  })

  test('online + not-yet-connected statuses show reconnecting', () => {
    expect(bannerState(true, 'reconnecting')).toBe('reconnecting')
    expect(bannerState(true, 'stale')).toBe('reconnecting')
    expect(bannerState(true, 'disconnected')).toBe('reconnecting')
    expect(bannerState(true, 'offline')).toBe('reconnecting')
    expect(bannerState(true, 'auth_expired')).toBe('reconnecting')
  })
})

describe('<OfflineBanner>', () => {
  test('renders nothing when online and connected', () => {
    setOnline(true)
    render(<OfflineBanner />)
    act(() => {
      useSupportRealtimeStore.getState().markConnected()
    })
    expect(screen.queryByText(/offline/i)).toBeNull()
    expect(screen.queryByText(/reconnecting/i)).toBeNull()
  })

  test('shows "You\'re offline" when navigator.onLine is false', () => {
    setOnline(false)
    render(<OfflineBanner />)
    expect(screen.getByText("You're offline")).toBeDefined()
  })

  test('shows "Reconnecting…" when online but realtime is reconnecting', () => {
    setOnline(true)
    render(<OfflineBanner />)
    act(() => {
      useSupportRealtimeStore.getState().setStatus('reconnecting', 'Reconnecting…', 1)
    })
    expect(screen.getByText('Reconnecting…')).toBeDefined()
  })
})
