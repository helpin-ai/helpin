import { Fragment, useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore, type ReactNode } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { UnicodeSpinner } from '@/components/pm/CodingSession/UnicodeSpinner';
import {
  BotIcon,
  SourceCodeIcon,
  Loading01Icon,
  ArrowUp02Icon,
  TerminalIcon,
  UserIcon,
  CancelCircleIcon,
  LockKeyIcon,
  RadioIcon,
  Wrench01Icon,
  File01Icon,
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
import { formatCodingSessionRelative } from './codingSessionUtils';
import { ApplyPatchDiff } from './ApplyPatchDiff';
import { CodingInteractionCard } from './CodingInteractionCard';
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
  transcriptMessages,
  liveAssistantMessage,
  liveReasoningMessage,
  liveTurnSegments,
  loading = false,
  onSendMessage,
  sendingMessage = false,
  messagePlaceholder = 'Reply to agent… (⌘↵ to send)',
  session,
  activeInteraction,
  acting,
  onAuthStart,
  onAuthCancel,
  onResolveInteraction,
}: {
  promptArtifact?: AgentRunArtifact | null;
  transcriptMessages: CodingSessionTranscriptMessage[];
  liveAssistantMessage: CodingSessionLiveAssistantMessage | null;
  liveReasoningMessage: CodingSessionLiveReasoningMessage | null;
  liveTurnSegments: CodingSessionLiveTurnSegment[];
  loading?: boolean;
  onSendMessage?: (content: string) => Promise<void>;
  sendingMessage?: boolean;
  messagePlaceholder?: string;
  session?: CodingSession | null;
  activeInteraction?: CodingSessionInteraction | null;
  acting?: string | null;
  onAuthStart?: () => void;
  onAuthCancel?: () => void;
  onResolveInteraction?: (interactionId: string, responsePayload: Record<string, unknown>, followupMessage?: string) => void;
}) {
  const scrollContainerRef = useRef<HTMLDivElement | null>(null);
  const autoFollowRef = useRef(true);
  const visibleLiveSegments = liveTurnSegments.filter((segment) => {
    if (segment.kind === 'assistant_message') {
      return segment.assistant_message.content.trim().length > 0;
    }
    return segment.tool_call.tool_name !== 'update_plan';
  });
  const showLivePlaceholder = visibleLiveSegments.length === 0 && liveAssistantMessage?.status === 'streaming';

  // Build a flat list of all renderable items for the virtualizer.
  type VirtualItem =
    | { kind: 'transcript'; message: CodingSessionTranscriptMessage }
    | { kind: 'thinking'; reasoning: CodingSessionLiveReasoningMessage }
    | { kind: 'live-message'; segment: CodingSessionLiveTurnSegment }
    | { kind: 'live-tool'; segment: CodingSessionLiveTurnSegment; isLast: boolean }
    | { kind: 'placeholder' }
    | { kind: 'empty' };

  const items = useMemo((): VirtualItem[] => {
    const list: VirtualItem[] = [];
    for (const message of transcriptMessages) {
      list.push({ kind: 'transcript', message });
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
    if (!loading && list.length === 0) {
      list.push({ kind: 'empty' });
    }
    return list;
  }, [transcriptMessages, liveReasoningMessage, visibleLiveSegments, showLivePlaceholder, loading]);

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

  const renderItem = useCallback((item: VirtualItem) => {
    switch (item.kind) {
      case 'transcript':
        return <TranscriptEntry message={item.message} actor={triggeredBy} />;
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
      case 'empty':
        return (
          <div className="rounded-lg border border-dashed border-border px-5 py-8 text-center text-sm text-muted-foreground">
            No transcript yet. Assistant and user-visible turns will appear here once the session starts talking.
          </div>
        );
    }
  }, [liveAssistantMessage, triggeredBy]);

  return (
    <section className="relative flex h-full min-h-[20rem] flex-col overflow-hidden rounded-xl border border-border bg-card shadow-sm xl:min-h-0">
      <div className="flex items-center justify-between border-b border-border px-4 py-3">
        <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          <BotIcon className="h-3.5 w-3.5" />
          Transcript
        </div>
        <Badge variant="outline" className="text-[10px]">
          {transcriptMessages.length + (visibleLiveSegments.length > 0 || showLivePlaceholder ? 1 : 0)} turns
        </Badge>
      </div>

      <div ref={scrollContainerRef} className="min-h-0 flex-1 overflow-auto pt-3">
        <div className="mx-auto flex w-full max-w-4xl flex-col gap-3 px-4">
          {promptArtifact ? <PromptArtifactCard artifact={promptArtifact} /> : null}
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

      {session?.status === 'running' && <RunningIndicator since={session.created_at} />}

      {(session?.pause_reason === 'authentication' || activeInteraction) ? (
        <InterruptionOverlay
          session={session ?? null}
          activeInteraction={activeInteraction ?? null}
          acting={acting ?? null}
          onAuthStart={onAuthStart ?? (() => {})}
          onAuthCancel={onAuthCancel ?? (() => {})}
          onResolveInteraction={onResolveInteraction ?? (() => {})}
        />
      ) : null}

      {onSendMessage ? (
        <MessageInput onSend={onSendMessage} sending={sendingMessage} placeholder={messagePlaceholder} />
      ) : null}
    </section>
  );
}

function PromptArtifactCard({ artifact }: { artifact: AgentRunArtifact }) {
  const sections = parsePromptArtifactSections(artifact.inline_content);
  if (sections.length === 0) return null;

  return (
    <div className="rounded-xl border border-border/70 bg-muted/30 px-4 py-3">
      <div className="mb-3 flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
        <File01Icon className="h-3.5 w-3.5" />
        Runtime Prompt
      </div>
      <div className="space-y-3">
        {sections.map((section) => (
          <details key={section.label} className="group rounded-lg border border-border/60 bg-background/70 px-3 py-2" open={section.label === 'Developer instructions'}>
            <summary className="cursor-pointer list-none text-xs font-medium text-foreground">
              {section.label}
            </summary>
            <pre className="mt-2 overflow-auto whitespace-pre-wrap break-words text-[11px] leading-5 text-muted-foreground">
              {section.content}
            </pre>
          </details>
        ))}
      </div>
    </div>
  );
}

function parsePromptArtifactSections(raw: string | null | undefined) {
  if (!raw) return [] as Array<{ label: string; content: string }>;

  const lines = raw.split('\n');
  const sections: Array<{ label: string; content: string }> = [];
  let currentLabel: string | null = null;
  let currentLines: string[] = [];

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
      || line === 'Turn input:'
      || line === 'Pending request replay:'
    ) {
      flush();
      currentLabel = line.slice(0, -1);
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
  onAuthStart,
  onAuthCancel,
  onResolveInteraction,
}: {
  session: CodingSession | null;
  activeInteraction: CodingSessionInteraction | null;
  acting: string | null;
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

      <div className="border-t border-border/80 bg-card/95 px-4 py-4 backdrop-blur-md">
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
          compact
        />
      ) : null}
      </div>
    </div>
  );
}

function MessageInput({
  onSend,
  sending,
  placeholder,
}: {
  onSend: (content: string) => Promise<void>;
  sending: boolean;
  placeholder: string;
}) {
  const [value, setValue] = useState('');

  const handleSubmit = async () => {
    const trimmed = value.trim();
    if (!trimmed || sending) return;
    await onSend(trimmed);
    setValue('');
  };

  return (
    <div className="border-t border-border bg-background px-3 py-3">
      <div className="flex items-center gap-2">
        <Textarea
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => {
            if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
              e.preventDefault();
              void handleSubmit();
            }
          }}
          placeholder={placeholder}
          className="min-h-[2.5rem] max-h-32 resize-none text-sm"
          disabled={sending}
          rows={1}
        />
        <Button
          size="icon"
          className="h-10 w-10 shrink-0 rounded-full"
          onClick={() => void handleSubmit()}
          disabled={!value.trim() || sending}
        >
          {sending ? <Loading01Icon className="h-5 w-5 animate-spin" /> : <ArrowUp02Icon className="h-5 w-5" />}
        </Button>
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

  const actorLabel = actor?.full_name || actor?.email || 'User';

  return (
    <div className="flex flex-col items-end gap-2">
      <div className="flex items-center justify-end gap-2 px-1 text-[11px] text-muted-foreground">
        <span>{formatCodingSessionRelative(message.timestamp)}</span>
        <span className="font-medium">{actorLabel}</span>
        {actor ? (
          <UserAvatar
            name={actorLabel}
            avatarUrl={actor.avatar_url}
            className="h-6 w-6"
            fallbackClassName="text-[10px]"
          />
        ) : (
          <span className="flex h-6 w-6 items-center justify-center rounded-full bg-blue-600 text-[10px] text-white">
            <UserIcon className="h-3 w-3" />
          </span>
        )}
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

function toolCallTimelineKey(toolCall: CodingSessionLiveToolCall) {
  return [
    toolCall.tool_name.trim().toLowerCase(),
    toolCall.args_text.trim(),
    toolCall.result?.output_summary?.trim() ?? '',
    toolCall.result?.content?.trim() ?? '',
  ].join('\n');
}

const CONTENT_COLLAPSE_CHAR_THRESHOLD = 600;

function CollapsibleMarkdown({ content }: { content: string }) {
  const [expanded, setExpanded] = useState(false);
  const isLong = content.length > CONTENT_COLLAPSE_CHAR_THRESHOLD;

  if (!isLong) {
    return <MarkdownContent content={content} className="text-[13px] leading-6 text-foreground" />;
  }

  return (
    <div>
      <div className={cn('relative', !expanded && 'max-h-[10rem] overflow-hidden')}>
        <MarkdownContent content={content} className="text-[13px] leading-6 text-foreground" />
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
              {streaming ? 'Live' : 'Finishing'}
            </Badge>
          ) : null}
        </div>

        {placeholder ? (
          <div className="whitespace-pre-wrap text-[13px] leading-6 text-muted-foreground">{content}</div>
        ) : (
          <CollapsibleMarkdown content={content} />
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
        ? 'rounded-bl-sm border border-border/60 bg-background text-foreground'
        : 'rounded-br-sm bg-blue-600 text-white dark:bg-blue-500',
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
                isAssistant ? 'from-background' : 'from-blue-600 dark:from-blue-500',
              )} />
            )}
          </div>
          <button
            type="button"
            className={cn(
              'mt-1 text-[11px] font-medium hover:underline',
              isAssistant ? 'text-primary' : 'text-white/80',
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
  const totalSec = Math.floor(ms / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  if (h > 0) return `${h}h ${m}m ${s}s`;
  if (m > 0) return `${m}m ${s.toString().padStart(2, '0')}s`;
  return `${s}s`;
}

/** Subscribes to a 1-second tick so elapsed time stays live. */
function useElapsedMs(since: string): number {
  const origin = useMemo(() => new Date(since).getTime(), [since]);
  const subscribe = useCallback((cb: () => void) => {
    const id = setInterval(cb, 1_000);
    return () => clearInterval(id);
  }, []);
  const getSnapshot = useCallback(() => Math.floor((Date.now() - origin) / 1000), [origin]);
  const tick = useSyncExternalStore(subscribe, getSnapshot);
  return tick * 1000;
}

function RunningIndicator({ since }: { since: string }) {
  const elapsed = useElapsedMs(since);
  return (
    <div className="flex items-center gap-2.5 border-t border-border bg-muted/50 px-4 py-2">
      <Loading01Icon className="h-3.5 w-3.5 animate-spin text-primary" />
      <span className="text-xs font-medium text-primary">Running</span>
      <span className="ml-auto text-xs tabular-nums text-muted-foreground">{formatElapsed(elapsed)}</span>
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
