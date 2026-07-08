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

beforeEach(() => {
  mockInvoke.mockReset()
  mockAddPluginListener.mockReset()
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
  test('registers on the helpin-push plugin channel and passes the payload through unchanged', async () => {
    let capturedHandler: ((payload: Record<string, string>) => void) | undefined
    const unregister = vi.fn().mockResolvedValue(undefined)
    mockAddPluginListener.mockImplementation(((
      _plugin: string,
      _event: string,
      handler: (payload: Record<string, string>) => void,
    ) => {
      capturedHandler = handler
      return Promise.resolve({ unregister } as unknown as PluginListener)
    }) as typeof addPluginListener)

    const cb = vi.fn()
    const unlisten = await onPushTapped(cb)

    expect(mockAddPluginListener).toHaveBeenCalledWith(
      'helpin-push',
      'push-tapped',
      expect.any(Function),
    )

    const payload = { conversation_id: 'conv_1', workspace_slug: 'acme' }
    capturedHandler?.(payload)
    expect(cb).toHaveBeenCalledWith(payload)

    unlisten()
    expect(unregister).toHaveBeenCalledTimes(1)
  })
})
