export const API_BASE = import.meta.env.VITE_API_URL || '/api'

interface ApiResponse<T> {
  data: T | null
  error: string | null
  status?: number
  isNetworkError?: boolean
}

async function request<T>(path: string, options: RequestInit = {}): Promise<ApiResponse<T>> {
  const token = localStorage.getItem('access_token')

  try {
    const response = await fetch(`${API_BASE}${path}`, {
      ...options,
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...options.headers,
      },
    })

    if (response.status === 401 && shouldAttemptRefresh(path)) {
      const refreshed = await tryRefreshToken()

      if (refreshed) {
        const nextToken = localStorage.getItem('access_token')
        const retryResponse = await fetch(`${API_BASE}${path}`, {
          ...options,
          credentials: 'include',
          headers: {
            'Content-Type': 'application/json',
            ...(nextToken ? { Authorization: `Bearer ${nextToken}` } : {}),
            ...options.headers,
          },
        })

        if (!retryResponse.ok) {
          const errorPayload = await retryResponse.json().catch(() => ({ error: retryResponse.statusText }))
          if (retryResponse.status === 403 && path.startsWith('/admin/')) {
            clearStoredSession()
            window.location.href = '/admin/forbidden'
          }
          return { data: null, error: errorPayload.error || retryResponse.statusText, status: retryResponse.status }
        }

        if (retryResponse.status === 204) {
          return { data: null as T, error: null, status: 204 }
        }

        return { data: await retryResponse.json(), error: null, status: retryResponse.status }
      }

      if (path === '/auth/me') {
        return { data: null, error: 'Session expired', status: 401 }
      }

      clearStoredSession()
      stopTokenRefreshTimer()
      window.location.href = '/admin/login'
      return { data: null, error: 'Session expired', status: 401 }
    }

    if (!response.ok) {
      const errorPayload = await response.json().catch(() => ({ error: response.statusText }))
      if (response.status === 403 && path.startsWith('/admin/')) {
        clearStoredSession()
        window.location.href = '/admin/forbidden'
      }
      return { data: null, error: errorPayload.error || response.statusText, status: response.status }
    }

    if (response.status === 204) {
      return { data: null as T, error: null, status: 204 }
    }

    return { data: await response.json(), error: null, status: response.status }
  } catch (error) {
    return {
      data: null,
      error: error instanceof Error ? error.message : 'Network error',
      isNetworkError: true,
    }
  }
}

function shouldAttemptRefresh(path: string): boolean {
  return path === '/auth/me' || !path.startsWith('/auth/')
}

function clearStoredSession(): void {
  localStorage.removeItem('access_token')
  localStorage.removeItem('refresh_token')
  localStorage.removeItem('remember_me')
}

async function tryRefreshToken(): Promise<boolean> {
  const refreshToken = localStorage.getItem('refresh_token')

  try {
    const response = await fetch(`${API_BASE}/auth/refresh`, {
      method: 'POST',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(refreshToken ? { refresh_token: refreshToken } : {}),
    })

    if (!response.ok) {
      return false
    }

    await response.json().catch(() => null)
    return true
  } catch {
    return false
  }
}

const TOKEN_REFRESH_INTERVAL = 5 * 60 * 1000

let refreshTimerId: ReturnType<typeof setInterval> | null = null

export function startTokenRefreshTimer(): void {
  stopTokenRefreshTimer()
  refreshTimerId = setInterval(() => {
    void tryRefreshToken()
  }, TOKEN_REFRESH_INTERVAL)
}

export function stopTokenRefreshTimer(): void {
  if (refreshTimerId !== null) {
    clearInterval(refreshTimerId)
    refreshTimerId = null
  }
}

export function setupVisibilityRefresh(): void {
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') {
      void tryRefreshToken()
    }
  })
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
}
