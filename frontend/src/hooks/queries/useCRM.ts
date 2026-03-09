import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  crmContactService,
  crmCompanyService,
  crmDealService,
  crmPipelineService,
  crmAssociationService,
  crmActivityService,
  crmPropertyService,
  crmListService,
  crmImportService,
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
  CreateCRMPipelineRequest,
  UpdateCRMPipelineRequest,
  CreateCRMAssociationRequest,
  CreateCRMActivityRequest,
  UpdateCRMActivityRequest,
  CreateCRMPropertyDefinitionRequest,
  UpdateCRMPropertyDefinitionRequest,
  CreateCRMPropertyGroupRequest,
  UpdateCRMPropertyGroupRequest,
  CreateCRMListRequest,
  UpdateCRMListRequest,
  CreateCRMImportRequest,
  ProcessCRMImportRequest,
  CRMObjectType,
} from '@/lib/crmTypes'

// ── Contacts ──

interface ContactFilters {
  lifecycle_stage?: string
  lead_status?: string
  owner_member_id?: string
  search?: string
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

export function useContact(wsId: string, id: string) {
  return useQuery({
    queryKey: queryKeys.crm.contact(wsId, id),
    queryFn: async () => unwrap(await crmContactService.get(wsId, id)),
    enabled: !!wsId && !!id,
  })
}

export function useContactActivities(wsId: string, contactId: string) {
  return useQuery({
    queryKey: queryKeys.crm.contactActivities(wsId, contactId),
    queryFn: async () => unwrap(await crmContactService.listActivities(wsId, contactId)),
    enabled: !!wsId && !!contactId,
  })
}

export function useContactAssociations(wsId: string, contactId: string) {
  return useQuery({
    queryKey: queryKeys.crm.contactAssociations(wsId, contactId),
    queryFn: async () => unwrap(await crmContactService.listAssociations(wsId, contactId)),
    enabled: !!wsId && !!contactId,
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

export function useCompanyAssociations(wsId: string, companyId: string) {
  return useQuery({
    queryKey: queryKeys.crm.companyAssociations(wsId, companyId),
    queryFn: async () => unwrap(await crmCompanyService.listAssociations(wsId, companyId)),
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
  pipeline_id?: string
  stage_id?: string
  owner_member_id?: string
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
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.pipelines(wsId) })
    },
  })
}

export function useUpdatePipeline(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateCRMPipelineRequest & { id: string }) =>
      unwrap(await crmPipelineService.update(wsId, id, data)),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.pipelines(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.crm.pipeline(wsId, id) })
    },
  })
}

export function useDeletePipeline(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await crmPipelineService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.pipelines(wsId) })
    },
  })
}

// ── Associations ──

export function useCreateAssociation(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMAssociationRequest) => unwrap(await crmAssociationService.create(data)),
    onSuccess: () => {
      // Invalidate all association queries since we don't know which objects were linked
      qc.invalidateQueries({ queryKey: ['crm', wsId] })
    },
  })
}

export function useDeleteAssociation(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await crmAssociationService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['crm', wsId] })
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

// ── Properties ──

export function useCRMProperties(wsId: string, objectType?: CRMObjectType) {
  return useQuery({
    queryKey: queryKeys.crm.properties(wsId, objectType),
    queryFn: async () => unwrap(await crmPropertyService.listDefinitions(wsId, objectType)),
    enabled: !!wsId,
  })
}

export function useCreateCRMProperty(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMPropertyDefinitionRequest) =>
      unwrap(await crmPropertyService.createDefinition(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.properties(wsId) })
    },
  })
}

export function useUpdateCRMProperty(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateCRMPropertyDefinitionRequest & { id: string }) =>
      unwrap(await crmPropertyService.updateDefinition(wsId, id, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.properties(wsId) })
    },
  })
}

export function useDeleteCRMProperty(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await crmPropertyService.deleteDefinition(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.properties(wsId) })
    },
  })
}

export function useCRMPropertyGroups(wsId: string, objectType?: CRMObjectType) {
  return useQuery({
    queryKey: queryKeys.crm.propertyGroups(wsId, objectType),
    queryFn: async () => unwrap(await crmPropertyService.listGroups(wsId, objectType)),
    enabled: !!wsId,
  })
}

export function useCreateCRMPropertyGroup(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMPropertyGroupRequest) =>
      unwrap(await crmPropertyService.createGroup(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.propertyGroups(wsId) })
    },
  })
}

export function useUpdateCRMPropertyGroup(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateCRMPropertyGroupRequest & { id: string }) =>
      unwrap(await crmPropertyService.updateGroup(wsId, id, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.propertyGroups(wsId) })
    },
  })
}

export function useDeleteCRMPropertyGroup(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await crmPropertyService.deleteGroup(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.propertyGroups(wsId) })
    },
  })
}

// ── Lists ──

interface ListFilters {
  object_type?: string
  list_type?: string
  search?: string
  page?: number
  per_page?: number
}

export function useCRMLists(wsId: string, filters?: ListFilters) {
  return useQuery({
    queryKey: [...queryKeys.crm.lists(wsId), filters],
    queryFn: async () => unwrap(await crmListService.list(wsId, filters)),
    enabled: !!wsId,
  })
}

export function useCRMList(wsId: string, id: string) {
  return useQuery({
    queryKey: queryKeys.crm.list(wsId, id),
    queryFn: async () => unwrap(await crmListService.get(wsId, id)),
    enabled: !!wsId && !!id,
  })
}

export function useCRMListMembers(wsId: string, listId: string) {
  return useQuery({
    queryKey: queryKeys.crm.listMembers(wsId, listId),
    queryFn: async () => unwrap(await crmListService.listMembers(wsId, listId)),
    enabled: !!wsId && !!listId,
  })
}

export function useCreateCRMList(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateCRMListRequest) => unwrap(await crmListService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.lists(wsId) })
    },
  })
}

export function useUpdateCRMList(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateCRMListRequest & { id: string }) =>
      unwrap(await crmListService.update(wsId, id, data)),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.lists(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.crm.list(wsId, id) })
    },
  })
}

export function useDeleteCRMList(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await crmListService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.lists(wsId) })
    },
  })
}

export function useAddCRMListMember(wsId: string, listId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (objectId: string) => unwrap(await crmListService.addMember(wsId, listId, objectId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.listMembers(wsId, listId) })
      qc.invalidateQueries({ queryKey: queryKeys.crm.list(wsId, listId) })
    },
  })
}

export function useRemoveCRMListMember(wsId: string, listId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (objectId: string) => unwrap(await crmListService.removeMember(wsId, listId, objectId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.crm.listMembers(wsId, listId) })
      qc.invalidateQueries({ queryKey: queryKeys.crm.list(wsId, listId) })
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
      // Also invalidate the objects that were imported
      qc.invalidateQueries({ queryKey: ['crm', wsId] })
    },
  })
}
