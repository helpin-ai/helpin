import { useState } from 'react';
import { Copy01Icon, File01Icon, Loading01Icon, LockKeyIcon, Tick01Icon } from '@/lib/icons';

import { cn } from '@/lib/utils';
import type {
  CodingSessionActor,
  CodingSessionLiveReasoningMessage,
  CodingSessionLiveToolCall,
  CodingSessionTranscriptMessage,
} from '@/lib/pmTypes';
import { MarkdownContent } from '@/components/pm/CodingSession/MarkdownContent';
import { describeToolCall } from '@/components/pm/CodingSession/toolCallPresentation';
import { formatCodingSessionRelative } from '@/components/pm/CodingSession/codingSessionUtils';
import { formatCodingSessionElapsed } from '@/components/pm/CodingSession/codingSessionPresentation';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { TranscriptRow } from './TranscriptRow';
import { formatToolDuration, toolStatusChrome } from './toolRowChrome';
import type { TranscriptSegment } from './segments';

export interface TranscriptToolGroupPresentation {
  count: number;
  totalDurationMs?: number;
  status: CodingSessionLiveToolCall['status'];
}

export interface RenderSegmentOptions {
  /** When true, reasoning and run-context rows expand on click. */
  expandable: boolean;
  /** Whether long assistant text should use the Show more / Show less control. */
  collapseLongAssistantContent?: boolean;
  /** Resolves the actor for user / review-decision segments (slider only). */
  resolveActor?: (message: CodingSessionTranscriptMessage) => CodingSessionActor | null;
  /** Surface-specific label when actor details are intentionally unavailable. */
  fallbackUserLabel?: string;
  /** Dock-only aggregation metadata for adjacent calls to the same tool. */
  toolGroup?: TranscriptToolGroupPresentation;
}

/** Renders a single normalized transcript segment as a flat one-line entry. */
export function TranscriptSegmentView({
  segment,
  options,
}: {
  segment: TranscriptSegment;
  options: RenderSegmentOptions;
}) {
  switch (segment.kind) {
    case 'assistant':
      return (
        <AssistantSegment
          content={segment.content}
          streaming={segment.streaming}
          expandable={options.collapseLongAssistantContent ?? options.expandable}
        />
      );
    case 'tool':
      return <ToolSegment toolCall={segment.toolCall} group={options.toolGroup} />;
    case 'reasoning':
      return <ReasoningSegment reasoning={segment.reasoning} expandable={options.expandable} />;
    case 'status':
      return <StatusSegment message={segment.message} />;
    case 'context':
      return <ContextSegment message={segment.message} expandable={options.expandable} />;
    case 'user':
      return (
        <UserSegment
          message={segment.message}
          actor={options.resolveActor?.(segment.message) ?? null}
          fallbackLabel={options.fallbackUserLabel}
        />
      );
    case 'review_decision':
      return <ReviewDecisionSegment message={segment.message} actor={options.resolveActor?.(segment.message) ?? null} />;
  }
}

// ─── Assistant ───────────────────────────────────────────────────────────────

function AssistantSegment({
  content,
  streaming,
  expandable,
}: {
  content: string;
  streaming?: boolean;
  expandable: boolean;
}) {
  return (
    <div className="group/assistant relative">
      {expandable ? (
        <CollapsibleMarkdown content={content} streaming={streaming} />
      ) : (
        <MarkdownContent content={content} streaming={streaming} className="text-[13px] leading-6 text-foreground/90" />
      )}
      {!streaming && <CopyMessageButton content={content} />}
    </div>
  );
}

function CopyMessageButton({ content }: { content: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      type="button"
      aria-label="Copy message"
      title="Copy message"
      onClick={() => {
        void navigator.clipboard.writeText(content).then(() => {
          setCopied(true);
          window.setTimeout(() => setCopied(false), 1500);
        });
      }}
      className="absolute -right-1 -top-1 rounded-md border border-border/70 bg-background/95 p-1 text-muted-foreground opacity-0 shadow-sm transition-opacity hover:text-foreground focus-visible:opacity-100 group-hover/assistant:opacity-100"
    >
      {copied ? <Tick01Icon className="h-3 w-3 text-emerald-600 dark:text-emerald-400" /> : <Copy01Icon className="h-3 w-3" />}
    </button>
  );
}

// ─── Tool call ───────────────────────────────────────────────────────────────

function ToolSegment({
  toolCall,
  group,
}: {
  toolCall: CodingSessionLiveToolCall;
  group?: TranscriptToolGroupPresentation;
}) {
  const status = group?.status ?? toolCall.status;
  const failed = status === 'failed';
  const { icon, className } = toolStatusChrome(status);
  const presentation = describeToolCall(toolCall);
  const grouped = !!group && group.count > 1;

  return (
    <TranscriptRow
      icon={icon}
      iconClassName={className}
      label={grouped
        ? `${presentation.secondaryLabel}${presentation.repositoryLabel ? ` · ${presentation.repositoryLabel}` : ''} x ${group.count}`
        : presentation.primaryLabel}
      tone={failed ? 'failed' : 'muted'}
      meta={formatToolDuration(grouped ? group.totalDurationMs : toolCall.duration_ms)}
    />
  );
}

// ─── Reasoning ───────────────────────────────────────────────────────────────

function ReasoningSegment({
  reasoning,
  expandable,
}: {
  reasoning: CodingSessionLiveReasoningMessage;
  expandable: boolean;
}) {
  const streaming = reasoning.status === 'streaming';
  const hasContent = reasoning.content.trim().length > 0;
  // Open while streaming, auto-collapse on completion — unless the user has
  // toggled the row themselves, in which case their choice wins.
  const [open, setOpen] = useState(streaming);
  const [userToggled, setUserToggled] = useState(false);
  const [prevStreaming, setPrevStreaming] = useState(streaming);
  if (prevStreaming !== streaming) {
    setPrevStreaming(streaming);
    if (!userToggled) setOpen(streaming);
  }

  const thoughtDurationMs =
    reasoning.started_at && reasoning.completed_at
      ? Date.parse(reasoning.completed_at) - Date.parse(reasoning.started_at)
      : 0;
  const label = streaming
    ? 'Thinking…'
    : thoughtDurationMs > 0
      ? `Thought for ${formatCodingSessionElapsed(thoughtDurationMs)}`
      : 'Thought';

  return (
    <TranscriptRow
      icon={<LockKeyIcon className="h-3 w-3" />}
      iconClassName="text-muted-foreground"
      label={label}
      tone="muted"
      meta={streaming ? 'Live' : undefined}
      expandable={expandable}
      open={expandable ? open : undefined}
      onOpenChange={(next) => {
        setUserToggled(true);
        setOpen(next);
      }}
    >
      <div className="text-xs text-muted-foreground">
        {hasContent ? (
          <div className={cn('whitespace-pre-wrap leading-6', streaming && 'italic')}>{reasoning.content}</div>
        ) : (
          <div className="rounded-lg border border-border bg-card px-3 py-2">
            Reasoning is being tracked separately from the assistant reply.
          </div>
        )}
        {reasoning.encrypted_value ? (
          <div className="mt-2 flex items-center gap-2 rounded-lg border border-border bg-card px-3 py-2">
            <LockKeyIcon className="h-3.5 w-3.5" />
            Encrypted reasoning payload attached.
          </div>
        ) : null}
      </div>
    </TranscriptRow>
  );
}

// ─── Status ──────────────────────────────────────────────────────────────────

function StatusSegment({ message }: { message: CodingSessionTranscriptMessage }) {
  return (
    <TranscriptRow
      icon={<Loading01Icon className="h-3 w-3" />}
      iconClassName="text-muted-foreground"
      label={message.content}
      meta={formatCodingSessionRelative(message.timestamp)}
      tone="muted"
    />
  );
}

// ─── Run context (prompt) ────────────────────────────────────────────────────

function ContextSegment({
  message,
  expandable,
}: {
  message: CodingSessionTranscriptMessage;
  expandable: boolean;
}) {
  const label = message.message_type === 'system_prompt'
    ? 'System prompt'
    : message.message_type === 'prompt'
      ? 'Prompt'
      : 'Developer prompt';
  return (
    <TranscriptRow
      icon={<File01Icon className="h-3 w-3" />}
      iconClassName="text-muted-foreground"
      label="Run context"
      tone="muted"
      expandable={expandable}
    >
      <div>
        <div className="mb-1 text-[11px] font-medium text-muted-foreground">{label}</div>
        <pre className="max-h-72 overflow-auto whitespace-pre-wrap break-words text-[11px] leading-5 text-foreground/80">
          {message.content}
        </pre>
      </div>
    </TranscriptRow>
  );
}

// ─── User message ────────────────────────────────────────────────────────────

function UserSegment({
  message,
  actor,
  fallbackLabel = 'User',
}: {
  message: CodingSessionTranscriptMessage;
  actor: CodingSessionActor | null;
  fallbackLabel?: string;
}) {
  const actorLabel = actor?.full_name || actor?.email || fallbackLabel;
  return (
    <div className="flex flex-col items-end gap-2">
      <div className="flex items-center justify-end gap-2 px-1 text-[11px] text-muted-foreground">
        <span>{formatCodingSessionRelative(message.timestamp)}</span>
        <span className="font-medium">{actorLabel}</span>
        <UserAvatar
          name={actorLabel}
          avatarUrl={actor?.avatar_url}
          avatarStyle={actor?.avatar_style}
          avatarSeed={actor?.avatar_seed}
          avatarBackgroundMode={actor?.avatar_background_mode}
          avatarBackgroundColor={actor?.avatar_background_color}
          className="h-6 w-6"
          fallbackClassName="text-[10px]"
        />
      </div>
      {message.content.trim() ? <UserMessageBubble content={message.content} /> : null}
    </div>
  );
}

// ─── Review / approval decision ──────────────────────────────────────────────

function ReviewDecisionSegment({
  message,
  actor,
}: {
  message: CodingSessionTranscriptMessage;
  actor: CodingSessionActor | null;
}) {
  const reviewerName = actor?.full_name || actor?.email || 'Reviewer';
  const normalizedContent = message.content.trim().toLowerCase();
  const decisionLabel = normalizedContent.startsWith('requested changes')
    ? 'requested changes'
    : normalizedContent.startsWith('approved')
      ? 'approved'
      : 'reviewed';
  return (
    <div className="ml-auto w-full max-w-[90%]">
      <div className="mb-2 flex items-center justify-end gap-2 px-1 text-[11px] text-muted-foreground">
        <span>{formatCodingSessionRelative(message.timestamp)}</span>
        <span className="font-medium">{reviewerName} {decisionLabel}</span>
        <UserAvatar
          name={reviewerName}
          avatarUrl={actor?.avatar_url}
          avatarStyle={actor?.avatar_style}
          avatarSeed={actor?.avatar_seed}
          avatarBackgroundMode={actor?.avatar_background_mode}
          avatarBackgroundColor={actor?.avatar_background_color}
          className="h-6 w-6"
          fallbackClassName="text-[10px]"
        />
      </div>
      <div className="rounded-2xl rounded-br-sm bg-blue-50 px-3.5 py-2.5 text-sm leading-relaxed text-foreground/85 shadow-sm dark:bg-blue-950/40 dark:text-foreground">
        <MarkdownContent content={message.content} className="text-inherit" />
      </div>
    </div>
  );
}

// ─── Shared collapsibles ─────────────────────────────────────────────────────

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

function UserMessageBubble({ content }: { content: string }) {
  const [expanded, setExpanded] = useState(false);
  const isLong = content.length > CONTENT_COLLAPSE_CHAR_THRESHOLD;

  return (
    <div className="max-w-[90%] rounded-2xl rounded-br-sm bg-blue-50 px-3.5 py-2.5 text-sm leading-relaxed text-foreground/85 shadow-sm dark:bg-blue-950/40 dark:text-foreground">
      {isLong ? (
        <div>
          <div className={cn('relative', !expanded && 'max-h-[10rem] overflow-hidden')}>
            <MarkdownContent content={content} className="text-inherit" />
            {!expanded && (
              <div className="pointer-events-none absolute inset-x-0 bottom-0 h-14 bg-gradient-to-t from-blue-50 to-transparent dark:from-blue-950/40" />
            )}
          </div>
          <button
            type="button"
            className="mt-1 text-[11px] font-medium text-blue-700 hover:underline dark:text-blue-200"
            onClick={() => setExpanded((prev) => !prev)}
          >
            {expanded ? 'Show less' : 'Show more'}
          </button>
        </div>
      ) : (
        <MarkdownContent content={content} className="text-inherit" />
      )}
    </div>
  );
}
