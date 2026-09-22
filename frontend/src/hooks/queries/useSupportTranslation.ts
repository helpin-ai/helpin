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

export function useCachedSupportTranslations(workspaceId: string, conversationId: string, ids: string[], newestMessageAt = 0) {
  const language = useSupportTranslationOptions(workspaceId, conversationId).data?.preference.reading_language;
  return useQuery({
    queryKey: ['support', workspaceId, 'cached-translations', conversationId, language, ids],
    enabled: !!workspaceId && !!conversationId && !!language && ids.length > 0,
    queryFn: async () => {
      const result: SupportTranslation[] = [];
      for (let i = 0; i < ids.length; i += 100) {
        result.push(...await api.get<SupportTranslation[]>(base(conversationId) + '/messages' + params(workspaceId) + '&ids=' + encodeURIComponent(ids.slice(i, i + 100).join(','))).then(unwrap));
      }
      return result;
    },
    staleTime: 30_000,
    // Realtime is primary. Poll only during fresh work, never indefinitely
    // because a reader opened an old conversation.
    refetchInterval: query => query.state.data?.some(item => item.status === 'pending') || Date.now() - newestMessageAt < 120_000 ? 5000 : false,
    retry: false,
  });
}
export const setLiveTranslate = (workspaceId: string, conversationId: string, enabled: boolean, customer_language: string) =>
  api.put<SupportTranslationOptions>(base(conversationId) + params(workspaceId), { enabled, customer_language }).then(unwrap);
