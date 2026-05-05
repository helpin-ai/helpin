type RouterLocationLike = {
  pathname?: string
  searchStr?: string
  search?: unknown
  hash?: string
  href?: string
}

const AUTH_REDIRECT_BLOCKLIST = new Set([
  '/login',
  '/logout',
  '/register',
  '/forgot-password',
  '/reset-password',
])

function appOrigin(): string {
  if (typeof window !== 'undefined' && window.location?.origin) {
    return window.location.origin
  }
  return 'https://app.helpin.ai'
}

function normalizePathname(pathname: string): string {
  const normalized = pathname.replace(/\/+$/, '')
  return normalized === '' ? '/' : normalized
}

export function normalizeSafeAppRedirect(value: string | null | undefined): string | null {
  const raw = value?.trim()
  if (!raw) return null
  if (raw.startsWith('//') || raw.startsWith('\\')) return null

  let url: URL
  try {
    url = new URL(raw, appOrigin())
  } catch {
    return null
  }

  if (url.origin !== appOrigin()) return null
  const pathname = normalizePathname(url.pathname)
  if (AUTH_REDIRECT_BLOCKLIST.has(pathname)) return null
  return `${url.pathname}${url.search}${url.hash}`
}

export function currentPathForLoginRedirect(location?: RouterLocationLike): string {
  if (location?.href) {
    const normalized = normalizeSafeAppRedirect(location.href)
    if (normalized) return normalized
  }

  const pathname = location?.pathname
    ?? (typeof window !== 'undefined' ? window.location.pathname : '/')
  const search = typeof location?.searchStr === 'string'
    ? location.searchStr
    : typeof window !== 'undefined'
      ? window.location.search
      : ''
  const hash = typeof location?.hash === 'string'
    ? location.hash
    : typeof window !== 'undefined'
      ? window.location.hash
      : ''

  return `${pathname || '/'}${search || ''}${hash || ''}`
}

export function loginRedirectFromSearch(search = typeof window !== 'undefined' ? window.location.search : ''): string | null {
  return normalizeSafeAppRedirect(new URLSearchParams(search).get('redirect'))
}

export function buildLoginPathForRedirect(redirect: string | null | undefined): string {
  const normalized = normalizeSafeAppRedirect(redirect)
  if (!normalized) return '/login'
  return `/login?redirect=${encodeURIComponent(normalized)}`
}

export function buildLoginPathForCurrentLocation(location?: RouterLocationLike): string {
  return buildLoginPathForRedirect(currentPathForLoginRedirect(location))
}
