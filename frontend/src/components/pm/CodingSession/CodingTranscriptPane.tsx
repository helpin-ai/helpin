import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import {
  BotIcon,
  SourceCodeIcon,
  Loading01Icon,
  SentIcon,
  TerminalIcon,
  UserIcon,
  CancelCircleIcon,
  LockKeyIcon,
  RadioIcon,
  Wrench01Icon,
} from '@/lib/icons';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';
import type {
  CodingSession,
  CodingSessionInteraction,
  CodingSessionLiveAssistantMessage,
  CodingSessionLiveReasoningMessage,
  CodingSessionLiveToolCall,
  CodingSessionLiveTurnSegment,
  CodingSessionTranscriptMessage,
} from '@/lib/pmTypes';
import { formatCodingSessionRelative } from './codingSessionUtils';
import { ApplyPatchDiff } from './ApplyPatchDiff';
import { CodingInteractionCard } from './CodingInteractionCard';
import { MarkdownContent } from './MarkdownContent';

export function CodingTranscriptPane({
  transcriptMessages,
  liveAssistantMessage,
  liveReasoningMessage,
  liveTurnSegments,
  loading = false,
  onSendMessage,
  sendingMessage = false,
  session,
  activeInteraction,
  acting,
  onAuthStart,
  onAuthCancel,
  onResolveInteraction,
}: {
  transcriptMessages: CodingSessionTranscriptMessage[];
  liveAssistantMessage: CodingSessionLiveAssistantMessage | null;
  liveReasoningMessage: CodingSessionLiveReasoningMessage | null;
  liveTurnSegments: CodingSessionLiveTurnSegment[];
  loading?: boolean;
  onSendMessage?: (content: string) => Promise<void>;
  sendingMessage?: boolean;
  session?: CodingSession | null;
  activeInteraction?: CodingSessionInteraction | null;
  acting?: string | null;
  onAuthStart?: () => void;
  onAuthCancel?: () => void;
  onResolveInteraction?: (interactionId: string, responsePayload: Record<string, unknown>, followupMessage?: string) => void;
}) {
  const scrollContainerRef = useRef<HTMLDivElement | null>(null);
  const followAnimationFrameRef = useRef<number | null>(null);
  const lastAutoScrollAtRef = useRef(0);
  const visibleLiveSegments = liveTurnSegments.filter((segment) => {
    if (segment.kind === 'assistant_message') {
      return segment.assistant_message.content.trim().length > 0;
    }
    return segment.tool_call.tool_name !== 'update_plan';
  });
  const showLivePlaceholder = visibleLiveSegments.length === 0 && liveAssistantMessage?.status === 'streaming';
  const scrollKey = useMemo(() => {
    const lastTranscript = transcriptMessages[transcriptMessages.length - 1];
    const lastLiveSegment = visibleLiveSegments[visibleLiveSegments.length - 1];
    const lastLiveSignature = lastLiveSegment
      ? lastLiveSegment.kind === 'assistant_message'
        ? `${lastLiveSegment.segment_id}:${lastLiveSegment.assistant_message.content.length}:${lastLiveSegment.assistant_message.status}`
        : `${lastLiveSegment.segment_id}:${lastLiveSegment.tool_call.status}:${lastLiveSegment.tool_call.result?.content.length ?? 0}`
      : '';

    return [
      transcriptMessages.length,
      lastTranscript?.event_id ?? '',
      lastTranscript?.content.length ?? 0,
      liveAssistantMessage?.message_id ?? '',
      liveAssistantMessage?.content.length ?? 0,
      liveReasoningMessage?.message_id ?? '',
      liveReasoningMessage?.content.length ?? 0,
      visibleLiveSegments.length,
      lastLiveSignature,
      showLivePlaceholder ? 1 : 0,
    ].join('|');
  }, [transcriptMessages, liveAssistantMessage, liveReasoningMessage, visibleLiveSegments, showLivePlaceholder]);

  useEffect(() => {
    const container = scrollContainerRef.current;
    if (!container) return;
    if (followAnimationFrameRef.current !== null) {
      window.cancelAnimationFrame(followAnimationFrameRef.current);
    }
    followAnimationFrameRef.current = window.requestAnimationFrame(() => {
      const now = window.performance.now();
      const useSmooth = now - lastAutoScrollAtRef.current > 120;
      if (typeof container.scrollTo === 'function') {
        container.scrollTo({
          top: container.scrollHeight,
          behavior: useSmooth ? 'smooth' : 'auto',
        });
      } else {
        container.scrollTop = container.scrollHeight;
      }
      lastAutoScrollAtRef.current = now;
      followAnimationFrameRef.current = null;
    });
    return () => {
      if (followAnimationFrameRef.current !== null) {
        window.cancelAnimationFrame(followAnimationFrameRef.current);
        followAnimationFrameRef.current = null;
      }
    };
  }, [scrollKey]);

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

      <div ref={scrollContainerRef} className="min-h-0 flex-1 overflow-auto px-4 py-4">
        <div className="mx-auto flex max-w-4xl flex-col gap-2">
          {transcriptMessages.map((message) => (
            <TranscriptEntry key={message.event_id} message={message} />
          ))}

          {liveReasoningMessage ? <ThinkingStrip reasoning={liveReasoningMessage} /> : null}

          {visibleLiveSegments.map((segment, idx) =>
            segment.kind === 'assistant_message' ? (
              <TranscriptEntry
                key={segment.segment_id}
                message={{
                  event_id: `live:${segment.segment_id}`,
                  message_id: segment.assistant_message.message_id,
                  role: 'assistant',
                  content: segment.assistant_message.content,
                  timestamp: segment.assistant_message.started_at ?? new Date().toISOString(),
                  sequence_no: Number.MAX_SAFE_INTEGER,
                }}
                live
                streaming={segment.assistant_message.status === 'streaming'}
              />
            ) : (
              <div key={segment.segment_id} className="w-full max-w-[90%]">
                <ActivityToolCallRow
                  toolCall={segment.tool_call}
                  isLast={idx === visibleLiveSegments.length - 1}
                />
              </div>
            ),
          )}

          {showLivePlaceholder ? (
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
          ) : null}

          {!loading && transcriptMessages.length === 0 && visibleLiveSegments.length === 0 && !showLivePlaceholder && !liveReasoningMessage ? (
            <div className="rounded-lg border border-dashed border-border px-5 py-8 text-center text-sm text-muted-foreground">
              No transcript yet. Assistant and user-visible turns will appear here once the session starts talking.
            </div>
          ) : null}
        </div>
      </div>

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
        <MessageInput onSend={onSendMessage} sending={sendingMessage} />
      ) : null}
    </section>
  );
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
      <div aria-hidden className="pointer-events-none absolute inset-x-0 bottom-full h-48" style={{ background: 'linear-gradient(to top, color-mix(in oklch, var(--card) 78%, oklch(0.93 0.03 70) 22%), transparent)' }} />

      <div className="border-t border-amber-200/70 bg-[color:color-mix(in_oklch,var(--card)_84%,oklch(0.94_0.03_72)_16%)] px-4 py-4 backdrop-blur-md dark:border-border/80 dark:bg-card/95">
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

function MessageInput({ onSend, sending }: { onSend: (content: string) => Promise<void>; sending: boolean }) {
  const [value, setValue] = useState('');

  const handleSubmit = async () => {
    const trimmed = value.trim();
    if (!trimmed || sending) return;
    await onSend(trimmed);
    setValue('');
  };

  return (
    <div className="border-t border-border bg-background px-3 py-3">
      <div className="flex gap-2">
        <Textarea
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => {
            if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
              e.preventDefault();
              void handleSubmit();
            }
          }}
          placeholder="Reply to agent… (⌘↵ to send)"
          className="min-h-[2.5rem] max-h-32 resize-none text-sm"
          disabled={sending}
          rows={1}
        />
        <Button
          size="sm"
          onClick={() => void handleSubmit()}
          disabled={!value.trim() || sending}
        >
          {sending ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <SentIcon className="h-3.5 w-3.5" />}
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
}: {
  message: CodingSessionTranscriptMessage;
  live?: boolean;
  streaming?: boolean;
  placeholder?: boolean;
}) {
  const isAssistant = message.role === 'assistant';
  const visibleToolCalls = (message.tool_calls ?? []).filter((tc) => tc.tool_name !== 'update_plan');
  const visibleTurnSegments = isAssistant
    ? (message.turn_segments ?? []).filter((segment) => (
        segment.kind !== 'tool_call' || segment.tool_call.tool_name !== 'update_plan'
      ))
    : [];
  const hasSegmentTimeline = visibleTurnSegments.length > 0;

  return (
    <div className={cn('flex flex-col gap-2', isAssistant ? 'items-start' : 'items-end')}>
      {/* Role + timestamp label */}
      <div className={cn('flex items-center gap-2 px-1 text-[11px] text-muted-foreground', isAssistant ? 'justify-start' : 'justify-end')}>
        <span className={cn(
          'flex h-6 w-6 items-center justify-center rounded-full text-[10px]',
          isAssistant ? 'bg-primary/10 text-primary' : 'bg-blue-600 text-white',
        )}>
          {isAssistant
            ? (live && streaming ? <RadioIcon className="h-3 w-3 animate-pulse" /> : <BotIcon className="h-3 w-3" />)
            : <UserIcon className="h-3 w-3" />}
        </span>
        <span className="font-medium">{message.role}</span>
        {live ? (
          <Badge variant="outline" className="h-5 px-1.5 text-[10px]">
            {streaming ? 'Live' : 'Finishing'}
          </Badge>
        ) : null}
        <span>{formatCodingSessionRelative(message.timestamp)}</span>
      </div>

      {hasSegmentTimeline ? (
        <div className="w-full max-w-[90%]">
          {visibleTurnSegments.map((segment, idx) => (
            segment.kind === 'assistant_message' ? (
              <AssistantMessageBubble
                key={segment.segment_id}
                content={segment.assistant_message.content}
              />
            ) : (
              <ActivityToolCallRow
                key={segment.segment_id}
                toolCall={segment.tool_call}
                isLast={idx === visibleTurnSegments.length - 1}
              />
            )
          ))}
        </div>
      ) : (
        <>
          {message.content.trim() ? (
            <AssistantMessageBubble
              content={message.content}
              isAssistant={isAssistant}
              placeholder={placeholder}
            />
          ) : null}

          {visibleToolCalls.length > 0 ? (
            <div className="w-full max-w-[90%]">
              {visibleToolCalls.map((tc, idx) => (
                <ActivityToolCallRow
                  key={tc.tool_call_id}
                  toolCall={tc}
                  isLast={idx === visibleToolCalls.length - 1}
                />
              ))}
            </div>
          ) : null}
        </>
      )}
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
  return (
    <div className={cn(
      'max-w-[90%] rounded-2xl px-3.5 py-2.5 text-sm leading-relaxed shadow-sm',
      isAssistant
        ? 'rounded-bl-sm border border-border/60 bg-background text-foreground'
        : 'rounded-br-sm border border-blue-200/80 bg-blue-50 text-blue-950 dark:border-blue-900/70 dark:bg-blue-950/30 dark:text-blue-50',
      placeholder && 'border-dashed text-muted-foreground',
    )}>
      {!placeholder
        ? <MarkdownContent content={content} className={isAssistant ? undefined : 'text-inherit'} />
        : <div className="whitespace-pre-wrap">{content}</div>}
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
      icon: <Loading01Icon className="h-3.5 w-3.5 animate-spin" />,
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

const TOOL_COLLAPSED_LINES = 4;

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
        'overflow-auto whitespace-pre-wrap break-all rounded-md border px-2.5 py-1.5 text-[11px] leading-5',
        failed
          ? 'border-destructive/30 bg-destructive/5 text-destructive dark:bg-destructive/10'
          : 'border-border bg-slate-950 text-slate-100',
        !expanded && isLong && 'max-h-[100px]',
        expanded && 'max-h-60',
      )}>
        {expanded || !isLong ? text : lines.slice(0, TOOL_COLLAPSED_LINES).join('\n')}
      </pre>
      {isLong && !expanded && (
        <div className={cn(
          'pointer-events-none absolute inset-x-0 bottom-0 h-8 rounded-b-md bg-gradient-to-t',
          failed ? 'from-destructive/5 to-transparent dark:from-destructive/10' : 'from-slate-950 to-transparent',
        )} />
      )}
      {isLong && (
        <button
          type="button"
          className="mt-0.5 text-[11px] font-medium text-primary hover:underline"
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
  const filePaths = !isApplyPatch && argsText ? extractFilePathsFromText(argsText) : [];

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
          <span className="text-xs font-medium capitalize text-foreground">
            {toolCall.tool_name.replaceAll('_', ' ')}
          </span>
          <div className="flex shrink-0 items-center gap-2">
            {toolCall.completed_at ?? toolCall.started_at ? (
              <span className="text-[11px] text-muted-foreground">
                {formatCodingSessionRelative(toolCall.completed_at ?? toolCall.started_at ?? '')}
              </span>
            ) : null}
          </div>
        </div>
        {filePaths.length > 0 && (
          <div className="mb-1.5 flex flex-wrap gap-1">
            {filePaths.map((fp) => (
              <span key={fp} className="inline-flex items-center rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium text-primary">
                {fp}
              </span>
            ))}
          </div>
        )}
        <div className="space-y-1.5 text-xs text-muted-foreground">
          {isApplyPatch
            ? <ApplyPatchDiff argsText={toolCall.args_text} />
            : argsText ? <CollapsibleCodeBlock text={argsText} /> : null}
          {resultText ? (
            isFailed
              ? <CollapsibleCodeBlock text={resultText} failed />
              : <p className="text-[11px] text-muted-foreground">{resultText.length > 200 ? `${resultText.slice(0, 200)}…` : resultText}</p>
          ) : null}
        </div>
      </div>
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
