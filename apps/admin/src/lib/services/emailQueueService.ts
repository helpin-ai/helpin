import { api } from '@/lib/api'
import type {
  EmailDiagnosticsResponse,
  EmailFallbackConversationDiagnosticsResponse,
  EmailQueueResponse,
} from '@/lib/pmTypes'

export const emailQueueService = {
  list: () => api.get<EmailQueueResponse>('/admin/email-queue'),
  diagnostics: () => api.get<EmailDiagnosticsResponse>('/admin/email-diagnostics'),
  conversationDiagnostics: (conversationId: string) =>
    api.get<EmailFallbackConversationDiagnosticsResponse>(
      `/admin/email-diagnostics/conversations/${encodeURIComponent(conversationId)}`,
    ),
}
