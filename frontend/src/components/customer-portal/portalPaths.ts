/**
 * The portal has two addresses. In the app it lives at /portal/{slug}
 * (/portal/{slug}/requests/{reference}); on a workspace's help center it is
 * mounted at /requests with short paths (/requests/{reference}). These map a
 * page between the two; the router always works with the app paths.
 */

const PAGES = new Set(['/new', '/sign-in', '/callback'])

function appPrefix(slug: string) {
  return `/portal/${encodeURIComponent(slug)}`
}

/**
 * portalMountedPath maps an app path to its path below the mounted portal
 * ("" for the list), or null when it is not one of the slug's portal pages.
 */
export function portalMountedPath(appPath: string, slug: string) {
  const prefix = appPrefix(slug)
  if (appPath === prefix || appPath === `${prefix}/`) return ''
  if (!appPath.startsWith(`${prefix}/`)) return null
  const rest = appPath.slice(prefix.length)
  if (PAGES.has(rest)) return rest
  const reference = /^\/requests\/([^/]+)$/.exec(rest)
  return reference ? `/${reference[1]}` : null
}

/** portalAppPath maps a path below the mounted portal to its app path. */
export function portalAppPath(mountedPath: string, slug: string) {
  const prefix = appPrefix(slug)
  const rest = mountedPath.replace(/\/+$/, '')
  if (rest === '') return prefix
  if (PAGES.has(rest)) return `${prefix}${rest}`
  if (/^\/[^/]+$/.test(rest)) return `${prefix}/requests${rest}`
  return null
}

/**
 * portalPublicRedirect returns where to send a visitor who opened the portal
 * in the app when it is served on the workspace's help center, keeping the
 * page, query, and hash. It returns null when the portal lives in the app.
 */
export function portalPublicRedirect(publicUrl: string | undefined, location: Pick<Location, 'origin' | 'pathname' | 'search' | 'hash'>, slug: string) {
  if (!publicUrl) return null
  let target: URL
  try {
    target = new URL(publicUrl)
  } catch {
    return null
  }
  if (target.pathname.startsWith('/portal/')) return null
  const path = portalMountedPath(location.pathname, slug)
  if (path === null) return null
  const base = `${target.origin}${target.pathname.replace(/\/+$/, '')}`
  if (base === `${location.origin}${location.pathname}`) return null
  return `${base}${path}${location.search}${location.hash}`
}
