import { useEffect, useMemo, useRef } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { SecurityCheckIcon } from '@/lib/icons';

import { PendingInteractionCard } from '@/components/agents/dock/PendingInteractionCard';
import { Card } from '@/components/ui/card';
import { supportService } from '@/lib/services/supportService';
import type { CodingSessionInteraction } from '@/lib/pmTypes';
import { isForegroundNetworkAvailable, useRealtimeFallbackPolling } from '@/hooks/useRealtimeFallbackPolling';
import type { WSEvent } from '@/hooks/useWebSocket';

interface AIRunApprovalCardProps {
  workspaceId: string;
  conversationId: string;
  /** Discover approvals only while the conversation is AI-handled. */
  enabled: boolean;
}

const EMPTY_RESULT = { run_id: '', interactions: [] as CodingSessionInteraction[] };
const TERMINAL_STATUSES = new Set(['completed', 'failed', 'cancelled']);
type ApprovalEventDetail = Partial<WSEvent> & { status?: string; update_kind?: string };

/**
 * Teammate-facing approvals raised by the conversation's AI chat run — e.g.
 * support_plan_confirm child launches that are not read-only. Rendered above
 * the message thread so pending approvals are visible without opening the run
 * sheet.
 */
export function AIRunApprovalCard({ workspaceId, conversationId, enabled }: AIRunApprovalCardProps) {
  const queryClient = useQueryClient();
  const polling = useRealtimeFallbackPolling(enabled && !!workspaceId && !!conversationId);
  const queryKey = useMemo(() => ['support', workspaceId, 'conversations', conversationId, 'ai-run-interactions'], [workspaceId, conversationId]);
  const runState = useRef<{ workspaceId: string; conversationId: string; id: string; status: string } | null>(null);
  const { data, refetch } = useQuery({
    queryKey,
    queryFn: async () => {
      const res = await supportService.listAIRunInteractions(workspaceId, conversationId);
      if (res.status === 404) return EMPTY_RESULT;
      // Retain already loaded approvals when a recovery request fails.
      if (res.error || !res.data) throw new Error(res.error || 'Failed to load AI run approvals');
      return res.data;
    },
    ...polling,
    refetchInterval: (query) => {
      const latest = runState.current;
      const known = latest?.workspaceId === workspaceId && latest.conversationId === conversationId ? latest : null;
      // A creation event can precede the conversation's active-run link update.
      const runId = query.state.data?.run_id || known?.id;
      if (!runId) return query.state.status === 'error' ? polling.refetchInterval : false;
      if (known?.id === runId && TERMINAL_STATUSES.has(known.status)) return false;
      return polling.refetchInterval;
    },
    // Re-enabling after a hidden/offline period must discover runs even after a 404.
    staleTime: 0,
  });

  useEffect(() => {
    if (!enabled || !workspaceId || !conversationId) return;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let disposed = false;
    let refreshing = false;
    let pending = false;

    async function flush() {
      pending = true;
      if (refreshing || !isForegroundNetworkAvailable()) return;
      refreshing = true;
      try {
        // An event during an older request needs one fresh read after it settles.
        const query = queryClient.getQueryCache().find({ queryKey, exact: true });
        if (query?.state.fetchStatus === 'fetching') await query.promise?.catch(() => undefined);
        while (pending && !disposed && isForegroundNetworkAvailable()) {
          pending = false;
          await refetch({ cancelRefetch: false });
        }
      } finally {
        refreshing = false;
      }
    }

    function scheduleRefresh() {
      void queryClient.invalidateQueries({ queryKey, exact: true, refetchType: 'none' });
      if (!isForegroundNetworkAvailable() || timer !== undefined) return;
      timer = setTimeout(() => {
        timer = undefined;
        void flush();
      }, 100);
    }

    function onRunEvent(event: Event) {
      const detail = (event as CustomEvent<ApprovalEventDetail>).detail;
      if (!detail || (detail.workspace_id && detail.workspace_id !== workspaceId)) return;
      const current = queryClient.getQueryData<typeof EMPTY_RESULT>(queryKey);
      const matchesConversation = detail.parent_type === 'support_conversation' && detail.parent_id === conversationId;
      if (!matchesConversation && (!current?.run_id || detail.entity_id !== current.run_id)) return;
      const changeKind = detail.data?.change_kind;
      if (changeKind === 'message' || changeKind === 'artifact' || detail.update_kind === 'duplicate') return;
      if (detail.update_kind === 'progress' && changeKind !== 'interaction' && changeKind !== 'state') return;
      const status = detail.status ?? detail.data?.status;
      if (detail.entity_id && typeof status === 'string') {
        runState.current = { workspaceId, conversationId, id: detail.entity_id, status };
      }
      scheduleRefresh();
    }

    function onInteractionEvent(event: Event) {
      const detail = (event as CustomEvent<ApprovalEventDetail>).detail;
      if (!detail || (detail.workspace_id && detail.workspace_id !== workspaceId)) return;
      const type = detail.data?.type;
      if (typeof type !== 'string' || !type.startsWith('interaction.')) return;
      const current = queryClient.getQueryData<typeof EMPTY_RESULT>(queryKey);
      const known = runState.current;
      const eventRunId = known?.workspaceId === workspaceId && known.conversationId === conversationId ? known.id : undefined;
      const runIds = [current?.run_id, eventRunId].filter(Boolean);
      // Before discovery, an interaction can be the first sign of a new run.
      if (runIds.length > 0 && !runIds.includes(detail.parent_id)) return;
      scheduleRefresh();
    }

    window.addEventListener('agent_run-created', onRunEvent);
    window.addEventListener('agent_run-updated', onRunEvent);
    window.addEventListener('coding_session_event-created', onInteractionEvent);
    return () => {
      disposed = true;
      if (timer !== undefined) clearTimeout(timer);
      window.removeEventListener('agent_run-created', onRunEvent);
      window.removeEventListener('agent_run-updated', onRunEvent);
      window.removeEventListener('coding_session_event-created', onInteractionEvent);
    };
  }, [enabled, workspaceId, conversationId, queryClient, queryKey, refetch]);

  // Server rows carry `id`; the interaction components expect `interaction_id`.
  const pending = (data?.interactions ?? [])
    .filter((i) => i.status === 'pending')
    .map((i) => ({
      ...i,
      interaction_id: i.interaction_id ?? (i as { id?: string }).id ?? '',
    }));
  if (!enabled || !data?.run_id || pending.length === 0) return null;

  return (
    <Card className="mx-4 mt-2 space-y-2 border-amber-500/40 bg-amber-500/5 p-3">
      <div className="flex items-center gap-2">
        <SecurityCheckIcon className="h-4 w-4 text-amber-600" />
        <span className="text-sm font-medium">AI agent needs approval</span>
      </div>
      {pending.map((interaction) => (
        <PendingInteractionCard
          key={interaction.interaction_id}
          workspaceId={workspaceId}
          runId={data.run_id}
          interaction={interaction}
          onResolved={() => void refetch()}
          resolve={async (interactionId, payload) => {
            const res = await supportService.resolveAIRunInteraction(
              workspaceId,
              conversationId,
              interactionId,
              payload,
            );
            return { error: res.error };
          }}
        />
      ))}
    </Card>
  );
}
