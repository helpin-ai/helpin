import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { usePageContextState } from '@/components/command-bar/pageContext';
import { commandBarService } from '@/lib/services/commandBarService';
import { dockChatService } from '@/lib/services/dockChatService';
import { parseDockPlanConfirm } from '@/lib/dockTypes';
import type { DockChatDetail } from '@/lib/dockTypes';
import type { AgentRun, CodingSessionInteraction, CommandBarPlanSummary } from '@/lib/pmTypes';
import { DockInput } from './DockInput';
import { DockTranscript } from './DockTranscript';
import { DockPlanConfirmCard } from './DockPlanConfirmCard';
import { ExecutionStrip } from './ExecutionStrip';
import { PendingInteractionCard } from './PendingInteractionCard';
import { planSummaryToRunPlan } from './planSummary';
import { useAgentRunStream, type AgentRunStreamFetchers } from './useAgentRunStream';
import {
  isStructuredInteractionKind,
  resolveDockComposerState,
  transformDockStream,
} from './dockChatState';

interface ChatViewProps {
  workspaceId: string;
  chatId: string;
  textareaRef: React.RefObject<HTMLTextAreaElement | null>;
  initialDraft?: string;
  onDraftConsumed?: () => void;
}

const ACTIVE_RUN_STATUSES = new Set(['queued', 'running', 'paused']);

/**
 * One dock chat: the backing chat-mode run's transcript, the composer, the
 * confirm/interaction cards, and strips for child plans launched from this
 * chat. All run reads go through the chat-scoped /dock endpoints so users
 * without PM permissions can use their own dock.
 */
export function ChatView({ workspaceId, chatId, textareaRef, initialDraft, onDraftConsumed }: ChatViewProps) {
  const [detail, setDetail] = useState<DockChatDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(true);
  const [plans, setPlans] = useState<CommandBarPlanSummary[]>([]);
  const [value, setValue] = useState('');
  const [sending, setSending] = useState(false);
  const [pendingEcho, setPendingEcho] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);

  const { pageContext, scopeOptions, activeScopeKey, setActiveScopeKey } = usePageContextState();
  const [contextCleared, setContextCleared] = useState(false);
  const effectivePageContext = contextCleared ? null : pageContext;

  useEffect(() => {
    if (initialDraft) {
      setValue(initialDraft);
      onDraftConsumed?.();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialDraft]);

  const run = detail?.run ?? null;
  const runActive = !!run && ACTIVE_RUN_STATUSES.has(run.status);

  const fetchers = useMemo<AgentRunStreamFetchers>(
    () => ({
      getSnapshot: (ws) => dockChatService.getChatRun(ws, chatId),
      listEvents: (ws, _runId, after) => dockChatService.listChatRunEvents(ws, chatId, after),
    }),
    [chatId],
  );

  const { streamState, pendingInteraction, loading: streamLoading, refetch, clearPendingInteraction } =
    useAgentRunStream(workspaceId, run?.id, !!run, runActive ? 5_000 : 0, fetchers);

  const refreshDetail = useCallback(async () => {
    const res = await dockChatService.getChat(workspaceId, chatId);
    if (res.data) setDetail(res.data);
    return res.data ?? null;
  }, [chatId, workspaceId]);

  // Load chat on mount / chat switch.
  useEffect(() => {
    setDetail(null);
    setPlans([]);
    setDetailLoading(true);
    setPendingEcho(null);
    void refreshDetail().finally(() => setDetailLoading(false));
  }, [refreshDetail]);

  // Refresh the run summary when its WS event fires (stream refetch is
  // handled inside useAgentRunStream; this keeps status/pause_reason fresh).
  useEffect(() => {
    if (!run?.id) return;
    const handler = (event: Event) => {
      const detailPayload = (event as CustomEvent<{ entity_id?: string }>).detail;
      if (detailPayload?.entity_id !== run.id) return;
      void refreshDetail();
    };
    window.addEventListener('agent_run-updated', handler);
    return () => window.removeEventListener('agent_run-updated', handler);
  }, [refreshDetail, run?.id]);

  // Child plans launched from this chat.
  useEffect(() => {
    const planIds = detail?.plan_ids ?? [];
    if (planIds.length === 0) {
      setPlans([]);
      return;
    }
    let cancelled = false;
    void (async () => {
      const results = await Promise.all(planIds.map((id) => commandBarService.getPlan(workspaceId, id)));
      if (cancelled) return;
      setPlans(results.flatMap((res) => (res.data?.plan ? [res.data.plan] : [])));
    })();
    return () => {
      cancelled = true;
    };
  }, [detail?.plan_ids, workspaceId]);

  // Authoritative pending-interaction fallback: when the run is paused on a
  // human interaction but the event stream hasn't surfaced it (missed WS
  // event, projection lag), fetch the interaction rows directly so the
  // approval card always renders instead of leaving the composer open.
  const [fallbackInteraction, setFallbackInteraction] = useState<CodingSessionInteraction | null>(null);
  const pausedOnInteraction =
    run?.status === 'paused' && (run.pause_reason === 'human_approval' || run.pause_reason === 'human_input');
  useEffect(() => {
    if (!pausedOnInteraction) {
      setFallbackInteraction(null);
      return;
    }
    let cancelled = false;
    void (async () => {
      const res = await dockChatService.listChatRunInteractions(workspaceId, chatId);
      if (cancelled || !res.data) return;
      const rows = res.data.interactions ?? [];
      const pending = rows.filter((row) => row.status === 'pending');
      const latest = pending[pending.length - 1] as (CodingSessionInteraction & { id?: string }) | undefined;
      if (!latest) {
        setFallbackInteraction(null);
        return;
      }
      // Raw interaction rows carry `id`; the coding-session shape uses
      // `interaction_id` — normalize so resolve calls work either way.
      setFallbackInteraction({ ...latest, interaction_id: latest.interaction_id ?? latest.id ?? '' });
    })();
    return () => {
      cancelled = true;
    };
  }, [chatId, pausedOnInteraction, workspaceId]);

  const transformed = useMemo(() => (streamState ? transformDockStream(streamState) : null), [streamState]);

  // Drop the optimistic echo once the transcript contains it.
  useEffect(() => {
    if (!pendingEcho || !transformed) return;
    const matched = transformed.stream.transcript_messages.some(
      (message) => message.role === 'user' && message.content.trim() === pendingEcho.trim(),
    );
    if (matched) setPendingEcho(null);
  }, [pendingEcho, transformed]);

  // Keep the transcript pinned to the bottom as content streams in.
  useEffect(() => {
    const node = scrollRef.current;
    if (node) node.scrollTop = node.scrollHeight;
  }, [transformed, pendingEcho, pendingInteraction]);

  const effectiveInteraction = pendingInteraction ?? fallbackInteraction;
  const dockConfirm = effectiveInteraction ? parseDockPlanConfirm(effectiveInteraction.request_payload) : null;
  const structuredPending = !dockConfirm && isStructuredInteractionKind(effectiveInteraction?.interaction_kind);
  const composer = resolveDockComposerState(
    run ? { status: run.status, pause_reason: run.pause_reason } : null,
    !!dockConfirm || structuredPending,
    detailLoading || sending,
  );

  const submit = async () => {
    const content = value.trim();
    if (!content || sending) return;
    setSending(true);
    setPendingEcho(content);
    setValue('');
    try {
      const res = await dockChatService.sendMessage(workspaceId, chatId, {
        content,
        page_context: effectivePageContext ?? undefined,
      });
      if (res.error || !res.data) {
        toast.error(res.error ?? 'Failed to send message');
        setPendingEcho(null);
        setValue(content);
        return;
      }
      setDetail(res.data);
      void refetch();
    } finally {
      setSending(false);
    }
  };

  const resolveInteraction = useCallback(
    async (interactionId: string, payload: { response_payload: Record<string, unknown>; followup_message?: string }) => {
      const res = await dockChatService.resolveInteraction(workspaceId, chatId, interactionId, payload);
      if (!res.error) {
        clearPendingInteraction(interactionId);
        setFallbackInteraction(null);
        void refreshDetail();
        void refetch();
      }
      return { error: res.error };
    },
    [chatId, clearPendingInteraction, refetch, refreshDetail, workspaceId],
  );

  const runsById = useMemo(() => {
    const map: Record<string, AgentRun> = {};
    for (const plan of plans) {
      for (const childRun of plan.runs ?? []) map[childRun.id] = childRun;
    }
    return map;
  }, [plans]);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div ref={scrollRef} className="max-h-[60vh] min-h-24 flex-1 space-y-3 overflow-y-auto px-4 py-3">
        {detailLoading && !detail && (
          <p className="py-6 text-center text-sm text-muted-foreground">Loading chat…</p>
        )}
        {!detailLoading && !run && !pendingEcho && (
          <p className="py-6 text-center text-sm text-muted-foreground">
            Ask a question about your workspace, or describe work for an agent to do.
          </p>
        )}
        {transformed && <DockTranscript stream={transformed.stream} active={runActive} />}
        {pendingEcho && (
          <div className="flex justify-end">
            <div className="max-w-[85%] rounded-2xl rounded-br-sm bg-primary px-3 py-2 text-sm text-primary-foreground opacity-80">
              {pendingEcho}
            </div>
          </div>
        )}
        {(sending || (runActive && run?.status !== 'paused' && !streamLoading && !transformed)) && (
          <p className="text-xs text-muted-foreground">Thinking…</p>
        )}
        {plans.length > 0 && (
          <div className="space-y-2 rounded-lg border border-indigo-200/60 bg-indigo-50/50 p-2 dark:border-indigo-500/20 dark:bg-indigo-500/[0.07]">
            <div className="px-1 text-[11px] font-medium uppercase tracking-wide text-indigo-600/80 dark:text-indigo-300/80">
              Child agent runs
            </div>
            {plans.map((plan) => (
              <ExecutionStrip
                key={plan.id}
                kind="plan"
                workspaceId={workspaceId}
                plan={planSummaryToRunPlan(plan)}
                runsById={runsById}
              />
            ))}
          </div>
        )}
        {effectiveInteraction && dockConfirm && (
          <DockPlanConfirmCard
            payload={dockConfirm}
            onDecision={(decision, note) =>
              resolveInteraction(effectiveInteraction.interaction_id, {
                response_payload: { decision },
                followup_message: note,
              })
            }
          />
        )}
        {effectiveInteraction && !dockConfirm && run && (
          <PendingInteractionCard
            workspaceId={workspaceId}
            runId={run.id}
            interaction={effectiveInteraction}
            resolve={resolveInteraction}
            onResolved={() => {
              clearPendingInteraction(effectiveInteraction.interaction_id);
              void refreshDetail();
              void refetch();
            }}
          />
        )}
      </div>
      {composer.visible && (
        <div className="border-t border-border/60 p-2">
          <DockInput
            mode="conversation"
            value={value}
            onChange={setValue}
            onSubmit={() => void submit()}
            pageContext={effectivePageContext}
            contextOptions={scopeOptions}
            activeContextKey={activeScopeKey}
            onContextKeyChange={(key) => {
              setContextCleared(false);
              setActiveScopeKey(key);
            }}
            onClearContext={() => setContextCleared(true)}
            busy={sending}
            disabled={!composer.enabled}
            textareaRef={textareaRef}
          />
        </div>
      )}
    </div>
  );
}
