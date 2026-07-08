import type { invoke as InvokeFn } from '@tauri-apps/api/core'
import type { getPushToken as GetPushTokenFn, onPushTokenChanged as OnPushTokenChangedFn } from '@helpin/plugin-push'
import type { api as ApiClient } from '@mobile/lib/api'
import { routeDeepLinkUrl, routePushTap } from '@mobile/push/push-registration'

// Mocked locally (not in a shared setup file), same rationale as
// haptics.test.ts: these fakes must never leak into other test files that
// import unrelated code.
vi.mock('@tauri-apps/api/core', () => ({
  invoke: vi.fn(),
}))

vi.mock('@helpin/plugin-push', () => ({
  getPushToken: vi.fn(),
  onPushTokenChanged: vi.fn(),
}))

vi.mock('@mobile/lib/api', () => ({
  api: { post: vi.fn(), del: vi.fn() },
}))

function markAsTauri() {
  ;(window as unknown as { __TAURI_INTERNALS__: unknown }).__TAURI_INTERNALS__ = {}
}

function clearTauriFlag() {
  delete (window as unknown as { __TAURI_INTERNALS__?: unknown }).__TAURI_INTERNALS__
}

// `registerForPush` keeps a module-scope "already subscribed" flag (by
// design — onPushTapped-adjacent subscriptions must survive repeated calls
// from different call sites without double-registering). That means the
// module itself is stateful across calls, so each test gets a genuinely
// fresh copy via `vi.resetModules()` + dynamic re-import — otherwise state
// from an earlier test (e.g. "already subscribed") would leak into later
// ones purely because of import caching, not because of anything the
// production code does wrong.
async function freshPushRegistration() {
  vi.resetModules()
  const core = await import('@tauri-apps/api/core')
  const plugin = await import('@helpin/plugin-push')
  const apiModule = await import('@mobile/lib/api')
  const mod = await import('@mobile/push/push-registration')

  const mockInvoke = vi.mocked(core.invoke as typeof InvokeFn)
  const mockGetPushToken = vi.mocked(plugin.getPushToken as typeof GetPushTokenFn)
  const mockOnPushTokenChanged = vi.mocked(plugin.onPushTokenChanged as typeof OnPushTokenChangedFn)
  const mockPost = vi.mocked((apiModule.api as typeof ApiClient).post)
  const mockDel = vi.mocked((apiModule.api as typeof ApiClient).del)

  mockInvoke.mockReset().mockResolvedValue({ platform: 'ios', app_version: '1.2.3' })
  mockGetPushToken.mockReset()
  mockOnPushTokenChanged.mockReset().mockResolvedValue(vi.fn())
  mockPost.mockReset().mockResolvedValue({ data: null, error: null })
  mockDel.mockReset().mockResolvedValue({ data: null, error: null })

  return { mod, mockInvoke, mockGetPushToken, mockOnPushTokenChanged, mockPost, mockDel }
}

beforeEach(() => {
  clearTauriFlag()
})

afterEach(() => {
  clearTauriFlag()
})

describe('registerForPush', () => {
  test("outside Tauri: no-ops and returns 'unavailable' (no token fetch, no post, no subscribe)", async () => {
    const { mod, mockGetPushToken, mockPost, mockOnPushTokenChanged } = await freshPushRegistration()
    mockGetPushToken.mockResolvedValue('token-abc')

    await expect(mod.registerForPush()).resolves.toBe('unavailable')

    expect(mockGetPushToken).not.toHaveBeenCalled()
    expect(mockPost).not.toHaveBeenCalled()
    expect(mockOnPushTokenChanged).not.toHaveBeenCalled()
  })

  test("posts { platform, token, app_version } and returns 'registered' on success", async () => {
    const { mod, mockGetPushToken, mockPost } = await freshPushRegistration()
    markAsTauri()
    mockGetPushToken.mockResolvedValue('token-abc')

    await expect(mod.registerForPush()).resolves.toBe('registered')

    expect(mockPost).toHaveBeenCalledWith('/user/push-devices', {
      platform: 'ios',
      token: 'token-abc',
      app_version: '1.2.3',
    })
  })

  test("token null: does not post, returns 'unavailable'", async () => {
    const { mod, mockGetPushToken, mockPost } = await freshPushRegistration()
    markAsTauri()
    mockGetPushToken.mockResolvedValue(null)

    await expect(mod.registerForPush()).resolves.toBe('unavailable')

    expect(mockPost).not.toHaveBeenCalled()
  })

  test("backend POST failure: returns 'unavailable' (never falsely reported as registered)", async () => {
    const { mod, mockGetPushToken, mockPost } = await freshPushRegistration()
    markAsTauri()
    mockGetPushToken.mockResolvedValue('token-abc')
    mockPost.mockResolvedValue({ data: null, error: 'internal server error' })

    await expect(mod.registerForPush()).resolves.toBe('unavailable')

    expect(mockPost).toHaveBeenCalledTimes(1)
  })

  test('second call does not double-subscribe onPushTokenChanged', async () => {
    const { mod, mockGetPushToken, mockOnPushTokenChanged } = await freshPushRegistration()
    markAsTauri()
    mockGetPushToken.mockResolvedValue('token-abc')

    await mod.registerForPush()
    await mod.registerForPush()

    expect(mockOnPushTokenChanged).toHaveBeenCalledTimes(1)
  })

  test('re-registers when the plugin reports a token change', async () => {
    const { mod, mockGetPushToken, mockOnPushTokenChanged, mockPost } = await freshPushRegistration()
    markAsTauri()
    mockGetPushToken.mockResolvedValue('token-abc')
    let capturedCb: ((token: string) => void) | undefined
    mockOnPushTokenChanged.mockImplementation(async (cb) => {
      capturedCb = cb
      return vi.fn()
    })

    await mod.registerForPush()
    mockPost.mockClear()

    capturedCb?.('token-rotated')
    // subscription callback fires the re-post asynchronously (fire-and-forget)
    await Promise.resolve()
    await Promise.resolve()

    expect(mockPost).toHaveBeenCalledWith('/user/push-devices', {
      platform: 'ios',
      token: 'token-rotated',
      app_version: '1.2.3',
    })
  })

  test("non-mobile platform reported by shell info: does not post, returns 'unavailable'", async () => {
    const { mod, mockInvoke, mockGetPushToken, mockPost } = await freshPushRegistration()
    markAsTauri()
    mockInvoke.mockResolvedValue({ platform: 'linux', app_version: '1.2.3' })
    mockGetPushToken.mockResolvedValue('token-abc')

    await expect(mod.registerForPush()).resolves.toBe('unavailable')

    expect(mockPost).not.toHaveBeenCalled()
  })
})

describe('unregisterPush', () => {
  test('outside Tauri: no-ops', async () => {
    const { mod, mockGetPushToken, mockDel } = await freshPushRegistration()

    await mod.unregisterPush()

    expect(mockGetPushToken).not.toHaveBeenCalled()
    expect(mockDel).not.toHaveBeenCalled()
  })

  test('deletes the current plugin token', async () => {
    const { mod, mockGetPushToken, mockDel } = await freshPushRegistration()
    markAsTauri()
    mockGetPushToken.mockResolvedValue('token-abc')

    await mod.unregisterPush()

    expect(mockDel).toHaveBeenCalledWith('/user/push-devices', { token: 'token-abc' })
  })

  test('no current token: does not call delete', async () => {
    const { mod, mockGetPushToken, mockDel } = await freshPushRegistration()
    markAsTauri()
    mockGetPushToken.mockResolvedValue(null)

    await mod.unregisterPush()

    expect(mockDel).not.toHaveBeenCalled()
  })

  test('swallows failures (sign-out must never block on this)', async () => {
    const { mod, mockGetPushToken } = await freshPushRegistration()
    markAsTauri()
    mockGetPushToken.mockRejectedValue(new Error('plugin unavailable'))

    await expect(mod.unregisterPush()).resolves.toBeUndefined()
  })

  test('resolves after 3s even when the DELETE hangs forever (sign-out must never hang)', async () => {
    vi.useFakeTimers()
    try {
      const { mod, mockGetPushToken, mockDel } = await freshPushRegistration()
      markAsTauri()
      mockGetPushToken.mockResolvedValue('token-abc')
      // Dead network: the DELETE never settles.
      mockDel.mockImplementation(() => new Promise(() => {}))

      let settled = false
      const promise = mod.unregisterPush().then(() => {
        settled = true
      })

      // Let the token fetch resolve and the race begin.
      await vi.advanceTimersByTimeAsync(0)
      expect(settled).toBe(false)

      await vi.advanceTimersByTimeAsync(3_000)
      await promise
      expect(settled).toBe(true)
    } finally {
      vi.useRealTimers()
    }
  })
})

describe('routePushTap', () => {
  test('prefers deep_link when present', () => {
    const navigate = vi.fn()
    routePushTap(
      { deep_link: 'helpin://w/acme/support/conv_1', workspace_slug: 'acme', conversation_id: 'conv_1' },
      navigate,
    )
    expect(navigate).toHaveBeenCalledWith('/w/acme/support/conv_1')
  })

  test('falls back to workspace_slug + conversation_id when deep_link is absent', () => {
    const navigate = vi.fn()
    routePushTap({ workspace_slug: 'acme', conversation_id: 'conv_1' }, navigate)
    expect(navigate).toHaveBeenCalledWith('/w/acme/support/conv_1')
  })

  test('garbage payload: no navigation', () => {
    const navigate = vi.fn()
    routePushTap({ foo: 'bar' }, navigate)
    expect(navigate).not.toHaveBeenCalled()
  })

  test('malformed deep_link: no navigation', () => {
    const navigate = vi.fn()
    routePushTap({ deep_link: 'not-a-real-link' }, navigate)
    expect(navigate).not.toHaveBeenCalled()
  })

  test('degraded slugless payload (conversation_id only, no deep_link): no navigation', () => {
    const navigate = vi.fn()
    routePushTap({ conversation_id: 'conv_1' }, navigate)
    expect(navigate).not.toHaveBeenCalled()
  })

  test('deep_link takes priority even when fallback fields disagree', () => {
    const navigate = vi.fn()
    routePushTap(
      {
        deep_link: 'helpin://w/acme/support/conv_1',
        workspace_slug: 'other',
        conversation_id: 'conv_2',
      },
      navigate,
    )
    expect(navigate).toHaveBeenCalledWith('/w/acme/support/conv_1')
  })

  test('slug containing a slash is rejected even via fallback', () => {
    const navigate = vi.fn()
    routePushTap({ workspace_slug: 'a/b', conversation_id: 'conv_1' }, navigate)
    expect(navigate).not.toHaveBeenCalled()
  })

  test('deep_link with an extra slash (too many segments) is rejected via the deep_link path', () => {
    const navigate = vi.fn()
    // Extra path segment means the fallback fields are also absent — the
    // deep_link parser itself must reject this, not just the fallback.
    routePushTap({ deep_link: 'helpin://w/a/b/support/conv_1' }, navigate)
    expect(navigate).not.toHaveBeenCalled()
  })

  test('deep_link with an empty slug segment is rejected via the deep_link path', () => {
    const navigate = vi.fn()
    routePushTap({ deep_link: 'helpin://w//support/conv_1' }, navigate)
    expect(navigate).not.toHaveBeenCalled()
  })

  test('deep_link with dot traversal segments is rejected via the deep_link path', () => {
    const navigate = vi.fn()
    routePushTap({ deep_link: 'helpin://w/../support/conv_1' }, navigate)
    routePushTap({ deep_link: 'helpin://w/acme/support/..' }, navigate)
    routePushTap({ deep_link: 'helpin://w/./support/conv_1' }, navigate)
    expect(navigate).not.toHaveBeenCalled()
  })

  test('dot traversal segments are rejected via the fallback path too', () => {
    const navigate = vi.fn()
    routePushTap({ workspace_slug: '..', conversation_id: 'conv_1' }, navigate)
    routePushTap({ workspace_slug: 'acme', conversation_id: '.' }, navigate)
    expect(navigate).not.toHaveBeenCalled()
  })
})

// `routeDeepLinkUrl` is the `onOpenUrl` (plugin-deep-link) counterpart to
// `routePushTap`'s `deep_link` field: it's a thin wrapper that hands the raw
// URL string to the SAME `deep_link` parsing path (see `routePushTap` above)
// so there is exactly one parser/validator for `helpin://w/{slug}/support/{id}`
// links. Segment validation (slashes, `.`/`..`, wrong segment count, etc.) is
// already covered by the `routePushTap` suite above via the `deep_link`
// field — these tests only cover what's NEW here: scheme handling.
describe('routeDeepLinkUrl', () => {
  test('valid helpin:// URL navigates to the parsed path', () => {
    const navigate = vi.fn()
    routeDeepLinkUrl('helpin://w/acme/support/conv_1', navigate)
    expect(navigate).toHaveBeenCalledWith('/w/acme/support/conv_1')
  })

  test('https:// scheme (e.g. a universal-link-shaped URL): no navigation', () => {
    const navigate = vi.fn()
    routeDeepLinkUrl('https://helpin.ai/w/acme/support/conv_1', navigate)
    expect(navigate).not.toHaveBeenCalled()
  })

  test('garbage / non-URL string: no navigation', () => {
    const navigate = vi.fn()
    routeDeepLinkUrl('not-a-url-at-all', navigate)
    expect(navigate).not.toHaveBeenCalled()
  })

  test('empty string: no navigation', () => {
    const navigate = vi.fn()
    routeDeepLinkUrl('', navigate)
    expect(navigate).not.toHaveBeenCalled()
  })
})
