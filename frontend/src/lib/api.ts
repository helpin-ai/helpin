import {
  configureSupportApi,
  createApiClient,
  fetchWithSessionAuth,
  setupVisibilityRefresh as setupSharedVisibilityRefresh,
  startTokenRefreshTimer as startSharedTokenRefreshTimer,
  stopTokenRefreshTimer as stopSharedTokenRefreshTimer,
} from '@helpin-ai/support-core'
import { buildLoginPathForCurrentLocation, storeRedirectAfterLogin } from '@/lib/authRedirect'

export const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080/api'

export type { ApiResponse } from '@helpin-ai/support-core'

const handleUnauthorized = () => {
  if (typeof window !== 'undefined') {
    storeRedirectAfterLogin()
    window.location.href = buildLoginPathForCurrentLocation()
  }
}

export const api = createApiClient(API_BASE, { onUnauthorized: handleUnauthorized })
configureSupportApi(api)

export function startTokenRefreshTimer(): void {
  startSharedTokenRefreshTimer(API_BASE)
}

export function stopTokenRefreshTimer(): void {
  stopSharedTokenRefreshTimer()
}

export function setupVisibilityRefresh(): void {
  setupSharedVisibilityRefresh(API_BASE)
}

/** Upload a file directly to S3 using a presigned PUT URL. */
export async function uploadToS3(
  presignedUrl: string,
  file: File,
  onProgress?: (pct: number) => void,
  extraHeaders?: Record<string, string>,
): Promise<{ ok: boolean; error: string | null }> {
  return new Promise((resolve) => {
    const xhr = new XMLHttpRequest()
    xhr.open('PUT', presignedUrl, true)
    xhr.setRequestHeader('Content-Type', file.type)
    if (extraHeaders) {
      for (const [key, value] of Object.entries(extraHeaders)) {
        xhr.setRequestHeader(key, value)
      }
    }

    if (onProgress) {
      xhr.upload.addEventListener('progress', (event) => {
        if (event.lengthComputable) {
          onProgress(Math.round((event.loaded / event.total) * 100))
        }
      })
    }

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve({ ok: true, error: null })
      } else {
        resolve({ ok: false, error: `Upload failed: ${xhr.status}` })
      }
    }
    xhr.onerror = () => resolve({ ok: false, error: 'Network error during upload' })
    xhr.send(file)
  })
}

export { fetchWithSessionAuth }
