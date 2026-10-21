export type IncomingStatus = 'attention' | 'processing' | 'recovered' | 'all'

export interface InboundEmail {
  id: string
  status: 'pending' | 'processing' | 'failed' | 'completed'
  workspace_id?: string
  workspace_name?: string
  conversation_id?: string
  subject: string
  sender_email: string
  sender_name: string
  recipient: string
  received_at: string
  updated_at: string
  available_at: string
  attempts: number
  requires_destination: boolean
  recovery_requested_at?: string
  recovery_actor_name?: string
}

export interface InboundEmailList {
  entries: InboundEmail[]
  total: number
  attention_count: number
  page: number
  per_page: number
}

export interface InboundConversation {
  id: string
  display_id: number
  subject: string
  workspace_id: string
  workspace_name: string
  workspace_slug: string
  inbox_name: string
  customer_name: string
  customer_email: string
  last_team_reply?: string
}

export interface InboundEmailDetail extends InboundEmail {
  body: string
  provider_id: string
  in_reply_to?: string
  last_error?: string
  attachments: { id: string; name: string; content_type: string; size: number }[]
  destination?: InboundConversation
}
