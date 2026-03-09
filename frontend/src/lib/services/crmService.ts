import { api } from '../api';
import type {
  CRMContact,
  CRMCompany,
  CRMDeal,
  CRMPipeline,
  CRMAssociation,
  CRMActivity,
  CreateCRMContactRequest,
  UpdateCRMContactRequest,
  CreateCRMCompanyRequest,
  UpdateCRMCompanyRequest,
  CreateCRMDealRequest,
  UpdateCRMDealRequest,
  CreateCRMPipelineRequest,
  UpdateCRMPipelineRequest,
  CreateCRMAssociationRequest,
  CreateCRMActivityRequest,
  UpdateCRMActivityRequest,
  CRMPaginatedResponse,
} from '../crmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

const filterQuery = (filters: Record<string, string | number | boolean | undefined>) => {
  const search = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (value === undefined || value === '') return;
    search.set(key, String(value));
  });
  const str = search.toString();
  return str ? `&${str}` : '';
};

export const crmContactService = {
  list: (workspaceId: string, filters?: { lifecycle_stage?: string; lead_status?: string; owner_member_id?: string; search?: string; page?: number; per_page?: number }) =>
    api.get<CRMPaginatedResponse<CRMContact[]>>(`/crm/contacts${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  get: (workspaceId: string, id: string) =>
    api.get<CRMContact>(`/crm/contacts/${id}${qs(workspaceId)}`),
  create: (payload: CreateCRMContactRequest) =>
    api.post<CRMContact>(`/crm/contacts${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCRMContactRequest) =>
    api.put<CRMContact>(`/crm/contacts/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) =>
    api.del(`/crm/contacts/${id}${qs(workspaceId)}`),
  listActivities: (workspaceId: string, contactId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMActivity[]>>(`/crm/contacts/${contactId}/activities${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
  listAssociations: (workspaceId: string, contactId: string) =>
    api.get<CRMAssociation[]>(`/crm/contacts/${contactId}/associations${qs(workspaceId)}`),
};

export const crmCompanyService = {
  list: (workspaceId: string, filters?: { industry?: string; owner_member_id?: string; search?: string; page?: number; per_page?: number }) =>
    api.get<CRMPaginatedResponse<CRMCompany[]>>(`/crm/companies${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  get: (workspaceId: string, id: string) =>
    api.get<CRMCompany>(`/crm/companies/${id}${qs(workspaceId)}`),
  create: (payload: CreateCRMCompanyRequest) =>
    api.post<CRMCompany>(`/crm/companies${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCRMCompanyRequest) =>
    api.put<CRMCompany>(`/crm/companies/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) =>
    api.del(`/crm/companies/${id}${qs(workspaceId)}`),
  listActivities: (workspaceId: string, companyId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMActivity[]>>(`/crm/companies/${companyId}/activities${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
  listAssociations: (workspaceId: string, companyId: string) =>
    api.get<CRMAssociation[]>(`/crm/companies/${companyId}/associations${qs(workspaceId)}`),
};

export const crmDealService = {
  list: (workspaceId: string, filters?: { pipeline_id?: string; stage_id?: string; owner_member_id?: string; search?: string; page?: number; per_page?: number }) =>
    api.get<CRMPaginatedResponse<CRMDeal[]>>(`/crm/deals${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  get: (workspaceId: string, id: string) =>
    api.get<CRMDeal>(`/crm/deals/${id}${qs(workspaceId)}`),
  create: (payload: CreateCRMDealRequest) =>
    api.post<CRMDeal>(`/crm/deals${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCRMDealRequest) =>
    api.put<CRMDeal>(`/crm/deals/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) =>
    api.del(`/crm/deals/${id}${qs(workspaceId)}`),
  listActivities: (workspaceId: string, dealId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMActivity[]>>(`/crm/deals/${dealId}/activities${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
  listAssociations: (workspaceId: string, dealId: string) =>
    api.get<CRMAssociation[]>(`/crm/deals/${dealId}/associations${qs(workspaceId)}`),
};

export const crmPipelineService = {
  list: (workspaceId: string) =>
    api.get<CRMPipeline[]>(`/crm/pipelines${qs(workspaceId)}`),
  get: (workspaceId: string, id: string) =>
    api.get<CRMPipeline>(`/crm/pipelines/${id}${qs(workspaceId)}`),
  create: (payload: CreateCRMPipelineRequest) =>
    api.post<CRMPipeline>(`/crm/pipelines${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCRMPipelineRequest) =>
    api.put<CRMPipeline>(`/crm/pipelines/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) =>
    api.del(`/crm/pipelines/${id}${qs(workspaceId)}`),
};

export const crmAssociationService = {
  create: (payload: CreateCRMAssociationRequest) =>
    api.post<CRMAssociation>(`/crm/associations${qs(payload.workspace_id)}`, payload),
  remove: (workspaceId: string, id: string) =>
    api.del(`/crm/associations/${id}${qs(workspaceId)}`),
};

export const crmActivityService = {
  list: (workspaceId: string, filters?: { activity_type?: string; contact_id?: string; company_id?: string; deal_id?: string; page?: number; per_page?: number }) =>
    api.get<CRMPaginatedResponse<CRMActivity[]>>(`/crm/activities${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  get: (workspaceId: string, id: string) =>
    api.get<CRMActivity>(`/crm/activities/${id}${qs(workspaceId)}`),
  create: (payload: CreateCRMActivityRequest) =>
    api.post<CRMActivity>(`/crm/activities${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCRMActivityRequest) =>
    api.put<CRMActivity>(`/crm/activities/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) =>
    api.del(`/crm/activities/${id}${qs(workspaceId)}`),
};
