import type { SupportChannelAccount } from '@/lib/pmTypes'
import { api } from '../api'

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`

export const supportChannelsService = {
  list: (workspaceId: string) => api.get<SupportChannelAccount[]>(`/support/channels${qs(workspaceId)}`),
  connect: (workspaceId: string, input: { provider: 'telegram'; token: string; mailbox_id: string | null }) =>
    api.post<SupportChannelAccount>(`/support/channels${qs(workspaceId)}`, input),
  update: (workspaceId: string, id: string, input: { mailbox_id?: string; ai_replies_disabled?: boolean }) =>
    api.patch<SupportChannelAccount>(`/support/channels/${id}${qs(workspaceId)}`, input),
  reconnect: (workspaceId: string, id: string, token?: string) =>
    api.post<SupportChannelAccount>(`/support/channels/${id}/reconnect${qs(workspaceId)}`, token ? { token } : {}),
  disconnect: (workspaceId: string, id: string) => api.del(`/support/channels/${id}${qs(workspaceId)}`),
}
