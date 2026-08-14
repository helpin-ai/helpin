import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { Loading01Icon } from '@/lib/icons';
import { DockInput } from './DockInput';
import { DockTranscript } from './DockTranscript';
import { PendingInteractionCard } from './PendingInteractionCard';
import { ApprovalAttentionBanner } from './ApprovalAttentionBanner';
import { StreamingStatusText } from '@/components/agents/StreamingStatusText';
import { deriveLiveStatusLabel, ScrollToLatestButton } from '@/components/agents/transcript';
import { dockChatService } from '@/lib/services/dockChatService';
import type { CodingSessionInteraction } from '@/lib/pmTypes';
import type { DockRunSummary } from '@/lib/dockTypes';
import { useAgentRunStream, type AgentRunStreamFetchers } from './useAgentRunStream';

interface DockRunViewProps {
  workspaceId: string;
  summary: DockRunSummary;
  draft: string;
  onDraftChange: (value: string) => void;
  onRunChanged: () => void;
  onRunContinued: (runId: string) => void;
}

const ACTIVE_STATUSES = new Set(['queued', 'running', 'paused']);

export function DockRunView({
  workspaceId,
  summary,
  draft,
  onDraftChange,
  onRunChanged,
  onRunContinued,
}: DockRunViewProps) {
  const run = summary.run;
  const active = ACTIVE_STATUSES.has(run.status);
  const [fallbackInteraction, setFallbackInteraction] = useState<CodingSessionInteraction | null>(null);
  const [sending, setSending] = useState(false);
  const [stopping, setStopping] = useState(false);
  const [sendError, setSendError] = useState<string | null>(null);
  const [authBusy, setAuthBusy] = useState(false);
  const [authState, setAuthState] = useState<{
    verification_url?: string;
    auth_url?: string;
    user_code?: string;
    error?: string;
  } | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const autoFollowRef = useRef(true);
  const [atBottom, setAtBottom] = useState(true);

  const fetchers = useMemo<AgentRunStreamFetchers>(() => ({
    getSnapshot: (ws, runId) => dockChatService.getRunSnapshot(ws, runId),
    listEvents: (ws, runId, after) => dockChatService.listRunEvents(ws, runId, after),
  }), []);
  const { streamState, pendingInteraction, refetch, clearPendingInteraction, loading } =
    useAgentRunStream(workspaceId, run.id, true, active ? 5_000 : 0, fetchers);

  const refreshInteractions = useCallback(async () => {
    if (run.status !== 'paused') return;
    const result = await dockChatService.listRunInteractions(workspaceId, run.id);
    if (!result.data) return;
    const pending = result.data.interactions.filter((interaction) => interaction.status === 'pending');
    const latest = pending[pending.length - 1] as (CodingSessionInteraction & { id?: string }) | undefined;
    setFallbackInteraction(latest ? { ...latest, interaction_id: latest.interaction_id ?? latest.id ?? '' } : null);
  }, [run.id, run.status, workspaceId]);

  useEffect(() => {
    if (run.status === 'paused') {
      const timer = window.setTimeout(() => void refreshInteractions(), 0);
      return () => window.clearTimeout(timer);
    }
    const timer = window.setTimeout(() => setFallbackInteraction(null), 0);
    return () => window.clearTimeout(timer);
  }, [refreshInteractions, run.status]);

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
  }, [streamState, pendingInteraction, fallbackInteraction, sendError]);

  const interaction = pendingInteraction ?? fallbackInteraction;
  const needsApproval = (
    (run.status === 'paused' && run.pause_reason === 'human_approval')
    || interaction?.interaction_kind.includes('approval') === true
  );
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
      if (run.status === 'failed' || run.status === 'cancelled') {
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

  const startAuth = async () => {
    if (authBusy) return;
    setAuthBusy(true);
    const result = await dockChatService.startRunAuth(workspaceId, run.id);
    if (result.error) toast.error(result.error);
    else setAuthState(result.data);
    setAuthBusy(false);
  };

  const cancelAuth = async () => {
    if (authBusy) return;
    setAuthBusy(true);
    const result = await dockChatService.cancelRunAuth(workspaceId, run.id);
    if (result.error) toast.error(result.error);
    else setAuthState(null);
    setAuthBusy(false);
  };

  const composerEnabled = run.status === 'failed'
    || run.status === 'cancelled'
    || (run.status === 'paused' && (run.pause_reason === 'human_input' || run.pause_reason === 'awaiting_user_message') && !interaction);
  const canStop = run.status === 'queued' || run.status === 'running';
  const cancellationPending = stopping || run.execution_stage === 'cancelling';
  const liveLabel = run.status === 'running' ? deriveLiveStatusLabel(streamState, run.status) : null;

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="relative min-h-0 flex-1">
        <div ref={scrollRef} className="absolute inset-0 overflow-y-auto px-3.5 py-3">
          {loading && !streamState ? (
            <div className="grid min-h-28 place-items-center text-[#8a8781]"><Loading01Icon className="h-4 w-4 animate-spin" /></div>
          ) : null}
          <DockTranscript stream={streamState} active={active} />
          {!loading && !streamState ? (
            <p className="py-8 text-center text-[13px] text-[#8a8781]">No activity has been recorded for this run yet.</p>
          ) : null}
          {liveLabel ? <StreamingStatusText className="mt-2 text-[12.5px]">{liveLabel}</StreamingStatusText> : null}
          {run.error_message ? (
            <div className="mt-3 rounded-xl border border-[#f2c9c5] bg-[#fdf6f5] p-3 text-[12.5px] text-[#8e2525] dark:border-red-900/60 dark:bg-red-950/20 dark:text-red-200">
              <p className="font-semibold">Run failed</p>
              <p className="mt-1 break-words leading-5">{run.error_message}</p>
              <button type="button" onClick={() => void retry()} disabled={sending} className="mt-2 rounded-[9px] border border-[#e4b8b3] bg-[#fffefa] px-3 py-1.5 font-semibold text-[#7f1d1d] hover:bg-[#fff5f3] disabled:opacity-50 dark:bg-[#292420]">
                Retry
              </button>
            </div>
          ) : null}
          {run.status === 'paused' && run.pause_reason === 'authentication' ? (
            <div className="mt-3 rounded-xl border border-[#f0c98a] bg-[#fffaf1] p-3 text-[13px] text-[#1c1b19] dark:border-amber-900/70 dark:bg-amber-950/20 dark:text-amber-100">
              <p className="text-[10.5px] font-bold uppercase tracking-[.1em] text-[#b45309]">Sign-in required</p>
              <p className="mt-1.5 leading-5">Connect the agent runtime to continue this run.</p>
              {authState?.user_code ? <code className="mt-2 block select-all rounded-md bg-white px-2 py-1.5 font-mono text-sm dark:bg-[#242320]">{authState.user_code}</code> : null}
              {authState?.verification_url || authState?.auth_url ? (
                <div className="mt-2 flex flex-wrap items-center gap-3">
                  <a href={authState.verification_url || authState.auth_url} target="_blank" rel="noreferrer" className="font-semibold text-[#9a4c05] underline-offset-2 hover:underline">Open sign-in page</a>
                  <button type="button" onClick={() => void cancelAuth()} disabled={authBusy} className="text-[#8a8781] underline-offset-2 hover:underline disabled:opacity-50">Cancel sign-in</button>
                </div>
              ) : (
                <button type="button" onClick={() => void startAuth()} disabled={authBusy} className="mt-2 rounded-[9px] border border-[#d9b36e] bg-white px-3 py-1.5 font-semibold text-[#8a4608] hover:bg-[#fffdf8] disabled:opacity-50 dark:bg-[#292420]">
                  {authBusy ? 'Starting…' : 'Start sign-in'}
                </button>
              )}
            </div>
          ) : null}
          {interaction ? (
            <div className="mt-3">
              <PendingInteractionCard
                workspaceId={workspaceId}
                runId={run.id}
                interaction={interaction}
                resolve={resolveInteraction}
                onResolved={onRunChanged}
              />
            </div>
          ) : null}
          {sendError ? (
            <div className="mt-3 flex items-center justify-between gap-3 rounded-lg border border-destructive/25 bg-destructive/5 px-3 py-2 text-xs text-destructive">
              <span className="min-w-0 break-words">{sendError}</span>
              <button type="button" onClick={() => setSendError(null)} className="shrink-0 font-semibold hover:underline">Dismiss</button>
            </div>
          ) : null}
        </div>
        {!atBottom ? <ScrollToLatestButton onClick={scrollToLatest} /> : null}
      </div>
      {needsApproval && !atBottom ? (
        <ApprovalAttentionBanner onReview={scrollToLatest} />
      ) : null}
      {(composerEnabled || canStop || run.status === 'running' || run.status === 'queued') ? (
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
            placeholder={cancellationPending ? 'Stopping agent…' : composerEnabled ? `Answer ${summary.agent.name || 'agent'}…` : run.status === 'queued' ? 'Agent is starting…' : 'Agent is working…'}
          />
        </div>
      ) : null}
    </div>
  );
}
