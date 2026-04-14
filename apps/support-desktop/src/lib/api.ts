import {
  configureSupportApi,
  createApiClient,
  setupVisibilityRefresh as setupSharedVisibilityRefresh,
  startTokenRefreshTimer as startSharedTokenRefreshTimer,
  stopTokenRefreshTimer as stopSharedTokenRefreshTimer,
} from '@helpin-ai/support-core'

export const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080/api'

const handleUnauthorized = () => {
  if (typeof window !== 'undefined') {
    window.location.href = '/login'
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
