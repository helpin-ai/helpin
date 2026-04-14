import { API_BASE } from '@/lib/api'

export const WEB_BASE_URL =
  import.meta.env.VITE_WEB_BASE_URL || API_BASE.replace(/\/api\/?$/, '')

export function buildWorkspaceWebUrl(pathname: string) {
  return new URL(pathname, `${WEB_BASE_URL.replace(/\/+$/, '')}/`).toString()
}

export function openWorkspaceWebUrl(pathname: string) {
  const url = buildWorkspaceWebUrl(pathname)
  if (typeof window !== 'undefined') {
    window.open(url, '_blank', 'noopener,noreferrer')
  }
  return url
}
