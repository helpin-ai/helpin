import {
  browserSupportsWebAuthn,
  browserSupportsWebAuthnAutofill,
  startAuthentication,
  startRegistration,
  WebAuthnAbortService,
  WebAuthnError,
} from '@simplewebauthn/browser';
import { API_BASE } from '@/lib/api';
import type {
  Passkey,
  PasskeyAuthenticationResponse,
  PasskeyListResponse,
  PasskeyOptionsResponse,
} from '@/lib/types';

type ApiResult<T> = {
  data: T | null;
  error: string | null;
  status?: number;
  cancelled?: boolean;
};

function unwrapPublicKeyOptions<T>(options: unknown): T {
  if (options && typeof options === 'object' && 'publicKey' in options) {
    const publicKey = (options as { publicKey?: unknown }).publicKey;
    if (publicKey && typeof publicKey === 'object') {
      return publicKey as T;
    }
  }
  return options as T;
}

async function request<T>(path: string, options: RequestInit = {}, withAuth = false): Promise<ApiResult<T>> {
  const token = withAuth ? localStorage.getItem('access_token') : null;

  try {
    const response = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...options.headers,
      },
    });

    if (!response.ok) {
      const payload = await response.json().catch(() => ({ error: response.statusText }));
      return { data: null, error: payload.error || response.statusText, status: response.status };
    }

    if (response.status === 204) {
      return { data: null as T, error: null, status: 204 };
    }

    return { data: await response.json(), error: null, status: response.status };
  } catch (error) {
    const failure = classifyPasskeyError(error);
    return {
      data: null,
      error: failure.message,
      cancelled: failure.cancelled,
    };
  }
}

function classifyPasskeyError(error: unknown, useAutofill = false): { message: string; cancelled: boolean } {
  if (error instanceof WebAuthnError && error.code === 'ERROR_CEREMONY_ABORTED') {
    return { message: 'The passkey request was cancelled.', cancelled: true };
  }

  if (typeof error === 'object' && error !== null && 'name' in error) {
    const name = String(error.name);
    if (useAutofill && (name === 'AbortError' || name === 'NotAllowedError')) {
      return { message: 'The passkey request was cancelled.', cancelled: true };
    }

    switch (name) {
      case 'NotAllowedError':
        return { message: 'No passkey found for this account or the request was cancelled.', cancelled: false };
      case 'InvalidStateError':
        return { message: 'This passkey is already registered on your account.', cancelled: false };
      case 'AbortError':
        return { message: 'The passkey request was cancelled.', cancelled: false };
      case 'SecurityError':
        return { message: 'Passkeys are only available on secure origins.', cancelled: false };
      case 'NotSupportedError':
        return { message: 'This browser does not support passkeys.', cancelled: false };
      default:
        break;
    }
  }

  if (error instanceof Error && error.message.trim()) {
    return { message: error.message, cancelled: false };
  }

  return { message: 'Passkey request failed', cancelled: false };
}

export function formatPasskeyError(error: unknown): string {
  return classifyPasskeyError(error).message;
}

async function beginRegistration(name?: string): Promise<ApiResult<Passkey>> {
  const optionsResult = await request<PasskeyOptionsResponse>(
    '/auth/passkey/registration-options',
    { method: 'POST', body: JSON.stringify({}) },
    true,
  );
  if (optionsResult.error || !optionsResult.data) {
    return { data: null, error: optionsResult.error || 'Failed to prepare passkey registration' };
  }

  try {
    const credential = await startRegistration({
      optionsJSON: unwrapPublicKeyOptions<Parameters<typeof startRegistration>[0]['optionsJSON']>(
        optionsResult.data.options,
      ),
    });

    return await request<Passkey>(
      '/auth/passkey/register',
      {
        method: 'POST',
        body: JSON.stringify({
          challenge: optionsResult.data.challenge,
          credential,
          ...(name?.trim() ? { name: name.trim() } : {}),
        }),
      },
      true,
    );
  } catch (error) {
    const failure = classifyPasskeyError(error);
    return { data: null, error: failure.cancelled ? null : failure.message, cancelled: failure.cancelled };
  }
}

async function beginAuthentication(
  emailHint?: string,
  rememberMe = false,
  options?: { useAutofill?: boolean },
): Promise<ApiResult<PasskeyAuthenticationResponse>> {
  const optionsResult = await request<PasskeyOptionsResponse>('/auth/passkey/authentication-options', {
    method: 'POST',
    body: JSON.stringify(emailHint?.trim() ? { email_hint: emailHint.trim() } : {}),
  });
  if (optionsResult.error || !optionsResult.data) {
    return { data: null, error: optionsResult.error || 'Failed to prepare passkey sign-in' };
  }

  try {
    const credential = await startAuthentication({
      optionsJSON: unwrapPublicKeyOptions<Parameters<typeof startAuthentication>[0]['optionsJSON']>(
        optionsResult.data.options,
      ),
      ...(options?.useAutofill ? { useBrowserAutofill: true } : {}),
    });

    return await request<PasskeyAuthenticationResponse>('/auth/passkey/authenticate', {
      method: 'POST',
      body: JSON.stringify({
        challenge: optionsResult.data.challenge,
        credential,
        remember_me: rememberMe,
      }),
    });
  } catch (error) {
    const failure = classifyPasskeyError(error, options?.useAutofill);
    return { data: null, error: failure.cancelled ? null : failure.message, cancelled: failure.cancelled };
  }
}

async function listPasskeys(): Promise<ApiResult<Passkey[]>> {
  const result = await request<PasskeyListResponse>('/auth/passkey/list', { method: 'GET' }, true);
  if (result.error || !result.data) {
    return { data: null, error: result.error || 'Failed to load passkeys', status: result.status };
  }
  return { data: result.data.passkeys, error: null, status: result.status };
}

async function deletePasskey(id: string): Promise<ApiResult<{ message: string }>> {
  return request<{ message: string }>(`/auth/passkey/${id}`, { method: 'DELETE' }, true);
}

export const passkeyService = {
  isSupported: () => browserSupportsWebAuthn(),
  isAutofillSupported: async () => browserSupportsWebAuthnAutofill(),
  cancelPendingAuthentication: () => WebAuthnAbortService.cancelCeremony(),
  beginRegistration,
  beginAuthentication,
  listPasskeys,
  deletePasskey,
};
