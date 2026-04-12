import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

function normalizeHostname(hostname: string) {
  return hostname.replace(/:\d+$/, '').trim().toLowerCase()
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

  const hostedSubdomain = resolveHostedSubdomain(host)
  if (hostedSubdomain) {
    return {
      subdomain: overrideParam || hostedSubdomain,
      basepath: '',
    }
  }

  // Dev / localhost / IP address / dev-*.helpin.ai fallback
  if (
    host === 'localhost' ||
    host === '127.0.0.1' ||
    /^\d{1,3}(\.\d{1,3}){3}$/.test(host) ||
    /^dev-\w+\.helpin\.ai$/.test(host)
  ) {
    return {
      subdomain: overrideParam || import.meta.env.VITE_HC_SUBDOMAIN || 'demo',
      basepath: '',
    }
  }

  // Custom domain — pass hostname as-is; backend resolves it
  return { subdomain: overrideParam || host, basepath: '' }
}
