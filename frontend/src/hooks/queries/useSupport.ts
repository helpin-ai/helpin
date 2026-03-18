import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { queryKeys } from '@/lib/queryKeys';
import { supportService } from '@/lib/services/supportService';
import { agentService } from '@/lib/services/agentService';
import { unwrap } from '@/lib/queryUtils';
import type { SupportInboxSettings, ConversationStatus, ConversationListResponse, VisitorContextResponse } from '@/lib/pmTypes';

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

export function useConversations(workspaceId: string, filters?: { status?: string; priority?: string }) {
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

export function useUnreadStats(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.support.unreadStats(workspaceId),
    queryFn: async () => unwrap(await supportService.getUnreadStats(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 15_000,
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
    mutationFn: (payload: { content: string; is_internal?: boolean }) =>
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

export function useUpdateConversationStatus(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ conversationId, status }: { conversationId: string; status: ConversationStatus }) =>
      supportService.updateConversationStatus(workspaceId, conversationId, status).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, variables.conversationId) });
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
    mutationFn: (payload: { subject: string; priority?: string; customer_name?: string; customer_email?: string }) =>
      supportService.createConversation(workspaceId, payload as any).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
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
      return (res.data ?? []).filter((a) => a.agent_kind === 'llm' && a.agent_class === 'support');
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
    },
    onError: (error: Error) => {
      toast.error('Failed to mark as unread', { description: error.message });
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
    },
    onError: (error: Error) => {
      toast.error('Failed to delete conversation', { description: error.message });
    },
  });
}

export function useAgentKnowledgeSources(workspaceId: string, agentId?: string) {
  return useQuery({
    queryKey: queryKeys.agents.knowledgeSources(workspaceId, agentId ?? ''),
    queryFn: async () => unwrap(await agentService.listKnowledgeSources(workspaceId, agentId!)),
    enabled: !!workspaceId && !!agentId,
    staleTime: 60_000,
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
  });
}
