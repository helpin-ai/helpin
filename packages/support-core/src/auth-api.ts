import { clearSession, getAccessToken, getRefreshToken, getRememberMe, writeSession } from './session'

export interface ApiResponse<T> {
  data: T | null
  error: string | null
  status?: number
  isNetworkError?: boolean
}

interface ApiClientOptions {
  onUnauthorized?: () => void
}

interface ApiClient {
  get: <T>(path: string) => Promise<ApiResponse<T>>
  post: <T>(path: string, body?: unknown) => Promise<ApiResponse<T>>
  put: <T>(path: string, body?: unknown) => Promise<ApiResponse<T>>
  patch: <T>(path: string, body?: unknown) => Promise<ApiResponse<T>>
  del: <T>(path: string) => Promise<ApiResponse<T>>
}

const TOKEN_REFRESH_INTERVAL = 5 * 60 * 1000

let refreshTimerId: ReturnType<typeof setInterval> | null = null
let visibilityCleanup: (() => void) | null = null

function resolveRequestUrl(apiBase: string, path: string): string {
  return `${apiBase}${path}`
}

function mergeAuthHeaders(token: string | null, headers?: HeadersInit): HeadersInit {
  return {
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...headers,
  }
}

function mergeJsonHeaders(token: string | null, headers?: HeadersInit): HeadersInit {
  return {
    'Content-Type': 'application/json',
    ...mergeAuthHeaders(token, headers),
  }
}

async function parseError(response: Response): Promise<{ error: string }> {
  return response.json().catch(() => ({ error: response.statusText }))
}

async function tryRefreshToken(apiBase: string): Promise<boolean> {
  const refreshToken = getRefreshToken()
  if (!refreshToken) {
    return false
  }

  try {
    const response = await fetch(resolveRequestUrl(apiBase, '/auth/refresh'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    })

    if (!response.ok) {
      return false
    }

    const data = await response.json()
    await writeSession({
      accessToken: data.access_token,
      refreshToken: data.refresh_token,
      rememberMe: getRememberMe(),
    })
    return true
  } catch {
    return false
  }
}

async function fetchWithAuthRetry(
  apiBase: string,
  path: string,
  init: RequestInit = {},
  onUnauthorized?: () => void,
): Promise<Response> {
  const token = getAccessToken()
  let response = await fetch(resolveRequestUrl(apiBase, path), {
    ...init,
    headers: mergeAuthHeaders(token, init.headers),
  })

  if (response.status !== 401 || path.startsWith('/auth/')) {
    return response
  }

  const refreshed = await tryRefreshToken(apiBase)
  if (!refreshed) {
    await clearSession()
    stopTokenRefreshTimer()
    onUnauthorized?.()
    return response
  }

  response = await fetch(resolveRequestUrl(apiBase, path), {
    ...init,
    headers: mergeAuthHeaders(getAccessToken(), init.headers),
  })

  if (response.status === 401) {
    await clearSession()
    stopTokenRefreshTimer()
    onUnauthorized?.()
  }

  return response
}

async function request<T>(
  apiBase: string,
  path: string,
  options: RequestInit = {},
  onUnauthorized?: () => void,
): Promise<ApiResponse<T>> {
  try {
    const response = await fetchWithAuthRetry(
      apiBase,
      path,
      {
        ...options,
        headers: mergeJsonHeaders(getAccessToken(), options.headers),
      },
      onUnauthorized,
    )

    if (!response.ok) {
      const errorPayload = await parseError(response)
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

export function createApiClient(apiBase: string, options: ApiClientOptions = {}): ApiClient {
  return {
    get: <T>(path: string) => request<T>(apiBase, path, {}, options.onUnauthorized),
    post: <T>(path: string, body?: unknown) =>
      request<T>(apiBase, path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }, options.onUnauthorized),
    put: <T>(path: string, body?: unknown) =>
      request<T>(apiBase, path, { method: 'PUT', body: body ? JSON.stringify(body) : undefined }, options.onUnauthorized),
    patch: <T>(path: string, body?: unknown) =>
      request<T>(apiBase, path, { method: 'PATCH', body: body ? JSON.stringify(body) : undefined }, options.onUnauthorized),
    del: <T>(path: string) => request<T>(apiBase, path, { method: 'DELETE' }, options.onUnauthorized),
  }
}

export async function fetchWithSessionAuth(
  apiBase: string,
  path: string,
  init: RequestInit = {},
  options: ApiClientOptions = {},
): Promise<Response> {
  const token = getAccessToken()
  return fetchWithAuthRetry(
    apiBase,
    path,
    {
      ...init,
      headers: mergeAuthHeaders(token, init.headers),
    },
    options.onUnauthorized,
  )
}

export function buildWorkspaceWebSocketUrl(apiBase: string, workspaceId: string): string {
  const token = getAccessToken()
  if (!token || !workspaceId) {
    return ''
  }

  const base = apiBase.replace(/^http/, 'ws').replace(/\/api\/?$/, '/api')
  return `${base}/ws?token=${encodeURIComponent(token)}&workspace_id=${encodeURIComponent(workspaceId)}`
}

export function startTokenRefreshTimer(apiBase: string): void {
  stopTokenRefreshTimer()
  refreshTimerId = setInterval(() => {
    if (getRefreshToken()) {
      void tryRefreshToken(apiBase)
    }
  }, TOKEN_REFRESH_INTERVAL)
}

export function stopTokenRefreshTimer(): void {
  if (refreshTimerId !== null) {
    clearInterval(refreshTimerId)
    refreshTimerId = null
  }
}

export function setupVisibilityRefresh(apiBase: string): () => void {
  if (typeof document === 'undefined') {
    return () => {}
  }

  if (visibilityCleanup) {
    return visibilityCleanup
  }

  const handler = () => {
    if (document.visibilityState === 'visible' && getRefreshToken()) {
      void tryRefreshToken(apiBase)
    }
  }

  document.addEventListener('visibilitychange', handler)
  visibilityCleanup = () => {
    document.removeEventListener('visibilitychange', handler)
    visibilityCleanup = null
  }
  return visibilityCleanup
}
