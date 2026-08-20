import { api } from '@mobile/lib/api'

export type MobileLifecycleStage =
  | 'subscriber'
  | 'lead'
  | 'marketing_qualified'
  | 'sales_qualified'
  | 'opportunity'
  | 'customer'
  | 'evangelist'

export type MobileLeadStatus = 'new' | 'open' | 'in_progress' | 'unqualified'

export interface MobileCRMContact {
  id: string
  workspace_id: string
  display_id: string
  first_name: string
  last_name?: string
  email?: string
  phone?: string
  job_title?: string
  lifecycle_stage: MobileLifecycleStage
  lead_status: MobileLeadStatus
  source?: string
  created_at: string
  updated_at: string
}

export interface CreateMobileCRMContactRequest {
  workspace_id: string
  first_name: string
  last_name?: string
  email: string
  phone?: string
  job_title?: string
  lifecycle_stage: MobileLifecycleStage
  lead_status: MobileLeadStatus
  source?: string
}

export type UpdateMobileCRMContactRequest = Partial<Omit<CreateMobileCRMContactRequest, 'workspace_id'>>

function workspaceQuery(workspaceId: string): string {
  return `workspace_id=${encodeURIComponent(workspaceId)}`
}

interface ContactSearchResponse {
  data: MobileCRMContact[]
  total: number
  page: number
}

export const mobileContactsService = {
  search: (workspaceId: string, search: string) =>
    api.get<ContactSearchResponse>(
      `/crm/contacts?${workspaceQuery(workspaceId)}&search=${encodeURIComponent(search)}&per_page=8`,
    ),
  get: (workspaceId: string, contactId: string) =>
    api.get<MobileCRMContact>(
      `/crm/contacts/${encodeURIComponent(contactId)}?${workspaceQuery(workspaceId)}`,
    ),
  create: (payload: CreateMobileCRMContactRequest) =>
    api.post<MobileCRMContact>(
      `/crm/contacts?${workspaceQuery(payload.workspace_id)}`,
      payload,
    ),
  update: (workspaceId: string, contactId: string, payload: UpdateMobileCRMContactRequest) =>
    api.put<MobileCRMContact>(
      `/crm/contacts/${encodeURIComponent(contactId)}?${workspaceQuery(workspaceId)}`,
      payload,
    ),
}
