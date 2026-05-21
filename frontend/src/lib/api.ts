import { buildLoginPathForCurrentLocation, storeRedirectAfterLogin } from '@/lib/authRedirect';

export const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080/api';

interface ApiResponse<T> {
  data: T | null;
  error: string | null;
  status?: number;
  isNetworkError?: boolean;
}

async function request<T>(path: string, options: RequestInit = {}): Promise<ApiResponse<T>> {
  try {
    const res = await fetch(`${API_BASE}${path}`, {
      ...options,
      credentials: 'include',
      headers: buildHeaders(options.headers),
    });

    if (res.status === 401 && shouldAttemptRefresh(path)) {
      // Try refresh (skip for auth endpoints — a 401 there means bad credentials)
      const refreshed = await tryRefreshToken();
      if (refreshed) {
        const retryRes = await fetch(`${API_BASE}${path}`, {
          ...options,
          credentials: 'include',
          headers: buildHeaders(options.headers),
        });
        if (!retryRes.ok) {
          const err = await retryRes.json().catch(() => ({ error: retryRes.statusText }));
          return { data: null, error: err.error || retryRes.statusText };
        }
        if (retryRes.status === 204) return { data: null as T, error: null };
        const data = await retryRes.json();
        return { data, error: null };
      }
      if (path === '/auth/me') {
        return { data: null, error: 'Session expired', status: 401 };
      }
      clearLegacyTokenStorage();
      stopTokenRefreshTimer();
      storeRedirectAfterLogin();
      window.location.href = buildLoginPathForCurrentLocation();
      return { data: null, error: 'Session expired' };
    }

    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }));
      return { data: null, error: err.error || res.statusText, status: res.status };
    }
    if (res.status === 204) return { data: null as T, error: null, status: 204 };
    const data = await res.json();
    return { data, error: null, status: res.status };
  } catch (e) {
    return { data: null, error: e instanceof Error ? e.message : 'Network error', isNetworkError: true };
  }
}

function shouldAttemptRefresh(path: string): boolean {
  return path === '/auth/me' || !path.startsWith('/auth/');
}

function buildHeaders(init?: HeadersInit): Headers {
  const headers = new Headers(init);
  if (!headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  if (!headers.has('Authorization')) {
    const accessToken = getStoredToken('access_token');
    if (accessToken) {
      headers.set('Authorization', `Bearer ${accessToken}`);
    }
  }
  return headers;
}

function getStoredToken(key: 'access_token' | 'refresh_token'): string | null {
  try {
    return localStorage.getItem(key)?.trim() || null;
  } catch {
    return null;
  }
}

function clearLegacyAuthTokens(): void {
  try {
    localStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token');
  } catch {
    // Ignore storage access failures; cookies are the source of truth.
  }
}

function clearLegacyTokenStorage(): void {
  try {
    localStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token');
    localStorage.removeItem('remember_me');
  } catch {
    // Ignore storage access failures; cookies are the source of truth.
  }
}

async function tryRefreshToken(): Promise<boolean> {
  try {
    const legacyRefreshToken = getStoredToken('refresh_token');
    const res = await fetch(`${API_BASE}/auth/refresh`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: legacyRefreshToken ? JSON.stringify({ refresh_token: legacyRefreshToken }) : undefined,
    });
    if (!res.ok) return false;
    await res.json().catch(() => null);
    clearLegacyAuthTokens();
    return true;
  } catch {
    return false;
  }
}

// ---------------------------------------------------------------------------
// Proactive token refresh
// ---------------------------------------------------------------------------

const TOKEN_REFRESH_INTERVAL = 5 * 60 * 1000; // 5 minutes

let refreshTimerId: ReturnType<typeof setInterval> | null = null;

/** Start a periodic timer that refreshes the access token every 5 minutes. */
export function startTokenRefreshTimer(): void {
  stopTokenRefreshTimer();
  refreshTimerId = setInterval(() => {
    tryRefreshToken();
  }, TOKEN_REFRESH_INTERVAL);
}

/** Stop the periodic token refresh timer. */
export function stopTokenRefreshTimer(): void {
  if (refreshTimerId !== null) {
    clearInterval(refreshTimerId);
    refreshTimerId = null;
  }
}

/**
 * Refresh the token when the user returns to the tab after being away.
 * Should be called once on app startup.
 */
export function setupVisibilityRefresh(): void {
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') {
      tryRefreshToken();
    }
  });
}

/** Upload a file directly to S3 using a presigned PUT URL. */
export async function uploadToS3(
  presignedUrl: string,
  file: File,
  onProgress?: (pct: number) => void,
  extraHeaders?: Record<string, string>,
): Promise<{ ok: boolean; error: string | null }> {
  return new Promise((resolve) => {
    const xhr = new XMLHttpRequest();
    xhr.open('PUT', presignedUrl, true);
    xhr.setRequestHeader('Content-Type', file.type);
    if (extraHeaders) {
      for (const [k, v] of Object.entries(extraHeaders)) {
        xhr.setRequestHeader(k, v);
      }
    }

    if (onProgress) {
      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100));
      });
    }

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve({ ok: true, error: null });
      } else {
        resolve({ ok: false, error: `Upload failed: ${xhr.status}` });
      }
    };
    xhr.onerror = () => resolve({ ok: false, error: 'Network error during upload' });
    xhr.send(file);
  });
}

export const api = {
  get: <T>(path: string, options?: RequestInit) => request<T>(path, options),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'PUT', body: body ? JSON.stringify(body) : undefined }),
  patch: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'PATCH', body: body ? JSON.stringify(body) : undefined }),
  del: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'DELETE', body: body ? JSON.stringify(body) : undefined }),
};
