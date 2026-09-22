import { API_BASE } from '@/lib/api'

export interface CustomerPortalBranding {
  name: string
  logo_url?: string | null
}

export interface CustomerPortalConfiguration {
  enabled: boolean
  requests_only: boolean
  intake_enabled: boolean
  branding: CustomerPortalBranding
}

export interface CustomerPortalCustomer {
  id: string
  email: string
  name?: string | null
}

export interface CustomerPortalSession {
  customer: CustomerPortalCustomer
}

export interface CustomerPortalRequest {
  id: string
  subject: string
  status: string
  updated_at: string
}

export interface CreateCustomerPortalRequestInput {
  subject: string
  message: string
}

export class CustomerPortalApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'CustomerPortalApiError'
    this.status = status
  }
}

async function portalRequest<T>(
  slug: string,
  path: string,
  init?: RequestInit,
): Promise<T> {
  const headers = new Headers(init?.headers)
  headers.set('Accept', 'application/json')
  if (init?.body) headers.set('Content-Type', 'application/json')

  const response = await fetch(
    `${API_BASE}/public/portal/${encodeURIComponent(slug)}${path}`,
    {
      ...init,
      credentials: 'include',
      headers,
    },
  )

  if (!response.ok) {
    let message = 'Something went wrong. Please try again.'
    try {
      const body = (await response.json()) as { error?: string; message?: string }
      message = body.error || body.message || message
    } catch {
      // The public API may deliberately return an empty body for unavailable portals.
    }
    throw new CustomerPortalApiError(response.status, message)
  }

  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export const customerPortalService = {
  configuration: (slug: string) =>
    portalRequest<CustomerPortalConfiguration>(slug, ''),
  session: (slug: string) =>
    portalRequest<CustomerPortalSession>(slug, '/session'),
  requestMagicLink: (slug: string, email: string) =>
    portalRequest<void>(slug, '/auth/magic-link', {
      method: 'POST',
      body: JSON.stringify({ email }),
    }),
  exchangeMagicLink: (slug: string, token: string) =>
    portalRequest<CustomerPortalSession>(slug, '/auth/exchange', {
      method: 'POST',
      body: JSON.stringify({ token }),
    }),
  signOut: (slug: string) =>
    portalRequest<void>(slug, '/session', { method: 'DELETE' }),
  requests: (slug: string) =>
    portalRequest<CustomerPortalRequest[]>(slug, '/requests'),
  createRequest: (slug: string, input: CreateCustomerPortalRequestInput) =>
    portalRequest<CustomerPortalRequest>(slug, '/requests', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
}
