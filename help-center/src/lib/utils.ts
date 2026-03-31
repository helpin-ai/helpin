import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

function normalizeHostname(hostname: string) {
  return hostname.replace(/:\d+$/, '').trim().toLowerCase()
}

/**
 * Extract the help center subdomain from the current hostname.
 * Supports:
 *  - ?subdomain=X query param → X (dev override)
 *  - {subdomain}.helpin.ai    → subdomain
 *  - localhost / IP addresses  → reads from VITE_HC_SUBDOMAIN env or defaults to "demo"
 *  - custom domain             → returns full hostname (resolved by backend)
 */
export function resolveSubdomain(hostname?: string, search?: string): string {
  const searchValue =
    search ??
    (typeof window !== 'undefined' ? window.location.search : '')

  // Dev override via query param (?subdomain=contentstudio)
  const params = new URLSearchParams(searchValue)
  const override = params.get('subdomain')
  if (override) return override

  const resolvedHostname = normalizeHostname(
    hostname ??
      (typeof window !== 'undefined' ? window.location.hostname : ''),
  )

  if (!resolvedHostname) {
    return import.meta.env.VITE_HC_SUBDOMAIN || 'demo'
  }

  // Dev / localhost / IP address fallback
  if (
    resolvedHostname === 'localhost' ||
    resolvedHostname === '127.0.0.1' ||
    /^\d{1,3}(\.\d{1,3}){3}$/.test(resolvedHostname)
  ) {
    return import.meta.env.VITE_HC_SUBDOMAIN || 'demo'
  }

  // Base help-center domains — not a workspace subdomain
  if (
    resolvedHostname === 'helpcenter.helpin.ai' ||
    resolvedHostname === 'helpcenter-stage.helpin.ai'
  ) {
    return ''
  }

  // *.helpin.ai pattern — extract subdomain
  const helpin = resolvedHostname.match(/^(.+)\.helpin\.ai$/)
  if (helpin?.[1]) {
    return helpin[1]
  }

  // Custom domain — pass hostname as-is; backend resolves it
  return resolvedHostname
}
