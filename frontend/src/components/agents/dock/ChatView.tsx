import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { ArrowRight01Icon, ArrowUp01Icon, Loading01Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { usePageContextState } from '@/components/command-bar/pageContext';
import { commandBarService } from '@/lib/services/commandBarService';
import { dockChatService } from '@/lib/services/dockChatService';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { uploadEditorFile } from '@/hooks/useEditorImageUpload';
import { parseDockPlanConfirm } from '@/lib/dockTypes';
import type { DockChatDetail, DockChatMediaAttachment, DockEntityReference } from '@/lib/dockTypes';
import type { AgentRun, AgentRunMessage, CodingSessionInteraction, CommandBarPageContext, CommandBarPlanSummary } from '@/lib/pmTypes';
import { DockInput } from './DockInput';
import { DockTranscript } from './DockTranscript';
import { DockUserMessage } from './DockUserMessage';
import { DockPlanConfirmCard } from './DockPlanConfirmCard';
import { ExecutionStrip } from './ExecutionStrip';
import { PendingInteractionCard } from './PendingInteractionCard';
import { ApprovalAttentionBanner } from './ApprovalAttentionBanner';
import { CodingPlanPanel } from '@/components/pm/CodingSession/CodingPlanPanel';
import type { AskAgentAvatarState } from '@/components/agents/AskAgentAvatar';
import { deriveAskAgentAvatarState } from '@/components/agents/askAgentPresence';
import { ScrollToLatestButton } from '@/components/agents/transcript';
import { AgentLiveStatus } from './AgentLiveStatus';
import { resolveAgentLiveProgress } from './agentProgress';
import { parseFollowUpSuggestions } from './followUpSuggestions';
import { starterSuggestionsForContext } from './starterSuggestions';
import { focusComposerAtEnd } from './composerFocus';
import { planSummaryToRunPlan } from './planSummary';
import type { AgentRunStreamState } from './useAgentRunStream';
import {
  hasAuthoritativeDockRuntimeTimeline,
  mergeMessagePages,
  mergePersistedChatMessages,
  resolveVisiblePendingEcho,
} from './dockChatTimeline';
import {
  isDockTranscriptStreaming,
  isStructuredInteractionKind,
  resolveDockComposerState,
  transformDockStream,
} from './dockChatState';
import { useDockStore } from '@/stores/dockStore';

interface ChatViewProps {
  workspaceId: string;
  chatId?: string;
  onCreateChat?: () => Promise<{ id: string } | null>;
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
  onCreateChat,
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
  const cachedTranscript = chatId ? useDockStore.getState().transcripts[chatId] : undefined;
  const cacheTranscript = useDockStore((state) => state.cacheTranscript);
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
  const [pendingEcho, setPendingEcho] = useState<{ id: string; content: string; timestamp: string } | null>(null);
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

  const run = detail?.run ?? null;
  const runActive = !!run && ACTIVE_RUN_STATUSES.has(run.status);
  useEffect(() => {
    onRunIdChange?.(run?.id ?? null);
  }, [onRunIdChange, run?.id]);
  useEffect(() => {
    onRunStatusChange?.(run?.id ?? null, run?.status ?? null);
  }, [onRunStatusChange, run?.id, run?.status]);

  const { currentPlan, streamState, pendingInteraction, refetch, clearPendingInteraction } =
    streamController;

  const acceptedSendVersion = useRef(0);
  const refreshDetail = useCallback(async () => {
	if (!chatId) return null;
    const version = acceptedSendVersion.current;
    const res = await dockChatService.getChat(workspaceId, chatId);
    // A fetch started before send acceptance must not erase the new run.
    if (version !== acceptedSendVersion.current) return null;
    if (res.error || !res.data) {
      setRefreshError(res.error ?? 'Unable to refresh conversation');
      return null;
    }
    if (res.data) setDetail(res.data);
    return res.data ?? null;
  }, [chatId, workspaceId]);

  const refreshMessages = useCallback(async () => {
	if (!chatId) return [];
    const res = await dockChatService.listMessages(workspaceId, chatId, undefined, 50);
    if (res.error || !res.data) {
      setRefreshError(res.error ?? 'Unable to refresh conversation');
      return [];
    }
    if (res.data) {
      setPersistedMessages((current) => mergeMessagePages(current, res.data?.messages ?? []));
      setNextMessagesBefore(res.data.next_before ?? null);
    }
    return res.data?.messages ?? [];
  }, [chatId, workspaceId]);

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
  const refreshConversation = useCallback(async () => {
    if (!chatId) return;
    setRefreshError(null);
    if (!useDockStore.getState().transcripts[chatId]) setDetailLoading(true);
    await Promise.all([refreshDetail(), refreshMessages()]);
    setDetailLoading(false);
  }, [chatId, refreshDetail, refreshMessages]);

  useEffect(() => {
	if (!chatId) {
	  setDetail(null);
	  setPersistedMessages([]);
	  setNextMessagesBefore(null);
	  setDetailLoading(false);
	  return;
	}
    autoFollowRef.current = true;
    const timer = window.setTimeout(() => {
      void refreshConversation();
    }, 0);
    return () => window.clearTimeout(timer);
	}, [chatId, refreshConversation]);

  useEffect(() => {
    if (!chatId || !detail || detailLoading) return;
    cacheTranscript(chatId, { detail, messages: persistedMessages, nextBefore: nextMessagesBefore });
  }, [cacheTranscript, chatId, detail, detailLoading, nextMessagesBefore, persistedMessages]);

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
  const visiblePendingEcho = useMemo(
    () => resolveVisiblePendingEcho(pendingEcho, persistedMessages),
    [pendingEcho, persistedMessages],
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
    !!chatId && run?.status === 'paused' && (run.pause_reason === 'human_approval' || run.pause_reason === 'human_input');
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
  }, [transformed, currentPlan, visiblePendingEcho, pendingInteraction, sendError]);

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
      const attachmentIDs = mediaAttachments.filter((attachment) => attachment.status === 'ready' && attachment.id).map((attachment) => attachment.id!);
      const hasSourceContext = !!effectivePageContext || messageReferences.length > 0;
      setAnalyzingMedia(attachmentIDs.length > 0 || hasSourceContext);
      setAnalyzingMediaLabel(
        attachmentIDs.length === 1
          ? `Analyzing ${mediaAttachments.find((attachment) => attachment.id === attachmentIDs[0])?.file_name ?? 'attachment'}…`
          : attachmentIDs.length > 1
            ? `Analyzing ${attachmentIDs.length} attachments…`
            : 'Checking context attachments…',
      );
      const sentAt = new Date().toISOString();
      setLaunchStartedAt(sentAt);
      setSendError(null);
      setPendingEcho({ id: clientMessageId, content, timestamp: sentAt });
      autoFollowRef.current = true;
      setAtBottom(true);
      try {
        let targetChatId = chatId;
        if (!targetChatId) {
          const created = await onCreateChat?.();
          if (!created) throw new Error('Unable to create chat. Please retry.');
          targetChatId = created.id;
        }
		const res = await dockChatService.sendMessage(workspaceId, targetChatId, {
          client_message_id: clientMessageId,
          content,
          page_context: effectivePageContext ?? undefined,
          references: messageReferences.length > 0 ? messageReferences : undefined,
          attachment_ids: attachmentIDs.length > 0 ? attachmentIDs : undefined,
        });
        if (res.error || !res.data) {
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
		}
        setReferences([]);
		setMediaAttachments((current) => {
		  current.forEach((attachment) => {
			if (attachment.preview_url) URL.revokeObjectURL(attachment.preview_url);
		  });
		  return [];
		});
        acceptedSendVersion.current += 1;
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
    [chatId, detail?.chat.title, effectivePageContext, mediaAttachments, onChatChanged, onCreateChat, references, refetch, refreshMessages, run?.id, sending, workspaceId],
  );

  const submit = async () => {
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
  }, [mediaAttachments.length, workspaceId]);

  const removeMediaAttachment = useCallback((attachment: DockChatMediaAttachment) => {
    setMediaAttachments((current) => current.filter((candidate) => candidate.local_id !== attachment.local_id));
    if (attachment.preview_url) URL.revokeObjectURL(attachment.preview_url);
    if (attachment.id) void pmAttachmentService.remove(workspaceId, attachment.id, { pendingOnly: true });
  }, [workspaceId]);

  const canStop = runActive && (run?.status === 'queued' || run?.status === 'running');
  const cancellationPending = stopping || run?.execution_stage === 'cancelling';
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
        void refreshDetail();
        void refetch();
      }
      return { error: res.error };
    },
    [chatId, clearPendingInteraction, refetch, refreshDetail, workspaceId],
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
    sending: sending || !!visiblePendingEcho,
    localStartedAt: launchStartedAt,
  }), [activeSubAgentName, currentPlan, launchStartedAt, run, sending, transformed, visiblePendingEcho]);

  const displayedLiveProgress = analyzingMedia ? {
    label: analyzingMediaLabel || 'Analyzing attachment…',
    tone: 'working' as const,
    startedAt: launchStartedAt ?? new Date().toISOString(),
    completed: false,
  } : liveProgress;

  const followUpSuggestions = useMemo(() => {
    if (run?.status !== 'completed' || sending || visiblePendingEcho) return [];
    const finalMessage = [...(transformed?.stream.transcript_messages ?? [])]
      .reverse()
      .find((message) => message.role === 'assistant' && message.content.trim());
    return finalMessage ? parseFollowUpSuggestions(finalMessage.content) : [];
  }, [run?.status, sending, transformed, visiblePendingEcho]);

  const hasTranscriptMessages = (transformed?.stream.transcript_messages ?? persistedMessages)
    .some((message) => message.content.trim());
  const starterSuggestions = !hasTranscriptMessages && !value.trim() && !sending && !visiblePendingEcho
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

  const needsApproval = (
    (run?.status === 'paused' && run.pause_reason === 'human_approval')
    || effectiveInteraction?.interaction_kind.includes('approval') === true
    || Object.values(runsById).some(
      (childRun) => childRun.status === 'paused' && childRun.pause_reason === 'human_approval',
    )
  );
  const runtimeStream = transformed?.stream ?? streamState;
  const showRuntimeTimeline = isDockTranscriptStreaming(run)
    || (run?.status !== 'cancelled' && runtimeStream !== null && hasAuthoritativeDockRuntimeTimeline(runtimeStream));

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="relative flex min-h-0 flex-1 flex-col">
      <div ref={scrollRef} data-agent-dock-chat-scroll className="min-h-0 flex-1 space-y-3 overflow-y-auto px-4 pb-24 pt-3">
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
        {detailLoading && !detail && (
          <p className="py-6 text-center text-sm text-muted-foreground">Loading chat…</p>
        )}
        {refreshError && (
          <div className="mx-3 my-2 flex items-center justify-between gap-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
            <span>Couldn’t refresh this conversation</span>
            <button type="button" className="font-semibold hover:underline" onClick={() => void refreshConversation()}>Retry</button>
          </div>
        )}
        {!detailLoading && !run && !visiblePendingEcho && (
          <p className="py-6 text-center text-sm text-muted-foreground">
            {requiredPageContext?.entity_type === 'support_conversation'
              ? 'Ask about this conversation, draft a reply, investigate the issue, or have an agent take the next step.'
              : 'Ask a question about your workspace, or describe work for an agent to do.'}
          </p>
        )}
        {transformed && (
          <DockTranscript
            stream={transformed.stream}
            active={isDockTranscriptStreaming(run)}
            useRuntimeTimeline={showRuntimeTimeline}
            workspaceId={workspaceId}
            chatId={chatId}
            fallbackActor={streamController.session?.triggered_by_user}
            subAgentRuns={subAgentTimelineItems}
            compactAssistantProgress
          />
        )}
        {followUpSuggestions.length > 0 && (
          <div className="mt-2 border-t border-border/40 pt-1" data-agent-follow-up-suggestions>
            {followUpSuggestions.map((suggestion) => (
              <button
                key={suggestion}
                type="button"
                className="flex w-full items-center gap-2 rounded-md px-1 py-2 text-left text-sm text-muted-foreground transition-colors hover:bg-muted/50 hover:text-foreground"
                onClick={() => {
                  insertSuggestion(suggestion);
                }}
              >
                <ArrowRight01Icon className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                <span className="min-w-0 truncate">{suggestion}</span>
              </button>
            ))}
          </div>
        )}
        {currentPlan && (
          <CodingPlanPanel plan={currentPlan} runStatus={run?.status} title="Work plan" />
        )}
        {visiblePendingEcho && <DockUserMessage content={visiblePendingEcho.content} timestamp={visiblePendingEcho.timestamp} pending />}
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
        {displayedLiveProgress ? (
          <div
            className="mt-2 shrink-0 border-t border-border/40 px-1 pt-2"
            data-agent-live-status-region
          >
            <AgentLiveStatus progress={displayedLiveProgress} />
          </div>
        ) : null}
      </div>
      {!atBottom && <ScrollToLatestButton onClick={scrollToLatest} />}
      </div>
      {needsApproval && !atBottom ? (
        <ApprovalAttentionBanner onReview={scrollToLatest} />
      ) : null}
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
                placeholder={cancellationPending ? 'Stopping agent…' : undefined}
                showShortcutHint={showComposerShortcutHint}
              />
          </div>
        </div>
      )}
    </div>
  );
}
