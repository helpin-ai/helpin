import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '@/lib/queryKeys';
import { supportService } from '@/lib/services/supportService';
import { agentService } from '@/lib/services/agentService';
import { unwrap } from '@/lib/queryUtils';
import type { SupportInboxSettings, ConversationStatus } from '@/lib/pmTypes';

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
  });
}

export function useRegenerateWidgetKey(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => supportService.regenerateWidgetKey(workspaceId).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.installation(workspaceId) });
    },
  });
}

// ── Conversations ───────────────────────────────────────────────────

export function useConversations(workspaceId: string, filters?: { status?: string; priority?: string }) {
  return useQuery({
    queryKey: [...queryKeys.support.conversations(workspaceId), filters] as const,
    queryFn: async () => {
      const res = await supportService.listConversations(workspaceId, filters);
      if (res.error) throw new Error(res.error);
      const data = res.data;
      return Array.isArray(data) ? data : (data as any)?.data ?? [];
    },
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
    staleTime: 10_000,
    refetchInterval: 15_000,
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
  });
}

export function useAssignAgent(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ conversationId, agentId }: { conversationId: string; agentId: string }) =>
      supportService.assignConversationAgent(workspaceId, conversationId, { agent_id: agentId }),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, variables.conversationId) });
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
  });
}

export function useRunConversationAgent(workspaceId: string) {
  return useMutation({
    mutationFn: (conversationId: string) =>
      supportService.runAgent(workspaceId, conversationId),
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
