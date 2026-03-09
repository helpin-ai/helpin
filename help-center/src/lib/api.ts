const BASE_URL = import.meta.env.VITE_API_URL || '/api'

interface ApiResponse<T> {
  data: T | null
  error: string | null
  status: number
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<ApiResponse<T>> {
  try {
    const res = await fetch(`${BASE_URL}${path}`, {
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
