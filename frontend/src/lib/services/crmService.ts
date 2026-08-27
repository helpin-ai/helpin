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
  CRMTimelineFilter,
  CRMTimelinePage,
  CRMCompanyTimelineFilter,
  CRMCompanyTimelinePage,
  CreateCRMContactRequest,
  UpdateCRMContactRequest,
  SeedCRMContactsRequest,
  SeedCRMContactsResponse,
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
  CRMImportJob,
  CreateCRMImportRequest,
  ProcessCRMImportRequest,
  CRMEmailAccount,
  CRMEmailAccountDiagnostics,
  CreateCRMEmailAccountRequest,
  CRMEmailThread,
  CRMEmailThreadDetail,
  CRMEmailMessage,
  CreateCRMEmailMessageRequest,
  CRMCalendarEvent,
  CreateCRMCalendarEventRequest,
  UpdateCRMCalendarEventRequest,
  CRMEnrichmentResult,
  CreateCRMEnrichmentRequest,
  CRMBuyerSignal,
  CRMSignalExternalEvidence,
  CRMSignalWorkspaceFeed,
  CRMSignalFeedFilters,
  CreateCRMBuyerSignalRequest,
  IngestCRMSignalExternalEvidenceRequest,
  CRMEntitySummary,
  CRMIntelligenceRefreshResult,
  CRMDealHealthScore,
  CreateCRMDealHealthScoreRequest,
  CRMSuggestion,
  CreateCRMSuggestionRequest,
  UpdateCRMSuggestionRequest,
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
  list: (
    workspaceId: string,
    filters?: {
      lifecycle_stage?: string;
      lead_status?: string;
      owner_member_id?: string;
      search?: string;
      filters?: string;
      page?: number;
      per_page?: number;
    },
  ) => api.get<CRMPaginatedResponse<CRMContact[]>>(`/crm/contacts${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  get: (workspaceId: string, id: string) => api.get<CRMContact>(`/crm/contacts/${id}${qs(workspaceId)}`),
  create: (payload: CreateCRMContactRequest) => api.post<CRMContact>(`/crm/contacts${qs(payload.workspace_id)}`, payload),
  seed: (payload: SeedCRMContactsRequest) => api.post<SeedCRMContactsResponse>(`/crm/contacts/seed${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCRMContactRequest) => api.put<CRMContact>(`/crm/contacts/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) => api.del(`/crm/contacts/${id}${qs(workspaceId)}`),
  listActivities: (workspaceId: string, contactId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMActivity[]>>(`/crm/contacts/${contactId}/activities${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
  listTimeline: (workspaceId: string, contactId: string, filter: CRMTimelineFilter, cursor?: string) =>
    api.get<CRMTimelinePage>(`/crm/contacts/${contactId}/timeline${qs(workspaceId)}${filterQuery({ filter, cursor, limit: 25 })}`),
  listAssociations: (workspaceId: string, contactId: string) => api.get<CRMAssociationEnriched[]>(`/crm/contacts/${contactId}/associations${qs(workspaceId)}`),
  listSupportConversations: (workspaceId: string, contactId: string, params?: { status?: string; search?: string; page?: number; per_page?: number }) =>
    api.get<{ data: SupportConversation[]; total: number; page: number }>(`/crm/contacts/${contactId}/support-conversations${qs(workspaceId)}${filterQuery(params ?? {})}`),
};

export const crmCompanyService = {
  list: (
    workspaceId: string,
    filters?: {
      industry?: string;
      owner_member_id?: string;
      search?: string;
      page?: number;
      per_page?: number;
    },
  ) => api.get<CRMPaginatedResponse<CRMCompany[]>>(`/crm/companies${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  get: (workspaceId: string, id: string) => api.get<CRMCompany>(`/crm/companies/${id}${qs(workspaceId)}`),
  create: (payload: CreateCRMCompanyRequest) => api.post<CRMCompany>(`/crm/companies${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCRMCompanyRequest) => api.put<CRMCompany>(`/crm/companies/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) => api.del(`/crm/companies/${id}${qs(workspaceId)}`),
  listActivities: (workspaceId: string, companyId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMActivity[]>>(`/crm/companies/${companyId}/activities${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
  listTimeline: (workspaceId: string, companyId: string, filter: CRMCompanyTimelineFilter, cursor?: string) =>
    api.get<CRMCompanyTimelinePage>(`/crm/companies/${companyId}/timeline${qs(workspaceId)}${filterQuery({ filter, cursor, limit: 25 })}`),
  listAssociations: (workspaceId: string, companyId: string) => api.get<CRMAssociationEnriched[]>(`/crm/companies/${companyId}/associations${qs(workspaceId)}`),
  listContacts: (workspaceId: string, companyId: string, params?: { search?: string; page?: number; per_page?: number }) =>
    api.get<CRMPaginatedResponse<CRMContact[]>>(`/crm/companies/${companyId}/contacts${qs(workspaceId)}${filterQuery(params ?? {})}`),
  listDeals: (workspaceId: string, companyId: string, params?: { search?: string; page?: number; per_page?: number }) =>
    api.get<CRMPaginatedResponse<CRMDeal[]>>(`/crm/companies/${companyId}/deals${qs(workspaceId)}${filterQuery(params ?? {})}`),
  listSupportConversations: (
    workspaceId: string,
    companyId: string,
    params?: {
      status?: string;
      search?: string;
      page?: number;
      per_page?: number;
    },
  ) => api.get<CRMPaginatedResponse<SupportConversation[]>>(`/crm/companies/${companyId}/support-conversations${qs(workspaceId)}${filterQuery(params ?? {})}`),
};

export const crmDealService = {
  list: (
    workspaceId: string,
    filters?: {
      pipeline_id?: string;
      stage_id?: string;
      owner_member_id?: string;
      contact_id?: string;
      search?: string;
      page?: number;
      per_page?: number;
    },
  ) => api.get<CRMPaginatedResponse<CRMDeal[]>>(`/crm/deals${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  get: (workspaceId: string, id: string) => api.get<CRMDeal>(`/crm/deals/${id}${qs(workspaceId)}`),
  create: (payload: CreateCRMDealRequest) => api.post<CRMDeal>(`/crm/deals${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCRMDealRequest) => api.put<CRMDeal>(`/crm/deals/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) => api.del(`/crm/deals/${id}${qs(workspaceId)}`),
  listActivities: (workspaceId: string, dealId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMActivity[]>>(`/crm/deals/${dealId}/activities${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
  listTimeline: (workspaceId: string, dealId: string, filter: CRMTimelineFilter, cursor?: string) =>
    api.get<CRMTimelinePage>(`/crm/deals/${dealId}/timeline${qs(workspaceId)}${filterQuery({ filter, cursor, limit: 25 })}`),
  listAssociations: (workspaceId: string, dealId: string) => api.get<CRMAssociationEnriched[]>(`/crm/deals/${dealId}/associations${qs(workspaceId)}`),
};

export const crmPipelineService = {
  list: (workspaceId: string) => api.get<CRMPipeline[]>(`/crm/pipelines${qs(workspaceId)}`),
  get: (workspaceId: string, id: string) => api.get<CRMPipeline>(`/crm/pipelines/${id}${qs(workspaceId)}`),
  create: (payload: CreateCRMPipelineRequest) => api.post<CRMPipeline>(`/crm/pipelines${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCRMPipelineRequest) => api.put<CRMPipeline>(`/crm/pipelines/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) => api.del(`/crm/pipelines/${id}${qs(workspaceId)}`),
};

export const crmAssociationService = {
  create: (payload: CreateCRMAssociationRequest) => api.post<CRMAssociation>(`/crm/associations${qs(payload.workspace_id)}`, payload),
  createFromPM: (payload: CreateCRMAssociationRequest) => api.post<CRMAssociation>(`/pm/associations${qs(payload.workspace_id)}`, payload),
  remove: (workspaceId: string, id: string) => api.del(`/crm/associations/${id}${qs(workspaceId)}`),
  removeFromPM: (workspaceId: string, id: string) => api.del(`/pm/associations/${id}${qs(workspaceId)}`),
  listByEpic: (workspaceId: string, epicId: string) => api.get<CRMAssociationEnriched[]>(`/pm/epics/${epicId}/associations${qs(workspaceId)}`),
  listByTask: (workspaceId: string, taskId: string) => api.get<CRMAssociationEnriched[]>(`/pm/tasks/${taskId}/associations${qs(workspaceId)}`),
};

export const crmActivityService = {
  list: (
    workspaceId: string,
    filters?: {
      activity_type?: string;
      contact_id?: string;
      company_id?: string;
      deal_id?: string;
      page?: number;
      per_page?: number;
    },
  ) => api.get<CRMPaginatedResponse<CRMActivity[]>>(`/crm/activities${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  get: (workspaceId: string, id: string) => api.get<CRMActivity>(`/crm/activities/${id}${qs(workspaceId)}`),
  create: (payload: CreateCRMActivityRequest) => api.post<CRMActivity>(`/crm/activities${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCRMActivityRequest) => api.put<CRMActivity>(`/crm/activities/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) => api.del(`/crm/activities/${id}${qs(workspaceId)}`),
};

export const crmImportService = {
  list: (workspaceId: string, page?: number) => api.get<CRMPaginatedResponse<CRMImportJob[]>>(`/crm/imports${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
  get: (workspaceId: string, id: string) => api.get<CRMImportJob>(`/crm/imports/${id}${qs(workspaceId)}`),
  create: (payload: CreateCRMImportRequest) => api.post<CRMImportJob>(`/crm/imports${qs(payload.workspace_id)}`, payload),
  process: (workspaceId: string, id: string, payload: ProcessCRMImportRequest) => api.post<CRMImportJob>(`/crm/imports/${id}/process${qs(workspaceId)}`, payload),
};

// ── Phase 3: Email & Calendar ──

export const crmEmailService = {
  listAccounts: (workspaceId: string, filters?: { member_id?: string; provider?: string }) => api.get<CRMEmailAccount[]>(`/crm/email/accounts${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  getAccount: (workspaceId: string, id: string) => api.get<CRMEmailAccount>(`/crm/email/accounts/${id}${qs(workspaceId)}`),
  createAccount: (payload: CreateCRMEmailAccountRequest) => api.post<CRMEmailAccount>(`/crm/email/accounts${qs(payload.workspace_id)}`, payload),
  disconnectAccount: (workspaceId: string, id: string) => api.del(`/crm/email/accounts/${id}${qs(workspaceId)}`),
  purgeAccountData: (workspaceId: string, id: string) => api.del(`/crm/email/accounts/${id}/data${qs(workspaceId)}`),
  getAccountDiagnostics: (workspaceId: string, id: string) => api.get<CRMEmailAccountDiagnostics>(`/crm/email/accounts/${id}/diagnostics${qs(workspaceId)}`),
  syncAccount: (workspaceId: string, id: string, mode: 'incremental' | 'historical') => api.post<CRMEmailAccount>(`/crm/email/accounts/${id}/sync${qs(workspaceId)}`, { mode }),
  initiateOAuth: (workspaceId: string, provider: 'gmail' | 'microsoft' = 'gmail') => api.get<{ redirect_url: string }>(`/crm/email/oauth/initiate${qs(workspaceId)}&provider=${provider}`),
  sendEmail: (
    workspaceId: string,
    payload: {
      account_id: string;
      to: string[];
      cc?: string[];
      subject: string;
      body_html: string;
    },
  ) => api.post<CRMEmailMessage>(`/crm/email/send${qs(workspaceId)}`, payload),
  listThreads: (
    workspaceId: string,
    filters?: {
      email_account_id?: string;
      contact_id?: string;
      company_id?: string;
      deal_id?: string;
      search?: string;
      scope?: 'all' | 'direct' | 'needs_reply';
      sort?: 'newest' | 'oldest';
      page?: number;
      per_page?: number;
    },
  ) => api.get<CRMPaginatedResponse<CRMEmailThread[]>>(`/crm/email/threads${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  getThread: (workspaceId: string, threadId: string) =>
    api.get<CRMEmailThreadDetail>(`/crm/email/threads/${threadId}${qs(workspaceId)}`),
  replyToThread: (workspaceId: string, threadId: string, payload: { mode: 'reply' | 'reply_all'; body_html: string }) =>
    api.post<CRMEmailMessage>(`/crm/email/threads/${threadId}/reply${qs(workspaceId)}`, payload),
  setNeedsReplyDismissed: (workspaceId: string, threadId: string, dismissed: boolean) =>
    dismissed
      ? api.put(`/crm/email/threads/${threadId}/needs-reply-dismissal${qs(workspaceId)}`, {})
      : api.del(`/crm/email/threads/${threadId}/needs-reply-dismissal${qs(workspaceId)}`),
  linkThreadDeal: (workspaceId: string, threadId: string, dealId?: string) =>
    api.patch(`/crm/email/threads/${threadId}/deal${qs(workspaceId)}`, { deal_id: dealId || null }),
  listMessages: (
    workspaceId: string,
    filters?: {
      thread_id?: string;
      email_account_id?: string;
      contact_id?: string;
      deal_id?: string;
      direction?: string;
      page?: number;
    },
  ) => api.get<CRMPaginatedResponse<CRMEmailMessage[]>>(`/crm/email/messages${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  createMessage: (payload: CreateCRMEmailMessageRequest) => api.post<CRMEmailMessage>(`/crm/email/messages${qs(payload.workspace_id)}`, payload),
  listByContact: (workspaceId: string, contactId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMEmailMessage[]>>(`/crm/contacts/${contactId}/emails${qs(workspaceId)}&per_page=50${page ? `&page=${page}` : ''}`),
  listByCompany: (workspaceId: string, companyId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMEmailMessage[]>>(`/crm/companies/${companyId}/emails${qs(workspaceId)}&per_page=50${page ? `&page=${page}` : ''}`),
  listByDeal: (workspaceId: string, dealId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMEmailMessage[]>>(`/crm/deals/${dealId}/emails${qs(workspaceId)}&per_page=50${page ? `&page=${page}` : ''}`),
};

export const crmCalendarService = {
  list: (
    workspaceId: string,
    filters?: {
      email_account_id?: string;
      deal_id?: string;
      start_after?: string;
      start_before?: string;
      page?: number;
    },
  ) => api.get<CRMPaginatedResponse<CRMCalendarEvent[]>>(`/crm/calendar/events${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  get: (workspaceId: string, id: string) => api.get<CRMCalendarEvent>(`/crm/calendar/events/${id}${qs(workspaceId)}`),
  create: (payload: CreateCRMCalendarEventRequest) => api.post<CRMCalendarEvent>(`/crm/calendar/events${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCRMCalendarEventRequest) => api.put<CRMCalendarEvent>(`/crm/calendar/events/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) => api.del(`/crm/calendar/events/${id}${qs(workspaceId)}`),
  listByContact: (workspaceId: string, contactId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMCalendarEvent[]>>(`/crm/contacts/${contactId}/calendar${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
  listByDeal: (workspaceId: string, dealId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMCalendarEvent[]>>(`/crm/deals/${dealId}/calendar${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
};

// ── Phase 4: Intelligence ──

export const crmEnrichmentService = {
  list: (
    workspaceId: string,
    filters?: {
      object_type?: string;
      object_id?: string;
      source?: string;
      page?: number;
    },
  ) => api.get<CRMPaginatedResponse<CRMEnrichmentResult[]>>(`/crm/enrichments${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  create: (payload: CreateCRMEnrichmentRequest) => api.post<CRMEnrichmentResult>(`/crm/enrichments${qs(payload.workspace_id)}`, payload),
  applySuggestion: (workspaceId: string, enrichmentId: string, field: string) => api.post<CRMEnrichmentResult>(`/crm/enrichments/${enrichmentId}/apply-suggestion${qs(workspaceId)}`, { field }),
};

export const crmSignalService = {
  feed: (workspaceId: string, filters?: CRMSignalFeedFilters) => api.get<CRMSignalWorkspaceFeed>(`/crm/signals/feed${qs(workspaceId)}${filterQuery({ ...(filters ?? {}) })}`),
  list: (
    workspaceId: string,
    filters?: {
      contact_id?: string;
      deal_id?: string;
      company_id?: string;
      signal_type?: string;
      source_type?: string;
      page?: number;
    },
  ) => api.get<CRMPaginatedResponse<CRMBuyerSignal[]>>(`/crm/signals${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  create: (payload: CreateCRMBuyerSignalRequest) => api.post<CRMBuyerSignal>(`/crm/signals${qs(payload.workspace_id)}`, payload),
  ingestExternalEvidence: (payload: IngestCRMSignalExternalEvidenceRequest) => api.post<CRMSignalExternalEvidence>(`/crm/signals/external-evidence${qs(payload.workspace_id)}`, payload),
  remove: (workspaceId: string, id: string) => api.del(`/crm/signals/${id}${qs(workspaceId)}`),
  dismiss: (workspaceId: string, id: string, reason: import('../crmTypes').CRMSignalDismissalReason) => api.post(`/crm/signals/${id}/dismiss${qs(workspaceId)}`, { reason }),
  review: (workspaceId: string, id: string) => api.post(`/crm/signals/${id}/review${qs(workspaceId)}`, {}),
  acted: (workspaceId: string, id: string) => api.post(`/crm/signals/${id}/acted${qs(workspaceId)}`, {}),
  listByContact: (workspaceId: string, contactId: string, page?: number) =>
    api.get<CRMPaginatedResponse<CRMBuyerSignal[]>>(`/crm/contacts/${contactId}/signals${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
  listByDeal: (workspaceId: string, dealId: string, page?: number) => api.get<CRMPaginatedResponse<CRMBuyerSignal[]>>(`/crm/deals/${dealId}/signals${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
  listByCompany: (workspaceId: string, companyId: string, page?: number) => api.get<CRMPaginatedResponse<CRMBuyerSignal[]>>(`/crm/companies/${companyId}/signals${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
};

export const crmSummaryService = {
  getForContact: (workspaceId: string, contactId: string) => api.get<CRMEntitySummary | null>(`/crm/contacts/${contactId}/summary${qs(workspaceId)}`),
  getForDeal: (workspaceId: string, dealId: string) => api.get<CRMEntitySummary | null>(`/crm/deals/${dealId}/summary${qs(workspaceId)}`),
  getForCompany: (workspaceId: string, companyId: string) => api.get<CRMEntitySummary | null>(`/crm/companies/${companyId}/summary${qs(workspaceId)}`),
  refreshContact: (workspaceId: string, contactId: string) => api.post<CRMEntitySummary>(`/crm/contacts/${contactId}/summary/refresh${qs(workspaceId)}`, { force: true }),
  refreshDeal: (workspaceId: string, dealId: string) => api.post<CRMEntitySummary>(`/crm/deals/${dealId}/summary/refresh${qs(workspaceId)}`, {}),
  refreshCompany: (workspaceId: string, companyId: string) => api.post<CRMEntitySummary>(`/crm/companies/${companyId}/summary/refresh${qs(workspaceId)}`, { force: true }),
  refreshContactIntelligence: (workspaceId: string, contactId: string) => api.post<CRMIntelligenceRefreshResult>(`/crm/contacts/${contactId}/intelligence/refresh${qs(workspaceId)}`, {}),
  refreshDealIntelligence: (workspaceId: string, dealId: string) => api.post<CRMIntelligenceRefreshResult>(`/crm/deals/${dealId}/intelligence/refresh${qs(workspaceId)}`, {}),
  refreshCompanyIntelligence: (workspaceId: string, companyId: string) => api.post<CRMIntelligenceRefreshResult>(`/crm/companies/${companyId}/intelligence/refresh${qs(workspaceId)}`, {}),
};

export const crmHealthScoreService = {
  list: (workspaceId: string, page?: number) => api.get<CRMPaginatedResponse<CRMDealHealthScore[]>>(`/crm/health-scores${qs(workspaceId)}${page ? `&page=${page}` : ''}`),
  getForDeal: (workspaceId: string, dealId: string) => api.get<CRMDealHealthScore>(`/crm/deals/${dealId}/health-score${qs(workspaceId)}`),
  create: (payload: CreateCRMDealHealthScoreRequest) => api.post<CRMDealHealthScore>(`/crm/health-scores${qs(payload.workspace_id)}`, payload),
};

export const crmSuggestionService = {
  list: (
    workspaceId: string,
    filters?: {
      user_id?: string;
      suggestion_type?: string;
      object_type?: string;
      object_id?: string;
      status?: string;
      page?: number;
    },
  ) => api.get<CRMPaginatedResponse<CRMSuggestion[]>>(`/crm/suggestions${qs(workspaceId)}${filterQuery(filters ?? {})}`),
  get: (workspaceId: string, id: string) => api.get<CRMSuggestion>(`/crm/suggestions/${id}${qs(workspaceId)}`),
  create: (payload: CreateCRMSuggestionRequest) => api.post<CRMSuggestion>(`/crm/suggestions${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCRMSuggestionRequest) => api.put<CRMSuggestion>(`/crm/suggestions/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) => api.del(`/crm/suggestions/${id}${qs(workspaceId)}`),
  accept: (workspaceId: string, id: string, edits?: Record<string, unknown>) => api.post<CRMSuggestion>(`/crm/suggestions/${id}/accept${qs(workspaceId)}`, edits || {}),
  dismiss: (workspaceId: string, id: string) => api.post<CRMSuggestion>(`/crm/suggestions/${id}/dismiss${qs(workspaceId)}`, {}),
};

export const crmWritingProfileService = {
  list: (workspaceId: string) => api.get<CRMWritingProfile[]>(`/crm/writing-profiles${qs(workspaceId)}`),
  getByMember: (workspaceId: string, memberId: string) => api.get<CRMWritingProfile>(`/crm/writing-profiles/member/${memberId}${qs(workspaceId)}`),
  create: (payload: CreateCRMWritingProfileRequest) => api.post<CRMWritingProfile>(`/crm/writing-profiles${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, id: string, payload: UpdateCRMWritingProfileRequest) => api.put<CRMWritingProfile>(`/crm/writing-profiles/${id}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, id: string) => api.del(`/crm/writing-profiles/${id}${qs(workspaceId)}`),
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
  get: (workspaceId: string) => api.get<CRMAutonomySettings>(`/crm/autonomy-settings${qs(workspaceId)}`),
  update: (workspaceId: string, settings: CRMAutonomySettings) => api.put<CRMAutonomySettings>(`/crm/autonomy-settings${qs(workspaceId)}`, settings),
};

// ── Email Sync Settings ──

export const crmEmailSyncSettingsService = {
  get: (workspaceId: string) => api.get<CRMEmailSyncSettings>(`/crm/email/sync-settings${qs(workspaceId)}`),
  update: (workspaceId: string, settings: UpdateCRMEmailSyncSettingsRequest) => api.put<CRMEmailSyncSettings>(`/crm/email/sync-settings${qs(workspaceId)}`, settings),
  getDefaultPrefixes: () => api.get<string[]>('/crm/email/sync-settings/default-prefixes'),
};

// ── Phase 6: Search ──

export const crmSearchService = {
  search: (workspaceId: string, q: string) => api.get<CRMSearchResult[]>(`/crm/search${qs(workspaceId)}&q=${encodeURIComponent(q)}`),
};
