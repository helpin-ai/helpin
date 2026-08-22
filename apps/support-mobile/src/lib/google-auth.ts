import { openUrl } from '@tauri-apps/plugin-opener'
import { API_BASE } from '@mobile/lib/api'
import { isTauri } from '@mobile/lib/host'

export interface GoogleAuthDeepLink {
  code?: string
  error?: string
}

export function buildGoogleAuthStartUrl(native = isTauri()): string {
  const origin = typeof window === 'undefined' ? 'http://localhost' : window.location.origin
  const url = new URL(`${API_BASE}/auth/google/start`, origin)
  url.searchParams.set('client', native ? 'mobile_native' : 'mobile_web')
  return url.toString()
}

export function parseGoogleAuthDeepLink(rawUrl: string): GoogleAuthDeepLink | null {
  try {
    const url = new URL(rawUrl)
    if (url.protocol !== 'helpin:' || url.hostname !== 'auth' || url.pathname !== '/google') return null
    const code = url.searchParams.get('code')?.trim()
    const error = url.searchParams.get('error')?.trim()
    if (code) return { code }
    if (error) return { error }
    return { error: 'invalid_callback' }
  } catch {
    return null
  }
}

export function googleAuthErrorMessage(reason: string): string {
  switch (reason) {
    case 'invalid_state':
      return 'Google sign in expired. Please try again.'
    case 'missing_code':
    case 'exchange_failed':
    case 'userinfo_failed':
    case 'signin_failed':
    case 'handoff_failed':
      return 'Google sign in could not be completed. Please try again.'
    default:
      return 'Google sign in failed. Please try again.'
  }
}

export async function startGoogleAuth(): Promise<void> {
  const url = buildGoogleAuthStartUrl()
  if (isTauri()) {
    await openUrl(url)
    return
  }
  window.location.assign(url)
}
