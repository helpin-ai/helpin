import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Extract the help center subdomain from the current hostname.
 * Supports:
 *  - ?subdomain=X query param → X (dev override)
 *  - {subdomain}.helpin.ai    → subdomain
 *  - localhost / IP addresses  → reads from VITE_HC_SUBDOMAIN env or defaults to "demo"
 *  - custom domain             → returns full hostname (resolved by backend)
 */
export function resolveSubdomain(): string {
  // Dev override via query param (?subdomain=contentstudio)
  const params = new URLSearchParams(window.location.search)
  const override = params.get('subdomain')
  if (override) return override

  const hostname = window.location.hostname

  // Dev / localhost / IP address fallback
  if (
    hostname === 'localhost' ||
    hostname === '127.0.0.1' ||
    /^\d{1,3}(\.\d{1,3}){3}$/.test(hostname)
  ) {
    return import.meta.env.VITE_HC_SUBDOMAIN || 'demo'
  }

  // Base help-center domains — not a workspace subdomain
  if (hostname === 'helpcenter.helpin.ai' || hostname === 'helpcenter-stage.helpin.ai') {
    return ''
  }

  // *.helpin.ai pattern — extract subdomain
  const helpin = hostname.match(/^(.+)\.helpin\.ai$/)
  if (helpin?.[1]) {
    return helpin[1]
  }

  // Custom domain — pass hostname as-is; backend resolves it
  return hostname
}
