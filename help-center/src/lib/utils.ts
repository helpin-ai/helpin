import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

function normalizeHostname(hostname: string) {
  return hostname.replace(/:\d+$/, '').trim().toLowerCase()
}

const PATH_HOST_TENANT_ROOTS = new Set([
  'helpin.center',
  'stage.helpin.center',
])

/**
 * Returns true if the given hostname is a multi-tenant root that serves help
 * centers under a path prefix (e.g. helpin.center/{slug}/...).
 */
export function isPathHostTenantRoot(hostname: string): boolean {
  return PATH_HOST_TENANT_ROOTS.has(normalizeHostname(hostname))
}

function extractFirstPathSegment(pathname: string): string {
  if (!pathname) return ''
  const trimmed = pathname.replace(/^\/+/, '')
  const slash = trimmed.indexOf('/')
  return slash === -1 ? trimmed : trimmed.slice(0, slash)
}

export interface HelpCenterContext {
  /** The workspace identifier the backend understands (subdomain or hostname). */
  subdomain: string
  /** Router basepath when serving under a path prefix; '' when at root. */
  basepath: string
}

/**
 * Resolve the help-center request context for a given hostname + pathname.
 *
 * Routing modes:
 *  - helpin.center / stage.helpin.center → path-based: first path segment is the
 *    workspace slug; basepath is `/{slug}`
 *  - localhost / IP                      → dev mode, slug from VITE_HC_SUBDOMAIN
 *  - custom domain                       → backend resolves by full hostname
 */
export function resolveHelpCenterContext(
  hostname: string,
  pathname: string,
  search?: string,
): HelpCenterContext {
  // Dev override via query param wins everywhere
  const searchValue =
    search ?? (typeof window !== 'undefined' ? window.location.search : '')
  const overrideParam = new URLSearchParams(searchValue).get('subdomain')

  const host = normalizeHostname(hostname)

  if (!host) {
    return {
      subdomain: overrideParam || import.meta.env.VITE_HC_SUBDOMAIN || 'demo',
      basepath: '',
    }
  }

  // Multi-tenant root: extract slug from first path segment
  if (isPathHostTenantRoot(host)) {
    const slug = overrideParam || extractFirstPathSegment(pathname)
    if (!slug) {
      return { subdomain: '', basepath: '' }
    }
    return { subdomain: slug, basepath: `/${slug}` }
  }

  // Dev / localhost / IP address fallback
  if (
    host === 'localhost' ||
    host === '127.0.0.1' ||
    /^\d{1,3}(\.\d{1,3}){3}$/.test(host)
  ) {
    return {
      subdomain: overrideParam || import.meta.env.VITE_HC_SUBDOMAIN || 'demo',
      basepath: '',
    }
  }

  // Custom domain — pass hostname as-is; backend resolves it
  return { subdomain: overrideParam || host, basepath: '' }
}

