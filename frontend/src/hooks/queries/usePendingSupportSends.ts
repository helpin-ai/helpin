import { useEffect, useMemo, useRef } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { unwrap } from '@/lib/queryUtils';
import { queryKeys } from '@/lib/queryKeys';
import { supportMessageClientID } from '@/lib/supportMessagePages';
import { useAuthStore } from '@/stores/authStore';
import type { SupportMessage } from '@/lib/pmTypes';

// HTTP, durable recovery, and realtime may arrive in any order. Confirmed
// messages always win over both local and persisted pending copies.
export function mergePendingSupportMessages(messages: SupportMessage[], remote?: SupportMessage[]) {
  const confirmed = new Map([...remote ?? [], ...messages].filter(m => !m.pending_send).map(m => [m.id, m]));
  const confirmedClients = new Set([...confirmed.values()].map(supportMessageClientID).filter(Boolean));
  const pending = (remote ?? messages).filter(m => m.pending_send);
  const pendingClients = new Set(pending.map(supportMessageClientID));
  const optimistic = messages.filter(m => m.pending_send && !m.pending_send_id && !pendingClients.has(supportMessageClientID(m)));
  return [...confirmed.values(), ...pending, ...optimistic]
    .filter(m => !m.pending_send || !confirmedClients.has(supportMessageClientID(m)))
    .sort((a, b) => Date.parse(a.created_at) - Date.parse(b.created_at));
}

export function usePendingSupportSends(workspaceId: string, conversationId: string, messages: SupportMessage[]) {
  const user = useAuthStore(s => s.user);
  const client = useQueryClient();
  const previous = useRef('');
  const query = useQuery({
    queryKey: ['support', workspaceId, 'pending-sends', conversationId, user?.id],
    enabled: !!conversationId && !!workspaceId && !!user,
    queryFn: () => api.get<SupportMessage[]>(`/support/inbox/conversations/${conversationId}/translation/sends?workspace_id=${encodeURIComponent(workspaceId)}`).then(unwrap),
    refetchInterval: q => q.state.data?.some(m => m.pending_send && m.pending_send !== 'failed') ? 1500 : 30_000,
    staleTime: 1000,
    retry: false,
  });
  const signature = JSON.stringify(query.data?.map(m => [m.id, m.pending_send]));
  useEffect(() => {
    if (previous.current && signature !== previous.current) {
      void client.invalidateQueries({ queryKey: queryKeys.support.messages(workspaceId, conversationId) });
    }
    previous.current = signature;
  }, [signature, workspaceId, conversationId, client]);
  return useMemo(() => mergePendingSupportMessages(messages, query.data).map(m => m.pending_send ? {
    ...m,
    sender_display_name: user?.full_name || user?.email || 'You',
    sender_avatar_url: user?.avatar_url,
  } : m), [messages, query.data, user]);
}
