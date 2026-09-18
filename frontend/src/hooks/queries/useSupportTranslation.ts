import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { unwrap } from '@/lib/queryUtils';
import { useAuthStore } from '@/stores/authStore';
import type {
  SupportTranslation,
  SupportTranslationOptions,
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
    refetchInterval: 30_000,
    retry: false,
  });
}
export const requestSupportTranslation = (
  workspaceId: string,
  conversationId: string,
  input: { message_id: string; target_language: string },
) =>
  api.post<SupportTranslation>(base(conversationId) + '/messages' + params(workspaceId), input).then(unwrap);
