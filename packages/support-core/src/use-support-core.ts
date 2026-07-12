import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { supportQueryKeys } from './support-query-keys'
import {
  isSupportConversationListQueryKey,
  updateConversationListUnreadCount,
  updateConversationUnreadCount,
} from './support-query-cache'
import type {
  AssignableMember,
  ConversationListResponse,
  ConversationStatus,
  SupportAIRewriteDraftRequest,
  SupportAIRewriteDraftResponse,
  SupportConversation,
  SupportMessage,
} from './support-types'
import type { VisitorContextResponse } from './visitor-types'
import {
  supportService,
  type AssignConversationUserPayload,
  type ConversationFilters,
  type SendMessagePayload,
} from './support-service'

function unwrapOrThrow<T>(value: { data: T | null; error: string | null }): T {
  if (value.error || value.data === null) {
    throw new Error(value.error || 'Request failed')
  }
  return value.data
}

export function useConversations(
  workspaceId: string,
  filters?: ConversationFilters,
  keepPrevious?: boolean,
) {
  return useQuery({
    queryKey: [...supportQueryKeys.conversations(workspaceId), filters] as const,
    queryFn: async (): Promise<ConversationListResponse> => {
      const res = await supportService.listConversations(workspaceId, filters)
      if (res.error) throw new Error(res.error)
      const data = res.data
      if (data && 'data' in data && Array.isArray(data.data)) {
        return data as ConversationListResponse
      }
      const arr = Array.isArray(data) ? data : []
      return {
        data: arr,
        total: arr.length,
        page: 1,
        per_page: 50,
        total_pages: 1,
        meta: {
          unread: {
            total: 0, my_inbox: 0, unassigned: 0, ai_active: 0,
            inbox: 0, mine: 0, waiting: 0, inbox_total: 0, mine_total: 0, waiting_total: 0, ai_active_total: 0,
          },
        },
      }
    },
    enabled: !!workspaceId,
    staleTime: 15_000,
    placeholderData: keepPrevious ? (prev: ConversationListResponse | undefined) => prev : undefined,
  })
}

export function useUnreadStats(workspaceId: string, mailboxId?: string | null, enabled = true) {
  return useQuery({
    queryKey: [...supportQueryKeys.unreadStats(workspaceId), mailboxId ?? 'all'] as const,
    queryFn: async () => unwrapOrThrow(await supportService.getUnreadStats(workspaceId, mailboxId)),
    enabled: !!workspaceId && enabled,
    staleTime: 15_000,
  })
}

export function useInboxScopes(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: supportQueryKeys.inboxScopes(workspaceId),
    queryFn: async () => unwrapOrThrow(await supportService.listInboxScopes(workspaceId)),
    enabled: !!workspaceId && enabled,
    staleTime: 15_000,
  })
}

export function useSupportMailboxes(workspaceId: string) {
  return useQuery({
    queryKey: supportQueryKeys.mailboxes(workspaceId),
    queryFn: async () => unwrapOrThrow(await supportService.listMailboxes(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 15_000,
  })
}

export function useSupportInboxViewCounts(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: supportQueryKeys.inboxViewCounts(workspaceId),
    queryFn: async () => unwrapOrThrow(await supportService.listInboxViewCounts(workspaceId)),
    enabled: !!workspaceId && enabled,
    staleTime: 15_000,
  })
}

export function useSupportBuiltinInboxViews(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: supportQueryKeys.builtinInboxViews(workspaceId),
    queryFn: async () => unwrapOrThrow(await supportService.listBuiltinInboxViews(workspaceId)),
    enabled: !!workspaceId && enabled,
    staleTime: 30_000,
  })
}

export function useSupportInboxViews(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: supportQueryKeys.inboxViews(workspaceId),
    queryFn: async () => unwrapOrThrow(await supportService.listInboxViews(workspaceId)),
    enabled: !!workspaceId && enabled,
    staleTime: 30_000,
  })
}

export function useMailboxMembers(workspaceId: string, mailboxId?: string | null) {
  return useQuery({
    queryKey: supportQueryKeys.mailboxMembers(workspaceId, mailboxId ?? ''),
    queryFn: async () => {
      const data = unwrapOrThrow(await supportService.listMailboxMembers(workspaceId, mailboxId!))
      return Array.isArray(data) ? data : []
    },
    enabled: !!workspaceId && !!mailboxId,
    staleTime: 15_000,
  })
}

export function useSupportTeammatePresence(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: supportQueryKeys.teammatePresence(workspaceId),
    queryFn: async () => unwrapOrThrow(await supportService.listTeammatePresence(workspaceId)),
    enabled: !!workspaceId && enabled,
    staleTime: 15_000,
    refetchInterval: 30_000,
    refetchIntervalInBackground: true,
  })
}

export function useConversation(workspaceId: string, conversationId: string | null) {
  return useQuery({
    queryKey: supportQueryKeys.conversation(workspaceId, conversationId ?? ''),
    queryFn: async () => unwrapOrThrow(await supportService.getConversation(workspaceId, conversationId!)),
    enabled: !!workspaceId && !!conversationId,
    staleTime: 30_000,
  })
}

export function useConversationMessages(workspaceId: string, conversationId: string | null) {
  return useQuery({
    queryKey: supportQueryKeys.messages(workspaceId, conversationId ?? ''),
    queryFn: async () => unwrapOrThrow(await supportService.listConversationMessages(workspaceId, conversationId!)),
    enabled: !!workspaceId && !!conversationId,
    staleTime: 5_000,
  })
}

export function useVisitorContext(workspaceId: string, conversationId: string | null) {
  return useQuery<VisitorContextResponse>({
    queryKey: supportQueryKeys.visitorContext(workspaceId, conversationId ?? ''),
    queryFn: async () => unwrapOrThrow(await supportService.getVisitorContext(workspaceId, conversationId!)),
    enabled: !!workspaceId && !!conversationId,
    staleTime: 60_000,
  })
}

export function useMarkConversationUnread(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (conversationId: string) => supportService.markConversationUnread(workspaceId, conversationId),
    onSuccess: (_data, conversationId) => {
      queryClient.setQueriesData<ConversationListResponse>(
        {
          queryKey: supportQueryKeys.conversations(workspaceId),
          predicate: (query) => isSupportConversationListQueryKey(query.queryKey, workspaceId),
        },
        (current) => {
          const currentUnreadCount = Array.isArray(current?.data)
            ? current.data.find((conversation) => conversation.id === conversationId)?.unread_count ?? 0
            : 0
          return updateConversationListUnreadCount(current, conversationId, Math.max(currentUnreadCount, 1))
        },
      )
      queryClient.setQueryData<SupportConversation>(
        supportQueryKeys.conversation(workspaceId, conversationId),
        (current) => updateConversationUnreadCount(current, Math.max(current?.unread_count ?? 0, 1)),
      )
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.unreadStats(workspaceId) })
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.inboxScopes(workspaceId) })
    },
  })
}

interface SendMessageMutationContext {
  previousMessages: SupportMessage[] | undefined
  optimisticId: string
}

export function useSendMessage(workspaceId: string, conversationId: string) {
  const queryClient = useQueryClient()
  const messagesKey = supportQueryKeys.messages(workspaceId, conversationId)

  return useMutation<SupportMessage, Error, SendMessagePayload, SendMessageMutationContext>({
    mutationFn: async (payload) => unwrapOrThrow(await supportService.sendMessage(workspaceId, conversationId, payload)),
    onMutate: async (payload) => {
      await queryClient.cancelQueries({ queryKey: messagesKey })
      const previousMessages = queryClient.getQueryData<SupportMessage[]>(messagesKey)
      const optimisticId = `pending-${crypto.randomUUID()}`
      const optimisticMessage: SupportMessage = {
        id: optimisticId,
        workspace_id: workspaceId,
        conversation_id: conversationId,
        sender_type: 'user',
        content: payload.content,
        is_internal: payload.is_internal ?? false,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
        pending: true,
      }
      queryClient.setQueryData<SupportMessage[]>(messagesKey, (current) => [...(current ?? []), optimisticMessage])
      return { previousMessages, optimisticId }
    },
    onSuccess: (message, _payload, context) => {
      queryClient.setQueryData<SupportMessage[]>(messagesKey, (current) => {
        const list = current ?? []
        const withoutOptimistic = list.filter((m) => m.id !== context.optimisticId)
        return [...withoutOptimistic, message]
      })
    },
    onError: (_error, _payload, context) => {
      queryClient.setQueryData<SupportMessage[]>(messagesKey, context?.previousMessages)
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversation(workspaceId, conversationId) })
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversations(workspaceId) })
    },
  })
}

export function useUpdateConversationStatus(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ conversationId, status }: { conversationId: string; status: ConversationStatus }) =>
      supportService.updateConversationStatus(workspaceId, conversationId, status).then(unwrapOrThrow),
    onSettled: (_data, _error, variables) => {
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversation(workspaceId, variables.conversationId) })
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversations(workspaceId) })
    },
  })
}

export function useAssignConversationUser(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ conversationId, userId }: { conversationId: string; userId: string | null }) => {
      const payload: AssignConversationUserPayload = { user_id: userId }
      return supportService.assignConversationUser(workspaceId, conversationId, payload).then(unwrapOrThrow)
    },
    onSettled: (_data, _error, variables) => {
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversation(workspaceId, variables.conversationId) })
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversations(workspaceId) })
    },
  })
}

export function useConversationAssignees(workspaceId: string, conversationId: string | null) {
  return useQuery<AssignableMember[]>({
    queryKey: supportQueryKeys.assignees(workspaceId, conversationId ?? ''),
    queryFn: async () => unwrapOrThrow(await supportService.listConversationAssignees(workspaceId, conversationId!)),
    enabled: !!workspaceId && !!conversationId,
    staleTime: 15_000,
  })
}

export function useMarkConversationRead(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (conversationId: string) => supportService.markConversationRead(workspaceId, conversationId),
    onSuccess: (_data, conversationId) => {
      queryClient.setQueriesData<ConversationListResponse>(
        {
          queryKey: supportQueryKeys.conversations(workspaceId),
          predicate: (query) => isSupportConversationListQueryKey(query.queryKey, workspaceId),
        },
        (current) => updateConversationListUnreadCount(current, conversationId, 0),
      )
      queryClient.setQueryData<SupportConversation>(
        supportQueryKeys.conversation(workspaceId, conversationId),
        (current) => updateConversationUnreadCount(current, 0),
      )
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.unreadStats(workspaceId) })
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.inboxScopes(workspaceId) })
    },
  })
}

export function useSupportCannedResponses(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: supportQueryKeys.cannedResponses(workspaceId),
    queryFn: async () => unwrapOrThrow(await supportService.listCannedResponses(workspaceId)),
    enabled: enabled && !!workspaceId,
    staleTime: 5 * 60_000,
  })
}

export function useUpdateConversationSubject(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ conversationId, subject }: { conversationId: string; subject: string }) =>
      unwrapOrThrow(await supportService.updateConversationSubject(workspaceId, conversationId, subject)),
    onSuccess: (_data, { conversationId }) => {
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversation(workspaceId, conversationId) })
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversations(workspaceId) })
    },
  })
}

export function useMoveConversation(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ conversationId, mailboxId }: { conversationId: string; mailboxId: string | null }) =>
      unwrapOrThrow(await supportService.moveConversation(workspaceId, conversationId, mailboxId)),
    onSuccess: (_data, { conversationId }) => {
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversation(workspaceId, conversationId) })
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversations(workspaceId) })
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.inboxScopes(workspaceId) })
    },
  })
}

export function useDeleteConversation(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (conversationId: string) => {
      const res = await supportService.deleteConversation(workspaceId, conversationId)
      if (res.error) throw new Error(res.error)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversations(workspaceId) })
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.unreadStats(workspaceId) })
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.inboxScopes(workspaceId) })
    },
  })
}

export function useSupportInstallation(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: supportQueryKeys.installation(workspaceId),
    queryFn: async () => unwrapOrThrow(await supportService.getInstallation(workspaceId)),
    enabled: enabled && !!workspaceId,
    staleTime: 60_000,
  })
}

export function useRewriteSupportDraft(workspaceId: string, conversationId: string | null) {
  return useMutation<SupportAIRewriteDraftResponse, Error, SupportAIRewriteDraftRequest>({
    mutationFn: async (payload) => {
      if (!conversationId) throw new Error('No conversation selected')
      return unwrapOrThrow(await supportService.rewriteConversationDraft(workspaceId, conversationId, payload))
    },
  })
}
