import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import { supportService } from '@/lib/services/supportService'
import type { SupportSLATier } from '@/lib/pmTypes'

export function useSLATiers(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.support.slaTiers(workspaceId),
    queryFn: () => supportService.listSLATiers(workspaceId).then(unwrap),
    enabled: Boolean(workspaceId) && enabled,
    staleTime: 60_000,
  })
}

/** A company's tier and the policies that can apply to it. */
export function useCompanySLA(workspaceId: string, companyId: string | null | undefined, enabled = true) {
  return useQuery({
    queryKey: queryKeys.support.companySLA(workspaceId, companyId ?? ''),
    queryFn: () => supportService.companySLA(workspaceId, companyId!).then(unwrap),
    enabled: Boolean(workspaceId && companyId) && enabled,
    staleTime: 60_000,
  })
}

/** Tier, policy and company SLA views all change together. */
function useInvalidateSLA(workspaceId: string) {
  const queryClient = useQueryClient()
  return () => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.support.slaTiers(workspaceId) })
    void queryClient.invalidateQueries({ queryKey: queryKeys.support.slaPolicies(workspaceId) })
    void queryClient.invalidateQueries({ queryKey: queryKeys.support.companySLAAll(workspaceId) })
    // Open conversations are re-checked in the background.
    void queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) })
  }
}

function useSLATierMutation<TVariables, TResult>(workspaceId: string, mutationFn: (variables: TVariables) => Promise<TResult>, failure: string) {
  const invalidate = useInvalidateSLA(workspaceId)
  return useMutation({
    mutationFn,
    onSuccess: invalidate,
    onError: (error: Error) => {
      toast.error(failure, { description: error.message })
    },
  })
}

function expectNoError(response: { error: string | null }) {
  if (response.error) throw new Error(response.error)
}

export function useCreateSLATier(workspaceId: string) {
  return useSLATierMutation(workspaceId, (input: { name: string; description: string }) =>
    supportService.createSLATier(workspaceId, input).then(unwrap), 'Failed to create the tier')
}

export function useUpdateSLATier(workspaceId: string) {
  return useSLATierMutation(workspaceId, ({ id, input }: { id: string; input: { name: string; description: string } }) =>
    supportService.updateSLATier(workspaceId, id, input).then(unwrap), 'Failed to save the tier')
}

export function useDeleteSLATier(workspaceId: string) {
  return useSLATierMutation(workspaceId, (id: string) =>
    supportService.deleteSLATier(workspaceId, id).then(expectNoError), 'Failed to delete the tier')
}

export function useReorderSLATiers(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (tierIds: string[]) => supportService.reorderSLATiers(workspaceId, tierIds).then(unwrap),
    onMutate: async (tierIds) => {
      const key = queryKeys.support.slaTiers(workspaceId)
      await queryClient.cancelQueries({ queryKey: key })
      const previous = queryClient.getQueryData<SupportSLATier[]>(key)
      if (previous) {
        const byId = new Map(previous.map((tier) => [tier.id, tier]))
        queryClient.setQueryData(key, tierIds.map((id, position) => ({ ...byId.get(id)!, position })))
      }
      return { previous }
    },
    onError: (error: Error, _ids, context) => {
      if (context?.previous) queryClient.setQueryData(queryKeys.support.slaTiers(workspaceId), context.previous)
      toast.error('Failed to reorder tiers', { description: error.message })
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.support.slaTiers(workspaceId) })
    },
  })
}

/** Sets or clears the tier of one or more companies. */
export function useSetCompanySLATier(workspaceId: string) {
  return useSLATierMutation(workspaceId, ({ companyIds, tierId }: { companyIds: string[]; tierId: string | null }) =>
    supportService.setCompanySLATier(workspaceId, companyIds, tierId).then(unwrap), 'Failed to set the SLA tier')
}

export function useCustomizeCompanySLA(workspaceId: string) {
  return useSLATierMutation(workspaceId, (companyId: string) =>
    supportService.customizeCompanySLA(workspaceId, companyId).then(unwrap), 'Failed to create the custom SLA')
}

export function useRemoveCompanySLA(workspaceId: string) {
  return useSLATierMutation(workspaceId, (companyId: string) =>
    supportService.removeCompanySLA(workspaceId, companyId).then(expectNoError), 'Failed to remove the custom SLA')
}
