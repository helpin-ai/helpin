import { dockWorkPlans, hasWorkPlanOrigin } from './dockWorkPlans';
import activityStyles from './DockActivityTimeline.module.css';
import { AIExecutionDetails } from "../AIExecutionDetails";
import { AIConnectionPicker } from '@/components/agents/AIConnectionPicker';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { Loading01Icon } from '@/lib/icons';
import { DockInput } from './DockInput';
import { DockTranscript } from './DockTranscript';
import { PendingInteractionCard } from './PendingInteractionCard';
import { DockInteractionLayer } from './DockInteractionLayer';
import { ScrollToLatestButton } from '@/components/agents/transcript';
import { CodingPlanPanel } from '@/components/pm/CodingSession/CodingPlanPanel';
import { dockChatService } from '@/lib/services/dockChatService';
import type { CodingSessionInteraction } from '@/lib/pmTypes';
import type { DockRunSummary } from '@/lib/dockTypes';
import { AgentLiveStatus } from './AgentLiveStatus';
import { resolveAgentLiveProgress } from './agentProgress';
import { hasAuthoritativeDockRuntimeTimeline } from './dockChatTimeline';
import { isDockTranscriptStreaming } from './dockChatState';
import { useDockNetworkActivity } from './useDockNetworkActivity';
import { useAgentRunStream, type AgentRunStreamFetchers } from './useAgentRunStream';

interface DockRunViewProps {
  active?: boolean;
  workspaceId: string;
  summary: DockRunSummary;
  draft: string;
  onDraftChange: (value: string) => void;
  onRunChanged: () => void;
  onRunContinued: (runId: string) => void;
}

export function DockRunView({
  active = true,
  workspaceId,
  summary,
  draft,
  onDraftChange,
  onRunChanged,
  onRunContinued,
}: DockRunViewProps) {
  const run = summary.run;
  const networkAvailable = useDockNetworkActivity();
  const [fallbackInteraction, setFallbackInteraction] = useState<CodingSessionInteraction | null>(null);
  const [sending, setSending] = useState(false);
  const [stopping, setStopping] = useState(false);
  const [pausing, setPausing] = useState(false);
  const [resuming, setResuming] = useState(false);
  const [sendError, setSendError] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const autoFollowRef = useRef(true);
  const [atBottom, setAtBottom] = useState(true);

  const fetchers = useMemo<AgentRunStreamFetchers>(() => ({
    requestKey: 'dock-run',
    getSnapshot: (ws, runId, signal) => dockChatService.getRunSnapshot(ws, runId, signal),
    listEvents: (ws, runId, after, signal) => dockChatService.listRunEvents(ws, runId, after, signal),
  }), []);
  const {
    session,
    currentPlan,
    streamState,
    pendingInteraction,
    refetch,
    clearPendingInteraction,
    loading,
  } = useAgentRunStream(workspaceId, run.id, active, 5_000, fetchers);
  const effectiveRun = session ?? run;
  const transcriptStreaming = isDockTranscriptStreaming(effectiveRun);
  const showRuntimeTimeline = transcriptStreaming || (
    effectiveRun.status !== 'cancelled'
    && streamState !== null
    && hasAuthoritativeDockRuntimeTimeline(streamState)
  );
  const completedRun = effectiveRun.status === 'completed';
  const liveProgress = useMemo(() => completedRun ? null : resolveAgentLiveProgress({
    run: effectiveRun,
    stream: streamState,
    currentPlan,
    sending: false,
  }), [completedRun, currentPlan, effectiveRun, streamState]);

  const refreshInteractions = useCallback(async () => {
    if (effectiveRun.status !== 'paused' || effectiveRun.pause_reason === 'manual') return;
    const result = await dockChatService.listRunInteractions(workspaceId, run.id);
    if (!result.data) return;
    const pending = result.data.interactions.filter((interaction) => interaction.status === 'pending');
    const latest = pending[pending.length - 1] as (CodingSessionInteraction & { id?: string }) | undefined;
    setFallbackInteraction(latest ? { ...latest, interaction_id: latest.interaction_id ?? latest.id ?? '' } : null);
  }, [effectiveRun.status, effectiveRun.pause_reason, run.id, workspaceId]);

  useEffect(() => {
    if (!active || !networkAvailable) return;
    if (effectiveRun.status === 'paused' && effectiveRun.pause_reason !== 'manual') {
      const timer = window.setTimeout(() => void refreshInteractions(), 0);
      return () => window.clearTimeout(timer);
    }
    const timer = window.setTimeout(() => setFallbackInteraction(null), 0);
    return () => window.clearTimeout(timer);
  }, [active, networkAvailable, effectiveRun.status, effectiveRun.pause_reason, refreshInteractions]);

  useEffect(() => {
    const node = scrollRef.current;
    if (!node) return;
    const update = () => {
      const follow = node.scrollHeight - node.scrollTop - node.clientHeight < 96;
      autoFollowRef.current = follow;
      setAtBottom(follow);
    };
    update();
    node.addEventListener('scroll', update, { passive: true });
    return () => node.removeEventListener('scroll', update);
  }, []);

  const scrollToLatest = useCallback(() => {
    const node = scrollRef.current;
    if (!node) return;
    autoFollowRef.current = true;
    setAtBottom(true);
    node.scrollTop = node.scrollHeight;
  }, []);

  useEffect(() => {
    const node = scrollRef.current;
    if (node && autoFollowRef.current) node.scrollTop = node.scrollHeight;
  }, [streamState, sendError]);

  const interaction = effectiveRun.pause_reason === 'manual' ? null : pendingInteraction ?? fallbackInteraction;
  const resolveInteraction = useCallback(async (
    interactionId: string,
    payload: { response_payload: Record<string, unknown>; followup_message?: string },
  ) => {
    const result = await dockChatService.resolveRunInteraction(workspaceId, run.id, interactionId, payload);
    if (!result.error) {
      clearPendingInteraction(interactionId);
      setFallbackInteraction(null);
      await Promise.all([refetch(), refreshInteractions()]);
      onRunChanged();
    }
    return { error: result.error };
  }, [clearPendingInteraction, onRunChanged, refetch, refreshInteractions, run.id, workspaceId]);

  const submit = async () => {
    const content = draft.trim();
    if (!content || sending) return;
    setSending(true);
    setSendError(null);
    try {
      if (effectiveRun.status === 'failed' || effectiveRun.status === 'cancelled') {
        const result = await dockChatService.continueRun(workspaceId, run.id, content);
        if (result.error || !result.data) throw new Error(result.error ?? 'Failed to continue agent run');
        onDraftChange('');
        onRunContinued(result.data.id);
      } else {
        const result = await dockChatService.sendRunMessage(workspaceId, run.id, content);
        if (result.error) throw new Error(result.error);
        onDraftChange('');
        await refetch();
        onRunChanged();
      }
    } catch (error) {
      setSendError(error instanceof Error ? error.message : 'Failed to send message');
    } finally {
      setSending(false);
    }
  };

  const retry = async () => {
    if (sending) return;
    setSending(true);
    setSendError(null);
    try {
      const result = await dockChatService.continueRun(workspaceId, run.id);
      if (result.error || !result.data) throw new Error(result.error ?? 'Failed to retry agent run');
      onRunContinued(result.data.id);
    } catch (error) {
      setSendError(error instanceof Error ? error.message : 'Failed to retry agent run');
    } finally {
      setSending(false);
    }
  };

  const stop = async () => {
    if (cancellationPending) return;
    setStopping(true);
    const result = await dockChatService.cancelRun(workspaceId, run.id);
    if (result.error) toast.error(result.error);
    else onRunChanged();
    setStopping(false);
  };

  const pause = async () => {
    if (pausePending) return;
    setPausing(true);
    try {
      const result = await dockChatService.pauseRun(workspaceId, run.id);
      if (result.error) toast.error(result.error);
      else { onRunChanged(); await refetch(); }
    } finally {
      setPausing(false);
    }
  };

  const resume = async () => {
    if (resumePending) return;
    setResuming(true);
    try {
      const result = await dockChatService.resumeRun(workspaceId, run.id);
      if (result.error) toast.error(result.error);
      else { onRunChanged(); await refetch(); }
    } finally {
      setResuming(false);
    }
  };

  const composerEnabled = effectiveRun.status === 'failed'
    || effectiveRun.status === 'cancelled'
    || (effectiveRun.status === 'paused' && (effectiveRun.pause_reason === 'human_input' || effectiveRun.pause_reason === 'awaiting_user_message') && !interaction);
  const canPause = effectiveRun.status === 'queued' || effectiveRun.status === 'running';
  const canResume = effectiveRun.status === 'paused' && effectiveRun.pause_reason === 'manual';
  const canStop = canPause || canResume;
  const pausePending = pausing || effectiveRun.execution_stage === 'pausing';
  const resumePending = resuming || effectiveRun.execution_stage === 'resuming';
  const cancellationPending = stopping || effectiveRun.execution_stage === 'cancelling';

  return (
    <DockInteractionLayer
      active={active}
      interactionId={interaction?.interaction_id}
      prompt={interaction && (
        <PendingInteractionCard
          workspaceId={workspaceId}
          runId={run.id}
          interaction={interaction}
          resolve={resolveInteraction}
          onResolved={onRunChanged}
        />
      )}
    >
      <div className="relative min-h-0 flex-1">
        <div ref={scrollRef} className={`${activityStyles.activityHost} absolute inset-0 overflow-y-auto px-5 py-3 sm:px-6`}>
          {loading && !streamState ? (
            <div className="grid min-h-28 place-items-center text-[#8a8781]"><Loading01Icon className="h-4 w-4 animate-spin" /></div>
          ) : null}
          <AIExecutionDetails input={run.input} />
          <DockTranscript
            stream={streamState}
            active={transcriptStreaming}
            runStatus={effectiveRun.status}
            pauseReason={effectiveRun.pause_reason}
            useRuntimeTimeline={showRuntimeTimeline}
            workspaceId={workspaceId}
            fallbackActor={session?.triggered_by_user}
            compactAssistantProgress
            completedRun={completedRun}
          />
          {!loading && !streamState ? (
            <p className="py-8 text-center text-[13px] text-[#8a8781]">No activity has been recorded for this run yet.</p>
          ) : null}

          {effectiveRun.error_message ? (
            <div className="mt-3 rounded-xl border border-[#f2c9c5] bg-[#fdf6f5] p-3 text-[12.5px] text-[#8e2525] dark:border-red-900/60 dark:bg-red-950/20 dark:text-red-200">
              <p className="font-semibold">Run failed</p>
              <p className="mt-1 break-words leading-5">{effectiveRun.error_message}</p>
              <button type="button" onClick={() => void retry()} disabled={sending} className="mt-2 rounded-[9px] border border-[#e4b8b3] bg-[#fffefa] px-3 py-1.5 font-semibold text-[#7f1d1d] hover:bg-[#fff5f3] disabled:opacity-50 dark:bg-[#292420]">
                Retry
              </button>
            </div>
          ) : null}
          {effectiveRun.status === 'paused' && effectiveRun.pause_reason === 'authentication' ? (
            <div className="mt-3 space-y-2"><p className="text-sm text-muted-foreground">Reconnect the required provider to continue.</p>
 {typeof run.input?.model_connection_id === 'string' && <AIConnectionPicker workspaceId={workspaceId} locked value={{ model_connection_id: run.input.model_connection_id, model_name: typeof run.input.model_name === 'string' ? run.input.model_name : undefined }} onChange={() => {}} />}
 </div>
          ) : null}
          {sendError ? (
            <div className="mt-3 flex items-center justify-between gap-3 rounded-lg border border-destructive/25 bg-destructive/5 px-3 py-2 text-xs text-destructive">
              <span className="min-w-0 break-words">{sendError}</span>
              <button type="button" onClick={() => setSendError(null)} className="shrink-0 font-semibold hover:underline">Dismiss</button>
            </div>
          ) : null}
          {liveProgress ? (
            <div className="mt-2 shrink-0 border-t border-border/40 px-1 pt-2" data-agent-live-status-region data-working={liveProgress.tone === 'working'}>
              <AgentLiveStatus progress={liveProgress} />
            </div>
          ) : null}
        </div>
        {!atBottom ? <ScrollToLatestButton onClick={scrollToLatest} /> : null}
      </div>
      {currentPlan && (!hasWorkPlanOrigin(currentPlan) || !streamState || !dockWorkPlans(streamState).some(plan => plan.origin?.event_id === currentPlan.origin?.event_id)) && <div className="max-h-48 shrink-0 overflow-y-auto px-5" data-current-work-plan><CodingPlanPanel plan={currentPlan} runStatus={effectiveRun.status} title="Current work plan" defaultOpen={false} /></div>}
      {(composerEnabled || canStop || effectiveRun.status === 'paused' || effectiveRun.status === 'running' || effectiveRun.status === 'queued') ? (
        <div className="border-t border-[#f1efea] dark:border-[#302f2b]">
          <DockInput
            mode="conversation"
            value={draft}
            onChange={onDraftChange}
            onSubmit={() => void submit()}
            pageContext={null}
            busy={sending}
            disabled={!composerEnabled}
            onStop={canStop ? () => void stop() : undefined}
            stopping={cancellationPending}
            onPause={canPause && !cancellationPending ? () => void pause() : undefined}
            pausing={pausePending}
            onResume={canResume && !cancellationPending ? () => void resume() : undefined}
            resuming={resumePending}
            placeholder={cancellationPending ? 'Stopping agent…' : pausePending ? 'Pausing agent…' : resumePending ? 'Resuming agent…' : canResume ? 'Agent paused — resume to continue' : composerEnabled ? `Answer ${summary.agent.name || 'agent'}…` : effectiveRun.status === 'queued' ? 'Agent is starting…' : 'Agent is working…'}
          />
        </div>
      ) : null}
    </DockInteractionLayer>
  );
}
