import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { UnicodeSpinner } from '@/components/pm/CodingSession/UnicodeSpinner';
import {
  ArrowUp02Icon,
  Loading01Icon,
  LockKeyIcon,
} from '@/lib/icons';

import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { isToolName } from '@/lib/toolNames';
import type {
  AgentRunArtifact,
  CodingSession,
  CodingSessionActor,
  CodingSessionInteraction,
  CodingSessionLiveAssistantMessage,
  CodingSessionLiveReasoningMessage,
  CodingSessionLiveTurnSegment,
  CodingSessionTranscriptMessage,
} from '@/lib/pmTypes';
import { useWorkspaceMembers } from '@/hooks/queries';
import { formatCodingSessionElapsed } from './codingSessionPresentation';
import type { CodingSessionComposerState } from './codingSessionComposer';
import type { PublishedPreview } from '@/components/pm/runPreviews';
import { CodingInteractionCard } from './CodingInteractionCard';
import { CodingReviewHistoryPanel, type CodingReviewHistoryItem } from './CodingReviewHistoryPanel';
import {
  ALL_SEGMENT_KINDS,
  collectSegments,
  TranscriptSegmentView,
  type TranscriptSegment,
} from '@/components/agents/transcript';

export function CodingTranscriptPane({
  promptArtifact,
  reviewArtifacts = [],
  transcriptMessages,
  liveAssistantMessage,
  liveReasoningMessage,
  liveTurnSegments,
  loading = false,
  onSendMessage,
  sendingMessage = false,
  messagePlaceholder = 'Reply to agent… (⌘↵ to send)',
  messageComposer,
  session,
  activeInteraction,
  acting,
  availablePreviewPanelKey,
  attachedPreview,
  onViewPreview,
  onAuthStart,
  onAuthCancel,
  onResolveInteraction,
}: {
  promptArtifact?: AgentRunArtifact | null;
  reviewArtifacts?: CodingReviewHistoryItem[];
  transcriptMessages: CodingSessionTranscriptMessage[];
  liveAssistantMessage: CodingSessionLiveAssistantMessage | null;
  liveReasoningMessage: CodingSessionLiveReasoningMessage | null;
  liveTurnSegments: CodingSessionLiveTurnSegment[];
  loading?: boolean;
  onSendMessage?: (content: string) => Promise<void>;
  sendingMessage?: boolean;
  messagePlaceholder?: string;
  messageComposer?: CodingSessionComposerState;
  session?: CodingSession | null;
  activeInteraction?: CodingSessionInteraction | null;
  acting?: string | null;
  availablePreviewPanelKey?: string | null;
  attachedPreview?: PublishedPreview | null;
  onViewPreview?: (panelKey: string) => void;
  onAuthStart?: () => void;
  onAuthCancel?: () => void;
  onResolveInteraction?: (interactionId: string, responsePayload: Record<string, unknown>, followupMessage?: string) => void;
}) {
  const scrollContainerRef = useRef<HTMLDivElement | null>(null);
  const autoFollowRef = useRef(true);
  const resolvedMessageComposer: CodingSessionComposerState = messageComposer ?? {
    visible: Boolean(onSendMessage),
    enabled: Boolean(onSendMessage),
    mode: 'answer',
    placeholder: messagePlaceholder,
  };
  const visibleLiveSegments = liveTurnSegments.filter((segment) => {
    if (segment.kind === 'assistant_message') {
      return segment.assistant_message.content.trim().length > 0;
    }
    return !isToolName(segment.tool_call.tool_name, 'update_plan');
  });
  const showLivePlaceholder = visibleLiveSegments.length === 0 && liveAssistantMessage?.status === 'streaming';
  const promptMessage = useMemo<CodingSessionTranscriptMessage | null>(() => {
    const sections = parsePromptArtifactSections(promptArtifact?.inline_content);
    const developerPrompt = sections.find((section) => section.label === 'Developer prompt');
    if (developerPrompt) {
      return {
        event_id: `prompt:${promptArtifact?.id ?? 'developer'}`,
        message_id: `prompt:${promptArtifact?.id ?? 'developer'}`,
        role: 'user',
        message_type: 'developer_prompt',
        content: developerPrompt.content,
        timestamp: promptArtifact?.created_at ?? new Date().toISOString(),
        sequence_no: Number.MIN_SAFE_INTEGER,
      };
    }

    const systemPrompt = session?.system_prompt?.trim();
    if (!systemPrompt) return null;
    return {
      event_id: `prompt:${session?.run_id ?? 'system'}`,
      message_id: `prompt:${session?.run_id ?? 'system'}`,
      role: 'user',
      message_type: 'system_prompt',
      content: systemPrompt,
      timestamp: session?.created_at ?? new Date().toISOString(),
      sequence_no: Number.MIN_SAFE_INTEGER,
    };
  }, [promptArtifact, session?.created_at, session?.run_id, session?.system_prompt]);

  // Flatten the reconciled stream into one ordered segment list shared with the
  // Ask Agents dock. The slider shows every kind and renders rows expandable.
  const segments = useMemo(
    () => collectSegments(
      {
        transcript_messages: transcriptMessages,
        live_turn_segments: liveTurnSegments,
        live_reasoning_message: liveReasoningMessage,
      },
      { includeLive: true, include: ALL_SEGMENT_KINDS, leadingContext: promptMessage },
    ),
    [transcriptMessages, liveTurnSegments, liveReasoningMessage, promptMessage],
  );

  // Build a flat list of virtual items: transcript segments plus the local
  // scroll affordances (live placeholder, running row, empty state, spacer).
  type VirtualItem =
    | { kind: 'segment'; segment: TranscriptSegment }
    | { kind: 'placeholder' }
    | { kind: 'running'; since: string }
    | { kind: 'empty' }
    | { kind: 'bottom-spacer' };

  const items = useMemo((): VirtualItem[] => {
    const list: VirtualItem[] = [];
    for (const segment of segments) {
      list.push({ kind: 'segment', segment });
    }
    if (showLivePlaceholder) {
      list.push({ kind: 'placeholder' });
    }
    const runningSince = session?.started_at ?? session?.created_at;
    if (session?.status === 'running' && runningSince) {
      list.push({ kind: 'running', since: runningSince });
    }
    if (!loading && list.length === 0) {
      list.push({ kind: 'empty' });
    }
    if (list.length > 0) {
      list.push({ kind: 'bottom-spacer' });
    }
    return list;
  }, [
    segments,
    showLivePlaceholder,
    session?.status,
    session?.started_at,
    session?.created_at,
    loading,
  ]);

  const virtualizer = useVirtualizer({
    count: items.length,
    getScrollElement: () => scrollContainerRef.current,
    estimateSize: () => 120,
    overscan: 8,
  });

  useEffect(() => {
    const container = scrollContainerRef.current;
    if (!container) return undefined;

    const updateAutoFollow = () => {
      const distanceFromBottom = container.scrollHeight - container.scrollTop - container.clientHeight;
      autoFollowRef.current = distanceFromBottom < 96;
    };

    updateAutoFollow();
    container.addEventListener('scroll', updateAutoFollow, { passive: true });
    return () => {
      container.removeEventListener('scroll', updateAutoFollow);
    };
  }, []);

  const scrollToTail = useCallback(() => {
    if (items.length === 0) return;
    requestAnimationFrame(() => {
      virtualizer.scrollToIndex(items.length - 1, {
        align: 'end',
        behavior: 'auto',
      });
    });
  }, [items.length, virtualizer]);

  const sessionScrollKey = session?.run_id ?? session?.id ?? 'unbound';
  const initialTailSessionKeyRef = useRef<string | null>(null);

  // When opening an existing run, land on the latest activity once. After that,
  // only auto-follow if the user stays near the tail.
  useEffect(() => {
    if (loading || items.length === 0) return;
    if (initialTailSessionKeyRef.current === sessionScrollKey) return;
    initialTailSessionKeyRef.current = sessionScrollKey;
    autoFollowRef.current = true;
    scrollToTail();
  }, [items.length, loading, scrollToTail, sessionScrollKey]);

  // Auto-scroll to bottom when new items arrive, but only if the user is already following the tail.
  const prevItemCountRef = useRef(items.length);
  useEffect(() => {
    if (items.length === 0) return;
    const isNewItem = items.length !== prevItemCountRef.current;
    const shouldFollow = autoFollowRef.current || prevItemCountRef.current === 0;
    prevItemCountRef.current = items.length;
    if (!isNewItem || !shouldFollow) return;
    scrollToTail();
  }, [items.length, scrollToTail]);

  // Also follow streaming content changes, but only while the user remains pinned near the bottom.
  const streamingSignature = useMemo(() => {
    const lastSegment = visibleLiveSegments[visibleLiveSegments.length - 1];
    return [
      liveAssistantMessage?.content.length ?? 0,
      liveReasoningMessage?.content.length ?? 0,
      lastSegment?.kind === 'assistant_message' ? lastSegment.assistant_message.content.length : 0,
      lastSegment?.kind === 'tool_call' ? (lastSegment.tool_call.result?.content.length ?? 0) : 0,
    ].join('|');
  }, [liveAssistantMessage, liveReasoningMessage, visibleLiveSegments]);

  useEffect(() => {
    if (items.length === 0 || !autoFollowRef.current) return;
    scrollToTail();
  }, [streamingSignature, items.length, scrollToTail]);

  const triggeredBy = session?.triggered_by_user ?? null;
  const { data: workspaceMembers } = useWorkspaceMembers(session?.workspace_id ?? '');

  const memberActorByUserId = useMemo(() => {
    const map = new Map<string, CodingSessionActor>();
    for (const member of workspaceMembers ?? []) {
      if (!member.user_id) continue;
      map.set(member.user_id, {
        id: member.user_id,
        email: member.email,
        full_name: member.full_name,
        avatar_url: member.avatar_url,
      });
    }
    return map;
  }, [workspaceMembers]);

  const actorForMessage = useCallback(
    (message: CodingSessionTranscriptMessage): CodingSessionActor | null => {
      const isResolution =
        message.message_type === 'review_checkpoint_resolution' ||
        message.message_type === 'approval_request_resolution';
      if (isResolution && message.resolver_user_id) {
        const resolver = memberActorByUserId.get(message.resolver_user_id);
        if (resolver) return resolver;
      }
      return triggeredBy;
    },
    [memberActorByUserId, triggeredBy],
  );

  const renderItem = useCallback((item: VirtualItem) => {
    switch (item.kind) {
      case 'segment':
        return (
          <TranscriptSegmentView
            segment={item.segment}
            options={{ expandable: true, resolveActor: actorForMessage }}
          />
        );
      case 'placeholder':
        return (
          <div className="text-[13px] leading-6 text-muted-foreground" data-coding-session-live-placeholder>
            Preparing reply…
          </div>
        );
      case 'running':
        return <RunningActivityRow since={item.since} />;
      case 'empty':
        return (
          <div className="rounded-lg border border-dashed border-border px-5 py-8 text-center text-sm text-muted-foreground">
            No activity yet. Assistant and user-visible turns will appear here once the session starts talking.
          </div>
        );
      case 'bottom-spacer':
        return (
          <div
            aria-hidden="true"
            className="h-[calc(env(safe-area-inset-bottom)+4rem)]"
            data-coding-session-activity-spacer
          />
        );
    }
  }, [actorForMessage]);

  return (
    <section className="relative flex h-full min-h-[20rem] flex-col overflow-hidden rounded-xl border border-border bg-card shadow-sm xl:min-h-0">
      <div className="border-b border-border/60 px-4 py-2">
        <div className="text-sm font-medium leading-5 text-muted-foreground">
          Activity
        </div>
      </div>

      <div ref={scrollContainerRef} className="min-h-0 flex-1 overflow-auto pt-3">
        <div className="mx-auto flex w-full max-w-4xl flex-col gap-3 px-4">
          <div
            className="relative w-full"
            style={{ height: virtualizer.getTotalSize() }}
          >
          {virtualizer.getVirtualItems().map((virtualRow) => {
            const item = items[virtualRow.index];
            return (
              <div
                key={virtualRow.key}
                data-index={virtualRow.index}
                ref={virtualizer.measureElement}
                className="pb-2"
                style={{
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  right: 0,
                  transform: `translateY(${virtualRow.start}px)`,
                  paddingLeft: '1rem',
                  paddingRight: '1rem',
                }}
              >
                {renderItem(item)}
              </div>
            );
          })}
          </div>
        </div>
      </div>

      {(session?.pause_reason === 'authentication' || activeInteraction) ? (
        <InterruptionOverlay
          session={session ?? null}
          activeInteraction={activeInteraction ?? null}
          acting={acting ?? null}
          availablePreviewPanelKey={availablePreviewPanelKey ?? null}
          attachedPreview={attachedPreview ?? null}
          reviewArtifacts={reviewArtifacts}
          onViewPreview={onViewPreview}
          onAuthStart={onAuthStart ?? (() => {})}
          onAuthCancel={onAuthCancel ?? (() => {})}
          onResolveInteraction={onResolveInteraction ?? (() => {})}
        />
      ) : null}

      {resolvedMessageComposer.visible ? (
        <MessageInput
          onSend={onSendMessage}
          sending={sendingMessage}
          enabled={resolvedMessageComposer.enabled && Boolean(onSendMessage)}
          placeholder={resolvedMessageComposer.placeholder}
          disabledReason={resolvedMessageComposer.disabledReason}
        />
      ) : null}
    </section>
  );
}

function parsePromptArtifactSections(raw: string | null | undefined) {
  if (!raw) return [] as Array<{ label: string; content: string }>;

  const lines = raw.split('\n');
  const sections: Array<{ label: string; content: string }> = [];
  let currentLabel: string | null = null;
  let currentLines: string[] = [];

  const normalizeLabel = (line: string) => {
    switch (line) {
      case 'Developer instructions:':
      case 'Developer prompt:':
        return 'Developer prompt';
      case 'Turn input:':
      case 'User prompt:':
        return 'User prompt';
      case 'Pending request replay:':
        return 'Pending request replay';
      default:
        return line.slice(0, -1);
    }
  };

  const flush = () => {
    if (!currentLabel) return;
    const content = currentLines.join('\n').trim();
    if (content) {
      sections.push({ label: currentLabel, content });
    }
  };

  for (const line of lines) {
    if (
      line === 'Developer instructions:'
      || line === 'Developer prompt:'
      || line === 'Turn input:'
      || line === 'User prompt:'
      || line === 'Pending request replay:'
    ) {
      flush();
      currentLabel = normalizeLabel(line);
      currentLines = [];
      continue;
    }
    currentLines.push(line);
  }
  flush();
  return sections;
}

function InterruptionOverlay({
  session,
  activeInteraction,
  acting,
  availablePreviewPanelKey,
  attachedPreview,
  onViewPreview,
  reviewArtifacts,
  onAuthStart,
  onAuthCancel,
  onResolveInteraction,
}: {
  session: CodingSession | null;
  activeInteraction: CodingSessionInteraction | null;
  acting: string | null;
  availablePreviewPanelKey?: string | null;
  attachedPreview?: PublishedPreview | null;
  reviewArtifacts: CodingReviewHistoryItem[];
  onViewPreview?: (panelKey: string) => void;
  onAuthStart: () => void;
  onAuthCancel: () => void;
  onResolveInteraction: (interactionId: string, responsePayload: Record<string, unknown>, followupMessage?: string) => void;
}) {
  const authState = session?.auth_state;
  const hasDeviceCode = Boolean(authState?.verification_url || authState?.user_code);
  const hasBrowserAuth = Boolean(authState?.auth_url);
  const authDescription = hasDeviceCode
    ? 'Complete device sign-in to continue this session.'
    : hasBrowserAuth
      ? 'Continue sign-in in your browser. This Codex runtime returned browser-based auth instead of a device code.'
      : authState?.state === 'pending'
        ? 'Preparing sign-in. This can take a few seconds.'
        : 'Start sign-in to continue this session.';

  return (
    <div className="relative">
      {/* Stacked gradient-blur scrim — each layer covers a slice with increasing blur toward the bottom */}
      <div aria-hidden className="pointer-events-none absolute inset-x-0 bottom-full h-10" style={{ backdropFilter: 'blur(1px)', maskImage: 'linear-gradient(to bottom, transparent, black)', WebkitMaskImage: 'linear-gradient(to bottom, transparent, black)' }} />
      <div aria-hidden className="pointer-events-none absolute inset-x-0 bottom-full h-20" style={{ backdropFilter: 'blur(4px)', maskImage: 'linear-gradient(to bottom, transparent 0%, transparent 50%, black 100%)', WebkitMaskImage: 'linear-gradient(to bottom, transparent 0%, transparent 50%, black 100%)' }} />
      <div aria-hidden className="pointer-events-none absolute inset-x-0 bottom-full h-36" style={{ backdropFilter: 'blur(8px)', maskImage: 'linear-gradient(to bottom, transparent 0%, transparent 70%, black 100%)', WebkitMaskImage: 'linear-gradient(to bottom, transparent 0%, transparent 70%, black 100%)' }} />
      <div aria-hidden className="pointer-events-none absolute inset-x-0 bottom-full h-48" style={{ backdropFilter: 'blur(14px)', maskImage: 'linear-gradient(to bottom, transparent 0%, transparent 80%, black 100%)', WebkitMaskImage: 'linear-gradient(to bottom, transparent 0%, transparent 80%, black 100%)' }} />
      {/* Colour fade on top of the blur layers */}
      <div aria-hidden className="pointer-events-none absolute inset-x-0 bottom-full h-48 bg-gradient-to-t from-card to-transparent" />

      <div
        className="max-h-[70vh] overflow-y-auto border-t border-border/80 bg-card/95 px-4 py-4 backdrop-blur-md transition-shadow data-[review-flash=true]:ring-2 data-[review-flash=true]:ring-primary/35"
        data-coding-session-interruption-panel
      >
        <div className="space-y-4">
      {session?.pause_reason === 'authentication' ? (
        <div className="rounded-lg border border-amber-200/80 bg-amber-50 p-4 dark:border-amber-800/50 dark:bg-amber-950/20">
          <div className="mb-2 flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-amber-700 dark:text-amber-400">
            <LockKeyIcon className="h-3.5 w-3.5" />
            Authentication required
          </div>
          <div className="text-sm font-semibold">ChatGPT sign-in required</div>
          <p className="mt-1 text-sm text-muted-foreground">
            {authDescription}
          </p>
          {authState?.user_code ? (
            <div className="mt-3 rounded-lg border border-border bg-card px-3 py-2 font-mono text-sm tracking-widest">
              {authState.user_code}
            </div>
          ) : null}
          <div className="mt-3 flex flex-wrap gap-2">
            <Button size="sm" onClick={onAuthStart} disabled={acting !== null}>
              Start sign-in
            </Button>
            {authState?.verification_url ? (
              <Button asChild variant="outline" size="sm">
                <a href={authState.verification_url} target="_blank" rel="noreferrer">
                  Open verification page
                </a>
              </Button>
            ) : null}
            {!authState?.verification_url && authState?.auth_url ? (
              <Button asChild variant="outline" size="sm">
                <a href={authState.auth_url} target="_blank" rel="noreferrer">
                  Continue in browser
                </a>
              </Button>
            ) : null}
            {authState?.state === 'pending' ? (
              <Button variant="outline" size="sm" onClick={onAuthCancel} disabled={acting !== null}>
                Cancel sign-in
              </Button>
            ) : null}
          </div>
        </div>
      ) : null}

      {activeInteraction ? (
        <CodingInteractionCard
          interaction={activeInteraction}
          acting={acting}
          onResolve={onResolveInteraction}
          availablePreviewPanelKey={availablePreviewPanelKey}
          attachedPreview={attachedPreview}
          onViewPreview={onViewPreview}
          compact
        />
      ) : null}
      {activeInteraction && reviewArtifacts.length > 0 ? (
        <CodingReviewHistoryPanel reviewArtifacts={reviewArtifacts} />
      ) : null}
          <div
            aria-hidden="true"
            className="h-[calc(env(safe-area-inset-bottom)+5rem)]"
            data-coding-session-interruption-spacer
          />
        </div>
      </div>
    </div>
  );
}

function MessageInput({
  onSend,
  sending,
  enabled,
  placeholder,
  disabledReason,
}: {
  onSend?: (content: string) => Promise<void>;
  sending: boolean;
  enabled: boolean;
  placeholder: string;
  disabledReason?: string;
}) {
  const [value, setValue] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const disabled = !enabled || sending;
  const resizeTextarea = useCallback((textarea = textareaRef.current) => {
    if (!textarea) return;
    textarea.style.height = 'auto';
    const nextHeight = Math.min(textarea.scrollHeight, 128);
    textarea.style.height = nextHeight > 0 ? `${nextHeight}px` : '';
    textarea.style.overflowY = textarea.scrollHeight > 128 ? 'auto' : 'hidden';
  }, []);

  useEffect(() => {
    resizeTextarea();
  }, [resizeTextarea, value]);

  const handleSubmit = async () => {
    const trimmed = value.trim();
    if (!trimmed || disabled || !onSend) return;
    await onSend(trimmed);
    setValue('');
  };

  return (
    <div className="border-t border-border bg-background px-4 pb-[calc(env(safe-area-inset-bottom)+1rem)] pt-3">
      <div className="flex items-center gap-2">
        <Textarea
          ref={textareaRef}
          value={value}
          onChange={(e) => {
            setValue(e.target.value);
            resizeTextarea(e.currentTarget);
          }}
          onKeyDown={(e) => {
            if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
              e.preventDefault();
              void handleSubmit();
            }
          }}
          placeholder={placeholder}
          className="min-h-[2.5rem] max-h-32 resize-none overflow-hidden text-sm focus-visible:border-ring/70 focus-visible:ring-2 focus-visible:ring-ring/15"
          disabled={disabled}
          title={disabledReason}
          rows={1}
        />
        <Button
          size="icon"
          className="h-10 w-10 shrink-0 rounded-full"
          onClick={() => void handleSubmit()}
          disabled={!enabled || !value.trim() || sending}
          data-coding-session-message-submit
          title={disabledReason}
        >
          {sending ? <Loading01Icon className="h-5 w-5 animate-spin" /> : <ArrowUp02Icon className="h-5 w-5" />}
        </Button>
      </div>
    </div>
  );
}

// ─── Running indicator ──────────────────────────────────────────────────────

function formatElapsed(ms: number): string {
  return formatCodingSessionElapsed(ms);
}

/** Subscribes to a 1-second tick so elapsed time stays live. */
function useElapsedMs(since: string): number {
  const origin = useMemo(() => new Date(since).getTime(), [since]);
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    setNow(Date.now());
    const id = window.setInterval(() => setNow(Date.now()), 1_000);
    return () => window.clearInterval(id);
  }, [origin]);

  if (!origin || Number.isNaN(origin)) return 0;
  return Math.max(0, now - origin);
}

function RunningEllipsis() {
  const [dotCount, setDotCount] = useState(1);

  useEffect(() => {
    const id = window.setInterval(() => {
      setDotCount((current) => current === 3 ? 1 : current + 1);
    }, 500);
    return () => window.clearInterval(id);
  }, []);

  return (
    <span aria-hidden className="inline-block w-[1.25em] text-left" data-agent-running-ellipsis>
      {'.'.repeat(dotCount)}
    </span>
  );
}

function RunningActivityRow({ since }: { since: string }) {
  const elapsed = useElapsedMs(since);
  return (
    <div className="flex gap-3" data-coding-session-running-activity>
      <div className="relative flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-primary/30 bg-primary/10 shadow-sm">
        <span
          aria-hidden
          className="absolute inset-0 animate-ping rounded-full bg-primary/20"
          data-agent-running-halo
        />
        <UnicodeSpinner
          name="braille"
          className="agent-working-chroma relative text-base leading-none"
          data-agent-working-spinner
        />
      </div>
      <div className="min-w-0 flex-1 pb-4">
        <div className="flex min-h-7 items-center gap-2">
          <span className="text-xs font-medium text-foreground/80">
            Agent running<RunningEllipsis />
          </span>
          <span className="text-[11px] tabular-nums text-muted-foreground">{formatElapsed(elapsed)}</span>
        </div>
      </div>
    </div>
  );
}
