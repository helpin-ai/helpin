import { addPluginListener, invoke, type PluginListener } from '@tauri-apps/api/core'
import { getPushToken, onPushTapped, onPushTokenChanged } from '@helpin/plugin-push'

// Mocked locally (not in a shared setup file), same rationale as
// haptics.test.ts: this fake `@tauri-apps/api/core` module must never leak
// into other test files that import unrelated code.
vi.mock('@tauri-apps/api/core', () => ({
  invoke: vi.fn(),
  addPluginListener: vi.fn(),
}))

const mockInvoke = vi.mocked(invoke)
const mockAddPluginListener = vi.mocked(addPluginListener)

type TapHandler = (payload: Record<string, string>) => void

/** Stubs addPluginListener, capturing the live-event handler that
 * onPushTapped/onPushTokenChanged register, plus a spy-able unregister. */
function stubListener() {
  const captured: { handler?: TapHandler } = {}
  const unregister = vi.fn().mockResolvedValue(undefined)
  mockAddPluginListener.mockImplementation(((
    _plugin: string,
    _event: string,
    handler: TapHandler,
  ) => {
    captured.handler = handler
    return Promise.resolve({ unregister } as unknown as PluginListener)
  }) as typeof addPluginListener)
  return { captured, unregister }
}

beforeEach(() => {
  mockInvoke.mockReset()
  mockAddPluginListener.mockReset()
  // Default: no pending tap buffered natively.
  mockInvoke.mockResolvedValue({ tap: null })
})

describe('getPushToken', () => {
  test('unwraps { token } to the token string', async () => {
    mockInvoke.mockResolvedValue({ token: 'fcm-token-123' })

    await expect(getPushToken()).resolves.toBe('fcm-token-123')
    expect(mockInvoke).toHaveBeenCalledWith('plugin:helpin-push|get_push_token')
  })

  test('unwraps { token: null } to null (no token available)', async () => {
    mockInvoke.mockResolvedValue({ token: null })

    await expect(getPushToken()).resolves.toBeNull()
  })
})

describe('onPushTokenChanged', () => {
  test('registers on the helpin-push plugin channel and forwards the token', async () => {
    let capturedHandler: ((payload: { token: string }) => void) | undefined
    const unregister = vi.fn().mockResolvedValue(undefined)
    mockAddPluginListener.mockImplementation(((
      _plugin: string,
      _event: string,
      handler: (payload: { token: string }) => void,
    ) => {
      capturedHandler = handler
      return Promise.resolve({ unregister } as unknown as PluginListener)
    }) as typeof addPluginListener)

    const cb = vi.fn()
    const unlisten = await onPushTokenChanged(cb)

    expect(mockAddPluginListener).toHaveBeenCalledWith(
      'helpin-push',
      'push-token-changed',
      expect.any(Function),
    )

    capturedHandler?.({ token: 'refreshed-token' })
    expect(cb).toHaveBeenCalledWith('refreshed-token')

    // The returned UnlistenFn must be a plain () => void wrapping unregister().
    unlisten()
    expect(unregister).toHaveBeenCalledTimes(1)
  })
})

describe('onPushTapped', () => {
  test('registers the live listener first, then drains take_pending_tap exactly once', async () => {
    const { captured } = stubListener()
    const cb = vi.fn()
    await onPushTapped(cb)

    expect(mockAddPluginListener).toHaveBeenCalledWith(
      'helpin-push',
      'push-tapped',
      expect.any(Function),
    )
    expect(mockInvoke).toHaveBeenCalledWith('plugin:helpin-push|take_pending_tap')
    expect(mockInvoke).toHaveBeenCalledTimes(1)
    // Listener must be registered BEFORE the drain, so a tap arriving in
    // between is caught by one path or the other, never lost.
    expect(mockAddPluginListener.mock.invocationCallOrder[0]).toBeLessThan(
      mockInvoke.mock.invocationCallOrder[0],
    )
    // No pending tap buffered -> cb not called at registration time.
    expect(cb).not.toHaveBeenCalled()
    expect(captured.handler).toBeDefined()
  })

  test('delivers a natively buffered cold-start tap exactly once via the drain', async () => {
    stubListener()
    const pending = { conversation_id: 'conv_1', workspace_slug: 'acme' }
    mockInvoke.mockResolvedValue({ tap: pending })

    const cb = vi.fn()
    await onPushTapped(cb)

    expect(cb).toHaveBeenCalledTimes(1)
    expect(cb).toHaveBeenCalledWith(pending)
  })

  test('live event still passes payloads through unchanged', async () => {
    const { captured, unregister } = stubListener()
    const cb = vi.fn()
    const unlisten = await onPushTapped(cb)

    const payload = { conversation_id: 'conv_1', workspace_slug: 'acme' }
    captured.handler?.(payload)
    expect(cb).toHaveBeenCalledWith(payload)

    unlisten()
    expect(unregister).toHaveBeenCalledTimes(1)
  })

  test('dedupes the same payload arriving via both the drain and the live event', async () => {
    const { captured } = stubListener()
    const payload = { conversation_id: 'conv_1', workspace_slug: 'acme' }
    mockInvoke.mockResolvedValue({ tap: payload })

    const cb = vi.fn()
    await onPushTapped(cb)
    // Drain already delivered it once...
    expect(cb).toHaveBeenCalledTimes(1)

    // ...the belt-and-braces live trigger of the SAME tap (fresh object,
    // key order shuffled — dedupe must be by content, not reference).
    captured.handler?.({ workspace_slug: 'acme', conversation_id: 'conv_1' })
    expect(cb).toHaveBeenCalledTimes(1)

    // A genuinely different tap still goes through.
    captured.handler?.({ conversation_id: 'conv_2', workspace_slug: 'acme' })
    expect(cb).toHaveBeenCalledTimes(2)
    expect(cb).toHaveBeenLastCalledWith({ conversation_id: 'conv_2', workspace_slug: 'acme' })
  })

  test('an identical payload after the dedupe window is delivered again (real second tap)', async () => {
    vi.useFakeTimers()
    try {
      const { captured } = stubListener()
      const payload = { conversation_id: 'conv_1' }

      const cb = vi.fn()
      await onPushTapped(cb)
      expect(cb).not.toHaveBeenCalled()

      captured.handler?.(payload)
      expect(cb).toHaveBeenCalledTimes(1)

      // Within the window: duplicate suppressed.
      vi.advanceTimersByTime(1_000)
      captured.handler?.({ ...payload })
      expect(cb).toHaveBeenCalledTimes(1)

      // Past the window: the user really tapped identical content again.
      vi.advanceTimersByTime(10_000)
      captured.handler?.({ ...payload })
      expect(cb).toHaveBeenCalledTimes(2)
    } finally {
      vi.useRealTimers()
    }
  })
})
