import { useState } from 'react';
import { toast } from 'sonner';
import { CodingInteractionCard } from '@/components/pm/CodingSession/CodingInteractionCard';
import { agentService } from '@/lib/services/agentService';
import type { CodingSessionInteraction } from '@/lib/pmTypes';

interface PendingInteractionCardProps {
  workspaceId: string;
  runId: string;
  interaction: CodingSessionInteraction;
  /** Called after a successful resolve so the caller can refetch run state. */
  onResolved?: () => void;
}

/**
 * Inline form rendered under an active execution strip when the agent has
 * paused for human input (request_user_input / approval / review checkpoint).
 *
 * Reuses CodingInteractionCard for the form chrome but submits to the new
 * run-scoped resolve endpoint, so it works for any agent run — not only
 * coding sessions.
 */
export function PendingInteractionCard({
  workspaceId,
  runId,
  interaction,
  onResolved,
}: PendingInteractionCardProps) {
  const [acting, setActing] = useState<string | null>(null);

  const handleResolve = async (
    interactionId: string,
    responsePayload: Record<string, unknown>,
    followupMessage?: string,
  ) => {
    setActing(interactionId);
    try {
      const res = await agentService.resolveInteraction(workspaceId, runId, interactionId, {
        response_payload: responsePayload,
        followup_message: followupMessage,
      });
      if (res.error) {
        toast.error(res.error);
        return;
      }
      onResolved?.();
    } finally {
      setActing(null);
    }
  };

  return (
    <CodingInteractionCard
      interaction={interaction}
      acting={acting}
      onResolve={handleResolve}
      compact
    />
  );
}
