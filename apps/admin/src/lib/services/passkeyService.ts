import { browserSupportsWebAuthn, startAuthentication } from '@simplewebauthn/browser'
import { API_BASE } from '@/lib/api'
import type { PasskeyAuthenticationResponse, PasskeyOptionsResponse } from '@/lib/types'

type ApiResult<T> = {
  data: T | null
  error: string | null
  status?: number
}

function formatPasskeyError(error: unknown): string {
  if (typeof error === 'object' && error !== null && 'name' in error) {
    switch (String(error.name)) {
      case 'NotAllowedError':
        return 'No passkey found for this account or the request was cancelled.'
      case 'AbortError':
        return 'The passkey request was cancelled.'
      case 'NotSupportedError':
        return 'This browser does not support passkeys.'
      default:
        break
    }
  }

  if (error instanceof Error && error.message.trim()) {
    return error.message
  }

  return 'Passkey request failed'
}

async function request<T>(path: string, options: RequestInit = {}): Promise<ApiResult<T>> {
  try {
    const response = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    })

    if (!response.ok) {
      const payload = await response.json().catch(() => ({ error: response.statusText }))
      return { data: null, error: payload.error || response.statusText, status: response.status }
    }

    return { data: await response.json(), error: null, status: response.status }
  } catch (error) {
    return { data: null, error: formatPasskeyError(error) }
  }
}

async function beginAuthentication(emailHint?: string, rememberMe = false): Promise<ApiResult<PasskeyAuthenticationResponse>> {
  const optionsResult = await request<PasskeyOptionsResponse>('/auth/passkey/authentication-options', {
    method: 'POST',
    body: JSON.stringify(emailHint?.trim() ? { email_hint: emailHint.trim() } : {}),
  })
  if (optionsResult.error || !optionsResult.data) {
    return { data: null, error: optionsResult.error || 'Failed to prepare passkey sign-in' }
  }

  try {
    const credential = await startAuthentication({
      optionsJSON: optionsResult.data.options as unknown as Parameters<typeof startAuthentication>[0]['optionsJSON'],
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
    return { data: null, error: formatPasskeyError(error) }
  }
}

export const passkeyService = {
  isSupported: () => browserSupportsWebAuthn(),
  beginAuthentication,
}
