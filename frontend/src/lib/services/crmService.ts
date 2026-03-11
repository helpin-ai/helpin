import { api } from '../api';
import type { SupportConversation } from '../pmTypes';
import type {
  CRMContact,
  CRMCompany,
  CRMDeal,
  CRMPipeline,
  CRMAssociation,
  CRMAssociationEnriched,
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
  CRMEmailAccount,
  CreateCRMEmailAccountRequest,
  CRMEmailThread,
  CRMEmailMessage,
  CreateCRMEmailMessageRequest,
  CRMCalendarEvent,
  CreateCRMCalendarEventRequest,
  UpdateCRMCalendarEventRequest,
  CRMEnrichmentResult,
  CreateCRMEnrichmentRequest,
  CRMBuyerSignal,
  CreateCRMBuyerSignalRequest,
  CRMDealHealthScore,
  CreateCRMDealHealthScoreRequest,
  CRMSuggestion,
  CreateCRMSuggestionRequest,
  UpdateCRMSuggestionRequest,
  CRMSequence,
  CreateCRMSequenceRequest,
  UpdateCRMSequenceRequest,
  CRMSequenceEnrollment,
  CreateCRMSequenceEnrollmentRequest,
  UpdateCRMSequenceEnrollmentRequest,
  CRMWritingProfile,
  CreateCRMWritingProfileRequest,
  UpdateCRMWritingProfileRequest,
  CRMSearchResult,
  CRMEmailSyncSettings,
  UpdateCRMEmailSyncSettingsRequest,
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
    api.get<CRMAssociationEnriched[]>(`/crm/contacts/${contactId}/associations${qs(workspaceId)}`),
  listSupportConversations: (workspaceId: string, contactId: string, params?: { page?: number; per_page?: number }) =>
    api.get<{ data: SupportConversation[]; total: number; page: number }>(`/crm/contacts/${contactId}/support-conversations${qs(workspaceId)}${filterQuery(params ?? {})}`),
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
    api.get<CRMAssociationEnriched[]>(`/crm/companies/${companyId}/associations${qs(workspaceId)}`),
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
    api.get<CRMAssociationEnriched[]>(`/crm/deals/${dealId}/associations${qs(workspaceId)}`),
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
  createFromPM: (payload: CreateCRMAssociationRequest) =>
    api.post<CRMAssociation>(`/pm/associations${qs(payload.workspace_id)}`, payload),
  remove: (workspaceId: string, id: string) =>
    api.del(`/crm/associations/${id}${qs(workspaceId)}`),
  removeFromPM: (workspaceId: string, id: string) =>
    api.del(`/pm/associations/${id}${qs(workspaceId)}`),
  listByEpic: (workspaceId: string, epicId: string) =>
    api.get<CRMAssociationEnriched[]>(`/pm/epics/${epicId}/associations${qs(workspaceId)}`),
  listByStory: (workspaceId: string, storyId: string) =>
    api.get<CRMAssociationEnriched[]>(`/pm/stories/${storyId}/associations${qs(workspaceId)}`),
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

// ── Phase 3: Email & Calendar ──

export const crmEmailService = {
  listAccounts: (workspaceId: string, filters?: { member_id?: string; provider?: string }) =>
    api.get<CRMEmailAccount[]>(`/crm/email/accounts${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  getAccount: (workspaceId: string, id: string) =>
    api.get<CRMEmailAccount>(`/crm/email/accounts/${id}${qs(workspaceId)}`),
  createAccount: (payload: CreateCRMEmailAccountRequest) =>
    api.post<CRMEmailAccount>(`/crm/email/accounts${qs(payload.workspace_id)}`, payload),
  disconnectAccount: (workspaceId: string, id: string) =>
    api.del(`/crm/email/accounts/${id}${qs(workspaceId)}`),
  purgeAccountData: (workspaceId: string, id: string) =>
    api.del(`/crm/email/accounts/${id}/data${qs(workspaceId)}`),
  initiateOAuth: (workspaceId: string, provider: 'gmail' | 'microsoft' = 'gmail') =>
    api.get<{ redirect_url: string }>(`/crm/email/oauth/initiate${qs(workspaceId)}&provider=${provider}`),
  sendEmail: (workspaceId: string, payload: { account_id: string; to: string[]; cc?: string[]; subject: string; body_html: string }) =>
    api.post<CRMEmailMessage>(`/crm/email/send${qs(workspaceId)}`, payload),
  listThreads: (workspaceId: string, filters?: { email_account_id?: string; contact_id?: string; deal_id?: string; search?: string; page?: number }) =>
    api.get<CRMPaginatedResponse<CRMEmailThread[]>>(`/crm/email/threads${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  listMessages: (workspaceId: string, filters?: { thread_id?: string; email_account_id?: string; contact_id?: string; deal_id?: string; direction?: string; page?: number }) =>
    api.get<CRMPaginatedResponse<CRMEmailMessage[]>>(`/crm/email/messages${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  createMessage: (payload: CreateCRMEmailMessageRequest) =>
    api.post<CRMEmailMessage>(`/crm/email/messages${qs(payload.workspace_id)}`, payload),
  listByContact: (workspaceId: string, contactId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMEmailMessage[]>>(`/crm/contacts/${contactId}/emails${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
  listByDeal: (workspaceId: string, dealId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMEmailMessage[]>>(`/crm/deals/${dealId}/emails${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
};

export const crmCalendarService = {
  list: (workspaceId: string, filters?: { email_account_id?: string; deal_id?: string; start_after?: string; start_before?: string; page?: number }) =>
    api.get<CRMPaginatedResponse<CRMCalendarEvent[]>>(`/crm/calendar/events${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  get: (workspaceId: string, id: string) =>
    api.get<CRMCalendarEvent>(`/crm/calendar/events/${id}${qs(workspaceId)}`),
  create: (payload: CreateCRMCalendarEventRequest) =>
    api.post<CRMCalendarEvent>(`/crm/calendar/events${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCRMCalendarEventRequest) =>
    api.put<CRMCalendarEvent>(`/crm/calendar/events/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) =>
    api.del(`/crm/calendar/events/${id}${qs(workspaceId)}`),
  listByContact: (workspaceId: string, contactId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMCalendarEvent[]>>(`/crm/contacts/${contactId}/calendar${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
  listByDeal: (workspaceId: string, dealId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMCalendarEvent[]>>(`/crm/deals/${dealId}/calendar${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
};

// ── Phase 4: Intelligence ──

export const crmEnrichmentService = {
  list: (workspaceId: string, filters?: { object_type?: string; object_id?: string; source?: string; page?: number }) =>
    api.get<CRMPaginatedResponse<CRMEnrichmentResult[]>>(`/crm/enrichments${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  create: (payload: CreateCRMEnrichmentRequest) =>
    api.post<CRMEnrichmentResult>(`/crm/enrichments${qs(payload.workspace_id)}`, payload),
};

export const crmSignalService = {
  list: (workspaceId: string, filters?: { contact_id?: string; deal_id?: string; signal_type?: string; source_type?: string; page?: number }) =>
    api.get<CRMPaginatedResponse<CRMBuyerSignal[]>>(`/crm/signals${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  create: (payload: CreateCRMBuyerSignalRequest) =>
    api.post<CRMBuyerSignal>(`/crm/signals${qs(payload.workspace_id)}`, payload),
  remove: (workspaceId: string, id: string) =>
    api.del(`/crm/signals/${id}${qs(workspaceId)}`),
  listByContact: (workspaceId: string, contactId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMBuyerSignal[]>>(`/crm/contacts/${contactId}/signals${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
  listByDeal: (workspaceId: string, dealId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMBuyerSignal[]>>(`/crm/deals/${dealId}/signals${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
};

export const crmHealthScoreService = {
  list: (workspaceId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMDealHealthScore[]>>(`/crm/health-scores${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
  getForDeal: (workspaceId: string, dealId: string) =>
    api.get<CRMDealHealthScore>(`/crm/deals/${dealId}/health-score${qs(workspaceId)}`),
  create: (payload: CreateCRMDealHealthScoreRequest) =>
    api.post<CRMDealHealthScore>(`/crm/health-scores${qs(payload.workspace_id)}`, payload),
};

export const crmSuggestionService = {
  list: (workspaceId: string, filters?: { user_id?: string; suggestion_type?: string; object_type?: string; object_id?: string; status?: string; page?: number }) =>
    api.get<CRMPaginatedResponse<CRMSuggestion[]>>(`/crm/suggestions${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  get: (workspaceId: string, id: string) =>
    api.get<CRMSuggestion>(`/crm/suggestions/${id}${qs(workspaceId)}`),
  create: (payload: CreateCRMSuggestionRequest) =>
    api.post<CRMSuggestion>(`/crm/suggestions${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCRMSuggestionRequest) =>
    api.put<CRMSuggestion>(`/crm/suggestions/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) =>
    api.del(`/crm/suggestions/${id}${qs(workspaceId)}`),
  accept: (workspaceId: string, id: string, edits?: Record<string, unknown>) =>
    api.post<CRMSuggestion>(`/crm/suggestions/${id}/accept${qs(workspaceId)}`, edits || {}),
  dismiss: (workspaceId: string, id: string) =>
    api.post<CRMSuggestion>(`/crm/suggestions/${id}/dismiss${qs(workspaceId)}`, {}),
};

// ── Phase 5: Sequences & Writing ──

export const crmSequenceService = {
  list: (workspaceId: string, filters?: { status?: string; search?: string; page?: number }) =>
    api.get<CRMPaginatedResponse<CRMSequence[]>>(`/crm/sequences${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  get: (workspaceId: string, id: string) =>
    api.get<CRMSequence>(`/crm/sequences/${id}${qs(workspaceId)}`),
  create: (payload: CreateCRMSequenceRequest) =>
    api.post<CRMSequence>(`/crm/sequences${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCRMSequenceRequest) =>
    api.put<CRMSequence>(`/crm/sequences/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) =>
    api.del(`/crm/sequences/${id}${qs(workspaceId)}`),
  listEnrollments: (workspaceId: string, sequenceId: string, filters?: { status?: string; page?: number }) =>
    api.get<CRMPaginatedResponse<CRMSequenceEnrollment[]>>(`/crm/sequences/${sequenceId}/enrollments${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  createEnrollment: (workspaceId: string, sequenceId: string, payload: CreateCRMSequenceEnrollmentRequest) =>
    api.post<CRMSequenceEnrollment>(`/crm/sequences/${sequenceId}/enrollments${qs(workspaceId)}`, payload),
  updateEnrollment: (workspaceId: string, id: string, payload: UpdateCRMSequenceEnrollmentRequest) =>
    api.put<CRMSequenceEnrollment>(`/crm/enrollments/${id}${qs(workspaceId)}`, payload),
  deleteEnrollment: (workspaceId: string, id: string) =>
    api.del(`/crm/enrollments/${id}${qs(workspaceId)}`),
};

export const crmWritingProfileService = {
  list: (workspaceId: string) =>
    api.get<CRMWritingProfile[]>(`/crm/writing-profiles${qs(workspaceId)}`),
  getByMember: (workspaceId: string, memberId: string) =>
    api.get<CRMWritingProfile>(`/crm/writing-profiles/member/${memberId}${qs(workspaceId)}`),
  create: (payload: CreateCRMWritingProfileRequest) =>
    api.post<CRMWritingProfile>(`/crm/writing-profiles${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCRMWritingProfileRequest) =>
    api.put<CRMWritingProfile>(`/crm/writing-profiles/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) =>
    api.del(`/crm/writing-profiles/${id}${qs(workspaceId)}`),
};

// ── Phase D: Deal Automation ──

export interface CRMAutonomySettings {
  enabled: boolean;
  auto_create_deals: boolean;
  auto_progress_deals: boolean;
  auto_execute_threshold: number;
  review_threshold: number;
}

export const crmAutonomyService = {
  get: (workspaceId: string) =>
    api.get<CRMAutonomySettings>(`/crm/autonomy-settings${qs(workspaceId)}`),
  update: (workspaceId: string, settings: CRMAutonomySettings) =>
    api.put<CRMAutonomySettings>(`/crm/autonomy-settings${qs(workspaceId)}`, settings),
};

// ── Email Sync Settings ──

export const crmEmailSyncSettingsService = {
  get: (workspaceId: string) =>
    api.get<CRMEmailSyncSettings>(`/crm/email/sync-settings${qs(workspaceId)}`),
  update: (workspaceId: string, settings: UpdateCRMEmailSyncSettingsRequest) =>
    api.put<CRMEmailSyncSettings>(`/crm/email/sync-settings${qs(workspaceId)}`, settings),
  getDefaultPrefixes: () =>
    api.get<string[]>('/crm/email/sync-settings/default-prefixes'),
};

// ── Phase 6: Search ──

export const crmSearchService = {
  search: (workspaceId: string, q: string) =>
    api.get<CRMSearchResult[]>(`/crm/search${qs(workspaceId)}&q=${encodeURIComponent(q)}`),
};
