import { api } from '@/lib/api'
import type { EmailQueueResponse } from '@/lib/pmTypes'

export const emailQueueService = {
  list: () => api.get<EmailQueueResponse>('/admin/email-queue'),
}
