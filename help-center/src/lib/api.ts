import { prefixBasepath } from '@/lib/pathUtils'
import { resolveHelpCenterContext } from '@/lib/utils'

interface ApiResponse<T> {
  data: T | null
  error: string | null
  status: number
}

function getBaseUrl() {
  if (typeof window === 'undefined') {
    return (
      process.env.INTERNAL_API_URL ||
      import.meta.env.INTERNAL_API_URL ||
      import.meta.env.VITE_API_URL ||
      'http://127.0.0.1:8080/api'
    )
  }

  const ctx = resolveHelpCenterContext(
    window.location.hostname,
    window.location.pathname,
    window.location.search,
  )
  if (ctx.basepath) {
    return prefixBasepath(ctx.basepath, '/api')
  }

  if (import.meta.env.VITE_API_URL) {
    return import.meta.env.VITE_API_URL
  }

  return prefixBasepath(ctx.basepath, '/api')
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<ApiResponse<T>> {
  try {
    const res = await fetch(`${getBaseUrl()}${path}`, {
      method,
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined,
    })

    if (!res.ok) {
      const text = await res.text()
      let message = `Request failed (${res.status})`
      try {
        const json = JSON.parse(text)
        message = json.error || json.message || message
      } catch {
        if (text) message = text
      }
      return { data: null, error: message, status: res.status }
    }

    const data = (await res.json()) as T
    return { data, error: null, status: res.status }
  } catch (err) {
    return {
      data: null,
      error: err instanceof Error ? err.message : 'Network error',
      status: 0,
    }
  }
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
}
