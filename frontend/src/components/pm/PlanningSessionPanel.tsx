import { useState } from 'react';
import { Bot, CheckCircle, Loader2, X } from 'lucide-react';
import { toast } from 'sonner';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  usePlanningSession,
  usePlanningMessages,
  useSendPlanningMessage,
  useFinalizePlanningSession,
  useAbandonPlanningSession,
  useFlowNodeMessages,
  useSendFlowNodeMessage,
  useSendFlowNodeAction,
  useCancelFlowRun,
} from '@/hooks/queries';
import { usePlanningStream } from '@/hooks/usePlanningStream';
import type { Epic } from '@/lib/pmTypes';

import { PlanningChat } from './PlanningChat';
import { PlanningSpecPreview } from './PlanningSpecPreview';

interface Props {
  epic: Epic;
  workspaceId: string;
  sessionId?: string;
  flowRunId?: string;
  nodeRunId?: string;
  onComplete?: () => void;
}

export function PlanningSessionPanel({
  epic,
  workspaceId,
  sessionId: sessionIdProp,
  flowRunId,
  nodeRunId,
  onComplete,
}: Props) {
  const sessionId = sessionIdProp ?? epic.active_planning_session_id;
  const [abandonConfirm, setAbandonConfirm] = useState(false);
  const flowBacked = Boolean(flowRunId && nodeRunId);

  const { data: session } = usePlanningSession(workspaceId, sessionId);
  const { data: legacyMessages } = usePlanningMessages(workspaceId, flowBacked ? undefined : sessionId);
  const { data: flowMessages } = useFlowNodeMessages(workspaceId, flowRunId, nodeRunId);
  const { isStreaming, turnPending, markTurnPending, activeToolCall, toolResults, error } = usePlanningStream(
    workspaceId,
    sessionId,
  );
  const legacySendMessage = useSendPlanningMessage(workspaceId, sessionId ?? '');
  const flowSendMessage = useSendFlowNodeMessage(workspaceId, flowRunId ?? '', nodeRunId ?? '');
  const legacyFinalizeMutation = useFinalizePlanningSession(workspaceId);
  const flowActionMutation = useSendFlowNodeAction(workspaceId, flowRunId ?? '', nodeRunId ?? '');
  const legacyAbandonMutation = useAbandonPlanningSession(workspaceId);
  const cancelFlowMutation = useCancelFlowRun(workspaceId);

  const isFinalizing = session?.status === 'finalizing';
  const agentBusy = isStreaming || turnPending || isFinalizing;
  const messages = flowBacked ? flowMessages : legacyMessages;
  const sendMessage = flowBacked ? flowSendMessage : legacySendMessage;
  const finalizePending = flowBacked ? flowActionMutation.isPending : legacyFinalizeMutation.isPending;
  const abandonPending = flowBacked ? cancelFlowMutation.isPending : legacyAbandonMutation.isPending;

  const handleSend = (content: string) => {
    markTurnPending();
    sendMessage.mutate(content, {
      onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to send message'),
    });
  };

  const handleFinalize = () => {
    if (flowBacked && flowRunId && nodeRunId) {
      markTurnPending();
      flowActionMutation.mutate(
        { actionType: 'finalize' },
        {
          onSuccess: () => {
            toast.success('Planning step finalized');
            onComplete?.();
          },
          onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to finalize'),
        },
      );
      return;
    }
    if (!sessionId) return;
    markTurnPending();
    legacyFinalizeMutation.mutate(sessionId, {
      onSuccess: () => {
        toast.success('Finalizing session — agent is writing the spec');
        onComplete?.();
      },
      onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to finalize'),
    });
  };

  const handleAbandon = () => {
    if (flowBacked && flowRunId) {
      cancelFlowMutation.mutate(flowRunId, {
        onSuccess: () => {
          toast.success('Flow cancelled');
          setAbandonConfirm(false);
          setTimeout(() => onComplete?.(), 0);
        },
        onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to cancel flow'),
      });
      return;
    }
    if (!sessionId) return;
    legacyAbandonMutation.mutate(sessionId, {
      onSuccess: () => {
        toast.success('Session abandoned');
        setAbandonConfirm(false);
        setTimeout(() => onComplete?.(), 0);
      },
      onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to abandon'),
    });
  };

  const tokenUsage = session?.token_usage;
  const totalTokens = tokenUsage ? tokenUsage.input + tokenUsage.output : 0;

  return (
    <div className="rounded-md border border-border/60 bg-background">
      {/* Header */}
      <div className="flex items-center justify-between border-b px-4 py-3">
        <div className="flex items-center gap-2">
          <Bot className="h-4 w-4 text-primary" />
          <span className="text-sm font-semibold">Planning Session</span>
          <span className="text-xs text-muted-foreground">· {epic.name}</span>
          {session?.status && (
            <Badge variant="secondary" className="text-[10px]">
              {session.status}
            </Badge>
          )}
        </div>
        <div className="flex items-center gap-2">
          {totalTokens > 0 && (
            <span className="text-[10px] text-muted-foreground">
              {totalTokens.toLocaleString()} tokens
            </span>
          )}
          {!abandonConfirm ? (
            <>
              <Button
                size="sm"
                variant="outline"
                onClick={() => setAbandonConfirm(true)}
                disabled={isFinalizing || abandonPending}
              >
                <X className="mr-1 h-3 w-3" />
                Abandon
              </Button>
              <Button
                size="sm"
                onClick={handleFinalize}
                disabled={isFinalizing || finalizePending || isStreaming}
              >
                {finalizePending || isFinalizing ? (
                  <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                ) : (
                  <CheckCircle className="mr-1 h-3 w-3" />
                )}
                {isFinalizing ? 'Finalizing...' : 'Finalize'}
              </Button>
            </>
          ) : (
            <div className="flex items-center gap-2">
              <span className="text-xs text-destructive">Abandon this session?</span>
              <Button
                size="sm"
                variant="destructive"
                onClick={handleAbandon}
                disabled={abandonPending}
              >
                {abandonPending ? <Loader2 className="h-3 w-3 animate-spin" /> : 'Yes, abandon'}
              </Button>
              <Button size="sm" variant="ghost" onClick={() => setAbandonConfirm(false)}>
                Cancel
              </Button>
            </div>
          )}
        </div>
      </div>

      {/* Error banner */}
      {error && (
        <div className="border-b border-destructive/30 bg-destructive/5 px-4 py-2 text-xs text-destructive">
          {error}
        </div>
      )}

      {/* Split view */}
      <div className="grid h-[520px] grid-cols-1 lg:grid-cols-2">
        <div className="h-full overflow-hidden border-r border-border/40">
          <PlanningChat
            messages={messages ?? []}
            isStreaming={isStreaming}
            agentBusy={agentBusy}
            activeToolCall={activeToolCall}
            toolResults={toolResults}
            onSend={handleSend}
            sending={sendMessage.isPending}
          />
        </div>
        <div className="h-full overflow-hidden">
          <PlanningSpecPreview session={session} />
        </div>
      </div>
    </div>
  );
}
