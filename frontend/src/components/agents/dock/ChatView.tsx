import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { useAuthStore } from '@/stores/authStore';
import { usePageContextState } from '@/components/command-bar/pageContext';
import { commandBarService } from '@/lib/services/commandBarService';
import { dockChatService } from '@/lib/services/dockChatService';
import { parseDockPlanConfirm } from '@/lib/dockTypes';
import type { DockChatDetail, DockEntityReference } from '@/lib/dockTypes';
import type { AgentRun, AgentRunMessage, CodingSessionInteraction, CommandBarPageContext, CommandBarPlanSummary } from '@/lib/pmTypes';
import { DockInput } from './DockInput';
import { DockTranscript } from './DockTranscript';
import { DockUserMessage } from './DockUserMessage';
import { DockPlanConfirmCard } from './DockPlanConfirmCard';
import { ExecutionStrip } from './ExecutionStrip';
import { PendingInteractionCard } from './PendingInteractionCard';
import { ApprovalAttentionBanner } from './ApprovalAttentionBanner';
import { CodingPlanPanel } from '@/components/pm/CodingSession/CodingPlanPanel';
import { StreamingStatusText } from '@/components/agents/StreamingStatusText';
import type { AskAgentAvatarState } from '@/components/agents/AskAgentAvatar';
import { deriveAskAgentAvatarState } from '@/components/agents/askAgentPresence';
import { deriveLiveStatusLabel, ScrollToLatestButton } from '@/components/agents/transcript';
import { planSummaryToRunPlan } from './planSummary';
import type { AgentRunStreamState } from './useAgentRunStream';
import { mergeMessagePages, mergePersistedChatMessages } from './dockChatTimeline';
import {
  isStructuredInteractionKind,
  resolveDockComposerState,
  transformDockStream,
} from './dockChatState';

interface ChatViewProps {
  workspaceId: string;
  chatId: string;
  scrollToLatestRequest: number;
  textareaRef: React.RefObject<HTMLTextAreaElement | null>;
  initialDraft?: string;
  onDraftConsumed?: () => void;
  draftValue?: string;
  onDraftChange?: (value: string) => void;
  onChatChanged?: () => void;
  onRunStatusChange?: (runId: string | null, status: AgentRun['status'] | null) => void;
  streamController: AgentRunStreamState;
  onPresenceChange?: (state: AskAgentAvatarState | null) => void;
  onRunIdChange?: (runId: string | null) => void;
  requiredPageContext?: CommandBarPageContext | null;
  showComposerShortcutHint?: boolean;
}

const ACTIVE_RUN_STATUSES = new Set(['queued', 'running', 'paused']);

function newClientMessageID() {
  if (typeof globalThis.crypto?.randomUUID === 'function') return globalThis.crypto.randomUUID();
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (character) => {
    const value = Math.floor(Math.random() * 16);
    return (character === 'x' ? value : ((value & 0x3) | 0x8)).toString(16);
  });
}

/**
 * One dock chat: the backing chat-mode run's transcript, the composer, the
 * confirm/interaction cards, and strips for child plans launched from this
 * chat. All run reads go through the chat-scoped /dock endpoints so users
 * without PM permissions can use their own dock.
 */
export function ChatView({
  workspaceId,
  chatId,
  scrollToLatestRequest,
  textareaRef,
  initialDraft,
  onDraftConsumed,
  draftValue,
  onDraftChange,
  onChatChanged,
  onRunStatusChange,
  streamController,
  onPresenceChange,
  onRunIdChange,
  requiredPageContext,
  showComposerShortcutHint,
}: ChatViewProps) {
  const [detail, setDetail] = useState<DockChatDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(true);
  const [plans, setPlans] = useState<CommandBarPlanSummary[]>([]);
  const [localValue, setLocalValue] = useState('');
  const value = draftValue ?? localValue;
  const setValue = useCallback((next: string) => {
    if (onDraftChange) onDraftChange(next);
    else setLocalValue(next);
  }, [onDraftChange]);
  const [sending, setSending] = useState(false);
  const [stopping, setStopping] = useState(false);
  const [pendingEcho, setPendingEcho] = useState<{ id: string; content: string } | null>(null);
  const [persistedMessages, setPersistedMessages] = useState<AgentRunMessage[]>([]);
  const [nextMessagesBefore, setNextMessagesBefore] = useState<number | null>(null);
  const [loadingEarlier, setLoadingEarlier] = useState(false);
  const [references, setReferences] = useState<DockEntityReference[]>([]);
  const [sendError, setSendError] = useState<{
    message: string;
    content: string;
    references: DockEntityReference[];
    clientMessageId: string;
  } | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const autoFollowRef = useRef(true);
  const [atBottom, setAtBottom] = useState(true);
  const currentUserId = useAuthStore((state) => state.user?.id);

  const { pageContext, scopeOptions, activeScopeKey, setActiveScopeKey } = usePageContextState();
  const [contextCleared, setContextCleared] = useState(false);
  const effectivePageContext = requiredPageContext ?? (contextCleared ? null : pageContext);

  useEffect(() => {
    if (!initialDraft) return;
    const timer = window.setTimeout(() => {
      setValue(initialDraft);
      onDraftConsumed?.();
    }, 0);
    return () => window.clearTimeout(timer);
  }, [initialDraft, onDraftConsumed, setValue]);

  const run = detail?.run ?? null;
  const isSharedTeammate = !!detail?.chat.user_id && !!currentUserId && detail.chat.user_id !== currentUserId;
  const runActive = !!run && ACTIVE_RUN_STATUSES.has(run.status);
  useEffect(() => {
    onRunIdChange?.(run?.id ?? null);
  }, [onRunIdChange, run?.id]);
  useEffect(() => {
    onRunStatusChange?.(run?.id ?? null, run?.status ?? null);
  }, [onRunStatusChange, run?.id, run?.status]);

  const { currentPlan, streamState, pendingInteraction, refetch, clearPendingInteraction } =
    streamController;

  const refreshDetail = useCallback(async () => {
    const res = await dockChatService.getChat(workspaceId, chatId);
    if (res.data) setDetail(res.data);
    return res.data ?? null;
  }, [chatId, workspaceId]);

  const refreshMessages = useCallback(async () => {
    const res = await dockChatService.listMessages(workspaceId, chatId, undefined, 50);
    if (res.data) {
      setPersistedMessages((current) => mergeMessagePages(current, res.data?.messages ?? []));
      setNextMessagesBefore(res.data.next_before ?? null);
    }
    return res.data?.messages ?? [];
  }, [chatId, workspaceId]);

  const loadEarlierMessages = useCallback(async () => {
    if (!nextMessagesBefore || loadingEarlier) return;
    setLoadingEarlier(true);
    try {
      const res = await dockChatService.listMessages(workspaceId, chatId, nextMessagesBefore, 50);
      if (!res.data) return;
      setPersistedMessages((current) => mergeMessagePages(current, res.data?.messages ?? []));
      setNextMessagesBefore(res.data.next_before ?? null);
    } finally {
      setLoadingEarlier(false);
    }
  }, [chatId, loadingEarlier, nextMessagesBefore, workspaceId]);

  // Load chat on mount / chat switch.
  useEffect(() => {
    autoFollowRef.current = true;
    const timer = window.setTimeout(() => {
      setPersistedMessages([]);
      setNextMessagesBefore(null);
      void Promise.all([refreshDetail(), refreshMessages()]).finally(() => setDetailLoading(false));
    }, 0);
    return () => window.clearTimeout(timer);
  }, [refreshDetail, refreshMessages]);

  // Refresh the run summary when its WS event fires (stream refetch is
  // handled inside useAgentRunStream; this keeps status/pause_reason fresh).
  useEffect(() => {
    if (!run?.id) return;
    let refreshTimer: ReturnType<typeof setTimeout> | null = null;
    const handler = (event: Event) => {
      const detailPayload = (event as CustomEvent<{ entity_id?: string }>).detail;
      if (detailPayload?.entity_id !== run.id) return;
      if (refreshTimer) return;
      refreshTimer = setTimeout(() => {
        refreshTimer = null;
        void refreshDetail();
      }, 100);
    };
    window.addEventListener('agent_run-updated', handler);
    return () => {
      if (refreshTimer) clearTimeout(refreshTimer);
      window.removeEventListener('agent_run-updated', handler);
    };
  }, [refreshDetail, run?.id]);

  const mergedStream = useMemo(
    () => mergePersistedChatMessages(streamState, persistedMessages),
    [persistedMessages, streamState],
  );
  const transformed = useMemo(
    () => (mergedStream ? transformDockStream(mergedStream, 'sequence') : null),
    [mergedStream],
  );
  const visiblePlanIDsKey = useMemo(() => {
    const ids = new Set(detail?.plan_ids ?? []);
    for (const childResult of transformed?.childResults ?? []) {
      if (childResult.result.plan_id) ids.add(childResult.result.plan_id);
    }
    return [...ids].join(',');
  }, [detail?.plan_ids, transformed?.childResults]);

  // Child plans launched from this chat. Result markers in every loaded
  // message page extend the recent-plan list, so older attempts reappear as
  // their surrounding history is paged in instead of disappearing at a
  // separate plan limit.
  useEffect(() => {
    const planIds = visiblePlanIDsKey ? visiblePlanIDsKey.split(',') : [];
    if (planIds.length === 0) {
      const timer = window.setTimeout(() => setPlans([]), 0);
      return () => window.clearTimeout(timer);
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
  }, [visiblePlanIDsKey, workspaceId]);

  // Authoritative pending-interaction fallback: when the run is paused on a
  // human interaction but the event stream hasn't surfaced it (missed WS
  // event, projection lag), fetch the interaction rows directly so the
  // approval card always renders instead of leaving the composer open.
  const [fallbackInteraction, setFallbackInteraction] = useState<CodingSessionInteraction | null>(null);
  const pausedOnInteraction =
    run?.status === 'paused' && (run.pause_reason === 'human_approval' || run.pause_reason === 'human_input');
  useEffect(() => {
    if (!pausedOnInteraction) {
      const timer = window.setTimeout(() => setFallbackInteraction(null), 0);
      return () => window.clearTimeout(timer);
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

  const presenceState = deriveAskAgentAvatarState({
    run: run ?? streamController.session,
    stream: transformed?.stream ?? streamState,
    sending,
    error: sendError?.message,
  });

  useEffect(() => {
    onPresenceChange?.(presenceState);
  }, [onPresenceChange, presenceState]);
  useEffect(() => () => onPresenceChange?.(null), [onPresenceChange]);

  // Reconcile the optimistic echo by its durable client id, never by text.
  useEffect(() => {
    if (!pendingEcho) return;
    const matched = persistedMessages.some((message) => message.client_message_id === pendingEcho.id);
    if (matched) {
      const timer = window.setTimeout(() => setPendingEcho(null), 0);
      return () => window.clearTimeout(timer);
    }
  }, [pendingEcho, persistedMessages]);

  // Track whether the user is near the tail; only then keep auto-following.
  useEffect(() => {
    const node = scrollRef.current;
    if (!node) return;
    const update = () => {
      const distanceFromBottom = node.scrollHeight - node.scrollTop - node.clientHeight;
      const follow = distanceFromBottom < 96;
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

  // Selecting a chat is an explicit request to resume at its latest message,
  // including when the already-active chat is selected again.
  useEffect(() => {
    scrollToLatest();
  }, [detailLoading, scrollToLatest, scrollToLatestRequest]);

  // Keep the transcript pinned to the bottom as content streams in, unless the
  // user has scrolled up to read earlier turns.
  useEffect(() => {
    const node = scrollRef.current;
    if (node && autoFollowRef.current) node.scrollTop = node.scrollHeight;
  }, [transformed, currentPlan, pendingEcho, pendingInteraction, sendError]);

  const effectiveInteraction = pendingInteraction ?? fallbackInteraction;
  const dockConfirm = effectiveInteraction ? parseDockPlanConfirm(effectiveInteraction.request_payload) : null;
  const structuredPending = !dockConfirm && isStructuredInteractionKind(effectiveInteraction?.interaction_kind);
  const composer = resolveDockComposerState(
    run ? { status: run.status, pause_reason: run.pause_reason } : null,
    !!dockConfirm || structuredPending,
    detailLoading || sending,
  );

  const sendContent = useCallback(
    async (content: string, messageReferences: DockEntityReference[] = references, retryClientMessageID?: string) => {
      if (!content || sending) return;
	  const clientMessageId = retryClientMessageID ?? newClientMessageID();
      const needsTitle = !detail?.chat.title.trim();
      setSending(true);
      setSendError(null);
      setPendingEcho({ id: clientMessageId, content });
      autoFollowRef.current = true;
      setAtBottom(true);
      try {
        const res = await dockChatService.sendMessage(workspaceId, chatId, {
          client_message_id: clientMessageId,
          content,
          page_context: effectivePageContext ?? undefined,
          references: messageReferences.length > 0 ? messageReferences : undefined,
        });
        if (res.error || !res.data) {
          setPendingEcho(null);
          setSendError({ message: res.error ?? 'Failed to send message', content, references: messageReferences, clientMessageId });
          return;
        }
		if (res.data.accepted_message) {
		  setPersistedMessages((current) => mergeMessagePages(current, [res.data!.accepted_message!]));
		}
        setReferences([]);
        setDetail(res.data);
        onChatChanged?.();
        if (needsTitle) {
          void (async () => {
            const titleResult = await dockChatService.generateTitle(workspaceId, chatId, {
              content,
              page_context: effectivePageContext ?? undefined,
            });
            if (!titleResult.data) return;
            setDetail((current) => current ? { ...current, chat: titleResult.data! } : current);
            onChatChanged?.();
          })();
        }
        if (run?.id && res.data.run?.id === run.id) {
          // Same backing run: reconcile the persisted user message immediately.
          void refetch();
        }
		void refreshMessages();
        // Successor run: useAgentRunStream will reset and fetch with the returned
        // run id instead of invoking this render's predecessor refetch closure.
      } finally {
        setSending(false);
      }
    },
    [chatId, detail?.chat.title, effectivePageContext, onChatChanged, references, refetch, refreshMessages, run?.id, sending, workspaceId],
  );

  const submit = async () => {
    const content = value.trim();
    if (!content) return;
    setValue('');
    await sendContent(content, references);
  };

  const canStop = !isSharedTeammate && runActive && (run?.status === 'queued' || run?.status === 'running');
  const handleStop = useCallback(async () => {
    if (stopping) return;
    setStopping(true);
    try {
      const res = await dockChatService.cancelChatRun(workspaceId, chatId);
      if (res.error) toast.error(res.error);
      void refreshDetail();
      void refetch();
    } finally {
      setStopping(false);
    }
  }, [chatId, refetch, refreshDetail, stopping, workspaceId]);

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

  // One-line live status under the transcript: prefer the running tool's
  // label, hidden while assistant text is actively streaming (the text itself
  // is the status then).
  const liveStatusLabel = useMemo(() => {
    if (sending) return 'Thinking…';
    if (!runActive || run?.status === 'paused') return null;
    const stream = transformed?.stream ?? null;
    const lastLive = stream?.live_turn_segments[stream.live_turn_segments.length - 1];
    const assistantStreaming =
      lastLive?.kind === 'assistant_message'
      && lastLive.assistant_message.status === 'streaming'
      && lastLive.assistant_message.content.trim().length > 0;
    if (assistantStreaming) return null;
    return deriveLiveStatusLabel(stream, run?.status);
  }, [run?.status, runActive, sending, transformed]);

  const runsById = useMemo(() => {
    const map: Record<string, AgentRun> = {};
    for (const plan of plans) {
      for (const childRun of plan.runs ?? []) map[childRun.id] = childRun;
    }
    return map;
  }, [plans]);

  const subAgentTimelineItems = useMemo(() => {
    const resultByPlanID = new Map(
      (transformed?.childResults ?? []).map((entry) => [entry.result.plan_id, entry]),
    );
    const firstVisibleTimestamp = transformed?.stream.transcript_messages.reduce<number | null>((earliest, message) => {
      const value = Date.parse(message.timestamp);
      if (!Number.isFinite(value)) return earliest;
      return earliest === null ? value : Math.min(earliest, value);
    }, null) ?? null;

    return plans.flatMap((plan) => {
      const resultEntry = resultByPlanID.get(plan.id);
      const resultSequence = resultEntry?.sequenceNo;
      const createdTimestamp = Date.parse(plan.created_at);
      const active = plan.status === 'running';
      // A recent-plan response can reach farther back than the loaded message
      // page. Do not strand an old plan at the top of the visible page; reveal
      // it when its surrounding page/result marker is loaded.
      if (
        !active
        && resultSequence === undefined
        && firstVisibleTimestamp !== null
        && Number.isFinite(createdTimestamp)
        && createdTimestamp < firstVisibleTimestamp
      ) {
        return [];
      }
      const displayPlan = planSummaryToRunPlan(plan);
      if (!displayPlan.errorMessage && resultEntry?.result.error) {
        displayPlan.errorMessage = resultEntry.result.error;
      }
      return [{
        id: plan.id,
        createdAt: plan.created_at,
        resultSequence,
        runCount: Math.max(plan.run_count, plan.steps.length, 1),
        content: (
          <ExecutionStrip
            kind="plan"
            workspaceId={workspaceId}
            plan={displayPlan}
            runsById={runsById}
          />
        ),
      }];
    });
  }, [plans, runsById, transformed, workspaceId]);

  const needsApproval = (
    (run?.status === 'paused' && run.pause_reason === 'human_approval')
    || effectiveInteraction?.interaction_kind.includes('approval') === true
    || Object.values(runsById).some(
      (childRun) => childRun.status === 'paused' && childRun.pause_reason === 'human_approval',
    )
  );

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="relative flex min-h-0 flex-1 flex-col">
      <div ref={scrollRef} data-agent-dock-chat-scroll className="min-h-0 flex-1 space-y-3 overflow-y-auto px-4 py-3">
		{nextMessagesBefore && (
		  <div className="flex justify-center">
		    <button
		      type="button"
		      className="text-xs font-medium text-muted-foreground hover:text-foreground"
		      disabled={loadingEarlier}
		      onClick={() => void loadEarlierMessages()}
		    >
		      {loadingEarlier ? 'Loading…' : 'Load earlier messages'}
		    </button>
		  </div>
		)}
        {detailLoading && !detail && (
          <p className="py-6 text-center text-sm text-muted-foreground">Loading chat…</p>
        )}
        {!detailLoading && !run && !pendingEcho && (
          <p className="py-6 text-center text-sm text-muted-foreground">
            {requiredPageContext?.entity_type === 'support_conversation'
              ? 'Ask about this conversation, draft a reply, investigate the issue, or have an agent take the next step.'
              : 'Ask a question about your workspace, or describe work for an agent to do.'}
          </p>
        )}
        {transformed && (
          <DockTranscript
            stream={transformed.stream}
            active={runActive}
            workspaceId={workspaceId}
            fallbackActor={streamController.session?.triggered_by_user}
            subAgentRuns={subAgentTimelineItems}
          />
        )}
        {currentPlan && (
          <CodingPlanPanel plan={currentPlan} runStatus={run?.status} title="Work plan" />
        )}
        {pendingEcho && <DockUserMessage content={pendingEcho.content} pending />}
        {sendError && (
          <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs">
            <p className="mb-1 line-clamp-2 text-foreground/80">{sendError.content}</p>
            <div className="flex items-center justify-between gap-2">
              <span className="min-w-0 truncate text-destructive">{sendError.message}</span>
              <div className="flex shrink-0 items-center gap-3">
                <button
                  type="button"
                  className="font-medium text-foreground hover:underline"
                  onClick={() => void sendContent(sendError.content, sendError.references, sendError.clientMessageId)}
                >
                  Retry
                </button>
                <button
                  type="button"
                  className="text-muted-foreground hover:underline"
                  onClick={() => {
                    setValue(sendError.content);
                    setReferences(sendError.references);
                    setSendError(null);
                  }}
                >
                  Edit message
                </button>
              </div>
            </div>
          </div>
        )}
        {liveStatusLabel && (
          <StreamingStatusText className="text-xs">{liveStatusLabel}</StreamingStatusText>
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
      {!atBottom && <ScrollToLatestButton onClick={scrollToLatest} />}
      </div>
      {needsApproval && !atBottom ? (
        <ApprovalAttentionBanner onReview={scrollToLatest} />
      ) : null}
      {composer.visible && (
        <div className="border-t border-border/60 p-2">
          <DockInput
            mode="conversation"
            value={value}
            onChange={setValue}
            onSubmit={() => void submit()}
            pageContext={effectivePageContext}
            contextOptions={requiredPageContext ? [] : scopeOptions}
            activeContextKey={activeScopeKey}
            onContextKeyChange={(key) => {
              setContextCleared(false);
              setActiveScopeKey(key);
            }}
            onClearContext={requiredPageContext ? undefined : () => setContextCleared(true)}
            workspaceId={workspaceId}
            references={references}
            onAddReference={(reference) => {
              setReferences((current) => {
                if (current.length >= 10) {
                  toast.error('You can attach up to 10 references.');
                  return current;
                }
                const exists = current.some(
                  (item) => item.entity_type === reference.entity_type && item.entity_id === reference.entity_id,
                );
                return exists ? current : [...current, reference];
              });
            }}
            onRemoveReference={(reference) => {
              setReferences((current) => current.filter(
                (item) => item.entity_type !== reference.entity_type || item.entity_id !== reference.entity_id,
              ));
            }}
            busy={sending}
            disabled={!composer.enabled}
            autoFocus
            textareaRef={textareaRef}
            onStop={canStop ? () => void handleStop() : undefined}
            stopping={stopping}
            showShortcutHint={showComposerShortcutHint}
          />
        </div>
      )}
    </div>
  );
}
