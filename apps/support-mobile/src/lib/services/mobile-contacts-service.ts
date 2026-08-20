import { api } from '@mobile/lib/api'

export interface MobileCRMContact {
  id: string
  first_name: string
  last_name?: string
  email?: string
  lifecycle_stage: string
}

interface ContactSearchResponse {
  data: MobileCRMContact[]
  total: number
  page: number
}

export const mobileContactsService = {
  search: (workspaceId: string, search: string) =>
    api.get<ContactSearchResponse>(
      `/crm/contacts?workspace_id=${encodeURIComponent(workspaceId)}&search=${encodeURIComponent(search)}&per_page=8`,
    ),
}
