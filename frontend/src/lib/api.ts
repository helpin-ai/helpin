export const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080/api';

interface ApiResponse<T> {
  data: T | null;
  error: string | null;
  status?: number;
  isNetworkError?: boolean;
}

async function request<T>(path: string, options: RequestInit = {}): Promise<ApiResponse<T>> {
  const token = localStorage.getItem('access_token');
  try {
    const res = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...options.headers,
      },
    });

    if (res.status === 401 && !path.startsWith('/auth/')) {
      // Try refresh (skip for auth endpoints — a 401 there means bad credentials)
      const refreshed = await tryRefreshToken();
      if (refreshed) {
        // Retry with new token
        const newToken = localStorage.getItem('access_token');
        const retryRes = await fetch(`${API_BASE}${path}`, {
          ...options,
          headers: {
            'Content-Type': 'application/json',
            ...(newToken ? { Authorization: `Bearer ${newToken}` } : {}),
            ...options.headers,
          },
        });
        if (!retryRes.ok) {
          const err = await retryRes.json().catch(() => ({ error: retryRes.statusText }));
          return { data: null, error: err.error || retryRes.statusText };
        }
        if (retryRes.status === 204) return { data: null as T, error: null };
        const data = await retryRes.json();
        return { data, error: null };
      }
      // Refresh failed, clear tokens
      localStorage.removeItem('access_token');
      localStorage.removeItem('refresh_token');
      localStorage.removeItem('remember_me');
      stopTokenRefreshTimer();
      window.location.href = '/login';
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

async function tryRefreshToken(): Promise<boolean> {
  const refreshToken = localStorage.getItem('refresh_token');
  if (!refreshToken) return false;
  try {
    const res = await fetch(`${API_BASE}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
    if (!res.ok) return false;
    const data = await res.json();
    localStorage.setItem('access_token', data.access_token);
    localStorage.setItem('refresh_token', data.refresh_token);
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
    const token = localStorage.getItem('refresh_token');
    if (token) {
      tryRefreshToken();
    }
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
    if (document.visibilityState === 'visible' && localStorage.getItem('refresh_token')) {
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
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'PUT', body: body ? JSON.stringify(body) : undefined }),
  patch: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'PATCH', body: body ? JSON.stringify(body) : undefined }),
  del: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
};
