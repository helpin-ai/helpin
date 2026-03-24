import { api } from '@/lib/api'
import type { SupportInstallationResponse } from '@/lib/pmTypes'

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`

export const supportService = {
  getInstallation: (workspaceId: string) =>
    api.get<SupportInstallationResponse>(`/support/inbox/installations${qs(workspaceId)}`),
}
