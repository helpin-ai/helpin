import { api } from '@/lib/api'
import type { EmailDiagnosticsResponse, EmailQueueResponse } from '@/lib/pmTypes'

export const emailQueueService = {
  list: () => api.get<EmailQueueResponse>('/admin/email-queue'),
  diagnostics: () => api.get<EmailDiagnosticsResponse>('/admin/email-diagnostics'),
}
