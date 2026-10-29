import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import { customFieldsService } from '@/lib/services/customFieldsService'
import type { CustomFieldDefinition, CustomFieldDefinitionInput, CustomFieldObjectType } from '@/lib/customFields'

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
