import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { unwrap } from '@/lib/queryUtils';
import { useAuthStore } from '@/stores/authStore';
import type {
  SupportTranslation,
  SupportTranslationOptions,
  SupportTranslationPreference,
  SupportTranslationConversation,
} from '@/lib/pmTypes';

export const translationOptionsKey = (workspaceId: string, conversationId: string, userId?: string) =>
  ['support', workspaceId, 'translation', conversationId, userId] as const;
const base = (conversationId: string) => `/support/inbox/conversations/${conversationId}/translation`;
const params = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;
export function useSupportTranslationOptions(workspaceId: string, conversationId: string) {
  const userId = useAuthStore((s) => s.user?.id);
  return useQuery({
    queryKey: translationOptionsKey(workspaceId, conversationId, userId),
    enabled: Boolean(workspaceId && conversationId),
    queryFn: () =>
      api.get<SupportTranslationOptions>(base(conversationId) + params(workspaceId)).then(unwrap),
    staleTime: 30_000,
    retry: false,
  });
}
export function useSaveTranslationSettings(workspaceId: string, conversationId: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (
      input: { preference: SupportTranslationPreference } | { conversation: SupportTranslationConversation },
    ) =>
      'preference' in input
        ? api.put(base(conversationId) + '/preference' + params(workspaceId), input.preference).then(unwrap)
        : api.put(base(conversationId) + params(workspaceId), input.conversation).then(unwrap),
    onSuccess: () => client.invalidateQueries({ queryKey: ['support', workspaceId, 'translation'] }),
  });
}
export const requestSupportTranslation = (
  workspaceId: string,
  conversationId: string,
  input: { message_id: string; target_language: string },
) =>
  api.post<SupportTranslation>(base(conversationId) + '/messages' + params(workspaceId), input).then(unwrap);
