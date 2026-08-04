import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

function normalizeHostname(hostname: string) {
  return hostname.replace(/:\d+$/, '').trim().toLowerCase()
}

function normalizeIdentifier(value?: string | null) {
  const normalized = (value || '').trim().toLowerCase()
  if (!normalized || /^[a-z]+:\/\//i.test(normalized)) return ''
  if (normalized.includes('/') || normalized.includes('\\')) return ''
  return normalized
}

export function normalizeHelpCenterBasepath(value?: string | null) {
  const raw = (value || '').trim()
  if (!raw || raw === '/') return ''

  let decoded = raw
  try {
    decoded = decodeURIComponent(raw)
  } catch {
    return ''
  }

  if (!decoded.startsWith('/')) {
    decoded = `/${decoded}`
  }

  decoded = decoded.replace(/\/+$/, '')
  if (!decoded || decoded === '/') return ''
  if (decoded.includes('//') || decoded.includes('\\')) return ''

  const segments = decoded.split('/').filter(Boolean)
  if (segments.some((segment) => segment === '.' || segment === '..')) {
    return ''
  }

  return decoded
}

const HOSTED_HELP_CENTER_ROOTS = [
  'stage.helpin.center',
  'helpin.center',
]

function resolveHostedSubdomain(hostname: string): string {
  const normalized = normalizeHostname(hostname)

  for (const root of HOSTED_HELP_CENTER_ROOTS) {
    if (normalized === root) {
      return ''
    }
    const suffix = `.${root}`
    if (!normalized.endsWith(suffix)) {
      continue
    }
    const candidate = normalized.slice(0, -suffix.length)
    if (candidate && !candidate.includes('.')) {
      return candidate
    }
  }

  return ''
}

export interface HelpCenterContext {
  /** The workspace identifier the backend understands (subdomain or hostname). */
  subdomain: string
  /** Router basepath. Hosted and custom-domain help centers serve at root. */
  basepath: string
}

interface BrowserHelpCenterContext {
  subdomain?: string
  basepath?: string
}

function readBrowserHelpCenterContext(): HelpCenterContext | null {
  if (typeof window === 'undefined') return null

  const snapshot = (
    window as Window & { __HELPIN_HC_CONTEXT__?: BrowserHelpCenterContext }
  ).__HELPIN_HC_CONTEXT__
  if (!snapshot) return null

  const subdomain = normalizeIdentifier(snapshot.subdomain)
  const basepath = normalizeHelpCenterBasepath(snapshot.basepath)
  if (!subdomain && !basepath) return null

  return {
    subdomain,
    basepath,
  }
}

/**
 * Resolve the help-center request context for a given hostname + pathname.
 *
 * Routing modes:
 *  - <slug>.helpin.center / <slug>.stage.helpin.center → hosted subdomain
 *  - localhost / IP                                    → dev mode via VITE_HC_SUBDOMAIN
 *  - custom domain                                     → backend resolves by full hostname
 */
export function resolveHelpCenterContext(
  hostname: string,
  _pathname: string,
  search?: string,
): HelpCenterContext {
  const browserContext = readBrowserHelpCenterContext()
  if (browserContext?.subdomain) {
    return browserContext
  }

  // Dev override via query param wins everywhere
  const searchValue =
    search ?? (typeof window !== 'undefined' ? window.location.search : '')
  const searchParams = new URLSearchParams(searchValue)
  const overrideParam = normalizeIdentifier(
    searchParams.get('helpin_tenant') || searchParams.get('subdomain'),
  )
  const basepath = normalizeHelpCenterBasepath(
    searchParams.get('helpin_basepath'),
  )

  const host = normalizeHostname(hostname)

  if (!host) {
    return {
      subdomain:
        overrideParam ||
        normalizeIdentifier(import.meta.env.VITE_HC_SUBDOMAIN) ||
        'demo',
      basepath,
    }
  }

  const hostedSubdomain = resolveHostedSubdomain(host)
  if (hostedSubdomain) {
    const hostedOverride =
      overrideParam === hostedSubdomain ? overrideParam : ''
    return {
      subdomain: hostedOverride || hostedSubdomain,
      basepath,
    }
  }

  // Dev / localhost / IP address / dev-*.helpin.ai / dev tunnel fallback
  if (
    host === 'localhost' ||
    host === '127.0.0.1' ||
    /^\d{1,3}(\.\d{1,3}){3}$/.test(host) ||
    /^dev-\w+\.helpin\.ai$/.test(host) ||
    /\.tryunhide\.com$/.test(host)
  ) {
    return {
      subdomain:
        overrideParam ||
        normalizeIdentifier(import.meta.env.VITE_HC_SUBDOMAIN) ||
        'demo',
      basepath,
    }
  }

  // Custom domain — pass hostname as-is; backend resolves it
  return { subdomain: host, basepath }
}
