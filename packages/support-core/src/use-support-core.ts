import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { supportQueryKeys } from './support-query-keys'
import {
  appendMessageToNewestPage,
  replaceMessageInPages,
  seedSupportMessagePages,
  type ConversationListPages,
  type SupportMessagePages,
} from './support-pages'
import {
  findConversationInListCache,
  isSupportConversationListQueryKey,
  updateConversationListUnreadCount,
  updateConversationUnreadCount,
  type ConversationListCache,
} from './support-query-cache'
import type {
  AssignableMember,
  ConversationListResponse,
  ConversationStatus,
  CreateConversationWithMessageRequest,
  CreateConversationWithMessageResponse,
  SupportAIRewriteDraftRequest,
  SupportAIRewriteDraftResponse,
  SupportConversation,
  SupportMessage,
  SupportMessagePage,
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
const CONVERSATION_PAGE_SIZE = 50
const MESSAGE_PAGE_SIZE = 20

function normalizeConversationResponse(
  data: ConversationListResponse | SupportConversation[] | null,
): ConversationListResponse {
  if (data && 'data' in data && Array.isArray(data.data)) return data

  const conversations = Array.isArray(data) ? data : []
  return {
    data: conversations,
    total: conversations.length,
    page: 1,
    per_page: CONVERSATION_PAGE_SIZE,
    total_pages: 1,
    meta: {
      unread: {
        total: 0, my_inbox: 0, unassigned: 0, ai_active: 0,
        inbox: 0, mine: 0, waiting: 0, inbox_total: 0, mine_total: 0, waiting_total: 0, ai_active_total: 0,
      },
    },
  }
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
      return normalizeConversationResponse(res.data)
    },
    enabled: !!workspaceId,
    staleTime: 15_000,
    placeholderData: keepPrevious ? (prev: ConversationListResponse | undefined) => prev : undefined,
  })
}

export function useInfiniteConversations(
  workspaceId: string,
  filters?: ConversationFilters,
  keepPrevious?: boolean,
) {
  return useInfiniteQuery<
    ConversationListResponse,
    Error,
    ConversationListPages,
    ReturnType<typeof supportQueryKeys.conversationPages>,
    number
  >({
    queryKey: supportQueryKeys.conversationPages(workspaceId, filters),
    queryFn: async ({ pageParam }) => {
      const res = await supportService.listConversations(workspaceId, {
        ...filters,
        page: pageParam,
        per_page: CONVERSATION_PAGE_SIZE,
      })
      if (res.error) throw new Error(res.error)
      return normalizeConversationResponse(res.data)
    },
    initialPageParam: 1,
    getNextPageParam: (lastPage) =>
      lastPage.page < lastPage.total_pages ? lastPage.page + 1 : undefined,
    enabled: !!workspaceId,
    staleTime: 15_000,
    placeholderData: keepPrevious ? (previous) => previous : undefined,
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

export function useCreateConversationWithMessage(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation<CreateConversationWithMessageResponse, Error, CreateConversationWithMessageRequest>({
    mutationFn: async (payload) => {
      const result = unwrapOrThrow(await supportService.createConversationWithMessage(workspaceId, payload))
      if (!result.conversation?.id || !result.message?.id) {
        throw new Error('Conversation send returned an invalid response')
      }
      return result
    },
    onSuccess: ({ conversation, message }) => {
      queryClient.setQueryData(
        supportQueryKeys.conversation(workspaceId, conversation.id),
        conversation,
      )
      queryClient.setQueryData(
        supportQueryKeys.messages(workspaceId, conversation.id),
        seedSupportMessagePages([message]),
      )
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversations(workspaceId) })
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.inboxScopes(workspaceId) })
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.unreadStats(workspaceId) })
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.inboxViewCounts(workspaceId) })
    },
  })
}

export function useConversationMessages(workspaceId: string, conversationId: string | null) {
  return useInfiniteQuery<
    SupportMessagePage,
    Error,
    SupportMessagePages,
    ReturnType<typeof supportQueryKeys.messages>,
    string | undefined
  >({
    queryKey: supportQueryKeys.messages(workspaceId, conversationId ?? ''),
    queryFn: async ({ pageParam }) =>
      unwrapOrThrow(
        await supportService.listConversationMessagePage(
          workspaceId,
          conversationId!,
          MESSAGE_PAGE_SIZE,
          pageParam,
        ),
      ),
    initialPageParam: undefined,
    getNextPageParam: (lastPage) => lastPage.next_cursor ?? undefined,
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
      queryClient.setQueriesData<ConversationListCache>(
        {
          queryKey: supportQueryKeys.conversations(workspaceId),
          predicate: (query) => isSupportConversationListQueryKey(query.queryKey, workspaceId),
        },
        (current) => {
          const currentUnreadCount = findConversationInListCache(current, conversationId)?.unread_count ?? 0
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
  previousMessages: SupportMessagePages | undefined
  optimisticId: string
}

export function useSendMessage(workspaceId: string, conversationId: string) {
  const queryClient = useQueryClient()
  const messagesKey = supportQueryKeys.messages(workspaceId, conversationId)

  return useMutation<SupportMessage, Error, SendMessagePayload, SendMessageMutationContext>({
    mutationFn: async (payload) => unwrapOrThrow(await supportService.sendMessage(workspaceId, conversationId, payload)),
    onMutate: async (payload) => {
      await queryClient.cancelQueries({ queryKey: messagesKey })
      const previousMessages = queryClient.getQueryData<SupportMessagePages>(messagesKey)
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
      queryClient.setQueryData<SupportMessagePages>(
        messagesKey,
        (current) => appendMessageToNewestPage(current, optimisticMessage),
      )
      return { previousMessages, optimisticId }
    },
    onSuccess: (message, _payload, context) => {
      queryClient.setQueryData<SupportMessagePages>(
        messagesKey,
        (current) => replaceMessageInPages(current, context.optimisticId, message),
      )
    },
    onError: (_error, _payload, context) => {
      queryClient.setQueryData<SupportMessagePages>(messagesKey, context?.previousMessages)
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversation(workspaceId, conversationId) })
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversations(workspaceId) })
    },
  })
}

/** Uploads directly to object storage, then confirms the attachment with Helpin. */
export function useUploadSupportAttachment(workspaceId: string, conversationId: string) {
  return useMutation<{ id: string; url: string }, Error, File>({
    mutationFn: async (file) => {
      const contentType = file.type || 'application/octet-stream'
      const initiated = unwrapOrThrow(await supportService.initiateAttachmentUpload(
        workspaceId,
        conversationId,
        { file_name: file.name, file_size: file.size, content_type: contentType },
      ))
      const response = await fetch(initiated.upload_url, {
        method: 'PUT',
        body: file,
        headers: { 'Content-Type': contentType, 'x-amz-acl': 'public-read' },
      })
      if (!response.ok) throw new Error('Upload to storage failed')
      unwrapOrThrow(await supportService.confirmAttachmentUpload(workspaceId, initiated.attachment.id))
      return { id: initiated.attachment.id, url: initiated.public_url }
    },
  })
}

export function useDeleteSupportAttachment(workspaceId: string) {
  return useMutation<void, Error, string>({
    mutationFn: async (attachmentId) => {
      unwrapOrThrow(await supportService.deleteAttachment(workspaceId, attachmentId))
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
      queryClient.setQueriesData<ConversationListCache>(
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

export function useSupportTags(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: supportQueryKeys.tags(workspaceId),
    queryFn: async () => unwrapOrThrow(await supportService.listTags(workspaceId)),
    enabled: enabled && !!workspaceId,
    staleTime: 5 * 60_000,
  })
}

export function useAddConversationTag(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ conversationId, tagId }: { conversationId: string; tagId: string }) =>
      unwrapOrThrow(await supportService.addConversationTag(workspaceId, conversationId, tagId)),
    onSuccess: (_data, { conversationId }) => {
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversation(workspaceId, conversationId) })
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversations(workspaceId) })
    },
  })
}

export function useRemoveConversationTag(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ conversationId, tagId }: { conversationId: string; tagId: string }) =>
      unwrapOrThrow(await supportService.removeConversationTag(workspaceId, conversationId, tagId)),
    onSuccess: (_data, { conversationId }) => {
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversation(workspaceId, conversationId) })
      queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversations(workspaceId) })
    },
  })
}

export function useSendConversationTranscript(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ conversationId, email, updateCustomerEmail }: {
      conversationId: string; email: string; updateCustomerEmail?: boolean
    }) => unwrapOrThrow(await supportService.sendConversationTranscript(workspaceId, conversationId, {
      email, update_customer_email: updateCustomerEmail,
    })),
    onSuccess: (_data, { conversationId, updateCustomerEmail }) => {
      if (updateCustomerEmail) {
        queryClient.invalidateQueries({ queryKey: supportQueryKeys.conversation(workspaceId, conversationId) })
      }
    },
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
