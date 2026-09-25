import { dockWorkPlans, hasWorkPlanOrigin } from './dockWorkPlans';
import activityStyles from './DockActivityTimeline.module.css';
import { useAskAgentDefaults } from "@/hooks/queries/useAskAgentDefaults";
import { AIConnectionPicker } from '@/components/agents/AIConnectionPicker';
import { AISettingsLink } from '@/components/agents/AISettingsLink';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { AIConnectionSelection } from '@/lib/services/aiConnectionService';
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { ArrowRight01Icon, ArrowUp01Icon, Loading01Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { usePageContextState } from '@/components/command-bar/pageContext';
import { commandBarService } from '@/lib/services/commandBarService';
import { dockChatService } from '@/lib/services/dockChatService';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { uploadEditorFile } from '@/hooks/useEditorImageUpload';
import { parseDockPlanConfirm } from '@/lib/dockTypes';
import type { DockChatDetail, DockChatMediaAttachment, DockEntityReference } from '@/lib/dockTypes';
import type { AgentRun, AgentRunMessage, CodingSessionInteraction, CommandBarPageContext, CommandBarPlanSummary } from '@/lib/pmTypes';
import { DockInput } from './DockInput';
import { DockExecutionPicker } from './DockExecutionPicker';
import { resolveExecutionPickerDisabled } from './executionPickerState';
import { DockArtifactDownloads } from './DockArtifactDownloads';
import { DockTranscript, type DockMessageSubmission } from './DockTranscript';
import { DockPlanConfirmCard } from './DockPlanConfirmCard';
import { ExecutionStrip } from './ExecutionStrip';
import { PendingInteractionCard } from './PendingInteractionCard';
import { DockInteractionLayer } from './DockInteractionLayer';
import { CodingPlanPanel } from '@/components/pm/CodingSession/CodingPlanPanel';
import type { AskAgentAvatarState } from '@/components/agents/AskAgentAvatar';
import { deriveAskAgentAvatarState } from '@/components/agents/askAgentPresence';
import { ScrollToLatestButton } from '@/components/agents/transcript';
import { AgentLiveStatus } from './AgentLiveStatus';
import { resolveAgentLiveProgress } from './agentProgress';
import { resolveVisibleTurn } from './agentTurnState';
import { chatFollowUpSuggestions } from './followUpSuggestions';
import { starterSuggestionsForContext } from './starterSuggestions';
import { focusComposerAtEnd } from './composerFocus';
import { planSummaryToRunPlan } from './planSummary';
import type { AgentRunStreamState } from './useAgentRunStream';
import {
  hasAuthoritativeDockRuntimeTimeline,
  mergeMessagePages,
  mergePersistedChatMessages,
  type PendingDockChatMessage,
} from './dockChatTimeline';
import {
  isDockTranscriptStreaming,
  resolveDockComposerState,
  transformDockStream,
} from './dockChatState';
import { createDockReadQueue } from './dockReadQueue';
import { useAuthStore } from '@/stores/authStore';
import { useDockStore } from '@/stores/dockStore';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';
import { isDockNetworkAvailable, useDockNetworkActivity } from './useDockNetworkActivity';

interface ChatViewProps {
  workspaceId: string;
  chatId?: string;
  rosterRunId?: string | null;
  active?: boolean;
  onCreateChat?: (options?: { executionEnabled?: boolean }) => Promise<{ id: string } | null>;
  scrollToLatestRequest: number;
  textareaRef: React.RefObject<HTMLTextAreaElement | null>;
  initialDraft?: string;
  onDraftConsumed?: () => void;
  /** References attached when the chat opens, e.g. from a list selection. */
  initialReferences?: DockEntityReference[];
  onReferencesConsumed?: () => void;
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
  rosterRunId,
  active = true,
  onCreateChat,
  scrollToLatestRequest,
  textareaRef,
  initialDraft,
  onDraftConsumed,
  initialReferences,
  onReferencesConsumed,
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
  const browserAvailable = useDockNetworkActivity();
  const networkAvailable = browserAvailable && active;
  const currentUserId = useAuthStore((state) => state.user?.id);
  const detailRefreshed = useRef(false);
  const cachedTranscript = chatId ? useDockStore.getState().transcripts[chatId] : undefined;
  const cacheTranscript = useDockStore((state) => state.cacheTranscript);
  const [aiConnection, setAIConnection] = useState<AIConnectionSelection>({});
  const [changingExecution, setChangingExecution] = useState(false);
  const [draftExecutionEnabled, setDraftExecutionEnabled] = useState(false);
  const [detail, setDetail] = useState<DockChatDetail | null>(cachedTranscript?.detail ?? null);
  const [detailLoading, setDetailLoading] = useState(!!chatId && !cachedTranscript);
  const [refreshError, setRefreshError] = useState<string | null>(null);
  const [plans, setPlans] = useState<CommandBarPlanSummary[]>([]);
  const [localValue, setLocalValue] = useState('');
  const value = draftValue ?? localValue;
  const setValue = useCallback((next: string) => {
    if (onDraftChange) onDraftChange(next);
    else setLocalValue(next);
  }, [onDraftChange]);
  const insertSuggestion = useCallback((next: string) => {
    setValue(next);
    window.requestAnimationFrame(() => focusComposerAtEnd(textareaRef.current, next));
  }, [setValue, textareaRef]);
  const [sending, setSending] = useState(false);
  const [launchStartedAt, setLaunchStartedAt] = useState<string | undefined>();
  const [stopping, setStopping] = useState(false);
  const [pausing, setPausing] = useState(false);
  const [resuming, setResuming] = useState(false);
  const [pendingEcho, setPendingEcho] = useState<PendingDockChatMessage | null>(null);
  const [latestSubmission, setLatestSubmission] = useState<DockMessageSubmission | null>(null);
  const [failedClientMessageIds, setFailedClientMessageIds] = useState<ReadonlySet<string>>(() => new Set());
  const [persistedMessages, setPersistedMessages] = useState<AgentRunMessage[]>(cachedTranscript?.messages ?? []);
  const [nextMessagesBefore, setNextMessagesBefore] = useState<number | null>(cachedTranscript?.nextBefore ?? null);
  const [loadingEarlier, setLoadingEarlier] = useState(false);
  const [references, setReferences] = useState<DockEntityReference[]>([]);
  const [mediaAttachments, setMediaAttachments] = useState<DockChatMediaAttachment[]>([]);
  const [analyzingMedia, setAnalyzingMedia] = useState(false);
  const [analyzingMediaLabel, setAnalyzingMediaLabel] = useState('');
  const [sendError, setSendError] = useState<{
    message: string;
    content: string;
    references: DockEntityReference[];
    clientMessageId: string;
  } | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const autoFollowRef = useRef(true);
  const [atBottom, setAtBottom] = useState(true);
  const { pageContext, scopeOptions, activeScopeKey, setActiveScopeKey } = usePageContextState();
  const [contextCleared, setContextCleared] = useState(false);
  // A context supplied by the source surface starts attached, but it must not
  // trap the chat there. Clearing it affects only subsequent turns in this
  // dock chat; it never changes the underlying support conversation.
  const effectivePageContext = contextCleared ? null : (requiredPageContext ?? pageContext);

  useEffect(() => {
    if (!initialDraft) return;
    const timer = window.setTimeout(() => {
      setValue(initialDraft);
      onDraftConsumed?.();
    }, 0);
    return () => window.clearTimeout(timer);
  }, [initialDraft, onDraftConsumed, setValue]);

  useEffect(() => {
    if (!initialReferences || initialReferences.length === 0) return;
    const timer = window.setTimeout(() => {
      setReferences((current) => {
        const next = current.slice();
        for (const reference of initialReferences) {
          if (next.length >= 10) break;
          const exists = next.some(
            (item) => item.entity_type === reference.entity_type && item.entity_id === reference.entity_id,
          );
          if (!exists) next.push(reference);
        }
        return next;
      });
      onReferencesConsumed?.();
    }, 0);
    return () => window.clearTimeout(timer);
  }, [initialReferences, onReferencesConsumed]);

  const workspaceSlug = useWorkspaceStore((state) => state.currentWorkspace?.id === workspaceId ? state.currentWorkspace.slug : undefined);
  const run = detail?.run ?? null;
  const executionEnabled = detail?.chat.execution_enabled ?? draftExecutionEnabled;
  const acceptedSelection = run?.input?.ai_selection;
  const acceptedProfileId = acceptedSelection && typeof acceptedSelection === 'object' && 'profile_id' in acceptedSelection && typeof acceptedSelection.profile_id === 'string'
    ? acceptedSelection.profile_id : typeof run?.input?.ai_profile_id === 'string' ? run.input.ai_profile_id : undefined;
  const agentDefaults = useAskAgentDefaults(!run && active ? workspaceId : "");
  const askAgentDefault = agentDefaults.data?.ai_profile_id;
  const agentDefaultUnavailable = !run && !aiConnection.ai_profile_id && (agentDefaults.isPending || agentDefaults.isError);
  const runActive = !!run && ACTIVE_RUN_STATUSES.has(run.status);
  useEffect(() => {
    if (detailRefreshed.current) onRunIdChange?.(run?.id ?? null);
  }, [detail, onRunIdChange, run?.id]);
  useEffect(() => {
    onRunStatusChange?.(run?.id ?? null, run?.status ?? null);
  }, [onRunStatusChange, run?.id, run?.status]);

  const { currentPlan, streamState, pendingInteraction, refetch, clearPendingInteraction } =
    streamController;

  const acceptedSendVersion = useRef(0);
  const canRefresh = useCallback(() => isDockNetworkAvailable()
    && useAuthStore.getState().user?.id === currentUserId, [currentUserId]);
  const detailReader = useMemo(() => createDockReadQueue(
    (signal) => dockChatService.getChat(workspaceId, chatId!, signal),
    canRefresh,
    setRefreshError,
  ), [canRefresh, chatId, workspaceId]);
  const messagesReader = useMemo(() => createDockReadQueue(
    (signal) => dockChatService.listMessages(workspaceId, chatId!, undefined, 50, signal),
    canRefresh,
    setRefreshError,
  ), [canRefresh, chatId, workspaceId]);
  const readScopeRef = useRef(detailReader);
  useLayoutEffect(() => {
    readScopeRef.current = detailReader;
  }, [detailReader]);
  useEffect(() => {
    detailReader.activate(); messagesReader.activate();
    return () => { detailReader.dispose(); messagesReader.dispose(); };
  }, [detailReader, messagesReader]);
  useEffect(() => {
    if (!networkAvailable) { detailReader.pause(); messagesReader.pause(); }
  }, [detailReader, messagesReader, networkAvailable]);
  const refreshDetail = useCallback(async (automatic = false) => {
	if (!chatId) return null;
    const version = acceptedSendVersion.current;
    const res = await detailReader.read(automatic);
    if (!res) return null;
    // A fetch started before send acceptance must not erase the new run.
    if (version !== acceptedSendVersion.current) return null;
    if (res.error || !res.data) {
      setRefreshError(res.error ?? 'Unable to refresh conversation');
      return null;
    }
    if (res.data) { detailRefreshed.current = true; setDetail(res.data); }
    return res.data ?? null;
  }, [chatId, detailReader]);

  const changeExecution = useCallback(async (enabled: boolean) => {
    if (!chatId) {
      setDraftExecutionEnabled(enabled);
      return;
    }
    setChangingExecution(true);
    try {
      const response = await dockChatService.updateChat(workspaceId, chatId, { execution_enabled: enabled });
      if (response.error || !response.data) {
        toast.error(response.error ?? 'Could not update execution settings.');
        return;
      }
      setDetail((current) => current ? { ...current, chat: response.data! } : current);
      onChatChanged?.();
      toast.message(enabled
        ? 'Code and Python tools will be available for your next message.'
        : 'Execution stopped. Check any interrupted external action before retrying.');
    } catch {
      toast.error('Could not update execution settings.');
    } finally {
      setChangingExecution(false);
    }
  }, [chatId, onChatChanged, workspaceId]);

  const refreshMessages = useCallback(async (automatic = false) => {
	if (!chatId) return [];
    const res = await messagesReader.read(automatic);
    if (!res) return [];
    if (res.error || !res.data) {
      setRefreshError(res.error ?? 'Unable to refresh conversation');
      return [];
    }
    if (res.data) {
      setPersistedMessages((current) => mergeMessagePages(current, res.data?.messages ?? []));
      setNextMessagesBefore(res.data.next_before ?? null);
    }
    return res.data?.messages ?? [];
  }, [chatId, messagesReader]);

  const loadEarlierMessages = useCallback(async () => {
    if (!chatId || !nextMessagesBefore || loadingEarlier) return;
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
  const refreshConversation = useCallback(async (automatic = false) => {
    if (!chatId) return;
    setRefreshError(null);
    if (!useDockStore.getState().transcripts[chatId]) setDetailLoading(true);
    await Promise.all([refreshDetail(automatic), refreshMessages(automatic)]);
    // This cycle owns initial loading. Later automatic batches can remain
    // active while its detail and message reads have already settled.
    if (readScopeRef.current === detailReader) setDetailLoading(false);
  }, [chatId, detailReader, refreshDetail, refreshMessages]);

  useEffect(() => {
    if (chatId && !networkAvailable) return;
    autoFollowRef.current = true;
    const timer = window.setTimeout(() => {
      if (!chatId) {
        setDetail(null);
        setPersistedMessages([]);
        setNextMessagesBefore(null);
        setDetailLoading(false);
        return;
      }
      if (isDockNetworkAvailable()) void refreshConversation(true);
    }, 0);
    return () => window.clearTimeout(timer);
	}, [chatId, networkAvailable, refreshConversation, rosterRunId]);

  useEffect(() => {
    if (!chatId || !detail || detailLoading) return;
    cacheTranscript(chatId, { detail,
      messages: persistedMessages.filter((message) => message.delivery_status === 'sent'
        || !message.client_message_id || !failedClientMessageIds.has(message.client_message_id)),
      nextBefore: nextMessagesBefore });
  }, [cacheTranscript, chatId, detail, detailLoading, failedClientMessageIds, nextMessagesBefore, persistedMessages]);

  // Recover persisted messages and the chat's active run after missed socket
  // updates, including successor runs and approvals resolved in another tab.
  useEffect(() => {
    if (!chatId || !networkAvailable) return;
    let timer: ReturnType<typeof setTimeout> | null = null;
    let includeMessages = false;
    const recover = (messages = true) => {
      includeMessages ||= messages;
      if (timer || !isDockNetworkAvailable()) return;
      timer = setTimeout(() => {
        timer = null;
        if (!isDockNetworkAvailable()) return;
        if (includeMessages) void refreshConversation(true);
        else void refreshDetail(true);
        includeMessages = false;
      }, 100);
    };
    const onRun = (event: Event) => {
      const payload = (event as CustomEvent<{ entity_id?: string; update_kind?: string; data?: { change_kind?: string } }>).detail;
      if (payload?.entity_id === run?.id && payload.update_kind !== 'duplicate') recover(payload.data?.change_kind === 'message');
    };
    const onSession = (event: Event) => {
      const payload = (event as CustomEvent<{ parent_id?: string; data?: { type?: string } }>).detail;
      if (payload?.parent_id === run?.id && /message\.completed|child|interaction/.test(payload?.data?.type ?? '')) recover();
    };
    const onFocus = () => recover();
    const unsubscribe = useSupportPresenceStore.subscribe((state, previous) => {
      if (state.wsConnected && !previous.wsConnected) recover();
    });
    window.addEventListener('focus', onFocus);
    window.addEventListener('coding_session_event-created', onSession);
    window.addEventListener('agent_run-updated', onRun);
    window.addEventListener('coding_session-updated', onRun);
    return () => {
      if (timer) clearTimeout(timer);
      unsubscribe();
      window.removeEventListener('focus', onFocus);
      window.removeEventListener('coding_session_event-created', onSession);
      window.removeEventListener('agent_run-updated', onRun);
      window.removeEventListener('coding_session-updated', onRun);
    };
  }, [chatId, networkAvailable, refreshConversation, refreshDetail, run?.id]);

  // A visible fallback snapshot can discover a lifecycle change without WS.
  // Re-read authoritative chat detail instead of replacing an accepted send
  // with an older snapshot that happens to share the same run id.
  const snapshotLifecycle = streamController.session
    ? `${streamController.session.id}:${streamController.session.status}:${streamController.session.pause_reason}`
    : null;
  const previousSnapshotLifecycle = useRef(snapshotLifecycle);
  useEffect(() => {
    const previous = previousSnapshotLifecycle.current;
    previousSnapshotLifecycle.current = snapshotLifecycle;
    if (!previous || !snapshotLifecycle || previous === snapshotLifecycle || !networkAvailable) return;
    if (!run || (streamController.session?.id === run.id
      && streamController.session.status === run.status
      && streamController.session.pause_reason === run.pause_reason)) return;
    const timer = window.setTimeout(() => { void refreshConversation(true); }, 0);
    return () => window.clearTimeout(timer);
  }, [networkAvailable, refreshConversation, run, snapshotLifecycle, streamController.session]);

  const mergedStream = useMemo(
    () => mergePersistedChatMessages(streamState, persistedMessages, { pendingMessage: pendingEcho, failedClientMessageIds }),
    [failedClientMessageIds, pendingEcho, persistedMessages, streamState],
  );
  // A lost acknowledgement may look like a failed send until recovery reads
  // the saved row. Hide the retry card in that same render, before cleanup.
  const sendErrorDelivered = !!sendError && mergedStream?.transcript_messages.some((message) => (
    message.client_message_id === sendError.clientMessageId && message.delivery_status === 'sent'
  ));
  const visibleSendError = sendErrorDelivered ? null : sendError;
  useEffect(() => {
    if (!sendErrorDelivered) return;
    const timer = window.setTimeout(() => setSendError((current) => (
      current?.clientMessageId === sendError?.clientMessageId ? null : current
    )), 0);
    return () => window.clearTimeout(timer);
  }, [sendError, sendErrorDelivered]);
  const transformed = useMemo(
    () => (mergedStream ? transformDockStream(mergedStream, 'sequence') : null),
    [mergedStream],
  );
  const visibleTurn = resolveVisibleTurn(transformed?.stream ?? null, launchStartedAt);
  const answerRecoveryKey = visibleTurn.answerPending || visibleTurn.missingAnswer
    ? `${run?.id}:${transformed?.stream.turn_state?.turn_id}` : null;
  const [recoveredAnswerKey, setRecoveredAnswerKey] = useState<string | null>(null);
  useEffect(() => {
    if (!answerRecoveryKey || answerRecoveryKey === recoveredAnswerKey || !networkAvailable) return;
    let cancelled = false;
    // Read durable messages immediately when the terminal event or snapshot
    // arrives without its answer; do not wait for the paused-run poll interval.
    const timer = window.setTimeout(() => {
      void Promise.allSettled([refreshConversation(true), refetch()]).then(() => {
        if (!cancelled) setRecoveredAnswerKey(answerRecoveryKey);
      });
    }, 0);
    return () => { cancelled = true; window.clearTimeout(timer); };
  }, [answerRecoveryKey, recoveredAnswerKey, networkAvailable, refreshConversation, refetch]);
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
    !!chatId && run?.status === 'paused' && (run.pause_reason === 'human_approval' || run.pause_reason === 'human_input');
  useEffect(() => {
    if (!pausedOnInteraction) {
      const timer = window.setTimeout(() => setFallbackInteraction(null), 0);
      return () => window.clearTimeout(timer);
    }
    if (!networkAvailable) return;
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
  }, [chatId, networkAvailable, pausedOnInteraction, workspaceId]);

  const presenceState = deriveAskAgentAvatarState({
    run: run ?? streamController.session,
    stream: transformed?.stream ?? streamState,
    sending,
    error: visibleSendError?.message,
  });

  useEffect(() => {
    onPresenceChange?.(presenceState);
  }, [onPresenceChange, presenceState]);
  useEffect(() => () => onPresenceChange?.(null), [onPresenceChange]);

  // Reconcile the optimistic echo by its durable client id, never by text.
  useEffect(() => {
    if (!pendingEcho) return;
    const matched = persistedMessages.some((message) => message.client_message_id === pendingEcho.id && message.delivery_status !== 'pending');
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
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(() => {
      if (autoFollowRef.current) node.scrollTop = node.scrollHeight;
    });
    if (node.firstElementChild) observer?.observe(node.firstElementChild);
    return () => { node.removeEventListener('scroll', update); observer?.disconnect(); };
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
  }, [transformed, currentPlan, visibleSendError]);

  const effectiveInteraction = run?.pause_reason === 'manual' ? null : pendingInteraction ?? fallbackInteraction;
  const dockConfirm = effectiveInteraction ? parseDockPlanConfirm(effectiveInteraction.request_payload) : null;
  const composer = resolveDockComposerState(
    run ? { status: run.status, pause_reason: run.pause_reason } : null,
    !!effectiveInteraction,
    detailLoading || sending || agentDefaultUnavailable,
  );

  const sendContent = useCallback(
    async (content: string, messageReferences: DockEntityReference[] = references, retryClientMessageID?: string) => {
      if (!content || sending || agentDefaultUnavailable) return;
      const clientMessageId = retryClientMessageID ?? newClientMessageID();
      const needsTitle = !detail?.chat.title.trim();
      setSending(true);
      const attachmentIDs = mediaAttachments.filter((attachment) => attachment.status === 'ready' && attachment.id).map((attachment) => attachment.id!);
      const hasSourceContext = !!effectivePageContext || messageReferences.length > 0;
      setAnalyzingMedia(attachmentIDs.length > 0 || hasSourceContext);
      setAnalyzingMediaLabel(
        attachmentIDs.length === 1
          ? `Analyzing ${mediaAttachments.find((attachment) => attachment.id === attachmentIDs[0])?.file_name ?? 'attachment'}…`
          : attachmentIDs.length > 1
            ? `Analyzing ${attachmentIDs.length} attachments…`
            : run?.id
              ? 'Preparing your message…'
              : 'Checking context attachments…',
      );
      const sentAt = new Date().toISOString();
      setLaunchStartedAt(sentAt);
      setSendError(null);
      setLatestSubmission({ clientMessageId, precedingLiveSegmentIds: new Set([
        ...(mergedStream?.live_turn_segments ?? []).map((segment) => `live:${segment.segment_id}`),
        ...(mergedStream?.live_reasoning_message ? [`live-reasoning:${mergedStream.live_reasoning_message.message_id}`] : []),
      ]) });
      setPendingEcho({ id: clientMessageId, content, timestamp: sentAt, actor_user_id: currentUserId });
      autoFollowRef.current = true;
      setAtBottom(true);
      try {
        let targetChatId = chatId;
        if (!targetChatId) {
          const created = await onCreateChat?.({ executionEnabled: draftExecutionEnabled });
          if (!created) throw new Error('Unable to create chat. Please retry.');
          targetChatId = created.id;
        }
		const res = await dockChatService.sendMessage(workspaceId, targetChatId, {
          client_message_id: clientMessageId,
          ...(!run ? aiConnection : {}),
          content,
          page_context: effectivePageContext ?? undefined,
          references: messageReferences.length > 0 ? messageReferences : undefined,
          attachment_ids: attachmentIDs.length > 0 ? attachmentIDs : undefined,
        });
        if (res.error || !res.data) {
          setFailedClientMessageIds((current) => new Set([...current, clientMessageId]));
          setPendingEcho(null);
          setSendError({ message: res.error ?? 'Failed to send message', content, references: messageReferences, clientMessageId });
          return;
        }
		if (res.data.accepted_message) {
		  // The accepted row is the durable acknowledgement. Clear the
		  // optimistic row before updating the stream so a follow-up message
		  // cannot briefly render both copies while projections converge.
		  setPendingEcho(null);
		  setPersistedMessages((current) => mergeMessagePages(current, [res.data!.accepted_message!]));
		  const acceptedClientId = res.data.accepted_message.client_message_id;
		  if (acceptedClientId) setFailedClientMessageIds((current) => {
		    if (!current.has(acceptedClientId)) return current;
		    const next = new Set(current);
		    next.delete(acceptedClientId);
		    return next;
		  });
		}
        setReferences([]);
		setMediaAttachments((current) => {
		  current.forEach((attachment) => {
			if (attachment.preview_url) URL.revokeObjectURL(attachment.preview_url);
		  });
		  return [];
		});
        acceptedSendVersion.current += 1;
        detailRefreshed.current = true;
        setDetail(res.data);
        setDetailLoading(false);
        onChatChanged?.();
        if (needsTitle) {
          void (async () => {
			const titleResult = await dockChatService.generateTitle(workspaceId, targetChatId, {
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
      } catch (error) {
        setFailedClientMessageIds((current) => new Set([...current, clientMessageId]));
        setPendingEcho(null);
        setSendError({
          message: error instanceof Error ? error.message : 'Failed to send message',
          content, references: messageReferences, clientMessageId,
        });
      } finally {
		setAnalyzingMedia(false);
		setAnalyzingMediaLabel('');
        setSending(false);
      }
    },
    [agentDefaultUnavailable, aiConnection, chatId, currentUserId, detail?.chat.title, draftExecutionEnabled, effectivePageContext, mediaAttachments, mergedStream, onChatChanged, onCreateChat, references, refetch, refreshMessages, run, sending, workspaceId],
  );

  const submit = async () => {
    if (agentDefaultUnavailable) return;
    const readyAttachmentCount = mediaAttachments.filter((attachment) => attachment.status === 'ready' && attachment.id).length;
    const content = value.trim() || (readyAttachmentCount === 1 ? 'Review the attached file.' : readyAttachmentCount > 1 ? 'Review the attached files.' : '');
    if (!content) return;
    setValue('');
    await sendContent(content, references);
  };

  const addMediaAttachments = useCallback(async (files: File[]) => {
    const accepted = files.filter((file) => (
      [
        'image/jpeg', 'image/png', 'image/gif', 'image/webp',
        'video/mp4', 'video/quicktime', 'video/webm', 'video/mpeg',
        'application/pdf', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
        'text/plain', 'text/markdown', 'text/csv', 'application/json',
      ].includes(file.type)
      && file.size > 0
      && file.size <= 20 * 1024 * 1024
    ));
    if (accepted.length !== files.length) {
      toast.error('Ask supports images, short videos, PDF, DOCX, TXT, Markdown, CSV, and JSON files up to 20 MB.');
    }
    for (const file of accepted) {
      const localId = newClientMessageID();
      const previewUrl = file.type.startsWith('image/') ? URL.createObjectURL(file) : undefined;
      setMediaAttachments((current) => [...current, {
        local_id: localId,
        file_name: file.name || 'attachment',
        file_type: file.type,
        file_size: file.size,
        preview_url: previewUrl,
        status: 'uploading',
      }]);
      try {
        const uploaded = await uploadEditorFile(file, {
          workspaceId,
          entityType: 'editor_upload',
          entityId: workspaceId,
          private: true,
        });
        setMediaAttachments((current) => current.map((attachment) => attachment.local_id === localId ? {
          ...attachment,
          id: uploaded.attachmentId,
          status: 'ready',
        } : attachment));
      } catch (error) {
        setMediaAttachments((current) => current.map((attachment) => attachment.local_id === localId ? {
          ...attachment,
          status: 'failed',
        } : attachment));
        toast.error(error instanceof Error ? error.message : 'Failed to upload file');
      }
    }
  }, [workspaceId]);

  const removeMediaAttachment = useCallback((attachment: DockChatMediaAttachment) => {
    setMediaAttachments((current) => current.filter((candidate) => candidate.local_id !== attachment.local_id));
    if (attachment.preview_url) URL.revokeObjectURL(attachment.preview_url);
    if (attachment.id) void pmAttachmentService.remove(workspaceId, attachment.id, { pendingOnly: true });
  }, [workspaceId]);

  const canPause = run?.status === 'queued' || run?.status === 'running';
  const canResume = run?.status === 'paused' && run.pause_reason === 'manual';
  const canStop = runActive && (canPause || canResume);
  const pausePending = pausing || run?.execution_stage === 'pausing';
  const resumePending = resuming || run?.execution_stage === 'resuming';
  const cancellationPending = stopping || run?.execution_stage === 'cancelling';
  const handlePause = useCallback(async () => {
    if (!chatId || pausePending) return;
    setPausing(true);
    try {
      const res = await dockChatService.pauseChatRun(workspaceId, chatId);
      if (res.error) { toast.error(res.error); return; }
      if (res.data) setDetail((current) => current ? { ...current, run: res.data } : current);
      await Promise.all([refreshDetail(), refetch()]);
    } finally {
      setPausing(false);
    }
  }, [chatId, pausePending, refetch, refreshDetail, workspaceId]);

  const handleResume = useCallback(async () => {
    if (!chatId || resumePending) return;
    setResuming(true);
    try {
      const res = await dockChatService.resumeChatRun(workspaceId, chatId);
      if (res.error) { toast.error(res.error); return; }
      if (res.data) setDetail((current) => current ? { ...current, run: res.data } : current);
      await Promise.all([refreshDetail(), refetch()]);
    } finally {
      setResuming(false);
    }
  }, [chatId, resumePending, refetch, refreshDetail, workspaceId]);
  const handleStop = useCallback(async () => {
    if (!chatId || cancellationPending) return;
    setStopping(true);
    try {
      const res = await dockChatService.cancelChatRun(workspaceId, chatId);
      if (res.error) {
        toast.error(res.error);
        return;
      }
      if (res.data) {
        setDetail((current) => current ? { ...current, run: res.data } : current);
      }
      await Promise.all([refreshDetail(), refetch()]);
    } finally {
      setStopping(false);
    }
  }, [cancellationPending, chatId, refetch, refreshDetail, workspaceId]);

  const resolveInteraction = useCallback(
    async (interactionId: string, payload: { response_payload: Record<string, unknown>; followup_message?: string }) => {
      if (!chatId) return { error: 'Chat is not ready' };
      const res = await dockChatService.resolveInteraction(workspaceId, chatId, interactionId, payload);
      if (!res.error) {
        clearPendingInteraction(interactionId);
        setFallbackInteraction(null);
        onChatChanged?.();
        void refreshDetail();
        void refetch();
      }
      return { error: res.error };
    },
    [chatId, clearPendingInteraction, onChatChanged, refetch, refreshDetail, workspaceId],
  );

  const activeSubAgentName = useMemo(() => {
    for (const plan of plans) {
      for (const [stepIndex, runId] of Object.entries(plan.run_ids_by_step ?? {})) {
        const childRun = plan.runs?.find((candidate) => candidate.id === runId);
        if (!childRun || !ACTIVE_RUN_STATUSES.has(childRun.status)) continue;
        return plan.steps[Number(stepIndex)]?.agent_name?.trim() || 'another agent';
      }
    }
    return null;
  }, [plans]);

  const liveProgress = useMemo(() => resolveAgentLiveProgress({
    run,
    stream: transformed?.stream ?? null,
    currentPlan,
    activeSubAgentName,
    sending: sending || !!pendingEcho,
    localStartedAt: launchStartedAt,
  }), [activeSubAgentName, currentPlan, launchStartedAt, run, sending, transformed, pendingEcho]);

  const displayedLiveProgress = analyzingMedia ? {
    label: analyzingMediaLabel || 'Analyzing attachment…',
    tone: 'working' as const,
    startedAt: launchStartedAt ?? new Date().toISOString(),
    completed: false,
  } : answerRecoveryKey && recoveredAnswerKey === answerRecoveryKey && visibleTurn.answerPending
    ? { label: 'Could not load the final answer. Reload to retry.', tone: 'waiting' as const }
    : liveProgress;

  const followUpSuggestions = useMemo(() => {
    if (sending || pendingEcho || analyzingMedia || effectiveInteraction) return [];
    // Parse the original answer; the chat display stream has its hints removed.
    return chatFollowUpSuggestions(run, mergedStream?.transcript_messages ?? []);
  }, [run, sending, pendingEcho, analyzingMedia, effectiveInteraction, mergedStream]);

  const hasTranscriptMessages = (transformed?.stream.transcript_messages ?? persistedMessages)
    .some((message) => message.content.trim());
  const executionPickerDisabled = resolveExecutionPickerDisabled({
    changingExecution,
    sending,
    pendingEcho: Boolean(pendingEcho),
    runStatus: run?.status,
  });

  const starterSuggestions = !hasTranscriptMessages && !value.trim() && !sending && !pendingEcho
    ? starterSuggestionsForContext(effectivePageContext?.entity_type)
    : [];

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

  const runtimeStream = transformed?.stream ?? streamState;
  const showRuntimeTimeline = isDockTranscriptStreaming(run)
    || (run?.status !== 'cancelled' && runtimeStream !== null && hasAuthoritativeDockRuntimeTimeline(runtimeStream));

  return (
    <DockInteractionLayer
      active={active}
      interactionId={effectiveInteraction?.interaction_id}
      prompt={effectiveInteraction && (dockConfirm ? (
        <DockPlanConfirmCard
          payload={dockConfirm}
          onDecision={(decision, note) => resolveInteraction(effectiveInteraction.interaction_id, {
            response_payload: { decision }, followup_message: note,
          })}
        />
      ) : run ? (
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
      ) : null)}
    >
      <div className="relative flex min-h-0 flex-1 flex-col">
      <div ref={scrollRef} data-agent-dock-chat-scroll className={`${activityStyles.activityHost} min-h-0 flex-1 overflow-y-auto px-5 pb-24 pt-3 sm:px-6`}>
      <div className="space-y-3">
		{nextMessagesBefore && (
		  <div className="flex justify-center">
		    <Button
		      type="button"
		      variant="outline"
		      size="xs"
		      className="text-muted-foreground shadow-sm hover:text-foreground"
		      disabled={loadingEarlier}
		      onClick={() => void loadEarlierMessages()}
		    >
		      {loadingEarlier
		        ? <Loading01Icon className="animate-spin" aria-hidden="true" />
		        : <ArrowUp01Icon aria-hidden="true" />}
		      {loadingEarlier ? 'Loading…' : 'Load earlier messages'}
		    </Button>
		  </div>
		)}
        {detailLoading && !detail && !hasTranscriptMessages && !pendingEcho && !sending && (
          <p className="py-6 text-center text-sm text-muted-foreground">Loading chat…</p>
        )}
        {refreshError && (
          <div className="mx-3 my-2 flex items-center justify-between gap-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
            <span>Couldn’t refresh this conversation</span>
            <button type="button" className="font-semibold hover:underline" onClick={() => void refreshConversation()}>Retry</button>
          </div>
        )}
        {!detailLoading && !run && !pendingEcho && !sending && !hasTranscriptMessages && (
          <p className="py-6 text-center text-sm text-muted-foreground">
            {requiredPageContext?.entity_type === 'support_conversation'
              ? 'Ask about this conversation, draft a reply, investigate the issue, or have an agent take the next step.'
              : 'Ask a question about your workspace, or describe work for an agent to do.'}
          </p>
        )}
        {transformed && (
          <DockTranscript
            stream={transformed.stream}
            latestSubmission={latestSubmission}
            active={isDockTranscriptStreaming(run)}
            runStatus={run?.status}
            pauseReason={run?.pause_reason}
            useRuntimeTimeline={showRuntimeTimeline}
            workspaceId={workspaceId}
            chatId={chatId}
            fallbackActor={streamController.session?.triggered_by_user}
            savedWorkPlans={detail?.work_plans}
            subAgentRuns={subAgentTimelineItems}
            compactAssistantProgress
          />
        )}
        <DockArtifactDownloads workspaceId={workspaceId} artifacts={detail?.artifacts ?? []} />
        {followUpSuggestions.length > 0 && (
          <div className="mt-2 border-t border-border/40 pt-1" data-agent-follow-up-suggestions>
            {followUpSuggestions.map((suggestion) => (
              <Tooltip key={suggestion}>
                <TooltipTrigger asChild>
                  <button
                    type="button"
                    className="flex w-full min-w-0 items-center gap-2 rounded-md px-1 py-2 text-left text-sm text-muted-foreground transition-colors hover:bg-muted/50 hover:text-foreground"
                    onClick={() => {
                      insertSuggestion(suggestion);
                    }}
                  >
                    <ArrowRight01Icon className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                    <span className="min-w-0 truncate">{suggestion}</span>
                  </button>
                </TooltipTrigger>
                <TooltipContent side="top" align="start" className="max-w-[min(28rem,calc(100vw-2rem))] whitespace-normal break-words text-left leading-relaxed">
                  {suggestion}
                </TooltipContent>
              </Tooltip>
            ))}
          </div>
        )}

        {visibleSendError && (
          <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs">
            <p className="mb-1 line-clamp-2 text-foreground/80">{visibleSendError.content}</p>
            <div className="flex items-center justify-between gap-2">
              <span className="min-w-0 truncate text-destructive">{visibleSendError.message}</span>
              <div className="flex shrink-0 items-center gap-3">
                <button
                  type="button"
                  className="font-medium text-foreground hover:underline"
                  onClick={() => void sendContent(visibleSendError.content, visibleSendError.references, visibleSendError.clientMessageId)}
                >
                  Retry
                </button>
                <button
                  type="button"
                  className="text-muted-foreground hover:underline"
                  onClick={() => {
                    setValue(visibleSendError.content);
                    setReferences(visibleSendError.references);
                    setSendError(null);
                  }}
                >
                  Edit message
                </button>
              </div>
            </div>
          </div>
        )}
        {displayedLiveProgress ? (
          <div
            className="mt-2 shrink-0 border-t border-border/40 px-1 pt-2"
            data-agent-live-status-region
            data-working={displayedLiveProgress.tone === 'working'}
          >
            <AgentLiveStatus progress={displayedLiveProgress} />
          </div>
        ) : null}
      </div>
      </div>
      {!atBottom && <ScrollToLatestButton onClick={scrollToLatest} />}
      </div>
      {currentPlan && (!hasWorkPlanOrigin(currentPlan) || !mergedStream || !dockWorkPlans(mergedStream).some(plan => plan.origin?.event_id === currentPlan.origin?.event_id)) && <div className="max-h-48 shrink-0 overflow-y-auto px-5" data-current-work-plan><CodingPlanPanel plan={currentPlan} runStatus={run?.status} title="Current work plan" defaultOpen={false} /></div>}
      {run?.status === 'paused' && run.pause_reason === 'authentication' && workspaceSlug && (
        <div className="px-3.5 py-2"><AISettingsLink slug={workspaceSlug} className="text-xs underline text-quiet-text-secondary">Review AI access to continue</AISettingsLink></div>
      )}
      {composer.visible && (
        <div className="border-t border-border/60">
          {starterSuggestions.length > 0 && composer.enabled && (
            <div className="px-3.5 pt-2" data-agent-starter-suggestions>
              <div className="flex flex-wrap gap-1.5">
                {starterSuggestions.map((suggestion) => (
                  <button
                    key={suggestion.label}
                    type="button"
                    className="inline-flex max-w-full items-center gap-1.5 rounded-full border border-border/70 px-2.5 py-1 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                    onClick={() => {
                      insertSuggestion(suggestion.prompt);
                    }}
                  >
                    <ArrowRight01Icon className="h-3 w-3 shrink-0" aria-hidden="true" />
                    <span className="truncate">{suggestion.label}</span>
                  </button>
                ))}
              </div>
            </div>
          )}
          <div className="p-2">
              <DockInput
                mode="conversation"
                executionPicker={(!chatId || (detail && detail.chat.user_id === currentUserId)) ? (
                  <DockExecutionPicker
                    enabled={Boolean(executionEnabled)}
                    disabled={executionPickerDisabled}
                    onChange={changeExecution}
                  />
                ) : null}
                profilePicker={run ? (
                  acceptedProfileId ? <AIConnectionPicker workspaceId={workspaceId} inDock compact locked value={{ ai_profile_id: acceptedProfileId }} onChange={() => {}} />
                    : <span className="text-xs text-quiet-text-secondary" title="This conversation keeps its saved AI configuration">Saved profile</span>
                ) : !aiConnection.ai_profile_id && agentDefaults.isPending ? (
                  <span role="status" className="text-xs text-quiet-text-secondary">Loading…</span>
                ) : !aiConnection.ai_profile_id && agentDefaults.isError ? (
                  <button type="button" className="text-xs text-quiet-text-secondary hover:text-foreground" title="Retry loading the agent’s default profile" onClick={() => void agentDefaults.refetch()}>Retry default</button>
                ) : <AIConnectionPicker workspaceId={workspaceId} inDock compact defaultProfileId={askAgentDefault} value={aiConnection} onChange={setAIConnection} disabled={sending} />}
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
                onClearContext={() => setContextCleared(true)}
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
                mediaAttachments={mediaAttachments}
                onAddMedia={(files) => void addMediaAttachments(files)}
                onRemoveMedia={removeMediaAttachment}
                busy={sending}
                disabled={!composer.enabled}
                autoFocus
                textareaRef={textareaRef}
                onStop={canStop ? () => void handleStop() : undefined}
                stopping={cancellationPending}
                onPause={canPause && !cancellationPending ? () => void handlePause() : undefined}
                pausing={pausePending}
                onResume={canResume && !cancellationPending ? () => void handleResume() : undefined}
                resuming={resumePending}
                placeholder={cancellationPending ? 'Stopping agent…' : pausePending ? 'Pausing agent…' : resumePending ? 'Resuming agent…' : undefined}
                showShortcutHint={showComposerShortcutHint}
              />
          </div>
        </div>
      )}
    </DockInteractionLayer>
  );
}
