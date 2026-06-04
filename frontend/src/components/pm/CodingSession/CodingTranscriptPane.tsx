import { Fragment, useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { UnicodeSpinner } from '@/components/pm/CodingSession/UnicodeSpinner';
import {
  BotIcon,
  SourceCodeIcon,
  File01Icon,
  Loading01Icon,
  ArrowUp02Icon,
  TerminalIcon,
  CancelCircleIcon,
  LockKeyIcon,
  RadioIcon,
  Wrench01Icon,
  Globe02Icon,
} from '@/lib/icons';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';
import type {
  AgentRunArtifact,
  CodingSession,
  CodingSessionActor,
  CodingSessionInteraction,
  CodingSessionLiveAssistantMessage,
  CodingSessionLiveReasoningMessage,
  CodingSessionLiveToolCall,
  CodingSessionLiveTurnSegment,
  CodingSessionTranscriptMessage,
} from '@/lib/pmTypes';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { useWorkspaceMembers } from '@/hooks/queries';
import { formatCodingSessionRelative } from './codingSessionUtils';
import {
  formatCodingSessionElapsed,
  isStatusTranscriptMessage,
} from './codingSessionPresentation';
import type { CodingSessionComposerState } from './codingSessionComposer';
import { ApplyPatchDiff } from './ApplyPatchDiff';
import type { PublishedPreview } from '@/components/pm/runPreviews';
import { CodingInteractionCard } from './CodingInteractionCard';
import { CodingReviewHistoryPanel, type CodingReviewHistoryItem } from './CodingReviewHistoryPanel';
import { MarkdownContent } from './MarkdownContent';
import { PublishedToolPreviewCard } from './PublishedToolPreviewCard';
import { describeToolCall } from './toolCallPresentation';

// ─── Tool call grouping ──────────────────────────────────────────────────────

const TOOL_GROUP_COLLAPSE_THRESHOLD = 2;

type ToolCategory = 'read' | 'search' | 'command' | 'write' | 'other';

function categorizeToolCall(toolName: string): ToolCategory {
  const name = toolName.toLowerCase();
  if (name.includes('read')) return 'read';
  if (name === 'grep' || name === 'glob' || name === 'find' || name.includes('search') || name.includes('grep')) return 'search';
  if (name === 'run_command' || name === 'bash' || name.includes('shell') || name.includes('exec')) return 'command';
  if (name === 'apply_patch' || name === 'write_file' || name === 'str_replace_editor' || name.includes('write') || name.includes('edit') || name.includes('patch')) return 'write';
  return 'other';
}

type SegmentGroup =
  | { kind: 'assistant'; segment: CodingSessionLiveTurnSegment }
  | { kind: 'tool_group'; toolCalls: CodingSessionLiveToolCall[] };

function partitionTurnSegments(segments: CodingSessionLiveTurnSegment[]): SegmentGroup[] {
  const groups: SegmentGroup[] = [];
  let pendingToolCalls: CodingSessionLiveToolCall[] = [];

  for (const seg of segments) {
    if (seg.kind === 'assistant_message') {
      if (pendingToolCalls.length > 0) {
        groups.push({ kind: 'tool_group', toolCalls: pendingToolCalls });
        pendingToolCalls = [];
      }
      groups.push({ kind: 'assistant', segment: seg });
    } else {
      pendingToolCalls.push(seg.tool_call);
    }
  }

  if (pendingToolCalls.length > 0) {
    groups.push({ kind: 'tool_group', toolCalls: pendingToolCalls });
  }

  return groups;
}

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
    return segment.tool_call.tool_name !== 'update_plan';
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

  // Build a flat list of all renderable items for the virtualizer.
  type VirtualItem =
    | { kind: 'context'; message: CodingSessionTranscriptMessage }
    | { kind: 'transcript'; message: CodingSessionTranscriptMessage }
    | { kind: 'status'; message: CodingSessionTranscriptMessage }
    | { kind: 'thinking'; reasoning: CodingSessionLiveReasoningMessage }
    | { kind: 'live-message'; segment: CodingSessionLiveTurnSegment }
    | { kind: 'live-tool'; segment: CodingSessionLiveTurnSegment; isLast: boolean }
    | { kind: 'placeholder' }
    | { kind: 'running'; since: string }
    | { kind: 'empty' }
    | { kind: 'bottom-spacer' };

  const items = useMemo((): VirtualItem[] => {
    const list: VirtualItem[] = [];
    if (promptMessage) {
      list.push({ kind: 'context', message: promptMessage });
    }
    for (const message of transcriptMessages) {
      if (isStatusTranscriptMessage(message)) {
        list.push({ kind: 'status', message });
      } else {
        list.push({ kind: 'transcript', message });
      }
    }
    if (liveReasoningMessage) {
      list.push({ kind: 'thinking', reasoning: liveReasoningMessage });
    }
    for (let i = 0; i < visibleLiveSegments.length; i++) {
      const segment = visibleLiveSegments[i];
      if (segment.kind === 'assistant_message') {
        list.push({ kind: 'live-message', segment });
      } else {
        list.push({ kind: 'live-tool', segment, isLast: i === visibleLiveSegments.length - 1 });
      }
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
    promptMessage,
    transcriptMessages,
    liveReasoningMessage,
    visibleLiveSegments,
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
      case 'transcript':
        return <TranscriptEntry message={item.message} actor={actorForMessage(item.message)} />;
      case 'context':
        return <RunContextDisclosure message={item.message} />;
      case 'status':
        return <StatusTimelineRow message={item.message} />;
      case 'thinking':
        return <ThinkingStrip reasoning={item.reasoning} />;
      case 'live-message': {
        const seg = item.segment;
        if (seg.kind !== 'assistant_message') return null;
        return (
          <TranscriptEntry
            message={{
              event_id: `live:${seg.segment_id}`,
              message_id: seg.assistant_message.message_id,
              role: 'assistant',
              content: seg.assistant_message.content,
              timestamp: seg.assistant_message.started_at ?? new Date().toISOString(),
              sequence_no: Number.MAX_SAFE_INTEGER,
            }}
            live
            streaming={seg.assistant_message.status === 'streaming'}
          />
        );
      }
      case 'live-tool': {
        const seg = item.segment;
        if (seg.kind !== 'tool_call') return null;
        return (
          <div className="w-full max-w-[90%]">
            <ActivityToolCallRow toolCall={seg.tool_call} isLast={item.isLast} />
          </div>
        );
      }
      case 'placeholder':
        return (
          <TranscriptEntry
            message={{
              event_id: `live:${liveAssistantMessage?.message_id ?? 'assistant'}`,
              message_id: liveAssistantMessage?.message_id,
              role: 'assistant',
              content: 'Preparing reply…',
              timestamp: liveAssistantMessage?.started_at ?? new Date().toISOString(),
              sequence_no: Number.MAX_SAFE_INTEGER,
            }}
            live
            streaming
            placeholder
          />
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
  }, [liveAssistantMessage, actorForMessage]);

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

function RunContextDisclosure({ message }: { message: CodingSessionTranscriptMessage }) {
  const label = message.message_type === 'system_prompt' ? 'System prompt' : 'Developer prompt';
  return (
    <details className="mb-3 rounded-lg border border-border/70 bg-muted/25 px-3 py-2">
      <summary className="cursor-pointer list-none text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
        Run context
      </summary>
      <div className="mt-2 border-t border-border/60 pt-2">
        <div className="mb-1 text-[11px] font-medium text-muted-foreground">{label}</div>
        <pre className="max-h-72 overflow-auto whitespace-pre-wrap break-words text-[11px] leading-5 text-foreground/80">
          {message.content}
        </pre>
      </div>
    </details>
  );
}

function StatusTimelineRow({ message }: { message: CodingSessionTranscriptMessage }) {
  return (
    <div className="flex gap-3">
      <div className="flex flex-col items-center">
        <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full border border-border bg-muted/60 text-muted-foreground">
          <Loading01Icon className="h-3.5 w-3.5" />
        </div>
        <div className="mt-1 h-full min-h-[1rem] w-px bg-border/50" />
      </div>
      <div className="min-w-0 flex-1 pb-4">
        <div className="mb-1 flex items-center gap-2">
          <span className="text-xs font-medium text-muted-foreground">Status</span>
          <span className="text-[11px] text-muted-foreground">{formatCodingSessionRelative(message.timestamp)}</span>
        </div>
        <p className="text-[13px] leading-6 text-muted-foreground">{message.content}</p>
      </div>
    </div>
  );
}

/** Renders a single transcript turn (bubble + standalone tool calls below). */
function TranscriptEntry({
  message,
  live = false,
  streaming = false,
  placeholder = false,
  actor,
}: {
  message: CodingSessionTranscriptMessage;
  live?: boolean;
  streaming?: boolean;
  placeholder?: boolean;
  actor?: CodingSessionActor | null;
}) {
  const isAssistant = message.role === 'assistant';
  const visibleToolCalls = (message.tool_calls ?? []).filter((tc) => tc.tool_name !== 'update_plan');
  const visibleTurnSegments = isAssistant
    ? (message.turn_segments ?? []).filter((segment) => (
        segment.kind !== 'tool_call' || segment.tool_call.tool_name !== 'update_plan'
      ))
    : [];
  const hasSegmentTimeline = visibleTurnSegments.length > 0;
  const segmentGroups = hasSegmentTimeline ? partitionTurnSegments(visibleTurnSegments) : [];
  const toolCallTimelineKeys = hasSegmentTimeline
    ? new Set(
      visibleTurnSegments
        .filter((segment): segment is Extract<typeof visibleTurnSegments[number], { kind: 'tool_call' }> => segment.kind === 'tool_call')
        .map((segment) => toolCallTimelineKey(segment.tool_call)),
    )
    : null;
  const fallbackToolCalls = hasSegmentTimeline && toolCallTimelineKeys
    ? visibleToolCalls.filter((toolCall) => !toolCallTimelineKeys.has(toolCallTimelineKey(toolCall)))
    : visibleToolCalls;

  if (isAssistant) {
    return (
      <div className="w-full max-w-[90%]">
        {hasSegmentTimeline ? (
          <>
            {segmentGroups.map((group, groupIdx) => {
              const isLastGroup = groupIdx === segmentGroups.length - 1 && fallbackToolCalls.length === 0;
              if (group.kind === 'assistant') {
                const seg = group.segment;
                if (seg.kind !== 'assistant_message') return null;
                return (
                  <AssistantTimelineRow
                    key={seg.segment_id}
                    content={seg.assistant_message.content}
                    timestamp={seg.assistant_message.started_at ?? message.timestamp}
                    live={live}
                    streaming={seg.assistant_message.status === 'streaming'}
                    isLast={isLastGroup}
                  />
                );
              }
              if (group.toolCalls.length >= TOOL_GROUP_COLLAPSE_THRESHOLD) {
                return (
                  <CollapsedToolCallGroup
                    key={group.toolCalls[0].tool_call_id}
                    toolCalls={group.toolCalls}
                    isLast={isLastGroup}
                  />
                );
              }
              return (
                <Fragment key={group.toolCalls[0].tool_call_id}>
                  {group.toolCalls.map((tc, tcIdx) => (
                    <ActivityToolCallRow
                      key={tc.tool_call_id}
                      toolCall={tc}
                      isLast={isLastGroup && tcIdx === group.toolCalls.length - 1}
                    />
                  ))}
                </Fragment>
              );
            })}

            {fallbackToolCalls.length >= TOOL_GROUP_COLLAPSE_THRESHOLD ? (
              <CollapsedToolCallGroup toolCalls={fallbackToolCalls} isLast />
            ) : fallbackToolCalls.length > 0 ? (
              fallbackToolCalls.map((tc, idx) => (
                <ActivityToolCallRow
                  key={tc.tool_call_id}
                  toolCall={tc}
                  isLast={idx === fallbackToolCalls.length - 1}
                />
              ))
            ) : null}
          </>
        ) : (
          <>
            {message.content.trim() ? (
              <AssistantTimelineRow
                content={message.content}
                timestamp={message.timestamp}
                live={live}
                streaming={streaming}
                placeholder={placeholder}
                isLast={visibleToolCalls.length === 0}
              />
            ) : null}

            {visibleToolCalls.length >= TOOL_GROUP_COLLAPSE_THRESHOLD ? (
              <CollapsedToolCallGroup toolCalls={visibleToolCalls} isLast />
            ) : visibleToolCalls.length > 0 ? (
              visibleToolCalls.map((tc, idx) => (
                <ActivityToolCallRow
                  key={tc.tool_call_id}
                  toolCall={tc}
                  isLast={idx === visibleToolCalls.length - 1}
                />
              ))
            ) : null}
          </>
        )}
      </div>
    );
  }

  if (message.message_type === 'developer_prompt' || message.message_type === 'system_prompt') {
    return (
      <PromptTranscriptCard
        content={message.content}
        timestamp={message.timestamp}
        label={message.message_type === 'system_prompt' ? 'System prompt' : 'Developer prompt'}
      />
    );
  }

  if (message.message_type === 'review_checkpoint_resolution' || message.message_type === 'approval_request_resolution') {
    return <ReviewDecisionTranscriptCard content={message.content} timestamp={message.timestamp} actor={actor ?? null} />;
  }

  const actorLabel = actor?.full_name || actor?.email || 'User';

  return (
    <div className="flex flex-col items-end gap-2">
      <div className="flex items-center justify-end gap-2 px-1 text-[11px] text-muted-foreground">
        <span>{formatCodingSessionRelative(message.timestamp)}</span>
        <span className="font-medium">{actorLabel}</span>
        <UserAvatar
          name={actorLabel}
          avatarUrl={actor?.avatar_url}
          className="h-6 w-6"
          fallbackClassName="text-[10px]"
        />
      </div>

      {message.content.trim() ? (
        <AssistantMessageBubble
          content={message.content}
          isAssistant={false}
          placeholder={placeholder}
        />
      ) : null}
    </div>
  );
}

function PromptTranscriptCard({
  content,
  timestamp,
  label,
}: {
  content: string;
  timestamp: string;
  label: string;
}) {
  return (
    <div className="w-full max-w-[90%]">
      <div className="mb-2 flex items-center gap-2 px-1 text-[11px] text-muted-foreground">
        <span>{formatCodingSessionRelative(timestamp)}</span>
        <span className="font-medium">{label}</span>
      </div>
      <details className="rounded-xl border border-border/70 bg-muted/30 px-4 py-3">
        <summary className="cursor-pointer list-none text-xs font-medium uppercase tracking-wide text-muted-foreground">
          {label}
        </summary>
        <pre className="mt-3 overflow-auto whitespace-pre-wrap break-words text-[11px] leading-5 text-foreground">
          {content}
        </pre>
      </details>
    </div>
  );
}

function ReviewDecisionTranscriptCard({
  content,
  timestamp,
  actor,
}: {
  content: string;
  timestamp: string;
  actor: CodingSessionActor | null;
}) {
  const reviewerName = actor?.full_name || actor?.email || 'Reviewer';
  const normalizedContent = content.trim().toLowerCase();
  const decisionLabel = normalizedContent.startsWith('requested changes')
    ? 'requested changes'
    : normalizedContent.startsWith('approved')
      ? 'approved'
      : 'reviewed';
  return (
    <div className="ml-auto w-full max-w-[90%]">
      <div className="mb-2 flex items-center justify-end gap-2 px-1 text-[11px] text-muted-foreground">
        <span>{formatCodingSessionRelative(timestamp)}</span>
        <span className="font-medium">{reviewerName} {decisionLabel}</span>
        <UserAvatar
          name={reviewerName}
          avatarUrl={actor?.avatar_url}
          className="h-6 w-6"
          fallbackClassName="text-[10px]"
        />
      </div>
      <div className="rounded-2xl rounded-br-sm bg-blue-50 px-3.5 py-2.5 text-sm leading-relaxed text-foreground/85 shadow-sm dark:bg-blue-950/40 dark:text-foreground">
        <MarkdownContent content={content} className="text-inherit" />
      </div>
    </div>
  );
}

function toolCallTimelineKey(toolCall: CodingSessionLiveToolCall) {
  return [
    toolCall.tool_name.trim().toLowerCase(),
    toolCall.args_text.trim(),
    toolCall.result?.output_summary?.trim() ?? '',
    toolCall.result?.content?.trim() ?? '',
  ].join('\n');
}

const CONTENT_COLLAPSE_CHAR_THRESHOLD = 600;

function CollapsibleMarkdown({ content, streaming = false }: { content: string; streaming?: boolean }) {
  const [expanded, setExpanded] = useState(false);
  const isLong = content.length > CONTENT_COLLAPSE_CHAR_THRESHOLD;

  if (!isLong) {
    return <MarkdownContent content={content} className="text-[13px] leading-6 text-foreground/85 dark:text-foreground" streaming={streaming} />;
  }

  return (
    <div>
      <div className={cn('relative', !expanded && 'max-h-[10rem] overflow-hidden')}>
        <MarkdownContent content={content} className="text-[13px] leading-6 text-foreground/85 dark:text-foreground" streaming={streaming} />
        {!expanded && (
          <div className="pointer-events-none absolute inset-x-0 bottom-0 h-16 bg-gradient-to-t from-card to-transparent" />
        )}
      </div>
      <button
        type="button"
        className="mt-1 text-[11px] font-medium text-primary hover:underline"
        onClick={() => setExpanded((prev) => !prev)}
      >
        {expanded ? 'Show less' : 'Show more'}
      </button>
    </div>
  );
}

function AssistantTimelineRow({
  content,
  isLast,
  live = false,
  streaming = false,
  placeholder = false,
}: {
  content: string;
  timestamp?: string;
  isLast: boolean;
  live?: boolean;
  streaming?: boolean;
  placeholder?: boolean;
}) {
  return (
    <div className="flex gap-3">
      <div className="flex flex-col items-center">
        <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full border border-primary/20 bg-primary/10 text-primary">
          {live && streaming ? <RadioIcon className="h-3.5 w-3.5 animate-pulse" /> : <BotIcon className="h-3.5 w-3.5" />}
        </div>
        {!isLast && <div className="mt-1 h-full min-h-[1rem] w-px bg-border/50" />}
      </div>

      <div className={cn('min-w-0 flex-1', isLast ? 'pb-0' : 'pb-4')}>
        <div className="mb-1 flex items-center gap-2">
          <span className="text-xs font-medium text-foreground">Assistant</span>
          {live ? (
            <Badge variant="outline" className="h-5 px-1.5 text-[10px]">
              {streaming ? 'Live' : 'Update'}
            </Badge>
          ) : null}
        </div>

        {placeholder ? (
          <div className="whitespace-pre-wrap text-[13px] leading-6 text-muted-foreground">{content}</div>
        ) : (
          <CollapsibleMarkdown content={content} streaming={live && streaming} />
        )}
      </div>
    </div>
  );
}

function AssistantMessageBubble({
  content,
  isAssistant = true,
  placeholder = false,
}: {
  content: string;
  isAssistant?: boolean;
  placeholder?: boolean;
}) {
  const [expanded, setExpanded] = useState(false);
  const isLong = content.length > CONTENT_COLLAPSE_CHAR_THRESHOLD;

  return (
    <div className={cn(
      'max-w-[90%] rounded-2xl px-3.5 py-2.5 text-sm leading-relaxed shadow-sm',
      isAssistant
        ? 'rounded-bl-sm border border-border/60 bg-background text-foreground/85 dark:text-foreground'
        : 'rounded-br-sm bg-blue-50 text-foreground/85 dark:bg-blue-950/40 dark:text-foreground',
      placeholder && 'border-dashed text-muted-foreground',
    )}>
      {placeholder ? (
        <div className="whitespace-pre-wrap">{content}</div>
      ) : isLong ? (
        <div>
          <div className={cn('relative', !expanded && 'max-h-[10rem] overflow-hidden')}>
            <MarkdownContent content={content} className={isAssistant ? undefined : 'text-inherit'} />
            {!expanded && (
              <div className={cn(
                'pointer-events-none absolute inset-x-0 bottom-0 h-14 bg-gradient-to-t to-transparent',
                isAssistant ? 'from-background' : 'from-blue-50 dark:from-blue-950/40',
              )} />
            )}
          </div>
          <button
            type="button"
            className={cn(
              'mt-1 text-[11px] font-medium hover:underline',
              isAssistant ? 'text-primary' : 'text-blue-700 dark:text-blue-200',
            )}
            onClick={() => setExpanded((prev) => !prev)}
          >
            {expanded ? 'Show less' : 'Show more'}
          </button>
        </div>
      ) : (
        <MarkdownContent content={content} className={isAssistant ? undefined : 'text-inherit'} />
      )}
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

// ─── Activity-style tool call row ────────────────────────────────────────────

function toolChrome(toolName: string, isFailed: boolean, isRunning: boolean): { icon: ReactNode; iconClass: string } {
  if (isFailed) {
    return {
      icon: <CancelCircleIcon className="h-3.5 w-3.5" />,
      iconClass: 'bg-destructive/10 border-destructive/30 text-destructive',
    };
  }
  if (isRunning) {
    return {
      icon: <UnicodeSpinner name="braille" className="text-xs" />,
      iconClass: 'bg-primary/10 border-primary/30 text-primary',
    };
  }
  const name = toolName.toLowerCase();
  if (name.includes('web_search')) {
    return {
      icon: <Globe02Icon className="h-3.5 w-3.5" />,
      iconClass: 'bg-sky-50 border-sky-200 dark:bg-sky-950/20 dark:border-sky-900/50 text-sky-600 dark:text-sky-400',
    };
  }
  if (name === 'run_command' || name === 'bash' || name.includes('shell') || name.includes('exec')) {
    return {
      icon: <TerminalIcon className="h-3.5 w-3.5" />,
      iconClass: 'bg-slate-100 border-slate-300 dark:bg-slate-900 dark:border-slate-700 text-slate-600 dark:text-slate-400',
    };
  }
  if (
    name.startsWith('publish_')
    || name.startsWith('preview_')
    || name.includes('plan_doc')
    || name.includes('draft')
  ) {
    return {
      icon: <File01Icon className="h-3.5 w-3.5" />,
      iconClass: 'bg-blue-50 border-blue-200 dark:bg-blue-950/20 dark:border-blue-900/50 text-blue-600 dark:text-blue-400',
    };
  }
  if (
    name === 'apply_patch'
    || name === 'write_file'
    || name === 'str_replace_editor'
    || name.includes('file')
    || name.includes('patch')
    || name.includes('write')
    || name.includes('edit')
  ) {
    return {
      icon: <SourceCodeIcon className="h-3.5 w-3.5" />,
      iconClass: 'bg-violet-50 border-violet-200 dark:bg-violet-950/20 dark:border-violet-900/50 text-violet-600 dark:text-violet-400',
    };
  }
  return {
    icon: <Wrench01Icon className="h-3.5 w-3.5" />,
    iconClass: 'bg-muted/50 border-border text-muted-foreground',
  };
}

const TOOL_COLLAPSED_LINES = 2;

function extractFilePathsFromText(text: string): string[] {
  const matches = text.match(/(?:^|\s)((?:\/|\.\.?\/)?[\w./-]+\.(?:ts|tsx|js|jsx|go|py|css|html|json|sql|md|yaml|yml|toml|sh))\b/g);
  if (!matches) return [];
  const unique = [...new Set(matches.map((m) => m.trim()))];
  return unique.slice(0, 6);
}

function CollapsibleCodeBlock({ text, failed }: { text: string; failed?: boolean }) {
  const [expanded, setExpanded] = useState(false);
  const lines = text.split('\n');
  const isLong = lines.length > TOOL_COLLAPSED_LINES;

  return (
    <div className="relative">
      <pre className={cn(
        'overflow-auto whitespace-pre-wrap break-all rounded-md border px-2.5 py-1.5 font-mono text-[11px] leading-5',
        failed
          ? 'border-destructive/20 bg-destructive/5 text-destructive dark:bg-destructive/10'
          : 'border-border/60 bg-muted/50 text-foreground/80',
        !expanded && isLong && 'max-h-[52px]',
        expanded && 'max-h-60',
      )}>
        {expanded || !isLong ? text : lines.slice(0, TOOL_COLLAPSED_LINES).join('\n')}
      </pre>
      {isLong && !expanded && (
        <div className={cn(
          'pointer-events-none absolute inset-x-0 bottom-0 h-6 rounded-b-md bg-gradient-to-t',
          failed ? 'from-destructive/5 to-transparent' : 'from-muted/80 to-transparent',
        )} />
      )}
      {isLong && (
        <button
          type="button"
          className="mt-1 text-[11px] font-medium text-primary hover:underline"
          onClick={() => setExpanded((prev) => !prev)}
        >
          {expanded ? 'Show less' : `Show more (${lines.length} lines)`}
        </button>
      )}
    </div>
  );
}

function ActivityToolCallRow({ toolCall, isLast }: { toolCall: CodingSessionLiveToolCall; isLast: boolean }) {
  const isFailed = toolCall.status === 'failed';
  const isRunning = toolCall.status === 'running';
  const { icon, iconClass } = toolChrome(toolCall.tool_name, isFailed, isRunning);
  const isApplyPatch = toolCall.tool_name === 'apply_patch';
  const argsText = toolCall.args_text.trim();
  const resultText = toolCall.result?.output_summary?.trim() || toolCall.result?.content?.trim() || '';
  const publishedPreviewCard = !isFailed && !isApplyPatch && argsText ? (
    <PublishedToolPreviewCard toolName={toolCall.tool_name} argsText={argsText} resultText={resultText} />
  ) : null;
  const presentation = describeToolCall(toolCall);
  const showSecondaryBadge = presentation.secondaryLabel.trim().toLowerCase() !== presentation.primaryLabel.trim().toLowerCase();
  const filePaths = !isApplyPatch && !publishedPreviewCard && argsText ? extractFilePathsFromText(argsText) : [];
  const chips = [...presentation.chips];
  for (const filePath of filePaths) {
    if (!chips.includes(filePath)) chips.push(filePath);
  }

  return (
    <div className="flex gap-3">
      {/* Icon + connector line */}
      <div className="flex flex-col items-center">
        <div className={cn('flex h-7 w-7 shrink-0 items-center justify-center rounded-full border', iconClass)}>
          {icon}
        </div>
        {!isLast && <div className="mt-1 h-full min-h-[1rem] w-px bg-border/50" />}
      </div>

      {/* Content */}
      <div className={cn('min-w-0 flex-1', isLast ? 'pb-0' : 'pb-4')}>
        <div className="mb-1 flex items-start justify-between gap-2">
          <div className="min-w-0 space-y-1">
            <p className="truncate text-xs font-medium text-foreground">{presentation.primaryLabel}</p>
            <div className="flex flex-wrap items-center gap-1.5">
              {showSecondaryBadge ? (
                <Badge variant="outline" className="h-5 rounded-full px-1.5 text-[10px] font-medium text-muted-foreground">
                  {presentation.secondaryLabel}
                </Badge>
              ) : null}
              {chips.map((chip) => (
                <span key={chip} className="inline-flex items-center rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium text-primary">
                  {chip}
                </span>
              ))}
            </div>
          </div>
        </div>
        <div className="space-y-1.5 text-xs text-muted-foreground">
          {publishedPreviewCard ?? (
            <>
              {isApplyPatch
                ? <ApplyPatchDiff argsText={argsText || resultText} />
                : argsText ? <CollapsibleCodeBlock text={argsText} /> : null}
              {resultText && !isApplyPatch ? (
                isFailed
                  ? <CollapsibleCodeBlock text={resultText} failed />
                  : <p className="text-[11px] text-muted-foreground">{resultText.length > 200 ? `${resultText.slice(0, 200)}…` : resultText}</p>
              ) : null}
              {isApplyPatch && isFailed && resultText ? (
                <CollapsibleCodeBlock text={resultText} failed />
              ) : null}
            </>
          )}
        </div>
      </div>
    </div>
  );
}

// ─── Collapsed tool call group ──────────────────────────────────────────────

function CollapsedToolCallGroup({
  toolCalls,
  isLast,
}: {
  toolCalls: CodingSessionLiveToolCall[];
  isLast: boolean;
}) {
  const [expanded, setExpanded] = useState(false);

  const { chips, failedCount, durationLabel } = useMemo(() => {
    const counts: Record<ToolCategory, number> = { read: 0, search: 0, command: 0, write: 0, other: 0 };
    let failed = 0;
    let durationMs = 0;

    for (const tc of toolCalls) {
      counts[categorizeToolCall(tc.tool_name)]++;
      if (tc.duration_ms) durationMs += tc.duration_ms;
      if (tc.status === 'failed') failed++;
    }

    const parts: string[] = [];
    if (counts.read > 0) parts.push(`${counts.read} read${counts.read !== 1 ? 's' : ''}`);
    if (counts.search > 0) parts.push(`${counts.search} search${counts.search !== 1 ? 'es' : ''}`);
    if (counts.command > 0) parts.push(`${counts.command} command${counts.command !== 1 ? 's' : ''}`);
    if (counts.write > 0) parts.push(`${counts.write} write${counts.write !== 1 ? 's' : ''}`);
    if (counts.other > 0) parts.push(`${counts.other} other`);

    const label = durationMs >= 1000
      ? `${Math.round(durationMs / 1000)}s`
      : durationMs > 0 ? `${durationMs}ms` : null;

    return { chips: parts, failedCount: failed, durationLabel: label };
  }, [toolCalls]);

  return (
    <div className="flex gap-3">
      {/* Timeline connector */}
      <div className="flex flex-col items-center">
        <div className={cn(
          'flex h-7 w-7 shrink-0 items-center justify-center rounded-full border',
          failedCount > 0
            ? 'border-destructive/30 bg-destructive/10 text-destructive'
            : 'border-blue-200 bg-blue-50 text-blue-600 dark:border-blue-900/50 dark:bg-blue-950/20 dark:text-blue-400',
        )}>
          <Wrench01Icon className="h-3.5 w-3.5" />
        </div>
        {!isLast && (
          <div className="mt-1 h-full min-h-[1rem] w-px border-l border-dashed border-border/60" />
        )}
      </div>

      {/* Content */}
      <div className={cn('min-w-0 flex-1', isLast ? 'pb-0' : 'pb-4')}>
        <button
          type="button"
          className={cn(
            'w-full rounded-lg border px-3 py-2 text-left transition-colors',
            'border-border/60 bg-muted/25 hover:bg-muted/40',
            expanded && 'rounded-b-none border-b-0',
          )}
          onClick={() => setExpanded((prev) => !prev)}
        >
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-2">
              <span className="text-xs font-medium text-foreground">
                Performed {toolCalls.length} tool calls
              </span>
              {failedCount > 0 && (
                <span className="inline-flex items-center gap-1 rounded bg-destructive/10 px-1.5 py-0.5 text-[10px] font-medium text-destructive">
                  <CancelCircleIcon className="h-3 w-3" />
                  {failedCount} failed
                </span>
              )}
            </div>
            <span className="text-[11px] font-medium text-primary">
              {expanded ? '▾ Hide' : '▸ Show'}
            </span>
          </div>
          <div className="mt-1 flex flex-wrap items-center gap-1.5">
            {chips.map((chip) => (
              <span key={chip} className="inline-flex items-center rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium text-primary">
                {chip}
              </span>
            ))}
            {durationLabel && (
              <span className="text-[10px] text-muted-foreground">· {durationLabel}</span>
            )}
          </div>
        </button>

        {expanded && (
          <div className="rounded-b-lg border border-t-0 border-border/60 bg-muted/15 py-1">
            {toolCalls.map((tc) => (
              <CompactToolCallRow key={tc.tool_call_id} toolCall={tc} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function CompactToolCallRow({ toolCall }: { toolCall: CodingSessionLiveToolCall }) {
  const isFailed = toolCall.status === 'failed';
  const { icon, iconClass } = toolChrome(toolCall.tool_name, isFailed, false);
  const presentation = describeToolCall(toolCall);

  return (
    <div className={cn(
      'flex items-center gap-2 px-3 py-1.5',
      isFailed && 'bg-destructive/5',
    )}>
      <div className={cn('flex h-5 w-5 shrink-0 items-center justify-center rounded-full border', iconClass)}>
        <span className="flex scale-75 items-center justify-center">{icon}</span>
      </div>
      <span className={cn(
        'min-w-0 truncate text-[11px]',
        isFailed ? 'font-medium text-destructive' : 'text-foreground/80',
      )}>
        {presentation.primaryLabel}
      </span>
      {presentation.chips.length > 0 && (
        <span className="shrink-0 text-[10px] text-muted-foreground">
          {presentation.chips[0]}
        </span>
      )}
      {isFailed && (
        <CancelCircleIcon className="ml-auto h-3 w-3 shrink-0 text-destructive" />
      )}
    </div>
  );
}

function ThinkingStrip({
  reasoning,
}: {
  reasoning: CodingSessionLiveReasoningMessage;
}) {
  const hasVisibleContent = reasoning.content.trim().length > 0;

  return (
    <details className="rounded-xl border border-border bg-muted/25 px-4 py-3" open={reasoning.status === 'streaming'}>
      <summary className="flex cursor-pointer list-none items-center justify-between gap-3">
        <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          <LockKeyIcon className="h-3.5 w-3.5" />
          Thinking
        </div>
        <div className="flex items-center gap-2">
          <Badge variant="outline" className="text-[10px]">
            {reasoning.status === 'streaming' ? 'Live' : 'Captured'}
          </Badge>
        </div>
      </summary>

      <div className="mt-3 border-t border-border pt-3 text-sm text-muted-foreground">
        {hasVisibleContent ? (
          <div className="whitespace-pre-wrap leading-6">{reasoning.content}</div>
        ) : (
          <div className="rounded-lg border border-border bg-card px-3 py-2 text-muted-foreground">
            Reasoning is being tracked separately from the assistant reply.
          </div>
        )}

        {reasoning.encrypted_value ? (
          <div className="mt-2 flex items-center gap-2 rounded-lg border border-border bg-card px-3 py-2 text-xs text-muted-foreground">
            <LockKeyIcon className="h-3.5 w-3.5" />
            Encrypted reasoning payload attached.
          </div>
        ) : null}
      </div>
    </details>
  );
}
