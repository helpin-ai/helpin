import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Extract the help center subdomain from the current hostname.
 * Supports:
 *  - {subdomain}.helpin.ai  → subdomain
 *  - localhost / dev         → reads from VITE_HC_SUBDOMAIN env or defaults to "demo"
 *  - custom domain           → returns full hostname (resolved by backend)
 */
export function resolveSubdomain(): string {
  const hostname = window.location.hostname

  // Dev / localhost fallback
  if (hostname === 'localhost' || hostname === '127.0.0.1') {
    return import.meta.env.VITE_HC_SUBDOMAIN || 'demo'
  }

  // *.helpin.ai pattern
  const helpin = hostname.match(/^(.+)\.helpin\.ai$/)
  if (helpin?.[1]) {
    return helpin[1]
  }

  // Custom domain — pass hostname as-is; backend resolves it
  return hostname
}
