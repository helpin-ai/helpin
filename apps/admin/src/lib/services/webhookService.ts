import { api } from '@/lib/api'
import type { WebhookEventListResponse, WebhookEvent } from '@/lib/pmTypes'

export const webhookService = {
  list: (params: { page?: number; per_page?: number; event_type?: string; provider?: string }) => {
    const search = new URLSearchParams()
    if (params.page) search.set('page', String(params.page))
    if (params.per_page) search.set('per_page', String(params.per_page))
    if (params.event_type) search.set('event_type', params.event_type)
    if (params.provider) search.set('provider', params.provider)
    return api.get<WebhookEventListResponse>(`/admin/webhook-events?${search.toString()}`)
  },

  getById: (id: string) => api.get<WebhookEvent>(`/admin/webhook-events/${id}`),
}
