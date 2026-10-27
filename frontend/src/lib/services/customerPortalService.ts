import { API_BASE, uploadToS3 } from '@/lib/api'
import type { AssistantResult, AssistantSessionView, AssistantTurn } from '@helpin-ai/shared'
import type { SupportAttachmentPolicy } from '@/lib/supportAttachmentFiles'

// The app calls the API directly. On a help center the portal calls the
// help center's own /api, so its session cookie stays first-party.
let portalApiBase = API_BASE

/** setCustomerPortalApiBase points portal API calls at another API root. */
export function setCustomerPortalApiBase(base: string) {
  portalApiBase = base.replace(/\/+$/, '')
}

export interface CustomerPortalBranding {
  name: string
  logo_url?: string | null
  /** Brand colour from the chat widget, as a CSS colour. */
  brand_color?: string | null
  /** The one name shown on every AI reply. */
  assistant_name?: string | null
}

export interface CustomerPortalConfiguration {
  enabled: boolean
  intake_enabled: boolean
  anonymous_intake_enabled: boolean
  file_uploads_enabled: boolean
  attachments?: CustomerPortalAttachmentPolicy
  branding: CustomerPortalBranding
  /** Where customers reach this portal; the app's /portal path redirects there. */
  public_url?: string
  /** Whether the portal offers the Ask AI help assistant. */
  assistant_enabled?: boolean
}

/** Server upload rules plus the portal's per-message file limit. */
export interface CustomerPortalAttachmentPolicy extends SupportAttachmentPolicy {
  max_files: number
}

export interface CustomerPortalUploadOptions {
  onProgress?: (percent: number) => void
  signal?: AbortSignal
}

export interface CustomerPortalUploadedFile {
  id: string
  name: string
}

export interface CustomerPortalCustomer {
  id: string
  email: string
  name?: string | null
}

/** A company whose portal the customer belongs to, with their role there. */
export interface CustomerPortalCompany {
  id: string
  name: string
  /** Requesters see their own requests; members see every company request. */
  role: 'requester' | 'member'
}

export interface CustomerPortalSession {
  customer: CustomerPortalCustomer
  companies?: CustomerPortalCompany[]
}

/** Which requests a list shows: the customer's own, or one company's. */
export type CustomerPortalRequestScope = { kind: 'mine' } | { kind: 'company'; companyId: string }

export interface CustomerPortalRequest {
  reference: string
  /** Short ticket number agents also see. */
  number?: number
  subject: string
  status: 'active' | 'waiting_on_customer' | 'resolved'
  last_activity_at: string | null
  last_message_preview?: string
  last_message_from?: 'customer' | 'support'
  /** Support replied since the customer last opened the request. */
  unread?: boolean
  /** Company lists only: whether the signed-in customer asked. */
  own?: boolean
  /** Company lists only: who asked. */
  requested_by?: string
  company?: { id: string; name: string }
}

export interface CustomerPortalRequestDetail extends CustomerPortalRequest {
  can_reply: boolean
  /** The customer may rate this resolved request. */
  can_rate?: boolean
  csat?: { rating: number; comment?: string | null; submitted_at: string } | null
  /** False for a colleague's request opened through company access. */
  own?: boolean
  /** Who asked, for a colleague's request. */
  requester_name?: string | null
  messages: Array<{
    id: string
    content: string
    sender_type: string
    sender_name?: string | null
    sender_avatar?: string | null
    /** A teammate's generated avatar, drawn as in the app. */
    sender_avatar_style?: { style?: string | null; seed?: string | null; background_mode?: string | null; background_color?: string | null } | null
    via_channel?: string
    created_at: string
    /** False for a colleague's message; older servers omit it. */
    from_you?: boolean
    attachments?: Array<{ id: string; file_name: string; file_type: string; file_size: number; url: string }>
  }>
}

/** A published help center article that may answer a request being written. */
export interface CustomerPortalArticleSuggestion {
  id: string
  title: string
  space_name?: string
  snippet?: string
  /** The public help center address; articles for signed-in customers have none. */
  url?: string
  /** Set for articles for signed-in customers, which open in the portal. */
  portal_article_id?: string
}

/** A help article for signed-in customers, read in the portal. */
export interface CustomerPortalHelpArticle {
  id: string
  title: string
  excerpt?: string
  snippet?: string
  space_name: string
  collection_name?: string
  published_at: string
}

export interface CustomerPortalHelpArticleDetail extends CustomerPortalHelpArticle {
  content_html: string
}

export type CustomerPortalRequestFilter = 'all' | CustomerPortalRequest['status']

export interface CreatedCustomerPortalRequest extends CustomerPortalRequest {
  created_at: string
  updated_at: string
  resolved_at?: string | null
}

/** What the customer saw while writing a request. */
export interface CustomerPortalIntake {
  intake_id: string
  suggested_article_ids: string[]
  opened_article_ids: string[]
  search_completed: boolean
}

export interface CreateCustomerPortalRequestInput {
  subject: string
  message: string
  attachment_ids?: string[]
  intake?: CustomerPortalIntake
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
    `${portalApiBase}/public/portal/${encodeURIComponent(slug)}${path}`,
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

  if (response.status === 204 || response.status === 202) return undefined as T
  return response.json() as Promise<T>
}

/** customerPortalSocketUrl is the live-update socket for a portal. */
export function customerPortalSocketUrl(slug: string) {
  const base = new URL(portalApiBase, window.location.href)
  base.protocol = base.protocol === 'https:' ? 'wss:' : 'ws:'
  base.pathname = `${base.pathname.replace(/\/$/, '')}/public/portal/${encodeURIComponent(slug)}/ws`
  return base.toString()
}

/**
 * portalAssistantRequest returns a help assistant response in the result
 * shape the shared conversation controller expects, instead of throwing.
 */
async function portalAssistantRequest<T>(slug: string, path: string, init?: RequestInit): Promise<AssistantResult<T>> {
  try {
    return { data: await portalRequest<T>(slug, path, init), error: null, status: 200 }
  } catch (error) {
    if (error instanceof CustomerPortalApiError) return { data: null, error: error.message, status: error.status }
    return { data: null, error: error instanceof Error ? error.message : 'Request failed', status: 0 }
  }
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
  startAnonymousIntake: (slug: string, email: string) =>
    portalRequest<{ intake_token: string }>(slug, '/intake/session', {
      method: 'POST', body: JSON.stringify({ email }),
    }),
  createAnonymousRequest: (slug: string, token: string, input: CreateCustomerPortalRequestInput) =>
    portalRequest<void>(slug, '/intake/requests', {
      method: 'POST', headers: { Authorization: `Bearer ${token}` }, body: JSON.stringify(input),
    }),
  exchangeMagicLink: (slug: string, token: string) =>
    portalRequest<CustomerPortalSession>(slug, '/auth/exchange', {
      method: 'POST',
      body: JSON.stringify({ token }),
    }),
  signOut: (slug: string) =>
    portalRequest<void>(slug, '/session', { method: 'DELETE' }),
  requests: (slug: string, filter: CustomerPortalRequestFilter = 'all', scope: CustomerPortalRequestScope = { kind: 'mine' }) => {
    const params = new URLSearchParams()
    if (filter !== 'all') params.set('status', filter)
    if (scope.kind === 'company') {
      params.set('scope', 'company')
      params.set('company_id', scope.companyId)
    }
    const query = params.toString()
    return portalRequest<CustomerPortalRequest[]>(slug, `/requests${query ? `?${query}` : ''}`)
  },
  suggestionEvent: (slug: string, event: { intake_id: string; event: 'shown' | 'opened'; article_ids: string[] }) =>
    portalRequest<void>(slug, '/suggestions/events', { method: 'POST', body: JSON.stringify(event) }),
  suggestions: (slug: string, text: string, signal?: AbortSignal) =>
    portalRequest<{ articles: CustomerPortalArticleSuggestion[]; search_completed: boolean }>(slug, `/suggestions?q=${encodeURIComponent(text)}`, { signal }),
  helpArticles: (slug: string) =>
    portalRequest<{ articles: CustomerPortalHelpArticle[] }>(slug, '/help').then((result) => result.articles),
  searchHelpArticles: (slug: string, query: string, signal?: AbortSignal) =>
    portalRequest<{ articles: CustomerPortalHelpArticle[] }>(slug, `/help/search?q=${encodeURIComponent(query)}`, { signal }).then((result) => result.articles),
  helpArticle: (slug: string, articleId: string) =>
    portalRequest<CustomerPortalHelpArticleDetail>(slug, `/help/articles/${encodeURIComponent(articleId)}`),
  assistantAsk: (slug: string, body: { visitor_id: string; session_id?: string; message: string; page_article_id?: string }) =>
    portalAssistantRequest<AssistantTurn>(slug, '/assistant/messages', { method: 'POST', body: JSON.stringify(body) }),
  assistantSession: (slug: string, sessionId: string, visitorId: string) =>
    portalAssistantRequest<AssistantSessionView>(slug, `/assistant/sessions/${encodeURIComponent(sessionId)}?visitor_id=${encodeURIComponent(visitorId)}`),
  assistantFeedback: (slug: string, messageId: string, body: { visitor_id: string; session_id: string; helpful: boolean }) =>
    portalAssistantRequest<{ saved: boolean }>(slug, `/assistant/messages/${encodeURIComponent(messageId)}/feedback`, { method: 'POST', body: JSON.stringify(body) }),
  requestDetail: (slug: string, reference: string) =>
    portalRequest<CustomerPortalRequestDetail>(slug, `/requests/${encodeURIComponent(reference)}`),
  reply: (slug: string, reference: string, content: string, attachment_ids: string[] = []) =>
    portalRequest<CustomerPortalRequestDetail>(slug, `/requests/${encodeURIComponent(reference)}/replies`, {
      method: 'POST', body: JSON.stringify({ content, attachment_ids }),
    }),
  rate: (slug: string, reference: string, rating: number, comment: string) =>
    portalRequest<CustomerPortalRequestDetail>(slug, `/requests/${encodeURIComponent(reference)}/csat`, {
      method: 'POST', body: JSON.stringify({ rating, comment }),
    }),
  uploadAttachment: (slug: string, file: File, reference?: string, options: CustomerPortalUploadOptions = {}) => {
    const path = reference ? `/requests/${encodeURIComponent(reference)}/attachments` : '/attachments'
    return uploadPortalFile(slug, path, file, {}, options)
  },
  uploadAnonymousAttachment: (slug: string, token: string, file: File, options: CustomerPortalUploadOptions = {}) =>
    uploadPortalFile(slug, '/intake/attachments', file, { Authorization: `Bearer ${token}` }, options),
  createRequest: (slug: string, input: CreateCustomerPortalRequestInput) =>
    portalRequest<CreatedCustomerPortalRequest>(slug, '/requests', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
}

// uploadPortalFile registers the file, uploads it to storage with progress,
// then confirms it. Validation errors from the server surface as messages.
async function uploadPortalFile(
  slug: string,
  path: string,
  file: File,
  headers: Record<string, string>,
  { onProgress, signal }: CustomerPortalUploadOptions,
): Promise<CustomerPortalUploadedFile> {
  const result = await portalRequest<{ attachment: { id: string }; upload_url: string }>(slug, path, {
    method: 'POST', headers, signal,
    body: JSON.stringify({ file_name: file.name, file_size: file.size, content_type: file.type }),
  })
  const uploaded = await uploadToS3(result.upload_url, file, onProgress, undefined, signal)
  if (!uploaded.ok) throw new Error(uploaded.error === 'Upload cancelled' ? 'Upload cancelled' : 'File upload failed. Please try again.')
  await portalRequest<void>(slug, `${path}/${encodeURIComponent(result.attachment.id)}/confirm`, { method: 'PATCH', headers, signal })
  return { id: result.attachment.id, name: file.name }
}
