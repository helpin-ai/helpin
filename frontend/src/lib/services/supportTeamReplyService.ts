import { API_BASE } from '@/lib/api'
import type { CustomerPortalBranding } from '@/lib/services/customerPortalService'

/** Where the "Ask our team to reply" link from an AI email stands. */
export type SupportTeamReplyState = 'open' | 'done' | 'already' | 'expired'

export interface SupportTeamReplyPage {
  state: SupportTeamReplyState
  branding: CustomerPortalBranding
  subject?: string
  customer_email?: string
  /** The workspace's reply expectation, or its outside-hours message. */
  reply_time_text: string
}

/** SupportTeamReplyLinkError means the link was not issued by Helpin. */
export class SupportTeamReplyLinkError extends Error {}

async function teamReplyRequest(token: string, init?: RequestInit): Promise<SupportTeamReplyPage> {
  // The token is the only credential, so no session headers are sent.
  const response = await fetch(`${API_BASE}/public/support/team-reply/${encodeURIComponent(token)}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init?.headers },
  })
  if (response.status === 404) throw new SupportTeamReplyLinkError('This link is not valid.')
  if (!response.ok) throw new Error('Something went wrong.')
  return (await response.json()) as SupportTeamReplyPage
}

export const supportTeamReplyService = {
  get: (token: string) => teamReplyRequest(token),
  request: (token: string, note: string) =>
    teamReplyRequest(token, { method: 'POST', body: JSON.stringify({ note }) }),
}
