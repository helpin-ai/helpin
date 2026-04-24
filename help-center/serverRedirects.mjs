function prefixBasepath(basepath, path) {
  if (!path) {
    return basepath || '/'
  }

  if (/^[a-z]+:/i.test(path) || path.startsWith('//')) {
    return path
  }

  const normalizedBasepath = basepath ? basepath.replace(/\/+$/, '') : ''
  const normalizedPath = path.startsWith('/') ? path : `/${path}`

  if (!normalizedBasepath) {
    return normalizedPath
  }

  if (normalizedPath === '/') {
    return `${normalizedBasepath}/`
  }

  if (
    normalizedPath === normalizedBasepath ||
    normalizedPath.startsWith(`${normalizedBasepath}/`)
  ) {
    return normalizedPath
  }

  return `${normalizedBasepath}${normalizedPath}`
}

export function shouldAttemptRedirectResolution(method, pathname) {
  const normalizedMethod = (method || 'GET').toUpperCase()
  if (!['GET', 'HEAD'].includes(normalizedMethod)) {
    return false
  }

  const segments = (pathname || '/').split('/').filter(Boolean)
  const contentIndex = segments.length > 0 && /^[a-z]{2}(?:-[a-z0-9]+)?$/i.test(segments[0])
    ? 1
    : 0
  const firstContentSegment = segments[contentIndex]
  const contentKey = segments[contentIndex + 1] || ''
  const hasPublicId = /(?:^|-[0-9a-f]{8})$/i.test(contentKey)

  if ((firstContentSegment === 'articles' || firstContentSegment === 'c') && hasPublicId) {
    return false
  }

  return !(
    pathname === '/api' ||
    pathname.startsWith('/api/') ||
    pathname.startsWith('/assets/') ||
    pathname.startsWith('/preview/') ||
    pathname === '/healthz'
  )
}

export function buildRedirectLocation(targetPath, { basepath = '', search = '' } = {}) {
  const normalizedTarget = prefixBasepath(basepath, targetPath || '/')
  if (!search) {
    return normalizedTarget
  }

  if (/^[a-z]+:/i.test(normalizedTarget) || normalizedTarget.startsWith('//')) {
    const url = new URL(normalizedTarget)
    if (!url.search) {
      url.search = search
    }
    return url.toString()
  }

  if (normalizedTarget.includes('?')) {
    return normalizedTarget
  }

  return `${normalizedTarget}${search}`
}

export function buildRedirectResolverURL(apiBase, subdomain, pathname) {
  const trimmedApiBase = (apiBase || '').replace(/\/+$/, '')
  const normalizedPath = pathname === '/' ? '' : pathname.replace(/^\/+/, '')
  const encodedPath = encodeURI(normalizedPath)
  return `${trimmedApiBase}/hc/${encodeURIComponent(subdomain)}/resolve/${encodedPath}`
}

function normalizeRedirectStatus(status) {
  if (status === 301 || status === 308) {
    return status
  }
  return 301
}

export async function resolvePublicRedirect({
  apiBase,
  subdomain,
  pathname,
  basepath = '',
  search = '',
  fetchImpl = fetch,
}) {
  if (!apiBase || !subdomain || !pathname) {
    return null
  }

  const response = await fetchImpl(buildRedirectResolverURL(apiBase, subdomain, pathname), {
    headers: {
      Accept: 'application/json',
    },
  })

  if (response.status === 404) {
    return null
  }

  if (!response.ok) {
    throw new Error(`redirect resolver failed with status ${response.status}`)
  }

  const payload = await response.json()
  if (!payload?.redirect || typeof payload.target !== 'string') {
    return null
  }

  return {
    status: normalizeRedirectStatus(payload.status),
    location: buildRedirectLocation(payload.target, { basepath, search }),
  }
}
