import type { LocationRewrite } from '@tanstack/react-router'
import { portalAppPath, portalMountedPath } from '@/components/customer-portal/portalPaths'

/** Where the help center mounted this portal page. */
export interface PortalMount {
  slug: string
  /** The help center's base path ("" at the root, or e.g. "/docs"). */
  basepath: string
}

declare global {
  interface Window {
    /** Injected by the help center server into the portal page. */
    __HELPIN_PORTAL__?: { slug?: unknown; basepath?: unknown }
  }
}

const SLUG = /^[A-Za-z0-9][A-Za-z0-9_-]*$/

function normalizeBasepath(value: unknown) {
  if (typeof value !== 'string') return ''
  const trimmed = value.trim().replace(/\/+$/, '')
  if (!trimmed.startsWith('/') || trimmed.includes('//') || trimmed.split('/').some((part) => part === '..')) return ''
  return trimmed
}

/**
 * readPortalMount reads the portal the help center server injected. In local
 * development, ?slug= (or VITE_PORTAL_DEV_SLUG) stands in for it.
 */
export function readPortalMount(win: Pick<Window, '__HELPIN_PORTAL__' | 'location'> = window, dev = import.meta.env.DEV): PortalMount | null {
  const injected = win.__HELPIN_PORTAL__
  if (injected && typeof injected.slug === 'string' && SLUG.test(injected.slug)) {
    return { slug: injected.slug, basepath: normalizeBasepath(injected.basepath) }
  }
  if (!dev) return null
  const slug = new URLSearchParams(win.location.search).get('slug') || (import.meta.env.VITE_PORTAL_DEV_SLUG as string | undefined) || ''
  return SLUG.test(slug) ? { slug, basepath: '' } : null
}

/** The public path prefix of the mounted portal. */
export function portalMountPrefix(mount: PortalMount) {
  return `${mount.basepath}/requests`
}

/**
 * portalMountRewrite maps the help center's /requests paths onto the portal's
 * /portal/{slug} routes and back, so links render as short public paths.
 */
export function portalMountRewrite(mount: PortalMount): LocationRewrite {
  const prefix = portalMountPrefix(mount)
  return {
    input: ({ url }) => {
      if (url.pathname !== prefix && !url.pathname.startsWith(`${prefix}/`)) return undefined
      const appPath = portalAppPath(url.pathname.slice(prefix.length), mount.slug)
      if (appPath === null) return undefined
      url.pathname = appPath
      return url
    },
    output: ({ url }) => {
      const mounted = portalMountedPath(url.pathname, mount.slug)
      if (mounted === null) return undefined
      url.pathname = `${prefix}${mounted}`
      return url
    },
  }
}
