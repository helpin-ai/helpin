import { API_BASE } from '@/lib/api'

export interface CustomerPortalBranding {
  name: string
  logo_url?: string | null
}

export interface CustomerPortalConfiguration {
  enabled: boolean
  requests_only: boolean
  intake_enabled: boolean
  file_uploads_enabled: boolean
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
  reference: string
  subject: string
  status: 'active' | 'waiting_on_customer' | 'resolved'
  last_activity_at: string | null
}

export interface CustomerPortalRequestDetail extends CustomerPortalRequest {
  can_reply: boolean
  messages: Array<{
    id: string
    content: string
    sender_type: string
    sender_name?: string | null
    via_channel?: string
    created_at: string
    attachments?: Array<{ id: string; file_name: string; file_type: string; file_size: number; url: string }>
  }>
}

export type CustomerPortalRequestFilter = 'all' | CustomerPortalRequest['status']

interface CreatedCustomerPortalRequest {
  id: string
  subject: string
  status: string
  updated_at: string
}

export interface CreateCustomerPortalRequestInput {
  subject: string
  message: string
  attachment_ids?: string[]
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
  requests: (slug: string, filter: CustomerPortalRequestFilter = 'all') =>
    portalRequest<CustomerPortalRequest[]>(slug, `/requests${filter === 'all' ? '' : `?status=${filter}`}`),
  requestDetail: (slug: string, reference: string) =>
    portalRequest<CustomerPortalRequestDetail>(slug, `/requests/${encodeURIComponent(reference)}`),
  reply: (slug: string, reference: string, content: string, attachment_ids: string[] = []) =>
    portalRequest<CustomerPortalRequestDetail>(slug, `/requests/${encodeURIComponent(reference)}/replies`, {
      method: 'POST', body: JSON.stringify({ content, attachment_ids }),
    }),
  uploadAttachment: async (slug: string, file: File, reference?: string) => {
    if (file.size <= 0 || file.size > 100 * 1024 * 1024) throw new Error('File must be under 100 MB.')
    const path = reference ? `/requests/${encodeURIComponent(reference)}/attachments` : '/attachments'
    const result = await portalRequest<{ attachment: { id: string }; upload_url: string }>(slug, path, {
      method: 'POST', body: JSON.stringify({ file_name: file.name, file_size: file.size, content_type: file.type }),
    })
    const uploaded = await fetch(result.upload_url, { method: 'PUT', headers: { 'Content-Type': file.type }, body: file })
    if (!uploaded.ok) throw new Error('File upload failed.')
    await portalRequest<void>(slug, `${path}/${encodeURIComponent(result.attachment.id)}/confirm`, { method: 'PATCH' })
    return { id: result.attachment.id, name: file.name }
  },
  createRequest: (slug: string, input: CreateCustomerPortalRequestInput) =>
    portalRequest<CreatedCustomerPortalRequest>(slug, '/requests', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
}
