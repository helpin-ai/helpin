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
  /**
   * Optional resolve override. The dock chat view supplies a chat-scoped
   * resolver (/dock/chats/{id}/interactions/...) so users without PM
   * permissions can respond in their own chat.
   */
  resolve?: (
    interactionId: string,
    payload: { response_payload: Record<string, unknown>; followup_message?: string },
  ) => Promise<{ error: string | null }>;
}

/**
 * Shared form rendered in the dock overlay or an execution strip when the agent has
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
  resolve,
}: PendingInteractionCardProps) {
  const [acting, setActing] = useState<string | null>(null);

  const handleResolve = async (
    interactionId: string,
    responsePayload: Record<string, unknown>,
    followupMessage?: string,
  ) => {
    setActing(interactionId);
    try {
      const payload = {
        response_payload: responsePayload,
        followup_message: followupMessage,
      };
      const res = resolve
        ? await resolve(interactionId, payload)
        : await agentService.resolveInteraction(workspaceId, runId, interactionId, payload);
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
