import { api } from '../api'

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`

export type IntercomRegion = 'us' | 'eu' | 'au'

export interface IntercomDocsPreview {
  workspace_name: string
  collections: number
  articles_published: number
  articles_draft: number
}

export interface IntercomConversationsPreview {
  workspace_name: string
  conversations: number
}

export interface SupportImportJob {
  id: string
  source: string
  status: 'pending' | 'running' | 'done' | 'failed' | 'cancelled'
  since: string
  mailbox_id?: string | null
  total: number
  imported: number
  skipped: number
  failed: number
  failures: { external_id: string; error: string }[]
  error: string
  started_by: string
  completed_at?: string | null
  created_at: string
}

export const intercomImportService = {
  previewDocs: (workspaceId: string, accessToken: string, region: IntercomRegion) =>
    api.post<IntercomDocsPreview>(`/docs/import/intercom/preview${qs(workspaceId)}`, { access_token: accessToken, region }),
  startDocs: (workspaceId: string, input: { access_token: string; region: IntercomRegion; target_space_id?: string; new_space_name?: string; import_status: string }) =>
    api.post<{ job_id: string }>(`/docs/import/intercom/start${qs(workspaceId)}`, input),
  previewConversations: (workspaceId: string, accessToken: string, region: IntercomRegion, since: string) =>
    api.post<IntercomConversationsPreview>(`/support/imports/intercom/preview${qs(workspaceId)}`, { access_token: accessToken, region, since }),
  startConversations: (workspaceId: string, input: { access_token: string; region: IntercomRegion; since: string; mailbox_id?: string }) =>
    api.post<SupportImportJob>(`/support/imports/intercom${qs(workspaceId)}`, input),
  listConversationImports: (workspaceId: string) => api.get<SupportImportJob[]>(`/support/imports${qs(workspaceId)}`),
  cancelConversationImport: (workspaceId: string, id: string) => api.post(`/support/imports/${id}/cancel${qs(workspaceId)}`, {}),
}
