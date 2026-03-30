import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { queryKeys } from '@/lib/queryKeys';
import { supportService } from '@/lib/services/supportService';
import { supportAttachmentService } from '@/lib/services/supportAttachmentService';
import { agentService } from '@/lib/services/agentService';
import { unwrap } from '@/lib/queryUtils';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import type {
  AgentKnowledgeSource,
  SupportContentSource,
  SupportContentPage,
  CreateSupportContentSourceRequest,
  UpdateSupportContentSourceRequest,
  SupportInboxSettings,
  ConversationStatus,
  ConversationListResponse,
  SupportConversation,
  VisitorContextResponse,
  SupportAIRewriteDraftRequest,
  CreateSupportMailboxRequest,
  UpdateSupportMailboxRequest,
  CreateSupportEmailRouteRequest,
} from '@/lib/pmTypes';

// ── Installation settings ───────────────────────────────────────────

export function useChatSettings(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.support.installation(workspaceId),
    queryFn: async () => unwrap(await supportService.getInstallation(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 60_000,
  });
}

export function useUpdateChatSettings(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (settings: Partial<SupportInboxSettings>) =>
      supportService.updateInstallationSettings(workspaceId, settings).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.installation(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to update chat settings', { description: error.message });
    },
  });
}

export function useRegenerateWidgetKey(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => supportService.regenerateWidgetKey(workspaceId).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.installation(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to regenerate widget key', { description: error.message });
    },
  });
}

// ── Conversations ───────────────────────────────────────────────────

export function useConversations(workspaceId: string, filters?: { status?: string; priority?: string; filter?: string; mailbox_id?: string | null; ai_state?: string }) {
  return useQuery({
    queryKey: [...queryKeys.support.conversations(workspaceId), filters] as const,
    queryFn: async (): Promise<ConversationListResponse> => {
      const res = await supportService.listConversations(workspaceId, filters);
      if (res.error) throw new Error(res.error);
      const data = res.data;
      // Handle both new ConversationListResponse and legacy array formats
      if (data && 'data' in data && Array.isArray(data.data)) {
        return data as ConversationListResponse;
      }
      // Legacy fallback
      const arr = Array.isArray(data) ? data : [];
      return { data: arr, total: arr.length, page: 1, per_page: 50, total_pages: 1, meta: { unread: { total: 0, my_inbox: 0, unassigned: 0 } } } as ConversationListResponse;
    },
    enabled: !!workspaceId,
    staleTime: 15_000,
  });
}

export function useUnreadStats(workspaceId: string, mailboxId?: string | null) {
  return useQuery({
    queryKey: [...queryKeys.support.unreadStats(workspaceId), mailboxId ?? 'shared'] as const,
    queryFn: async () => unwrap(await supportService.getUnreadStats(workspaceId, mailboxId)),
    enabled: !!workspaceId,
    staleTime: 15_000,
  });
}

export function useInboxScopes(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.support.inboxScopes(workspaceId),
    queryFn: async () => unwrap(await supportService.listInboxScopes(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 15_000,
  });
}

export function useSupportMailboxes(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.support.mailboxes(workspaceId),
    queryFn: async () => unwrap(await supportService.listMailboxes(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 15_000,
  });
}

export function useMailboxMembers(workspaceId: string, mailboxId?: string | null) {
  return useQuery({
    queryKey: queryKeys.support.mailboxMembers(workspaceId, mailboxId ?? ''),
    queryFn: async () => {
      const data = unwrap(await supportService.listMailboxMembers(workspaceId, mailboxId!));
      return Array.isArray(data) ? data : [];
    },
    enabled: !!workspaceId && !!mailboxId,
    staleTime: 15_000,
  });
}

export function useSupportEmailRoutes(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.support.emailRoutes(workspaceId),
    queryFn: async () => unwrap(await supportService.listEmailRoutes(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 15_000,
  });
}

export function useSupportTeammatePresence(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.support.teammatePresence(workspaceId),
    queryFn: async () => unwrap(await supportService.listTeammatePresence(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 15_000,
    refetchInterval: 30_000,
    refetchIntervalInBackground: true,
  });
}

export function useUpdateMySupportTeammatePresence(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (manualStatus: 'online' | 'away' | 'offline' | null) =>
      supportService.updateMyTeammatePresence(workspaceId, manualStatus).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.teammatePresence(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to update support status', { description: error.message });
    },
  });
}

export function useCreateSupportEmailRoute(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateSupportEmailRouteRequest) =>
      supportService.createEmailRoute(workspaceId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.emailRoutes(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to enable email forwarding', { description: error.message });
    },
  });
}

export function useDisableSupportEmailRoute(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (routeId: string) => supportService.disableEmailRoute(workspaceId, routeId).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.emailRoutes(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to disable email forwarding', { description: error.message });
    },
  });
}

export function useConversation(workspaceId: string, conversationId: string | null) {
  return useQuery({
    queryKey: queryKeys.support.conversation(workspaceId, conversationId ?? ''),
    queryFn: async () => unwrap(await supportService.getConversation(workspaceId, conversationId!)),
    enabled: !!workspaceId && !!conversationId,
    staleTime: 30_000,
  });
}

export function useConversationMessages(workspaceId: string, conversationId: string | null) {
  return useQuery({
    queryKey: queryKeys.support.messages(workspaceId, conversationId ?? ''),
    queryFn: async () => unwrap(await supportService.listConversationMessages(workspaceId, conversationId!)),
    enabled: !!workspaceId && !!conversationId,
    staleTime: 5_000,
  });
}

export function useVisitorContext(workspaceId: string, conversationId: string | null) {
  return useQuery<VisitorContextResponse>({
    queryKey: queryKeys.support.visitorContext(workspaceId, conversationId ?? ''),
    queryFn: async () => unwrap(await supportService.getVisitorContext(workspaceId, conversationId!)),
    enabled: !!workspaceId && !!conversationId,
    staleTime: 60_000,
  });
}

// ── Mutations ───────────────────────────────────────────────────────

export function useSendMessage(workspaceId: string, conversationId: string | null) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: { content: string; is_internal?: boolean; attachment_ids?: string[] }) =>
      supportService.createConversationMessage(workspaceId, conversationId!, payload).then(unwrap),
    onSuccess: () => {
      if (conversationId) {
        queryClient.invalidateQueries({ queryKey: queryKeys.support.messages(workspaceId, conversationId) });
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to send message', { description: error.message });
    },
  });
}

export function useRewriteSupportDraft(workspaceId: string, conversationId: string | null) {
  return useMutation({
    mutationFn: (payload: SupportAIRewriteDraftRequest) =>
      supportService.rewriteConversationDraft(workspaceId, conversationId!, payload).then(unwrap),
    onError: (error: Error) => {
      toast.error('Failed to rewrite draft', { description: error.message });
    },
  });
}

export function useUploadSupportAttachment(workspaceId: string, conversationId: string | null) {
  return useMutation({
    mutationFn: async ({ file }: { file: File }) => {
      if (!conversationId) throw new Error('No conversation');

      // Step 1: Initiate — get presigned URL
      const initData = unwrap(await supportAttachmentService.initiateUpload(
        workspaceId, conversationId, {
          file_name: file.name,
          file_size: file.size,
          content_type: file.type || 'application/octet-stream',
        },
      ));

      // Step 2: Upload to S3 via presigned PUT URL
      const uploadResp = await fetch(initData.upload_url, {
        method: 'PUT',
        body: file,
        headers: {
          'Content-Type': file.type || 'application/octet-stream',
          'x-amz-acl': 'public-read',
        },
      });
      if (!uploadResp.ok) throw new Error('Upload to storage failed');

      // Step 3: Confirm upload
      await supportAttachmentService.confirmUpload(workspaceId, initData.attachment.id);

      return { id: initData.attachment.id, url: initData.public_url };
    },
    onError: (error: Error) => {
      toast.error('Failed to upload file', { description: error.message });
    },
  });
}

export function useUpdateConversationStatus(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ conversationId, status }: { conversationId: string; status: ConversationStatus }) =>
      supportService.updateConversationStatus(workspaceId, conversationId, status).then(unwrap),
    onSuccess: (_data, variables) => {
      const { conversationId, status } = variables;

      // Auto-advance: when resolving/spamming, select the next conversation in the list
      if (status === 'resolved' || status === 'spam' || status === 'closed') {
        const { selectedConversationId, selectConversation } = useSupportInboxStore.getState();
        if (selectedConversationId === conversationId) {
          // Find the next conversation from the cached list (before invalidation)
          const cached = queryClient.getQueriesData<ConversationListResponse>({
            queryKey: queryKeys.support.conversations(workspaceId),
          });
          const conversations: SupportConversation[] = cached.flatMap(([, data]) => data?.data ?? []);
          const currentIdx = conversations.findIndex((c) => c.id === conversationId);
          // Pick the next one below, or the one above, or clear selection
          const next = conversations[currentIdx + 1] ?? conversations[currentIdx - 1];
          selectConversation(next?.id ?? null);
        }
      }

      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, conversationId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to update conversation status', { description: error.message });
    },
  });
}

export function useAssignAgent(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ conversationId, agentId }: { conversationId: string; agentId: string }) =>
      supportService.assignConversationAgent(workspaceId, conversationId, { agent_id: agentId }).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, variables.conversationId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to assign agent', { description: error.message });
    },
  });
}

export function useCreateConversation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: { subject: string; priority?: string; customer_name?: string; customer_email?: string; mailbox_id?: string | null }) =>
      supportService.createConversation(workspaceId, payload as any).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to create conversation', { description: error.message });
    },
  });
}

export function useRunConversationAgent(workspaceId: string) {
  return useMutation({
    mutationFn: (conversationId: string) =>
      supportService.runAgent(workspaceId, conversationId).then(unwrap),
    onError: (error: Error) => {
      toast.error('Failed to run agent', { description: error.message });
    },
  });
}

export function useSupportAgents(workspaceId: string) {
  return useQuery({
    queryKey: [...queryKeys.agents.all(workspaceId), 'support'] as const,
    queryFn: async () => {
      const res = await agentService.list(workspaceId);
      if (res.error) throw new Error(res.error);
      return (res.data ?? []).filter((agent) =>
        agent.allowed_targets?.includes('support_conversation') ||
        agent.preset_key === 'support_agent',
      );
    },
    enabled: !!workspaceId,
    staleTime: 60_000,
  });
}

export function useMarkConversationUnread(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (conversationId: string) =>
      supportService.markConversationUnread(workspaceId, conversationId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to mark as unread', { description: error.message });
    },
  });
}

export function useMarkConversationRead(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (conversationId: string) =>
      supportService.markConversationRead(workspaceId, conversationId),
    onSuccess: (_data, conversationId) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, conversationId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to mark conversation as read', { description: error.message });
    },
  });
}

export function useUpdateConversationSubject(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ conversationId, subject }: { conversationId: string; subject: string }) =>
      supportService.updateConversationSubject(workspaceId, conversationId, subject).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, variables.conversationId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to update subject', { description: error.message });
    },
  });
}

export function useDeleteConversation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (conversationId: string) =>
      supportService.deleteConversation(workspaceId, conversationId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to delete conversation', { description: error.message });
    },
  });
}

export function useCreateMailbox(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateSupportMailboxRequest) =>
      supportService.createMailbox(workspaceId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.mailboxes(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to create team inbox', { description: error.message });
    },
  });
}

export function useUpdateMailbox(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ mailboxId, payload }: { mailboxId: string; payload: UpdateSupportMailboxRequest }) =>
      supportService.updateMailbox(workspaceId, mailboxId, payload).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.mailboxes(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.mailboxMembers(workspaceId, variables.mailboxId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to update team inbox', { description: error.message });
    },
  });
}

export function useArchiveMailbox(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (mailboxId: string) =>
      supportService.archiveMailbox(workspaceId, mailboxId).then(unwrap),
    onSuccess: (_data, mailboxId) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.mailboxes(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.mailboxMembers(workspaceId, mailboxId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to archive team inbox', { description: error.message });
    },
  });
}

export function useReorderMailboxes(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (mailboxIds: string[]) =>
      supportService.reorderMailboxes(workspaceId, mailboxIds),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.mailboxes(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to reorder team inboxes', { description: error.message });
    },
  });
}

export function useMoveConversation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ conversationId, mailboxId }: { conversationId: string; mailboxId: string | null }) =>
      supportService.moveConversation(workspaceId, conversationId, mailboxId).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, variables.conversationId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to move conversation', { description: error.message });
    },
  });
}

export function useAgentKnowledgeSources(workspaceId: string, agentId?: string) {
  return useQuery({
    queryKey: queryKeys.agents.knowledgeSources(workspaceId, agentId ?? ''),
    queryFn: async (): Promise<AgentKnowledgeSource[]> => unwrap(await agentService.listKnowledgeSources(workspaceId, agentId!)),
    enabled: !!workspaceId && !!agentId,
    staleTime: 60_000,
    refetchInterval: (query) => {
      const sources = query.state.data as AgentKnowledgeSource[] | undefined;
      return sources?.some((source) => source.sync_status === 'queued' || source.sync_status === 'running')
        ? 2_000
        : false;
    },
  });
}

export function useUpdateAgentKnowledgeSources(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ agentId, spaceIds }: { agentId: string; spaceIds: string[] }) =>
      agentService.updateKnowledgeSources(workspaceId, agentId, spaceIds).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.knowledgeSources(workspaceId, variables.agentId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to update knowledge sources', { description: error.message });
    },
    onSettled: (_data, _error, variables) => {
      if (!variables?.agentId) {
        return;
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.knowledgeSources(workspaceId, variables.agentId) });
    },
  });
}

export function useReindexAgentKnowledgeSource(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ agentId, spaceId }: { agentId: string; spaceId: string }) =>
      agentService.reindexKnowledgeSource(workspaceId, agentId, spaceId).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.knowledgeSources(workspaceId, variables.agentId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to reindex help center docs', { description: error.message });
    },
    onSettled: (_data, _error, variables) => {
      if (!variables?.agentId) {
        return;
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.knowledgeSources(workspaceId, variables.agentId) });
    },
  });
}

export function useSupportContentSources(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.agents.contentSources(workspaceId),
    queryFn: async (): Promise<SupportContentSource[]> => unwrap(await agentService.listContentSources(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 60_000,
    refetchInterval: (query) => {
      const sources = query.state.data as SupportContentSource[] | undefined;
      return sources?.some((source) => source.sync_status === 'queued' || source.sync_status === 'running')
        ? 2_000
        : false;
    },
  });
}

export function useSupportContentSourcePages(workspaceId: string, contentSourceId?: string) {
  return useQuery({
    queryKey: queryKeys.agents.contentSourcePages(workspaceId, contentSourceId ?? ''),
    queryFn: async (): Promise<SupportContentPage[]> => unwrap(await agentService.listContentSourcePages(workspaceId, contentSourceId!)),
    enabled: !!workspaceId && !!contentSourceId,
    staleTime: 30_000,
  });
}

export function useSupportContentSourcePage(workspaceId: string, contentSourceId: string, pageId?: string) {
  return useQuery({
    queryKey: queryKeys.agents.contentSourcePage(workspaceId, contentSourceId, pageId ?? ''),
    queryFn: async (): Promise<SupportContentPage> => unwrap(await agentService.getContentSourcePage(workspaceId, contentSourceId, pageId!)),
    enabled: !!workspaceId && !!contentSourceId && !!pageId,
    staleTime: 60_000,
  });
}

export function useCreateSupportContentSource(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateSupportContentSourceRequest) =>
      agentService.createContentSource(workspaceId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.contentSources(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to create content source', { description: error.message });
    },
  });
}

export function useUpdateSupportContentSource(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ contentSourceId, payload }: { contentSourceId: string; payload: UpdateSupportContentSourceRequest }) =>
      agentService.updateContentSource(workspaceId, contentSourceId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.contentSources(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to update content source', { description: error.message });
    },
  });
}

export function useDeleteSupportContentSource(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (contentSourceId: string) =>
      agentService.deleteContentSource(workspaceId, contentSourceId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.contentSources(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.all(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to delete content source', { description: error.message });
    },
  });
}

export function useReindexSupportContentSource(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (contentSourceId: string) =>
      agentService.reindexContentSource(workspaceId, contentSourceId).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.contentSources(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to reindex content source', { description: error.message });
    },
  });
}

export function useAgentContentSources(workspaceId: string, agentId?: string) {
  return useQuery({
    queryKey: queryKeys.agents.selectedContentSources(workspaceId, agentId ?? ''),
    queryFn: async (): Promise<string[]> => unwrap(await agentService.listAgentContentSources(workspaceId, agentId!)),
    enabled: !!workspaceId && !!agentId,
    staleTime: 60_000,
  });
}

export function useUpdateAgentContentSources(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ agentId, contentSourceIds }: { agentId: string; contentSourceIds: string[] }) =>
      agentService.updateAgentContentSources(workspaceId, agentId, contentSourceIds).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.selectedContentSources(workspaceId, variables.agentId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.contentSources(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to update content sources', { description: error.message });
    },
    onSettled: (_data, _error, variables) => {
      if (!variables?.agentId) {
        return;
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.selectedContentSources(workspaceId, variables.agentId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.contentSources(workspaceId) });
    },
  });
}
