import type { WebhookDelivery, WebhookEndpoint, WebhookEndpointInput, WebhookEndpointSecret } from '@/lib/pmTypes'
import { api } from '../api'

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`

export const supportWebhooksService = {
  list: (workspaceId: string) => api.get<WebhookEndpoint[]>(`/support/webhooks${qs(workspaceId)}`),
  create: (workspaceId: string, input: WebhookEndpointInput) =>
    api.post<WebhookEndpointSecret>(`/support/webhooks${qs(workspaceId)}`, input),
  update: (workspaceId: string, id: string, input: WebhookEndpointInput) =>
    api.put<WebhookEndpoint>(`/support/webhooks/${id}${qs(workspaceId)}`, input),
  remove: (workspaceId: string, id: string) => api.del(`/support/webhooks/${id}${qs(workspaceId)}`),
  rotateSecret: (workspaceId: string, id: string) =>
    api.post<WebhookEndpointSecret>(`/support/webhooks/${id}/rotate-secret${qs(workspaceId)}`),
  sendTest: (workspaceId: string, id: string) =>
    api.post<WebhookDelivery>(`/support/webhooks/${id}/test${qs(workspaceId)}`),
  deliveries: (workspaceId: string, id: string) =>
    api.get<WebhookDelivery[]>(`/support/webhooks/${id}/deliveries${qs(workspaceId)}&limit=50`),
  redeliver: (workspaceId: string, id: string, deliveryId: string) =>
    api.post<WebhookDelivery>(`/support/webhooks/${id}/deliveries/${deliveryId}/redeliver${qs(workspaceId)}`),
}
