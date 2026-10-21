import { api } from '@/lib/api'
import type {
  InboundConversation,
  InboundEmailDetail,
  InboundEmailList,
  IncomingStatus,
} from '@/lib/inboundEmailTypes'

const base = '/admin/inbound-emails'
const receiptPath = (id: string) => `${base}/${encodeURIComponent(id)}`

export const inboundEmailService = {
  list: (status: IncomingStatus, query: string, page: number) =>
    api.get<InboundEmailList>(
      `${base}?${new URLSearchParams({ status, q: query, page: String(page) })}`,
    ),
  detail: (id: string) => api.get<InboundEmailDetail>(receiptPath(id)),
  conversations: (id: string, query: string) =>
    api.get<InboundConversation[]>(
      `${receiptPath(id)}/conversations?${new URLSearchParams({ q: query })}`,
    ),
  recover: (id: string, conversationID?: string) =>
    api.post<{ status: string }>(`${receiptPath(id)}/recover`, { conversation_id: conversationID }),
  attachment: (id: string, attachmentID: string) =>
    api.getBlob(`${receiptPath(id)}/attachments/${encodeURIComponent(attachmentID)}`),
}
