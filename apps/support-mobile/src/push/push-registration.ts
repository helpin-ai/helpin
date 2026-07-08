import { invoke } from '@tauri-apps/api/core'
import { getPushToken, onPushTokenChanged } from '@helpin/plugin-push'
import { api } from '@mobile/lib/api'
import { isTauri } from '@mobile/lib/host'

const PUSH_DEVICES_PATH = '/user/push-devices'

type MobilePlatform = 'ios' | 'android'

interface MobileShellInfo {
  platform: string
  app_version: string
}

function isMobilePlatform(value: string | null | undefined): value is MobilePlatform {
  return value === 'ios' || value === 'android'
}

/** Best-effort fallback when the native `mobile_shell_info` call is unavailable. */
function detectPlatformFromNavigator(): MobilePlatform | null {
  if (typeof navigator === 'undefined') return null
  const ua = navigator.userAgent ?? ''
  if (/android/i.test(ua)) return 'android'
  if (/iphone|ipad|ipod/i.test(ua)) return 'ios'
  return null
}

/**
 * Resolves the current platform + app version. Returns `platform: null` on
 * desktop/browser (or any non-iOS/Android shell) — callers treat that as a
 * no-op, since push registration only makes sense for mobile devices.
 */
async function resolveShellInfo(): Promise<{ platform: MobilePlatform | null; appVersion: string }> {
  try {
    const info = await invoke<MobileShellInfo>('mobile_shell_info')
    const platform = isMobilePlatform(info.platform) ? info.platform : detectPlatformFromNavigator()
    return { platform, appVersion: info.app_version }
  } catch {
    return { platform: detectPlatformFromNavigator(), appVersion: 'dev' }
  }
}

async function postToken(token: string): Promise<boolean> {
  const { platform, appVersion } = await resolveShellInfo()
  if (!platform) return false
  const { error } = await api.post(PUSH_DEVICES_PATH, { platform, token, app_version: appVersion })
  if (error) {
    console.debug('[push] device registration POST failed', { error })
    return false
  }
  return true
}

// Module-scope guard: onPushTokenChanged must be subscribed exactly once —
// registerForPush() is safe to call repeatedly (e.g. from both the
// permission-priming sheet and the You-screen notifications row) without
// creating a second listener.
let tokenChangeSubscribed = false

/** Outcome of a {@link registerForPush} attempt. */
export type PushRegistrationResult = 'registered' | 'unavailable'

/**
 * Registers this device for push notifications: fetches the current FCM/APNs
 * token from the plugin (triggering the OS permission prompt on first call,
 * per `getPushToken()`'s contract) and POSTs it to the backend. Also
 * subscribes to token rotation so a refreshed token is re-registered
 * automatically — but only once, no matter how many times this function is
 * called.
 *
 * Returns `'registered'` ONLY when a token was actually obtained AND the
 * backend POST succeeded — callers must persist `{decision:'enabled'}` on
 * that outcome alone. Everything else — outside Tauri, non-mobile platform,
 * OS permission denied / no Play services (null token), or a failed POST —
 * returns `'unavailable'`, so a denied or simulator user is never recorded
 * as permanently "enabled" with no recovery path.
 */
export async function registerForPush(): Promise<PushRegistrationResult> {
  if (!isTauri()) return 'unavailable'

  const token = await getPushToken()
  let registered = false
  if (token) {
    registered = await postToken(token)
  } else {
    console.debug('[push] no push token available (permission denied or unsupported environment)')
  }

  if (!tokenChangeSubscribed) {
    tokenChangeSubscribed = true
    await onPushTokenChanged((newToken) => {
      void postToken(newToken)
    })
  }

  return registered ? 'registered' : 'unavailable'
}

/** How long sign-out is willing to wait for the push-device DELETE. */
const UNREGISTER_TIMEOUT_MS = 3_000

const UNREGISTER_TIMED_OUT = Symbol('unregister-timed-out')

/**
 * Unregisters this device's current push token from the backend. Called from
 * `signOut()` BEFORE the session is cleared (needs the authenticated API
 * call). Failures are swallowed and logged — sign-out must never block or
 * fail because a push-device delete didn't go through.
 *
 * The DELETE is raced against a {@link UNREGISTER_TIMEOUT_MS} timeout
 * (support-core's fetch layer has no timeout of its own, and sign-out must
 * never hang on a dead network). When the timeout wins, the token may leak
 * server-side until FCM/APNs reports it unregistered and the backend prunes
 * it — an accepted trade-off for a sign-out that always completes.
 */
export async function unregisterPush(): Promise<void> {
  if (!isTauri()) return
  try {
    const token = await getPushToken()
    if (!token) return
    const timeout = new Promise<typeof UNREGISTER_TIMED_OUT>((resolve) => {
      setTimeout(() => resolve(UNREGISTER_TIMED_OUT), UNREGISTER_TIMEOUT_MS)
    })
    const result = await Promise.race([api.del(PUSH_DEVICES_PATH, { token }), timeout])
    if (result === UNREGISTER_TIMED_OUT) {
      console.debug('[push] unregisterPush timed out after 3s (non-fatal); token may linger server-side')
    }
  } catch (error) {
    console.debug('[push] unregisterPush failed (non-fatal)', error)
  }
}

const DEEP_LINK_PREFIX = 'helpin://w/'

/**
 * Loosely validates a slug/id path segment: non-empty, no path separators,
 * and not a relative-path traversal segment (`.` / `..`).
 */
function isValidSegment(value: string | undefined | null): value is string {
  return (
    typeof value === 'string' &&
    value.length > 0 &&
    !value.includes('/') &&
    value !== '.' &&
    value !== '..'
  )
}

function parseDeepLink(deepLink: string | undefined): string | null {
  if (typeof deepLink !== 'string' || !deepLink.startsWith(DEEP_LINK_PREFIX)) return null
  const rest = deepLink.slice(DEEP_LINK_PREFIX.length)
  const parts = rest.split('/')
  if (parts.length !== 3) return null
  const [slug, section, conversationId] = parts
  if (section !== 'support') return null
  if (!isValidSegment(slug) || !isValidSegment(conversationId)) return null
  return `/w/${slug}/support/${conversationId}`
}

function buildFallbackPath(workspaceSlug: string | undefined, conversationId: string | undefined): string | null {
  if (!isValidSegment(workspaceSlug) || !isValidSegment(conversationId)) return null
  return `/w/${workspaceSlug}/support/${conversationId}`
}

/**
 * PURE tap-routing decision: prefers parsing `data.deep_link`
 * (`helpin://w/{slug}/support/{id}` -> `/w/{slug}/support/{id}`), falling
 * back to building the path from `workspace_slug` + `conversation_id` when no
 * (valid) `deep_link` is present.
 *
 * The degraded slugless variant (`conversation_id` only, no `deep_link`, no
 * `workspace_slug`) results in NO navigation — we don't know which workspace
 * to route into, and guessing wrong would be worse than a no-op. Garbage or
 * malformed payloads likewise result in no navigation rather than throwing.
 */
export function routePushTap(data: Record<string, string>, navigate: (to: string) => void): void {
  const path = parseDeepLink(data.deep_link) ?? buildFallbackPath(data.workspace_slug, data.conversation_id)
  if (path) navigate(path)
}
