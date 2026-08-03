import { useQuery } from '@tanstack/react-query';
import { SecurityCheckIcon } from '@/lib/icons';

import { PendingInteractionCard } from '@/components/agents/dock/PendingInteractionCard';
import { Card } from '@/components/ui/card';
import { supportService } from '@/lib/services/supportService';
import type { CodingSessionInteraction } from '@/lib/pmTypes';

interface AIRunApprovalCardProps {
  workspaceId: string;
  conversationId: string;
  /** Only poll while the conversation is AI-handled. */
  enabled: boolean;
}

const EMPTY_RESULT = { run_id: '', interactions: [] as CodingSessionInteraction[] };

/**
 * Teammate-facing approvals raised by the conversation's AI chat run — e.g.
 * support_plan_confirm child launches that are not read-only. Rendered above
 * the message thread so pending approvals are visible without opening the run
 * sheet.
 */
export function AIRunApprovalCard({ workspaceId, conversationId, enabled }: AIRunApprovalCardProps) {
  const { data, refetch } = useQuery({
    queryKey: ['support', workspaceId, 'conversations', conversationId, 'ai-run-interactions'],
    queryFn: async () => {
      const res = await supportService.listAIRunInteractions(workspaceId, conversationId);
      // 404 (no active AI run) and transient errors render as "no approvals".
      if (res.error || !res.data) return EMPTY_RESULT;
      return res.data;
    },
    enabled,
    refetchInterval: 15_000,
    staleTime: 10_000,
  });

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
