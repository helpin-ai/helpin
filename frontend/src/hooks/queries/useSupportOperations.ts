import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import { supportService } from '@/lib/services/supportService'
import { findCachedConversation, patchCachedConversation } from '@/lib/supportQueryCache'
import type { ConversationPriority, SupportConversationActions, SupportSLAPolicy, SupportSLAPolicyInput } from '@/lib/pmTypes'

function refreshConversation(queryClient: QueryClient, workspaceId: string, conversationId: string) {
  void queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) })
  void queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, conversationId) })
  void queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) })
  void queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) })
  void queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxViewCounts(workspaceId) })
}

export function useUpdateConversationPriority(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ conversationId, priority }: { conversationId: string; priority: ConversationPriority }) =>
      supportService.updateConversationPriority(workspaceId, conversationId, priority).then(unwrap),
    onMutate: async ({ conversationId, priority }) => {
      await queryClient.cancelQueries({ queryKey: queryKeys.support.conversation(workspaceId, conversationId) })
      const previous = findCachedConversation(queryClient, workspaceId, conversationId)?.priority
      patchCachedConversation(queryClient, workspaceId, conversationId, { priority })
      return { previous }
    },
    onError: (error: Error, { conversationId }, context) => {
      if (context?.previous) patchCachedConversation(queryClient, workspaceId, conversationId, { priority: context.previous })
      toast.error('Failed to change priority', { description: error.message })
    },
    // SLA targets follow priority on the server, so refetch either way.
    onSettled: (_data, _error, variables) => refreshConversation(queryClient, workspaceId, variables.conversationId),
  })
}

export function useSnoozeConversation(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ conversationId, until }: { conversationId: string; until: Date }) =>
      supportService.snoozeConversation(workspaceId, conversationId, until.toISOString()).then(unwrap),
    onSuccess: (_data, variables) => refreshConversation(queryClient, workspaceId, variables.conversationId),
    onError: (error: Error) => {
      toast.error('Failed to snooze', { description: error.message })
    },
  })
}

export function useUnsnoozeConversation(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ conversationId }: { conversationId: string }) =>
      supportService.unsnoozeConversation(workspaceId, conversationId).then(unwrap),
    onSuccess: (_data, variables) => refreshConversation(queryClient, workspaceId, variables.conversationId),
    onError: (error: Error) => {
      toast.error('Failed to wake the conversation', { description: error.message })
    },
  })
}

export function useSLAPolicies(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.support.slaPolicies(workspaceId),
    queryFn: () => supportService.listSLAPolicies(workspaceId).then(unwrap),
    enabled: Boolean(workspaceId) && enabled,
  })
}

function useSLAPolicyMutation<TVariables>(
  workspaceId: string,
  mutationFn: (variables: TVariables) => Promise<unknown>,
  failure: string,
) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.support.slaPolicies(workspaceId) })
      // Tiers count the policies that use them; companies list the policies that apply.
      void queryClient.invalidateQueries({ queryKey: queryKeys.support.slaTiers(workspaceId) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.support.companySLAAll(workspaceId) })
      // Policies re-evaluate open conversations in the background.
      void queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) })
    },
    onError: (error: Error) => {
      toast.error(failure, { description: error.message })
    },
  })
}

export function useCreateSLAPolicy(workspaceId: string) {
  return useSLAPolicyMutation(workspaceId, (input: SupportSLAPolicyInput) =>
    supportService.createSLAPolicy(workspaceId, input).then(unwrap), 'Failed to create the SLA policy')
}

export function useUpdateSLAPolicy(workspaceId: string) {
  return useSLAPolicyMutation(workspaceId, ({ id, input }: { id: string; input: SupportSLAPolicyInput }) =>
    supportService.updateSLAPolicy(workspaceId, id, input).then(unwrap), 'Failed to save the SLA policy')
}

export function useDeleteSLAPolicy(workspaceId: string) {
  return useSLAPolicyMutation(workspaceId, (id: string) =>
    supportService.deleteSLAPolicy(workspaceId, id).then((response) => {
      if (response.error) throw new Error(response.error)
    }), 'Failed to delete the SLA policy')
}

export function useReorderSLAPolicies(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (policyIds: string[]) => supportService.reorderSLAPolicies(workspaceId, policyIds).then(unwrap),
    onMutate: async (policyIds) => {
      const key = queryKeys.support.slaPolicies(workspaceId)
      await queryClient.cancelQueries({ queryKey: key })
      const previous = queryClient.getQueryData<SupportSLAPolicy[]>(key)
      if (previous) {
        const byId = new Map(previous.map((policy) => [policy.id, policy]))
        queryClient.setQueryData(key, policyIds.map((id, position) => ({ ...byId.get(id)!, position })))
      }
      return { previous }
    },
    onError: (error: Error, _ids, context) => {
      if (context?.previous) queryClient.setQueryData(queryKeys.support.slaPolicies(workspaceId), context.previous)
      toast.error('Failed to reorder SLA policies', { description: error.message })
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.support.slaPolicies(workspaceId) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.support.companySLAAll(workspaceId) })
    },
  })
}

/** Runs a macro's actions on a conversation. Callers report the outcome. */
export function useApplyMacro(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ conversationId, macroId }: { conversationId: string; macroId: string }) =>
      supportService.applyMacro(workspaceId, conversationId, macroId).then(unwrap),
    onSettled: (_data, _error, variables) => refreshConversation(queryClient, workspaceId, variables.conversationId),
  })
}

/** Applies one change to many conversations on the server. */
export function useBulkConversationActions(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ conversationIds, actions }: { conversationIds: string[]; actions: SupportConversationActions }) =>
      supportService.bulkConversationActions(workspaceId, conversationIds, actions).then(unwrap),
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: ['support', workspaceId] })
      void queryClient.invalidateQueries({ queryKey: queryKeys.support.workspaceUnread() })
    },
  })
}
