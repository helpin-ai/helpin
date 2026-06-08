import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { supportQueryKeys } from './support-query-keys'
import {
  isSupportConversationListQueryKey,
  updateConversationListUnreadCount,
  updateConversationUnreadCount,
} from './support-query-cache'
import type {
  ConversationListResponse,
  SupportConversation,
} from './support-types'
import type { VisitorContextResponse } from './visitor-types'
import { supportService, type ConversationFilters } from './support-service'

function unwrapOrThrow<T>(value: { data: T | null; error: string | null }): T {
  if (value.error || value.data === null) {
    throw new Error(value.error || 'Request failed')
  }
  return value.data
}

export function useConversations(workspaceId: string, filters?: ConversationFilters) {
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
        meta: { unread: { total: 0, my_inbox: 0, unassigned: 0, ai_active: 0 } },
      }
    },
    enabled: !!workspaceId,
    staleTime: 15_000,
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
