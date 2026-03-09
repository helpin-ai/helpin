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
  CRMPropertyDefinition,
  CRMPropertyGroup,
  CreateCRMPropertyDefinitionRequest,
  UpdateCRMPropertyDefinitionRequest,
  CreateCRMPropertyGroupRequest,
  UpdateCRMPropertyGroupRequest,
  CRMList,
  CRMListMember,
  CreateCRMListRequest,
  UpdateCRMListRequest,
  CRMImportJob,
  CreateCRMImportRequest,
  ProcessCRMImportRequest,
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

export const crmPropertyService = {
  listDefinitions: (workspaceId: string, objectType?: string) =>
    api.get<CRMPropertyDefinition[]>(`/crm/properties${qs(workspaceId)}${objectType ? `&object_type=${objectType}` : ''}`),
  createDefinition: (payload: CreateCRMPropertyDefinitionRequest) =>
    api.post<CRMPropertyDefinition>(`/crm/properties${qs(payload.workspace_id)}`, payload),
  updateDefinition: (workspaceId: string, id: string, payload: UpdateCRMPropertyDefinitionRequest) =>
    api.put<CRMPropertyDefinition>(`/crm/properties/${id}${qs(workspaceId)}`, payload),
  deleteDefinition: (workspaceId: string, id: string) =>
    api.del(`/crm/properties/${id}${qs(workspaceId)}`),
  listGroups: (workspaceId: string, objectType?: string) =>
    api.get<CRMPropertyGroup[]>(`/crm/property-groups${qs(workspaceId)}${objectType ? `&object_type=${objectType}` : ''}`),
  createGroup: (payload: CreateCRMPropertyGroupRequest) =>
    api.post<CRMPropertyGroup>(`/crm/property-groups${qs(payload.workspace_id)}`, payload),
  updateGroup: (workspaceId: string, id: string, payload: UpdateCRMPropertyGroupRequest) =>
    api.put<CRMPropertyGroup>(`/crm/property-groups/${id}${qs(workspaceId)}`, payload),
  deleteGroup: (workspaceId: string, id: string) =>
    api.del(`/crm/property-groups/${id}${qs(workspaceId)}`),
};

export const crmListService = {
  list: (workspaceId: string, filters?: { object_type?: string; list_type?: string; search?: string; page?: number; per_page?: number }) =>
    api.get<CRMPaginatedResponse<CRMList[]>>(`/crm/lists${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  get: (workspaceId: string, id: string) =>
    api.get<CRMList>(`/crm/lists/${id}${qs(workspaceId)}`),
  create: (payload: CreateCRMListRequest) =>
    api.post<CRMList>(`/crm/lists${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCRMListRequest) =>
    api.put<CRMList>(`/crm/lists/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) =>
    api.del(`/crm/lists/${id}${qs(workspaceId)}`),
  listMembers: (workspaceId: string, listId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMListMember[]>>(`/crm/lists/${listId}/members${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
  addMember: (workspaceId: string, listId: string, objectId: string) =>
    api.post<CRMListMember>(`/crm/lists/${listId}/members${qs(workspaceId)}`, { object_id: objectId }),
  removeMember: (workspaceId: string, listId: string, objectId: string) =>
    api.del(`/crm/lists/${listId}/members/${objectId}${qs(workspaceId)}`),
};

export const crmImportService = {
  list: (workspaceId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMImportJob[]>>(`/crm/imports${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
  get: (workspaceId: string, id: string) =>
    api.get<CRMImportJob>(`/crm/imports/${id}${qs(workspaceId)}`),
  create: (payload: CreateCRMImportRequest) =>
    api.post<CRMImportJob>(`/crm/imports${qs(payload.workspace_id)}`, payload),
  process: (workspaceId: string, id: string, payload: ProcessCRMImportRequest) =>
    api.post<CRMImportJob>(`/crm/imports/${id}/process${qs(workspaceId)}`, payload),
};
