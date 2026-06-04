import {
  browserSupportsWebAuthn,
  browserSupportsWebAuthnAutofill,
  startAuthentication,
  WebAuthnAbortService,
  WebAuthnError,
} from '@simplewebauthn/browser'
import { API_BASE } from '@/lib/api'
import type { PasskeyAuthenticationResponse, PasskeyOptionsResponse } from '@/lib/types'

type ApiResult<T> = {
  data: T | null
  error: string | null
  code?: string
  status?: number
  cancelled?: boolean
}

function unwrapPublicKeyOptions<T>(options: unknown): T {
  if (options && typeof options === 'object' && 'publicKey' in options) {
    const publicKey = (options as { publicKey?: unknown }).publicKey
    if (publicKey && typeof publicKey === 'object') {
      return publicKey as T
    }
  }
  return options as T
}

function classifyPasskeyError(error: unknown, useAutofill = false): { message: string; cancelled: boolean } {
  if (error instanceof WebAuthnError && error.code === 'ERROR_CEREMONY_ABORTED') {
    return { message: 'The passkey request was cancelled.', cancelled: true }
  }

  if (typeof error === 'object' && error !== null && 'name' in error) {
    const name = String(error.name)
    if (useAutofill && (name === 'AbortError' || name === 'NotAllowedError')) {
      return { message: 'The passkey request was cancelled.', cancelled: true }
    }

    switch (name) {
      case 'NotAllowedError':
        return { message: 'No passkey found for this account or the request was cancelled.', cancelled: false }
      case 'AbortError':
        return { message: 'The passkey request was cancelled.', cancelled: false }
      case 'NotSupportedError':
        return { message: 'This browser does not support passkeys.', cancelled: false }
      default:
        break
    }
  }

  if (error instanceof Error && error.message.trim()) {
    return { message: error.message, cancelled: false }
  }

  return { message: 'Passkey request failed', cancelled: false }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<ApiResult<T>> {
  try {
    const response = await fetch(`${API_BASE}${path}`, {
      ...options,
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    })

    if (!response.ok) {
      const payload = await response.json().catch(() => ({ error: response.statusText }))
      return { data: null, error: payload.error || response.statusText, code: payload.code, status: response.status }
    }

    return { data: await response.json(), error: null, status: response.status }
  } catch (error) {
    const failure = classifyPasskeyError(error)
    return { data: null, error: failure.message, cancelled: failure.cancelled }
  }
}

async function beginAuthentication(
  emailHint?: string,
  rememberMe = true,
  options?: { useAutofill?: boolean },
): Promise<ApiResult<PasskeyAuthenticationResponse>> {
  const optionsResult = await request<PasskeyOptionsResponse>('/auth/passkey/authentication-options', {
    method: 'POST',
    body: JSON.stringify(emailHint?.trim() ? { email_hint: emailHint.trim() } : {}),
  })
  if (optionsResult.error || !optionsResult.data) {
    return { data: null, error: optionsResult.error || 'Failed to prepare passkey sign-in', code: optionsResult.code }
  }

  try {
    const credential = await startAuthentication({
      optionsJSON: unwrapPublicKeyOptions<Parameters<typeof startAuthentication>[0]['optionsJSON']>(
        optionsResult.data.options,
      ),
      ...(options?.useAutofill ? { useBrowserAutofill: true } : {}),
    })

    return await request<PasskeyAuthenticationResponse>('/auth/passkey/authenticate', {
      method: 'POST',
      body: JSON.stringify({
        challenge: optionsResult.data.challenge,
        credential,
        remember_me: rememberMe,
      }),
    })
  } catch (error) {
    const failure = classifyPasskeyError(error, options?.useAutofill)
    return { data: null, error: failure.cancelled ? null : failure.message, cancelled: failure.cancelled }
  }
}

export const passkeyService = {
  isSupported: () => browserSupportsWebAuthn(),
  isAutofillSupported: async () => browserSupportsWebAuthnAutofill(),
  cancelPendingAuthentication: () => WebAuthnAbortService.cancelCeremony(),
  beginAuthentication,
}
