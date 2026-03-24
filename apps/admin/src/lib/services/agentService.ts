import { api } from '@/lib/api'
import type { Agent, SupportAIPreviewRequest, SupportAIPreviewResponse } from '@/lib/pmTypes'

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`

export const agentService = {
  list: (workspaceId: string) =>
    api.get<Agent[]>(`/pm/agents${qs(workspaceId)}`),
  previewSupportReply: (workspaceId: string, agentId: string, payload: SupportAIPreviewRequest) =>
    api.post<SupportAIPreviewResponse>(`/pm/agents/${agentId}/support-preview${qs(workspaceId)}`, payload),
}
