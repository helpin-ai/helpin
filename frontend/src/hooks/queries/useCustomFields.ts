import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import { customFieldsService } from '@/lib/services/customFieldsService'
import type { CustomFieldDefinition, CustomFieldDefinitionInput, CustomFieldObjectType, SupportRequestType, SupportRequestTypeInput } from '@/lib/customFields'
import type { SupportConversation } from '@/lib/pmTypes'

const NO_FIELDS: CustomFieldDefinition[] = []

/** Field definitions of an object type; archived ones and usage on request. */
export function useCustomFields(
  workspaceId: string,
  objectType: CustomFieldObjectType,
  options: { archived?: boolean; usage?: boolean; enabled?: boolean } = {},
) {
  const { archived = false, usage = false, enabled = true } = options
  const query = useQuery({
    queryKey: queryKeys.customFields.list(workspaceId, objectType, { archived, usage }),
    queryFn: () => customFieldsService.list(workspaceId, objectType, { archived, usage }).then(unwrap),
    enabled: Boolean(workspaceId) && enabled,
    staleTime: 60_000,
  })
  return { ...query, fields: query.data ?? NO_FIELDS }
}

function expectNoError(response: { error: string | null }) {
  if (response.error) throw new Error(response.error)
}

function useFieldMutation<TVariables, TResult>(
  workspaceId: string,
  objectType: CustomFieldObjectType,
  mutationFn: (variables: TVariables) => Promise<TResult>,
  failure: string,
) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.customFields.all(workspaceId, objectType) })
    },
    onError: (error: Error) => {
      toast.error(failure, { description: error.message })
    },
  })
}

export function useCreateCustomField(workspaceId: string, objectType: CustomFieldObjectType) {
  return useFieldMutation(workspaceId, objectType, (input: CustomFieldDefinitionInput) =>
    customFieldsService.create(workspaceId, objectType, input).then(unwrap), 'Failed to create the field')
}

export function useUpdateCustomField(workspaceId: string, objectType: CustomFieldObjectType) {
  return useFieldMutation(workspaceId, objectType, ({ id, input }: { id: string; input: CustomFieldDefinitionInput }) =>
    customFieldsService.update(workspaceId, objectType, id, input).then(unwrap), 'Failed to save the field')
}

export function useArchiveCustomField(workspaceId: string, objectType: CustomFieldObjectType) {
  return useFieldMutation(workspaceId, objectType, ({ id, archived }: { id: string; archived: boolean }) =>
    customFieldsService.setArchived(workspaceId, objectType, id, archived).then(expectNoError), 'Failed to update the field')
}

export function useReorderCustomFields(workspaceId: string, objectType: CustomFieldObjectType) {
  return useFieldMutation(workspaceId, objectType, (ids: string[]) =>
    customFieldsService.reorder(workspaceId, objectType, ids).then(unwrap), 'Failed to reorder fields')
}

/** Writes defined field values on a CRM record, one key at a time. */
export function useSetCRMFieldValues(workspaceId: string, objectType: 'crm_contact' | 'crm_company', recordId: string) {
  const queryClient = useQueryClient()
  // The collection prefix covers the record, the list and filter options.
  const recordsKey = objectType === 'crm_contact' ? queryKeys.crm.contacts(workspaceId) : queryKeys.crm.companies(workspaceId)
  return useMutation({
    mutationFn: (values: Record<string, unknown>) =>
      customFieldsService.setCRMValues(workspaceId, objectType, recordId, values).then(expectNoError),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: recordsKey })
      void queryClient.invalidateQueries({ queryKey: queryKeys.customFields.all(workspaceId, objectType) })
    },
    onError: (error: Error) => {
      toast.error('Failed to save the field', { description: error.message })
    },
  })
}

const NO_REQUEST_TYPES: SupportRequestType[] = []

/** The workspace's request types in order, disabled ones included. */
export function useRequestTypes(workspaceId: string, enabled = true) {
  const query = useQuery({
    queryKey: queryKeys.customFields.requestTypes(workspaceId),
    queryFn: () => customFieldsService.listRequestTypes(workspaceId).then(unwrap),
    enabled: Boolean(workspaceId) && enabled,
    staleTime: 60_000,
  })
  return { ...query, requestTypes: query.data ?? NO_REQUEST_TYPES }
}

function useRequestTypeMutation<TVariables, TResult>(workspaceId: string, mutationFn: (variables: TVariables) => Promise<TResult>, failure: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.customFields.requestTypes(workspaceId) })
    },
    onError: (error: Error) => {
      toast.error(failure, { description: error.message })
    },
  })
}

export function useCreateRequestType(workspaceId: string) {
  return useRequestTypeMutation(workspaceId, (input: SupportRequestTypeInput) =>
    customFieldsService.createRequestType(workspaceId, input).then(unwrap), 'Failed to create the request type')
}

export function useUpdateRequestType(workspaceId: string) {
  return useRequestTypeMutation(workspaceId, ({ id, input }: { id: string; input: SupportRequestTypeInput }) =>
    customFieldsService.updateRequestType(workspaceId, id, input).then(unwrap), 'Failed to save the request type')
}

export function useDeleteRequestType(workspaceId: string) {
  return useRequestTypeMutation(workspaceId, (id: string) =>
    customFieldsService.deleteRequestType(workspaceId, id).then(expectNoError), 'Failed to delete the request type')
}

export function useReorderRequestTypes(workspaceId: string) {
  return useRequestTypeMutation(workspaceId, (ids: string[]) =>
    customFieldsService.reorderRequestTypes(workspaceId, ids).then(unwrap), 'Failed to reorder request types')
}

/** Sets a conversation's request type or field values; the reply is the updated conversation. */
export function useUpdateConversationFields(workspaceId: string, conversationId: string) {
  const queryClient = useQueryClient()
  const onSuccess = (conversation: SupportConversation) => {
    queryClient.setQueryData(queryKeys.support.conversation(workspaceId, conversationId), (current: SupportConversation | undefined) =>
      current ? { ...current, request_type_id: conversation.request_type_id, custom_fields: conversation.custom_fields } : current)
    void queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) })
    void queryClient.invalidateQueries({ queryKey: queryKeys.customFields.requestTypes(workspaceId) })
  }
  const onError = (error: Error) => {
    toast.error('Failed to save', { description: error.message })
  }
  const setType = useMutation({
    mutationFn: (requestTypeId: string | null) =>
      customFieldsService.setConversationRequestType(workspaceId, conversationId, requestTypeId).then(unwrap),
    onSuccess,
    onError,
  })
  const setFields = useMutation({
    mutationFn: (values: Record<string, unknown>) =>
      customFieldsService.setConversationFields(workspaceId, conversationId, values).then(unwrap),
    onSuccess,
    onError,
  })
  return { setType, setFields }
}
