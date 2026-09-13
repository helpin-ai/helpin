import type { CRMPipeline } from '@/lib/crmTypes';
import { useInfiniteQuery, useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  crmContactService,
  crmCompanyService,
  crmDealService,
  crmPipelineService,
  crmAssociationService,
  crmActivityService,
  crmImportService,
  crmEmailService,
  crmCalendarService,
  crmEnrichmentService,
  crmSignalService,
  crmSummaryService,
  crmHealthScoreService,
  crmSuggestionService,
  crmWritingProfileService,
  crmSearchService,
  crmAutonomyService,
  crmEmailSyncSettingsService,
} from '@/lib/services/crmService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type {
  CreateCRMContactRequest,
  UpdateCRMContactRequest,
  CreateCRMCompanyRequest,
  UpdateCRMCompanyRequest,
  CreateCRMDealRequest,
  UpdateCRMDealRequest,
	SetCRMDealCustomerRequest,
  CreateCRMPipelineRequest,
  UpdateCRMPipelineRequest,
  CreateCRMAssociationRequest,
  CreateCRMActivityRequest,
  UpdateCRMActivityRequest,
  CreateCRMImportRequest,
  ProcessCRMImportRequest,
  CreateCRMEmailAccountRequest,
  CreateCRMEmailMessageRequest,
  CreateCRMCalendarEventRequest,
  UpdateCRMCalendarEventRequest,
  CreateCRMEnrichmentRequest,
  CreateCRMSignalRequest,
  CreateCRMDealHealthScoreRequest,
  CreateCRMSuggestionRequest,
  UpdateCRMSuggestionRequest,
  CreateCRMWritingProfileRequest,
  UpdateCRMWritingProfileRequest,
  CRMTimelineFilter,
  CRMCompanyTimelineFilter,
  CRMSignalFeedFilters,
} from '@/lib/crmTypes'

// ── Contacts ──

interface ContactFilters {
  lifecycle_stage?: string
  lead_status?: string
  owner_member_id?: string
  search?: string
  filters?: string
  page?: number
  per_page?: number
}

export function useContacts(wsId: string, filters?: ContactFilters) {
  return useQuery({
    queryKey: [...queryKeys.crm.contacts(wsId), filters],
    queryFn: async () => unwrap(await crmContactService.list(wsId, filters)),
    enabled: !!wsId,
  })
}

export function useContact(wsId: string, id: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.crm.contact(wsId, id),
    queryFn: async () => unwrap(await crmContactService.get(wsId, id)),
    enabled: enabled && !!wsId && !!id,
  })
}

export function useContactActivities(wsId: string, contactId: string) {
  return useQuery({
    queryKey: queryKeys.crm.contactActivities(wsId, contactId),
    queryFn: async () => unwrap(await crmContactService.listActivities(wsId, contactId)),
    enabled: !!wsId && !!contactId,
  })
}

export function useContactTimeline(wsId: string, contactId: string, filter: CRMTimelineFilter, enabled = true) {
  return useInfiniteQuery({
    queryKey: queryKeys.crm.contactTimeline(wsId, contactId, filter),
    queryFn: async ({ pageParam }) => unwrap(await crmContactService.listTimeline(wsId, contactId, filter, pageParam)),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage?.next_cursor,
    enabled: enabled && !!wsId && !!contactId,
  })
}

export function useContactAssociations(wsId: string, contactId: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.crm.contactAssociations(wsId, contactId),
    queryFn: async () => unwrap(await crmContactService.listAssociations(wsId, contactId)),
    enabled: enabled && !!wsId && !!contactId,
  })
}

export function useContactSupportConversations(
  wsId: string,
  contactId: string,
  filtersOrEnabled?: { status?: string; search?: string; page?: number; per_page?: number } | boolean,
  enabled = true,
) {
  const filters = typeof filtersOrEnabled === 'boolean' ? undefined : filtersOrEnabled
  const queryEnabled = typeof filtersOrEnabled === 'boolean' ? filtersOrEnabled : enabled
  return useQuery({
    queryKey: queryKeys.crm.contactSupportConversations(wsId, contactId, filters),
    queryFn: async () => unwrap(await crmContactService.listSupportConversations(wsId, contactId, filters)),
    enabled: queryEnabled && !!wsId && !!contactId,
  })
}

export function useCreateContact(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMContactRequest) => unwrap(await crmContactService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.contacts(wsId) })
    },
  })
}

export function useUpdateContact(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateCRMContactRequest & { id: string }) =>
      unwrap(await crmContactService.update(wsId, id, data)),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.contacts(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.crm.contact(wsId, id) })
      qc.invalidateQueries({
        queryKey: queryKeys.crm.contactActivities(wsId, id),
      })
      qc.invalidateQueries({ queryKey: ['support', wsId] })
    },
  })
}

export function useDeleteContact(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await crmContactService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.contacts(wsId) })
    },
  })
}

// ── Companies ──

interface CompanyFilters {
  industry?: string
  owner_member_id?: string
  search?: string
  page?: number
  per_page?: number
}

export function useCompanies(wsId: string, filters?: CompanyFilters) {
  return useQuery({
    queryKey: [...queryKeys.crm.companies(wsId), filters],
    queryFn: async () => unwrap(await crmCompanyService.list(wsId, filters)),
    enabled: !!wsId,
  })
}

export function useCompany(wsId: string, id: string) {
  return useQuery({
    queryKey: queryKeys.crm.company(wsId, id),
    queryFn: async () => unwrap(await crmCompanyService.get(wsId, id)),
    enabled: !!wsId && !!id,
  })
}

export function useCompanyActivities(wsId: string, companyId: string) {
  return useQuery({
    queryKey: queryKeys.crm.companyActivities(wsId, companyId),
    queryFn: async () => unwrap(await crmCompanyService.listActivities(wsId, companyId)),
    enabled: !!wsId && !!companyId,
  })
}

export function useCompanyTimeline(wsId: string, companyId: string, filter: CRMCompanyTimelineFilter, enabled = true) {
  return useInfiniteQuery({
    queryKey: queryKeys.crm.companyTimeline(wsId, companyId, filter),
    queryFn: async ({ pageParam }) => unwrap(await crmCompanyService.listTimeline(wsId, companyId, filter, pageParam)),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage?.next_cursor,
    enabled: enabled && !!wsId && !!companyId,
  })
}

export function useCompanyAssociations(wsId: string, companyId: string) {
  return useQuery({
    queryKey: queryKeys.crm.companyAssociations(wsId, companyId),
    queryFn: async () => unwrap(await crmCompanyService.listAssociations(wsId, companyId)),
    enabled: !!wsId && !!companyId,
  })
}

export function useCompanyContacts(
  wsId: string,
  companyId: string,
  filters?: { search?: string; page?: number; per_page?: number },
) {
  return useQuery({
    queryKey: queryKeys.crm.companyContacts(wsId, companyId, filters),
    queryFn: async () => unwrap(await crmCompanyService.listContacts(wsId, companyId, filters)),
    enabled: !!wsId && !!companyId,
  })
}

export function useCompanyDeals(
  wsId: string,
  companyId: string,
  filters?: { search?: string; page?: number; per_page?: number },
) {
  return useQuery({
    queryKey: queryKeys.crm.companyDeals(wsId, companyId, filters),
    queryFn: async () => unwrap(await crmCompanyService.listDeals(wsId, companyId, filters)),
    enabled: !!wsId && !!companyId,
  })
}

export function useCompanySupportConversations(
  wsId: string,
  companyId: string,
  filters?: {
    status?: string
    search?: string
    page?: number
    per_page?: number
  },
) {
  return useQuery({
    queryKey: queryKeys.crm.companySupportConversations(wsId, companyId, filters),
    queryFn: async () => unwrap(await crmCompanyService.listSupportConversations(wsId, companyId, filters)),
    enabled: !!wsId && !!companyId,
  })
}

export function useCreateCompany(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMCompanyRequest) => unwrap(await crmCompanyService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.companies(wsId) })
    },
  })
}

export function useUpdateCompany(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateCRMCompanyRequest & { id: string }) =>
      unwrap(await crmCompanyService.update(wsId, id, data)),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.companies(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.crm.company(wsId, id) })
    },
  })
}

export function useDeleteCompany(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await crmCompanyService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.companies(wsId) })
    },
  })
}

// ── Deals ──

interface DealFilters {
  filters?: string
  pipeline_id?: string
  stage_id?: string
  owner_member_id?: string
  contact_id?: string
  search?: string
  page?: number
  per_page?: number
}

export function useDeals(wsId: string, filters?: DealFilters) {
  return useQuery({
    queryKey: [...queryKeys.crm.deals(wsId), filters],
    queryFn: async () => unwrap(await crmDealService.list(wsId, filters)),
    enabled: !!wsId,
  })
}

export function useDeal(wsId: string, id: string) {
  return useQuery({
    queryKey: queryKeys.crm.deal(wsId, id),
    queryFn: async () => unwrap(await crmDealService.get(wsId, id)),
    enabled: !!wsId && !!id,
  })
}

export function useDealActivities(wsId: string, dealId: string) {
  return useQuery({
    queryKey: queryKeys.crm.dealActivities(wsId, dealId),
    queryFn: async () => unwrap(await crmDealService.listActivities(wsId, dealId)),
    enabled: !!wsId && !!dealId,
  })
}

export function useDealTimeline(wsId: string, dealId: string, filter: CRMTimelineFilter) {
  return useInfiniteQuery({
    queryKey: queryKeys.crm.dealTimeline(wsId, dealId, filter),
    queryFn: async ({ pageParam }) => unwrap(await crmDealService.listTimeline(wsId, dealId, filter, pageParam)),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage?.next_cursor,
    enabled: !!wsId && !!dealId,
  })
}

export function useDealAssociations(wsId: string, dealId: string) {
  return useQuery({
    queryKey: queryKeys.crm.dealAssociations(wsId, dealId),
    queryFn: async () => unwrap(await crmDealService.listAssociations(wsId, dealId)),
    enabled: !!wsId && !!dealId,
  })
}

export function useCreateDeal(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMDealRequest) => unwrap(await crmDealService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.deals(wsId) })
    },
  })
}

export function useUpdateDeal(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateCRMDealRequest & { id: string }) =>
      unwrap(await crmDealService.update(wsId, id, data)),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.deals(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.crm.deal(wsId, id) })
    },
  })
}

export function useSetDealCustomer(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: SetCRMDealCustomerRequest & { id: string }) =>
      unwrap(await crmDealService.setCustomer(wsId, id, data)),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.dealAssociations(wsId, id) })
      qc.invalidateQueries({ queryKey: queryKeys.crm.deals(wsId) })
      qc.invalidateQueries({ queryKey: ['crm', wsId, 'companies'] })
    },
  })
}

export function useDeleteDeal(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await crmDealService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.deals(wsId) })
    },
  })
}

// ── Pipelines ──

export function usePipelines(wsId: string) {
  return useQuery({
    queryKey: queryKeys.crm.pipelines(wsId),
    queryFn: async () => unwrap(await crmPipelineService.list(wsId)),
    enabled: !!wsId,
  })
}

export function usePipeline(wsId: string, id: string) {
  return useQuery({
    queryKey: queryKeys.crm.pipeline(wsId, id),
    queryFn: async () => unwrap(await crmPipelineService.get(wsId, id)),
    enabled: !!wsId && !!id,
  })
}

export function useCreatePipeline(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMPipelineRequest) => unwrap(await crmPipelineService.create(data)),
    onSuccess: async (pipeline) => {
      qc.setQueryData<CRMPipeline[]>(queryKeys.crm.pipelines(wsId), (current) => [
        ...(current ?? []).map((item) => pipeline.is_default ? { ...item, is_default: false } : item),
        pipeline,
      ])
      await qc.invalidateQueries({ queryKey: queryKeys.crm.pipelines(wsId) })
    },
  })
}

export function useUpdatePipeline(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateCRMPipelineRequest & { id: string }) => {
      const response = await crmPipelineService.update(wsId, id, data)
      if (response.error) throw Object.assign(new Error(response.error), { status: response.status })
      return unwrap(response)
    },
    onSuccess: async (pipeline, { id }) => {
      qc.setQueryData<CRMPipeline[]>(queryKeys.crm.pipelines(wsId), (current) => current?.map((item) => item.id === id ? pipeline : pipeline.is_default ? { ...item, is_default: false } : item))
      qc.setQueryData(queryKeys.crm.pipeline(wsId, id), pipeline)
      // Stage names, probabilities and migrated deals also appear outside settings.
      await qc.invalidateQueries({ queryKey: ['crm', wsId] })
    },
    onError: async () => {
      await qc.invalidateQueries({ queryKey: queryKeys.crm.pipelines(wsId) })
    },
  })
}

export function useDeletePipeline(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await crmPipelineService.remove(wsId, id)),
    onSuccess: async (_, id) => {
      qc.setQueryData<CRMPipeline[]>(queryKeys.crm.pipelines(wsId), (current) => current?.filter((item) => item.id !== id))
      qc.removeQueries({ queryKey: queryKeys.crm.pipeline(wsId, id), exact: true })
      await qc.invalidateQueries({ queryKey: queryKeys.crm.pipelines(wsId) })
    },
  })
}

// ── Associations ──

export function useLegacyEpicAssociations(wsId: string, epicId: string) {
  return useQuery({
    queryKey: queryKeys.pm.epicAssociations(wsId, epicId),
    queryFn: async () => unwrap(await crmAssociationService.listByEpic(wsId, epicId)),
    enabled: !!wsId && !!epicId,
  })
}

export function useLegacyTaskAssociations(wsId: string, taskId: string) {
  return useQuery({
    queryKey: queryKeys.pm.taskAssociations(wsId, taskId),
    queryFn: async () => unwrap(await crmAssociationService.listByTask(wsId, taskId)),
    enabled: !!wsId && !!taskId,
  })
}

export function useCreateAssociation(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMAssociationRequest) => unwrap(await crmAssociationService.create(data)),
    onSuccess: (_, data) => {
      qc.invalidateQueries({ queryKey: ['crm', wsId] })
      if (
        data.from_object_type === 'epic' ||
        data.to_object_type === 'epic' ||
        data.from_object_type === 'task' ||
        data.to_object_type === 'task'
      ) {
        qc.invalidateQueries({ queryKey: ['pm', wsId] })
      }
    },
  })
}

export function useLegacyCreateAssociationFromPM(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMAssociationRequest) => unwrap(await crmAssociationService.createFromPM(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['crm', wsId] })
      qc.invalidateQueries({ queryKey: ['pm', wsId] })
    },
  })
}

export function useDeleteAssociation(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await crmAssociationService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['crm', wsId] })
      qc.invalidateQueries({ queryKey: ['pm', wsId] })
    },
  })
}

export function useLegacyDeleteAssociationFromPM(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await crmAssociationService.removeFromPM(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['crm', wsId] })
      qc.invalidateQueries({ queryKey: ['pm', wsId] })
    },
  })
}

// ── Activities ──

interface ActivityFilters {
  activity_type?: string
  contact_id?: string
  company_id?: string
  deal_id?: string
  page?: number
  per_page?: number
}

export function useCRMActivities(wsId: string, filters?: ActivityFilters) {
  return useQuery({
    queryKey: [...queryKeys.crm.activities(wsId), filters],
    queryFn: async () => unwrap(await crmActivityService.list(wsId, filters)),
    enabled: !!wsId,
  })
}

export function useCreateCRMActivity(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMActivityRequest) => unwrap(await crmActivityService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.activities(wsId) })
      // Also invalidate entity-specific activity queries
      qc.invalidateQueries({ queryKey: ['crm', wsId] })
    },
  })
}

export function useUpdateCRMActivity(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateCRMActivityRequest & { id: string }) =>
      unwrap(await crmActivityService.update(wsId, id, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.activities(wsId) })
      qc.invalidateQueries({ queryKey: ['crm', wsId] })
    },
  })
}

export function useDeleteCRMActivity(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await crmActivityService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.activities(wsId) })
      qc.invalidateQueries({ queryKey: ['crm', wsId] })
    },
  })
}

// ── Imports ──

export function useCRMImports(wsId: string) {
  return useQuery({
    queryKey: queryKeys.crm.imports(wsId),
    queryFn: async () => unwrap(await crmImportService.list(wsId)),
    enabled: !!wsId,
  })
}

export function useCRMImport(wsId: string, id: string) {
  return useQuery({
    queryKey: queryKeys.crm.import(wsId, id),
    queryFn: async () => unwrap(await crmImportService.get(wsId, id)),
    enabled: !!wsId && !!id,
  })
}

export function useCreateCRMImport(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMImportRequest) => unwrap(await crmImportService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.imports(wsId) })
    },
  })
}

export function useProcessCRMImport(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: ProcessCRMImportRequest & { id: string }) =>
      unwrap(await crmImportService.process(wsId, id, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.imports(wsId) })
      qc.invalidateQueries({ queryKey: ['crm', wsId] })
    },
  })
}

// ── Phase 3: Email & Calendar ──

export function useEmailAccounts(wsId: string, filters?: { member_id?: string }) {
  return useQuery({
    queryKey: [...queryKeys.crm.emailAccounts(wsId), filters ?? {}],
    queryFn: async () => unwrap(await crmEmailService.listAccounts(wsId, filters)),
    enabled: !!wsId,
    refetchInterval: (query) => {
      const accounts = query.state.data
      const hasActiveSync = accounts?.some((account) => {
        const phase = account.sync_state?.phase
        return typeof phase === 'string' && ['backfill', 'incremental', 'recovery'].includes(phase)
      })
      return hasActiveSync ? 2_500 : false
    },
  })
}

export function useInfiniteEmailThreads(
  wsId: string,
  filters: {
    contact_id?: string
    company_id?: string
    deal_id?: string
    search?: string
    scope?: 'all' | 'direct' | 'needs_reply'
    sort?: 'newest' | 'oldest'
  },
) {
  return useInfiniteQuery({
    queryKey: [...queryKeys.crm.emailThreads(wsId), filters, 'infinite'],
    queryFn: async ({ pageParam }) => unwrap(await crmEmailService.listThreads(wsId, { ...filters, page: pageParam, per_page: 40 })),
    initialPageParam: 1,
    getNextPageParam: (lastPage) => (lastPage.page * 40 < lastPage.total ? lastPage.page + 1 : undefined),
    enabled: !!wsId && !!(filters.contact_id || filters.company_id || filters.deal_id),
  })
}

export function useEmailThread(wsId: string, threadId?: string) {
  return useQuery({
    queryKey: [...queryKeys.crm.emailThreads(wsId), threadId],
    queryFn: async () => unwrap(await crmEmailService.getThread(wsId, threadId!)),
    enabled: !!wsId && !!threadId,
  })
}

export function useReplyToEmailThread(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ threadId, mode, body_html, draft_id, attachment_ids }: { threadId: string; mode: 'reply' | 'reply_all'; body_html: string; draft_id?: string; attachment_ids?: string[] }) =>
      unwrap(await crmEmailService.replyToThread(wsId, threadId, { mode, body_html, draft_id, attachment_ids })),
    onSuccess: (_, variables) => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.emailThreads(wsId) })
      qc.invalidateQueries({ queryKey: [...queryKeys.crm.emailThreads(wsId), variables.threadId] })
      qc.invalidateQueries({ queryKey: ['crm', wsId, 'contacts'] })
      qc.invalidateQueries({ queryKey: ['crm', wsId, 'companies'] })
    },
  })
}

export function useSetEmailThreadDismissed(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ threadId, dismissed }: { threadId: string; dismissed: boolean }) =>
      unwrap(await crmEmailService.setNeedsReplyDismissed(wsId, threadId, dismissed)),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.crm.emailThreads(wsId) }),
  })
}

export function useLinkEmailThreadDeal(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ threadId, dealId }: { threadId: string; dealId?: string }) =>
      unwrap(await crmEmailService.linkThreadDeal(wsId, threadId, dealId)),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.crm.emailThreads(wsId) }),
  })
}

export function useCreateEmailAccount(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMEmailAccountRequest) => unwrap(await crmEmailService.createAccount(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.emailAccounts(wsId) })
    },
  })
}

export function useDisconnectEmailAccount(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await crmEmailService.disconnectAccount(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['crm', wsId] })
    },
  })
}

export function usePurgeEmailAccount(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await crmEmailService.purgeAccountData(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['crm', wsId] })
    },
  })
}

export function useEmailAccountDiagnostics(wsId: string, accountId: string, enabled = true) {
  return useQuery({
    queryKey: [...queryKeys.crm.emailAccounts(wsId), accountId, 'diagnostics'],
    queryFn: async () => unwrap(await crmEmailService.getAccountDiagnostics(wsId, accountId)),
    enabled: enabled && !!wsId && !!accountId,
    refetchInterval: (query) => {
      const phase = query.state.data?.sync.phase
      return phase && !['idle', 'error', 'disconnected'].includes(phase) ? 2_500 : false
    },
  })
}

export function useSyncEmailAccount(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, mode }: { id: string; mode: 'incremental' | 'historical' }) =>
      unwrap(await crmEmailService.syncAccount(wsId, id, mode)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.emailAccounts(wsId) })
    },
  })
}

export function useContactEmails(wsId: string, contactId: string) {
  return useQuery({
    queryKey: queryKeys.crm.contactEmails(wsId, contactId),
    queryFn: async () => unwrap(await crmEmailService.listByContact(wsId, contactId)),
    enabled: !!wsId && !!contactId,
  })
}

export function useDealEmails(wsId: string, dealId: string) {
  return useQuery({
    queryKey: queryKeys.crm.dealEmails(wsId, dealId),
    queryFn: async () => unwrap(await crmEmailService.listByDeal(wsId, dealId)),
    enabled: !!wsId && !!dealId,
  })
}

export function useInfiniteContactEmails(wsId: string, contactId: string) {
  return useInfiniteQuery({
    queryKey: [...queryKeys.crm.contactEmails(wsId, contactId), 'infinite'],
    queryFn: async ({ pageParam }) => unwrap(await crmEmailService.listByContact(wsId, contactId, pageParam)),
    initialPageParam: 1,
    getNextPageParam: (lastPage) => (lastPage.page * 50 < lastPage.total ? lastPage.page + 1 : undefined),
    enabled: !!wsId && !!contactId,
  })
}

export function useInfiniteCompanyEmails(wsId: string, companyId: string) {
  return useInfiniteQuery({
    queryKey: [...queryKeys.crm.companyEmails(wsId, companyId), 'infinite'],
    queryFn: async ({ pageParam }) => {
      const page = unwrap(await crmEmailService.listByCompany(wsId, companyId, pageParam))
      if (!page) throw new Error('Failed to load company emails')
      return page
    },
    initialPageParam: 1,
    getNextPageParam: (lastPage) => {
      if (!lastPage) return undefined
      return lastPage.page * 50 < lastPage.total ? lastPage.page + 1 : undefined
    },
    enabled: !!wsId && !!companyId,
  })
}

export function useInfiniteDealEmails(wsId: string, dealId: string) {
  return useInfiniteQuery({
    queryKey: [...queryKeys.crm.dealEmails(wsId, dealId), 'infinite'],
    queryFn: async ({ pageParam }) => unwrap(await crmEmailService.listByDeal(wsId, dealId, pageParam)),
    initialPageParam: 1,
    getNextPageParam: (lastPage) => (lastPage.page * 50 < lastPage.total ? lastPage.page + 1 : undefined),
    enabled: !!wsId && !!dealId,
  })
}

export function useCreateEmailMessage(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMEmailMessageRequest) => unwrap(await crmEmailService.createMessage(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['crm', wsId] })
    },
  })
}

export function useContactCalendar(wsId: string, contactId: string) {
  return useQuery({
    queryKey: queryKeys.crm.contactCalendar(wsId, contactId),
    queryFn: async () => unwrap(await crmCalendarService.listByContact(wsId, contactId)),
    enabled: !!wsId && !!contactId,
  })
}

export function useDealCalendar(wsId: string, dealId: string) {
  return useQuery({
    queryKey: queryKeys.crm.dealCalendar(wsId, dealId),
    queryFn: async () => unwrap(await crmCalendarService.listByDeal(wsId, dealId)),
    enabled: !!wsId && !!dealId,
  })
}

export function useCreateCalendarEvent(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMCalendarEventRequest) => unwrap(await crmCalendarService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['crm', wsId] })
    },
  })
}

export function useUpdateCalendarEvent(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateCRMCalendarEventRequest & { id: string }) =>
      unwrap(await crmCalendarService.update(wsId, id, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['crm', wsId] })
    },
  })
}

// ── Phase 4: Intelligence ──

export function useEnrichments(wsId: string, filters?: { object_type?: string; object_id?: string }) {
  return useQuery({
    queryKey: [...queryKeys.crm.enrichments(wsId), filters],
    queryFn: async () => unwrap(await crmEnrichmentService.list(wsId, filters)),
    enabled: !!wsId,
  })
}

export function useCreateEnrichment(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMEnrichmentRequest) => unwrap(await crmEnrichmentService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.enrichments(wsId) })
    },
  })
}

export function useApplyEnrichmentSuggestion(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ enrichmentId, field }: { enrichmentId: string; field: string }) =>
      unwrap(await crmEnrichmentService.applySuggestion(wsId, enrichmentId, field)),
    onSuccess: (result) => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.enrichments(wsId) })
      qc.invalidateQueries({ queryKey: ['crm', wsId] })
      if (result.object_type === 'contact') {
        qc.invalidateQueries({
          queryKey: queryKeys.crm.contactActivities(wsId, result.object_id),
        })
      }
    },
  })
}

export function useCRMSignals(
  wsId: string,
  filters?: { contact_id?: string; deal_id?: string; company_id?: string; signal_type?: string; commercial_only?: boolean },
) {
  return useQuery({
    queryKey: [...queryKeys.crm.signals(wsId), filters],
    queryFn: async () => unwrap(await crmSignalService.list(wsId, filters)),
    enabled: !!wsId,
  })
}

export function useSignalWorkspaceFeed(wsId: string, filters?: CRMSignalFeedFilters) {
  return useQuery({
    queryKey: [...queryKeys.crm.signals(wsId), 'feed', filters],
    queryFn: async () => unwrap(await crmSignalService.feed(wsId, filters)),
    enabled: !!wsId,
  })
}

export function useSignalOperations(wsId: string) {
  return useQuery({
    queryKey: [...queryKeys.crm.signals(wsId), 'operations'],
    queryFn: async () => {
      const [precision, outcomes, rules, policy, routingSettings, rollout, shadowGate, shadowPreview] = await Promise.all([
        unwrap(await crmSignalService.precision(wsId)),
		unwrap(await crmSignalService.outcomes(wsId)),
        unwrap(await crmSignalService.rules(wsId)),
        unwrap(await crmSignalService.routingPolicy(wsId)),
		unwrap(await crmSignalService.routingSettings(wsId)),
		unwrap(await crmSignalService.rollout(wsId)),
		unwrap(await crmSignalService.shadowGate(wsId)),
		unwrap(await crmSignalService.shadowPreview(wsId)),
      ])
      return { precision: precision.data, outcomes: outcomes.data, rules: rules.data, policy, routingSettings, rollout, shadowGate, shadowPreview }
    },
    enabled: !!wsId,
  })
}

export function useUpdateSignalRoutingSettings(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (payload: { default_signal_owner_member_id?: string; clear_default_signal_owner?: boolean; minimum_lane_priority?: number }) => unwrap(await crmSignalService.updateRoutingSettings(wsId, payload)),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.crm.signals(wsId) }),
  })
}

export function useActivateSignalRollout(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async () => unwrap(await crmSignalService.activateRollout(wsId)),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.crm.signals(wsId) }),
  })
}

export function useActivateSignalRule(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ ruleKey, version }: { ruleKey: string; version: number }) => unwrap(await crmSignalService.activateRule(wsId, ruleKey, version)),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.crm.signals(wsId) }),
  })
}

export function useSaveSignalRoutingPolicy(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (payload: { minimum_priority: number; required_trust: string; route_to_owner: boolean; channels: string[] }) => {
      const policy = unwrap(await crmSignalService.createRoutingPolicy(wsId, payload))
      unwrap(await crmSignalService.activateRoutingPolicy(wsId, policy.version))
      return policy
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.crm.signals(wsId) }),
  })
}

export function useContactSignals(wsId: string, contactId: string) {
  return useQuery({
    queryKey: queryKeys.crm.contactSignals(wsId, contactId),
    queryFn: async () => unwrap(await crmSignalService.listByContact(wsId, contactId)),
    enabled: !!wsId && !!contactId,
  })
}

export function useDealSignals(wsId: string, dealId: string) {
  return useQuery({
    queryKey: queryKeys.crm.dealSignals(wsId, dealId),
    queryFn: async () => unwrap(await crmSignalService.listByDeal(wsId, dealId)),
    enabled: !!wsId && !!dealId,
  })
}

export function useCompanySignals(wsId: string, companyId: string) {
  return useQuery({
    queryKey: queryKeys.crm.companySignals(wsId, companyId),
    queryFn: async () => unwrap(await crmSignalService.listByCompany(wsId, companyId)),
    enabled: !!wsId && !!companyId,
  })
}

export function useCreateCRMSignal(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMSignalRequest) => unwrap(await crmSignalService.create(data)),
    onSuccess: (_result, data) => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.signals(wsId) })
      if (data.contact_id) qc.invalidateQueries({ queryKey: queryKeys.crm.contactSignals(wsId, data.contact_id) })
      if (data.deal_id) qc.invalidateQueries({ queryKey: queryKeys.crm.dealSignals(wsId, data.deal_id) })
      if (data.company_id) qc.invalidateQueries({ queryKey: queryKeys.crm.companySignals(wsId, data.company_id) })
    },
  })
}

export function useDismissCRMSignal(wsId: string, contactId?: string, dealId?: string, companyId?: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ signalId, reason }: { signalId: string; reason: import('@/lib/crmTypes').CRMSignalDismissalReason }) => unwrap(await crmSignalService.dismiss(wsId, signalId, reason)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.signals(wsId) })
      if (contactId) qc.invalidateQueries({ queryKey: queryKeys.crm.contactSignals(wsId, contactId) })
      if (dealId) qc.invalidateQueries({ queryKey: queryKeys.crm.dealSignals(wsId, dealId) })
      if (companyId) qc.invalidateQueries({ queryKey: queryKeys.crm.companySignals(wsId, companyId) })
    },
  })
}

export function useCRMSignalFeedback(wsId: string, contactId?: string, dealId?: string, companyId?: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ signalId, action }: { signalId: string; action: 'reviewed' | 'acted' }) =>
      unwrap(await (action === 'acted' ? crmSignalService.acted(wsId, signalId) : crmSignalService.review(wsId, signalId))),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.signals(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.crm.situations(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.crm.playbooks(wsId) })
      qc.invalidateQueries({ queryKey: ['crm', wsId, 'suggestions'] })
      if (contactId) qc.invalidateQueries({ queryKey: queryKeys.crm.contactSignals(wsId, contactId) })
      if (dealId) qc.invalidateQueries({ queryKey: queryKeys.crm.dealSignals(wsId, dealId) })
      if (companyId) qc.invalidateQueries({ queryKey: queryKeys.crm.companySignals(wsId, companyId) })
    },
  })
}

export function useContactSummary(wsId: string, contactId: string) {
  return useQuery({
    queryKey: queryKeys.crm.contactSummary(wsId, contactId),
    queryFn: async () => unwrap(await crmSummaryService.getForContact(wsId, contactId)),
    enabled: !!wsId && !!contactId,
    refetchInterval: (query) => {
      const summary = query.state.data as { status?: string } | null | undefined
      return summary?.status === 'pending_refresh' || summary?.status === 'stale' ? 15000 : false
    },
  })
}

export function useRefreshContactSummary(wsId: string, contactId: string) {
  const qc = useQueryClient()
  const queryKey = queryKeys.crm.contactSummary(wsId, contactId)
  return useMutation({
    mutationFn: async () => unwrap(await crmSummaryService.refreshContactIntelligence(wsId, contactId)),
    onSuccess: (result) => {
      qc.setQueryData(queryKey, result.summary)
      qc.invalidateQueries({ queryKey: queryKeys.crm.contactSignals(wsId, contactId) })
      qc.invalidateQueries({ queryKey: queryKeys.crm.signals(wsId) })
    },
    onSettled: () => qc.invalidateQueries({ queryKey }),
  })
}

export function useCompanySummary(wsId: string, companyId: string) {
  return useQuery({
    queryKey: queryKeys.crm.companySummary(wsId, companyId),
    queryFn: async () => unwrap(await crmSummaryService.getForCompany(wsId, companyId)),
    enabled: !!wsId && !!companyId,
    refetchInterval: (query) => {
      const summary = query.state.data as { status?: string } | null | undefined
      return summary?.status === 'pending_refresh' || summary?.status === 'stale' ? 15000 : false
    },
  })
}

export function useRefreshCompanySummary(wsId: string, companyId: string) {
  const qc = useQueryClient()
  const queryKey = queryKeys.crm.companySummary(wsId, companyId)
  return useMutation({
    mutationFn: async () => unwrap(await crmSummaryService.refreshCompanyIntelligence(wsId, companyId)),
    onSuccess: (result) => {
      qc.setQueryData(queryKey, result.summary)
      qc.invalidateQueries({ queryKey: queryKeys.crm.companySignals(wsId, companyId) })
      qc.invalidateQueries({ queryKey: queryKeys.crm.signals(wsId) })
    },
    onSettled: () => qc.invalidateQueries({ queryKey }),
  })
}

export function useDealSummary(wsId: string, dealId: string) {
  return useQuery({
    queryKey: queryKeys.crm.dealSummary(wsId, dealId),
    queryFn: async () => unwrap(await crmSummaryService.getForDeal(wsId, dealId)),
    enabled: !!wsId && !!dealId,
    refetchInterval: (query) => {
      const summary = query.state.data as { status?: string } | null | undefined
      return summary?.status === 'pending_refresh' || summary?.status === 'stale' ? 15000 : false
    },
  })
}

export function useRefreshDealSummary(wsId: string, dealId: string) {
  const qc = useQueryClient()
  const queryKey = queryKeys.crm.dealSummary(wsId, dealId)
  return useMutation({
    mutationFn: async () => unwrap(await crmSummaryService.refreshDealIntelligence(wsId, dealId)),
    onSuccess: (result) => {
      qc.setQueryData(queryKey, result.summary)
      qc.invalidateQueries({ queryKey: queryKeys.crm.dealSignals(wsId, dealId) })
      qc.invalidateQueries({ queryKey: queryKeys.crm.signals(wsId) })
    },
    onSettled: () => qc.invalidateQueries({ queryKey }),
  })
}

export function useHealthScores(wsId: string) {
  return useQuery({
    queryKey: queryKeys.crm.healthScores(wsId),
    queryFn: async () => unwrap(await crmHealthScoreService.list(wsId)),
    enabled: !!wsId,
  })
}

export function useDealHealthScore(wsId: string, dealId: string) {
  return useQuery({
    queryKey: queryKeys.crm.dealHealthScore(wsId, dealId),
    queryFn: async () => unwrap(await crmHealthScoreService.getForDeal(wsId, dealId)),
    enabled: !!wsId && !!dealId,
  })
}

export function useCreateHealthScore(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMDealHealthScoreRequest) => unwrap(await crmHealthScoreService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.healthScores(wsId) })
    },
  })
}

export function useSuggestions(wsId: string, filters?: { status?: string; suggestion_type?: string }) {
  return useQuery({
    queryKey: [...queryKeys.crm.suggestions(wsId), filters],
    queryFn: async () => unwrap(await crmSuggestionService.list(wsId, filters)),
    enabled: !!wsId,
  })
}

export function useCreateSuggestion(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMSuggestionRequest) => unwrap(await crmSuggestionService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.suggestions(wsId) })
    },
  })
}

export function useUpdateSuggestion(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateCRMSuggestionRequest & { id: string }) =>
      unwrap(await crmSuggestionService.update(wsId, id, data)),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.suggestions(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.crm.suggestion(wsId, id) })
    },
  })
}

// ── Phase D: Deal Automation ──

export function usePendingSuggestions(wsId: string, page = 1) {
  return useQuery({
    queryKey: [...queryKeys.crm.suggestions(wsId, 'pending'), page],
    queryFn: async () => unwrap(await crmSuggestionService.list(wsId, { status: 'pending', page, per_page: 20 })),
    enabled: !!wsId,
  })
}

export function useAcceptSuggestion(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, edits }: { id: string; edits?: Record<string, unknown> }) =>
      unwrap(await crmSuggestionService.accept(wsId, id, edits)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['crm'] })
    },
  })
}

export function useDismissSuggestion(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, reason }: { id: string; reason: import('@/lib/crmTypes').CRMSignalDismissalReason }) => unwrap(await crmSuggestionService.dismiss(wsId, id, reason)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['crm'] })
    },
  })
}

export function useAutonomySettings(wsId: string) {
  return useQuery({
    queryKey: queryKeys.crm.autonomySettings(wsId),
    queryFn: async () => unwrap(await crmAutonomyService.get(wsId)),
    enabled: !!wsId,
  })
}

export function useUpdateAutonomySettings(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (settings: import('@/lib/services/crmService').CRMAutonomySettings) =>
      unwrap(await crmAutonomyService.update(wsId, settings)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.autonomySettings(wsId) })
    },
  })
}

// ── Email Sync Settings ──

export function useEmailSyncSettings(wsId: string) {
  return useQuery({
    queryKey: queryKeys.crm.emailSyncSettings(wsId),
    queryFn: async () => unwrap(await crmEmailSyncSettingsService.get(wsId)),
    enabled: !!wsId,
  })
}

export function useUpdateEmailSyncSettings(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (settings: import('@/lib/crmTypes').UpdateCRMEmailSyncSettingsRequest) =>
      unwrap(await crmEmailSyncSettingsService.update(wsId, settings)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.emailSyncSettings(wsId) })
    },
  })
}

export function useWritingProfiles(wsId: string) {
  return useQuery({
    queryKey: queryKeys.crm.writingProfiles(wsId),
    queryFn: async () => unwrap(await crmWritingProfileService.list(wsId)),
    enabled: !!wsId,
  })
}

export function useCreateWritingProfile(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMWritingProfileRequest) => unwrap(await crmWritingProfileService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.writingProfiles(wsId) })
    },
  })
}

export function useUpdateWritingProfile(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateCRMWritingProfileRequest & { id: string }) =>
      unwrap(await crmWritingProfileService.update(wsId, id, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.writingProfiles(wsId) })
    },
  })
}

// ── Phase 6: Search ──

export function useCRMSearch(wsId: string, q: string) {
  return useQuery({
    queryKey: queryKeys.crm.search(wsId, q),
    queryFn: async () => unwrap(await crmSearchService.search(wsId, q)),
    enabled: !!wsId && !!q && q.length >= 2,
  })
}
