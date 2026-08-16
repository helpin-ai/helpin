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
import { canonicalToolName } from '@/lib/toolNames';
import { formatCodingSessionRelative } from '@/components/pm/CodingSession/codingSessionUtils';
import { formatCodingSessionElapsed } from '@/components/pm/CodingSession/codingSessionPresentation';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { TranscriptRow } from './TranscriptRow';
import { toolStatusChrome } from './toolRowChrome';
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
  /** Show the original tool input/result/error inline inside a working group. */
  showToolDetails?: boolean;
  /** Show reasoning inline because the surrounding working group is disclosed. */
  showReasoningDetails?: boolean;
  /** Structural assistant role within its conversational interval. */
  assistantPresentation?: 'progress' | 'final';
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
          presentation={options.assistantPresentation ?? 'final'}
        />
      );
    case 'tool':
      return (
        <ToolSegment
          toolCall={segment.toolCall}
          group={options.toolGroup}
          showDetails={options.showToolDetails ?? options.expandable}
        />
      );
    case 'reasoning':
      return (
        <ReasoningSegment
          reasoning={segment.reasoning}
          expandable={options.expandable}
          showDetails={options.showReasoningDetails}
        />
      );
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
  presentation,
}: {
  content: string;
  streaming?: boolean;
  expandable: boolean;
  presentation: 'progress' | 'final';
}) {
  return (
    <div className="group/assistant">
      {expandable ? (
        <CollapsibleMarkdown content={content} streaming={streaming} presentation={presentation} />
      ) : (
        <MarkdownContent
          content={content}
          streaming={streaming}
          className={cn(
            'text-[13px] leading-6',
            presentation === 'progress' ? 'text-muted-foreground' : 'text-foreground',
          )}
        />
      )}
      {!streaming && presentation === 'final' ? (
        <div className="mt-1 flex justify-start">
          <CopyMessageButton content={content} />
        </div>
      ) : null}
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
      className="rounded-md border border-border/70 bg-background/95 p-1 text-muted-foreground opacity-0 shadow-sm transition-opacity hover:text-foreground focus-visible:opacity-100 group-hover/assistant:opacity-100"
    >
      {copied ? <Tick01Icon className="h-3 w-3 text-emerald-600 dark:text-emerald-400" /> : <Copy01Icon className="h-3 w-3" />}
    </button>
  );
}

// ─── Tool call ───────────────────────────────────────────────────────────────

function ToolSegment({
  toolCall,
  group,
  showDetails = false,
}: {
  toolCall: CodingSessionLiveToolCall;
  group?: TranscriptToolGroupPresentation;
  showDetails?: boolean;
}) {
  const status = group?.status ?? toolCall.status;
  const failed = status === 'failed';
  const { icon, className } = toolStatusChrome(status);
  const grouped = !!group && group.count > 1;
  const canonicalName = canonicalToolName(toolCall.tool_name).toLowerCase();
  const humanizedName = canonicalName
    .split(/[_-]+/)
    .filter(Boolean)
    .join(' ');
  const friendlyLabel = humanizedName.charAt(0).toUpperCase() + humanizedName.slice(1);
  const canShowError = showDetails && failed;

  return (
    <TranscriptRow
      icon={icon}
      iconClassName={className}
      label={(
        <>
          <span className="font-mono text-foreground/80">
            {canonicalName}{grouped ? ` ×${group.count}` : ''}
          </span>
          <span className="text-foreground/50"> · {friendlyLabel}</span>
        </>
      )}
      tone={failed ? 'failed' : 'muted'}
      expandable={canShowError}
      defaultOpen={false}
      lazyMount
    >
      {canShowError ? <ToolCallError error={toolCall.result?.error} /> : null}
    </TranscriptRow>
  );
}

function ToolCallError({ error }: { error?: string }) {
  return (
    <div className="space-y-1" data-tool-call-details>
      <div className="text-[10px] font-medium uppercase tracking-wide text-destructive">Error</div>
      <pre className="max-h-80 overflow-auto whitespace-pre-wrap break-words rounded-md border border-destructive/30 bg-destructive/5 px-2.5 py-2 font-mono text-[11px] leading-5 text-destructive">
        {error?.trim() || 'Error details unavailable.'}
      </pre>
    </div>
  );
}

// ─── Reasoning ───────────────────────────────────────────────────────────────

function ReasoningSegment({
  reasoning,
  expandable,
  showDetails = false,
}: {
  reasoning: CodingSessionLiveReasoningMessage;
  expandable: boolean;
  showDetails?: boolean;
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

  const detail = (
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
  );

  if (showDetails) {
    return (
      <div>
        <TranscriptRow
          icon={<LockKeyIcon className="h-3 w-3" />}
          iconClassName="text-muted-foreground"
          label={label}
          tone="muted"
          meta={streaming ? 'Live' : undefined}
        />
        <div className="ml-5 mt-1.5">{detail}</div>
      </div>
    );
  }

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
      {detail}
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

function CollapsibleMarkdown({
  content,
  streaming = false,
  presentation = 'final',
}: {
  content: string;
  streaming?: boolean;
  presentation?: 'progress' | 'final';
}) {
  const [expanded, setExpanded] = useState(false);
  const isLong = content.length > CONTENT_COLLAPSE_CHAR_THRESHOLD;
  const contentClassName = cn(
    'text-[13px] leading-6',
    presentation === 'progress' ? 'text-muted-foreground' : 'text-foreground',
  );

  if (!isLong) {
    return <MarkdownContent content={content} className={contentClassName} streaming={streaming} />;
  }

  return (
    <div>
      <div className={cn('relative', !expanded && 'max-h-[10rem] overflow-hidden')}>
        <MarkdownContent content={content} className={contentClassName} streaming={streaming} />
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
