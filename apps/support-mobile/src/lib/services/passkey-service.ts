import {
  browserSupportsWebAuthn,
  startAuthentication,
  WebAuthnAbortService,
  WebAuthnError,
} from '@simplewebauthn/browser'
import { API_BASE } from '@mobile/lib/api'
import type { PasskeyOptionsResponse, SigninResponse } from '@mobile/lib/types'

export interface PasskeyResult {
  data: SigninResponse | null
  error: string | null
  cancelled?: boolean
}

function unwrapPublicKeyOptions<T>(options: unknown): T {
  if (options && typeof options === 'object' && 'publicKey' in options) {
    const publicKey = (options as { publicKey?: unknown }).publicKey
    if (publicKey && typeof publicKey === 'object') return publicKey as T
  }
  return options as T
}

export function classifyPasskeyError(error: unknown): { message: string; cancelled: boolean } {
  if (error instanceof WebAuthnError && error.code === 'ERROR_CEREMONY_ABORTED') {
    return { message: 'The passkey request was cancelled.', cancelled: true }
  }
  if (typeof error === 'object' && error !== null && 'name' in error) {
    switch (String(error.name)) {
      case 'NotAllowedError':
        return { message: 'No passkey was found, or the request was cancelled.', cancelled: false }
      case 'AbortError':
        return { message: 'The passkey request was cancelled.', cancelled: true }
      case 'SecurityError':
        return { message: 'Passkeys are not available for this app origin.', cancelled: false }
      case 'NotSupportedError':
        return { message: 'Passkeys are not supported on this device.', cancelled: false }
      default:
        break
    }
  }
  return {
    message: error instanceof Error && error.message.trim() ? error.message : 'Passkey sign in failed',
    cancelled: false,
  }
}

async function request<T>(path: string, init: RequestInit): Promise<{ data: T | null; error: string | null }> {
  try {
    const response = await fetch(`${API_BASE}${path}`, {
      ...init,
      credentials: 'include',
      headers: { 'Content-Type': 'application/json', ...init.headers },
    })
    if (!response.ok) {
      const payload = await response.json().catch(() => ({ error: response.statusText }))
      return { data: null, error: payload.error || response.statusText }
    }
    return { data: await response.json(), error: null }
  } catch (error) {
    return { data: null, error: error instanceof Error ? error.message : 'Network error' }
  }
}

async function beginAuthentication(emailHint?: string): Promise<PasskeyResult> {
  const optionsResult = await request<PasskeyOptionsResponse>('/auth/passkey/authentication-options', {
    method: 'POST',
    body: JSON.stringify(emailHint?.trim() ? { email_hint: emailHint.trim() } : {}),
  })
  if (optionsResult.error || !optionsResult.data) {
    return { data: null, error: optionsResult.error || 'Could not prepare passkey sign in' }
  }

  try {
    const credential = await startAuthentication({
      optionsJSON: unwrapPublicKeyOptions<Parameters<typeof startAuthentication>[0]['optionsJSON']>(
        optionsResult.data.options,
      ),
    })
    return await request<SigninResponse>('/auth/passkey/authenticate', {
      method: 'POST',
      body: JSON.stringify({
        challenge: optionsResult.data.challenge,
        credential,
        remember_me: true,
      }),
    })
  } catch (error) {
    const failure = classifyPasskeyError(error)
    return { data: null, error: failure.cancelled ? null : failure.message, cancelled: failure.cancelled }
  }
}

export const passkeyService = {
  isSupported: () => browserSupportsWebAuthn(),
  cancelPendingAuthentication: () => WebAuthnAbortService.cancelCeremony(),
  beginAuthentication,
}
