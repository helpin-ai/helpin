import { browserSupportsWebAuthn, startAuthentication, startRegistration } from '@simplewebauthn/browser';
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
};

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
    return {
      data: null,
      error: formatPasskeyError(error),
    };
  }
}

export function formatPasskeyError(error: unknown): string {
  if (typeof error === 'object' && error !== null && 'name' in error) {
    const name = String(error.name);
    switch (name) {
      case 'NotAllowedError':
        return 'No passkey found for this account or the request was cancelled.';
      case 'InvalidStateError':
        return 'This passkey is already registered on your account.';
      case 'AbortError':
        return 'The passkey request was cancelled.';
      case 'SecurityError':
        return 'Passkeys are only available on secure origins.';
      case 'NotSupportedError':
        return 'This browser does not support passkeys.';
      default:
        break;
    }
  }

  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }

  return 'Passkey request failed';
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
      optionsJSON: optionsResult.data.options as unknown as Parameters<typeof startRegistration>[0]['optionsJSON'],
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
    return { data: null, error: formatPasskeyError(error) };
  }
}

async function beginAuthentication(emailHint?: string, rememberMe = false): Promise<ApiResult<PasskeyAuthenticationResponse>> {
  const optionsResult = await request<PasskeyOptionsResponse>('/auth/passkey/authentication-options', {
    method: 'POST',
    body: JSON.stringify(emailHint?.trim() ? { email_hint: emailHint.trim() } : {}),
  });
  if (optionsResult.error || !optionsResult.data) {
    return { data: null, error: optionsResult.error || 'Failed to prepare passkey sign-in' };
  }

  try {
    const credential = await startAuthentication({
      optionsJSON: optionsResult.data.options as unknown as Parameters<typeof startAuthentication>[0]['optionsJSON'],
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
    return { data: null, error: formatPasskeyError(error) };
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
  beginRegistration,
  beginAuthentication,
  listPasskeys,
  deletePasskey,
};
