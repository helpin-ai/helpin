import { type ComponentPropsWithoutRef, type CSSProperties, type ReactNode, startTransition, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { format, formatDistanceToNow, isSameDay, parseISO } from 'date-fns';
import { Bot, FileText, Loader2, Send, ShieldCheck, Sparkles, Wrench } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

import { AgentRunDetail } from '@/components/pm/AgentRunDetail';
import { StructuredQuestionCard } from '@/components/pm/StructuredQuestionCard';
import { ACTIVE_RUN_STATUSES, STATUS_META, getAgentRunDisplayStatus, isPausedAgentRun } from '@/components/pm/agentRunConstants';
import { parseMessageApprovalRequest, parseMessageStructuredQuestions, type ParsedApprovalRequest } from '@/components/pm/agentRunInteractions';
import { isPublishedPreviewToolName, parseArtifactPublishedPreview, parsePublishedPreviewRawInput, resolveMessagePublishedPreview, type PublishedPreview } from '@/components/pm/agentRunPreviews';
import { StreamingTagRouter, INITIAL_SEGMENTS, type StreamSegments } from '@/components/pm/streamingTagRouter';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { Textarea } from '@/components/ui/textarea';
import type {
  AgentRun,
  AgentRunArtifact,
  AgentRunMessage,
  AgentRunStreamEvent,
} from '@/lib/pmTypes';
import { agentService } from '@/lib/services/agentService';
import { cn } from '@/lib/utils';

interface AgentRunDrawerProps {
  workspaceId: string;
  runId: string | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  canEdit?: boolean;
  title?: string;
  description?: string;
}

interface MarkdownPanelPreview {
  title?: string;
  summary?: string;
  markdown: string;
}

interface LiveToolEvent {
  id: string;
  name: string;
  input?: string;
  output?: string;
  durationMs?: number;
  status: 'running' | 'completed';
}

interface StoryPlanStoryPreview {
  ref?: string;
  title: string;
  type?: string;
  description?: string;
  acceptanceCriteria: string[];
  dependencyRefs: string[];
  filesToModify: string[];
}

interface StoryPlanPreviewModel {
  summary?: string;
  proposedStories: StoryPlanStoryPreview[];
  risks: string[];
  openQuestions: string[];
}

type TranscriptEntry =
  | { type: 'separator'; label: string }
  | {
      type: 'message';
      message: AgentRunMessage;
      messageIndex: number;
      isConsecutive: boolean;
      isLastInGroup: boolean;
    };

interface ParsedApprovalRequestWithMeta extends ParsedApprovalRequest {
  messageId: string;
  index: number;
}

const RIGHT_PANEL_MIN_WIDTH_PCT = 24;
const RIGHT_PANEL_MAX_WIDTH_PCT = 48;
const RIGHT_PANEL_DEFAULT_WIDTH_PCT = 34;

interface AutonomousRuntimeStreamDisplay {
  assistantText: string;
  processingText: string;
  errorText: string;
}

function toolEventSignature(name: string, content?: string) {
  return `${name.trim().toLowerCase()}\u0000${(content ?? '').trim()}`;
}

function formatMessageTimestamp(value: string) {
  try {
    return formatDistanceToNow(parseISO(value), { addSuffix: true });
  } catch {
    return value;
  }
}

function roleLabel(role: string) {
  switch (role) {
    case 'assistant':
      return 'Agent';
    case 'user':
      return 'You';
    case 'tool':
      return 'Tool';
    default:
      return role;
  }
}

function formatThreadDayLabel(value: string) {
  try {
    const date = parseISO(value);
    const today = new Date();
    const yesterday = new Date();
    yesterday.setDate(today.getDate() - 1);
    if (isSameDay(date, today)) return 'Today';
    if (isSameDay(date, yesterday)) return 'Yesterday';
    return format(date, 'MMM d');
  } catch {
    return value;
  }
}

function isTranscriptChatRole(role: AgentRunMessage['role']) {
  return role === 'assistant' || role === 'user';
}

function buildTranscriptEntries(messages: AgentRunMessage[]): TranscriptEntry[] {
  const items: TranscriptEntry[] = [];
  let lastDate: string | null = null;

  messages.forEach((message, index) => {
    if (!lastDate || !isSameDay(parseISO(lastDate), parseISO(message.created_at))) {
      items.push({ type: 'separator', label: formatThreadDayLabel(message.created_at) });
      lastDate = message.created_at;
    }

    const prev = index > 0 ? messages[index - 1] : null;
    const next = index < messages.length - 1 ? messages[index + 1] : null;
    const canGroupCurrent = isTranscriptChatRole(message.role);
    const canGroupPrev = prev ? isTranscriptChatRole(prev.role) : false;
    const canGroupNext = next ? isTranscriptChatRole(next.role) : false;

    const isConsecutive =
      canGroupCurrent &&
      canGroupPrev &&
      prev !== null &&
      prev.role === message.role &&
      isSameDay(parseISO(prev.created_at), parseISO(message.created_at)) &&
      Math.abs(new Date(message.created_at).getTime() - new Date(prev.created_at).getTime()) < 120000;

    const isLastInGroup =
      !canGroupCurrent ||
      !canGroupNext ||
      next === null ||
      next.role !== message.role ||
      !isSameDay(parseISO(message.created_at), parseISO(next.created_at)) ||
      Math.abs(new Date(next.created_at).getTime() - new Date(message.created_at).getTime()) >= 120000;

    items.push({
      type: 'message',
      message,
      messageIndex: index,
      isConsecutive,
      isLastInGroup,
    });
  });

  return items;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function asStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value.map((entry) => asString(entry).trim()).filter((entry) => entry.length > 0);
}

function toTitleCase(value: string) {
  return value
    .split(/[_\s-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');
}

function parseStoryPlanPreviewModel(preview: PublishedPreview | undefined): StoryPlanPreviewModel | null {
  if (!preview || preview.format !== 'json') return null;
  const record = asRecord(preview.content);
  const proposedStories = Array.isArray(record?.proposed_stories) ? record.proposed_stories : [];
  if (!record || proposedStories.length === 0) return null;

  const normalizedStories: StoryPlanStoryPreview[] = proposedStories.flatMap((entry) => {
      const story = asRecord(entry);
      if (!story) return [];

      const implementationBrief = asRecord(story.implementation_brief);
      const filesToModify = Array.isArray(implementationBrief?.files_to_modify)
        ? implementationBrief.files_to_modify
          .map((fileEntry) => asRecord(fileEntry))
          .map((fileEntry) => asString(fileEntry?.path).trim())
          .filter((path) => path.length > 0)
        : [];

      const title = asString(story.name).trim() || asString(story.title).trim();
      if (!title) return [];

      return [{
        ref: asString(story.ref).trim() || undefined,
        title,
        type: asString(story.story_type).trim() || asString(story.type).trim() || asString(story.slice_type).trim() || undefined,
        description: asString(story.description).trim() || undefined,
        acceptanceCriteria: asStringArray(story.acceptance_criteria),
        dependencyRefs: asStringArray(story.dependency_refs),
        filesToModify,
      } satisfies StoryPlanStoryPreview];
    })

  if (normalizedStories.length === 0) return null;

  return {
    summary: asString(record.summary).trim() || undefined,
    proposedStories: normalizedStories,
    risks: asStringArray(record.risks),
    openQuestions: asStringArray(record.open_questions),
  };
}

function TranscriptDaySeparator({ label }: { label: string }) {
  return (
    <div className="flex items-center justify-center py-3">
      <span className="rounded-full bg-background/90 px-3 py-1 text-[10.5px] font-medium text-muted-foreground shadow-sm ring-1 ring-border/50">
        {label}
      </span>
    </div>
  );
}

function TranscriptBubble({
  role,
  timestamp,
  isConsecutive,
  isLastInGroup,
  children,
}: {
  role: AgentRunMessage['role'];
  timestamp: string;
  isConsecutive: boolean;
  isLastInGroup: boolean;
  children: ReactNode;
}) {
  const isUser = role === 'user';
  const label = roleLabel(role);
  const avatar = isUser ? (
    <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-blue-600 text-[11px] font-semibold text-white shadow-sm">
      Y
    </div>
  ) : (
    <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary shadow-sm">
      <Bot className="h-3.5 w-3.5" />
    </div>
  );

  return (
    <div className={cn('flex items-end gap-2', isConsecutive ? 'mt-1.5' : 'mt-5', isUser ? 'justify-end' : 'justify-start')}>
      {!isUser ? (isLastInGroup ? avatar : <div className="w-7 shrink-0" />) : null}
      <div className={cn('min-w-0 max-w-[78%]', isUser ? 'items-end' : '')}>
        {!isConsecutive ? (
          <div className={cn('mb-1 flex items-center gap-2 px-1', isUser ? 'justify-end' : 'justify-start')}>
            <span className="text-[11px] font-medium text-muted-foreground">{label}</span>
            <span className="text-[11px] text-muted-foreground/80">{timestamp}</span>
          </div>
        ) : null}
        <div
          className={cn(
            'rounded-2xl px-3.5 py-2.5 text-sm leading-relaxed shadow-sm',
            isUser
              ? 'rounded-br-sm border border-blue-200/80 bg-blue-50 text-blue-950 dark:border-blue-900/70 dark:bg-blue-950/30 dark:text-blue-50'
              : 'rounded-bl-sm border border-border/60 bg-background text-foreground',
          )}
        >
          {children}
        </div>
      </div>
      {isUser ? (isLastInGroup ? avatar : <div className="w-7 shrink-0" />) : null}
    </div>
  );
}

function ProcessingBox({
  children,
  className,
  active = false,
}: {
  children: ReactNode;
  className?: string;
  active?: boolean;
}) {
  return (
    <details className={cn('mt-3 rounded-xl border border-border/60 bg-muted/25 px-3 py-2', className)}>
      <summary className="flex cursor-pointer list-none items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
        <div className="flex h-5 w-5 items-center justify-center rounded-full bg-primary/10 text-primary">
          {active ? <Loader2 className="h-3 w-3 animate-spin" /> : <Bot className="h-3 w-3" />}
        </div>
        <span>Agent Processing</span>
        <span className="ml-auto text-[10px] normal-case tracking-normal text-muted-foreground/80">
          {active ? 'Running' : 'Show details'}
        </span>
      </summary>
      <div className="mt-3 space-y-2">{children}</div>
    </details>
  );
}

export function parseToolMessage(message: AgentRunMessage) {
  const blocks = Array.isArray(message.content_blocks) ? message.content_blocks : [];
  for (const rawBlock of blocks) {
    const block = asRecord(rawBlock);
    if (!block || asString(block.type) !== 'tool_result') continue;
    return {
      name: asString(block.tool_name) || 'tool',
      content: asString(block.output) || message.content || '',
      input: asString(block.input),
      isError: block.is_error === true,
    };
  }
  return {
    name: 'tool',
    content: message.content || '',
    input: '',
    isError: false,
  };
}

function collectRenderedPersistedToolCounts(messages: AgentRunMessage[]) {
  const counts = new Map<string, number>();

  for (let index = 0; index < messages.length; index += 1) {
    const message = messages[index];
    if (message.role === 'tool') {
      const parsed = parseToolMessage(message);
      const signature = toolEventSignature(parsed.name, parsed.content);
      counts.set(signature, (counts.get(signature) ?? 0) + 1);
      continue;
    }

    const toolInvocations = Array.isArray(message.tool_invocations) ? message.tool_invocations : [];
    const shouldRenderAssistantToolInvocations =
      toolInvocations.length > 0 && messages[index + 1]?.role !== 'tool';

    if (!shouldRenderAssistantToolInvocations) continue;

    for (const invocation of toolInvocations) {
      const toolName = typeof invocation.tool_name === 'string' ? invocation.tool_name : 'tool';
      const outputSummary = typeof invocation.output_summary === 'string' ? invocation.output_summary : '';
      const signature = toolEventSignature(toolName, outputSummary);
      counts.set(signature, (counts.get(signature) ?? 0) + 1);
    }
  }

  return counts;
}

export function getVisibleLiveTools(liveTools: LiveToolEvent[], messages: AgentRunMessage[]) {
  const persistedCounts = collectRenderedPersistedToolCounts(messages);

  return liveTools.filter((tool) => {
    if (tool.status !== 'completed') return true;
    const signature = toolEventSignature(tool.name, tool.output);
    const persistedCount = persistedCounts.get(signature) ?? 0;
    if (persistedCount <= 0) return true;
    persistedCounts.set(signature, persistedCount - 1);
    return false;
  });
}

function latestArtifactContent(artifacts: AgentRunArtifact[], artifactType: string): string {
  const latest = artifacts
    .filter((artifact) => artifact.artifact_type === artifactType && typeof artifact.inline_content === 'string' && artifact.inline_content.trim().length > 0)
    .sort((left, right) => {
      if (left.sequence_no !== right.sequence_no) return right.sequence_no - left.sequence_no;
      return new Date(right.created_at).getTime() - new Date(left.created_at).getTime();
    })[0];
  return latest?.inline_content?.trim() ?? '';
}

function parseCodexChunkLine(line: string): { assistantText?: string; eventText?: string; errorText?: string } | null {
  const trimmed = line.trim();
  if (!trimmed || !trimmed.startsWith('{')) return null;

  try {
    const payload = JSON.parse(trimmed) as Record<string, unknown>;
    const eventType = asString(payload.type) || '';
    const item = asRecord(payload.item);
    const itemType = asString(item?.type) || '';

    if (eventType === 'error') {
      return { errorText: asString(payload.message) || asString(asRecord(payload.error)?.message) || 'Codex reported an error.' };
    }

    if (eventType === 'turn.failed') {
      return { errorText: asString(asRecord(payload.error)?.message) || 'Codex turn failed.' };
    }

    if (itemType === 'agent_message') {
      const assistantText = asString(item?.text) || '';
      return assistantText ? { assistantText } : null;
    }

    if (itemType === 'command_execution') {
      const command =
        asString(item?.command) ||
        asString(item?.title) ||
        asString(asRecord(item?.metadata)?.command) ||
        'command';
      if (eventType === 'item.started') {
        return { eventText: `Running ${command}` };
      }
      if (eventType === 'item.completed') {
        return { eventText: `Finished ${command}` };
      }
    }

    if (eventType === 'turn.started') return { eventText: 'Codex started the turn.' };
    if (eventType === 'turn.completed') return { eventText: 'Codex finished the turn.' };
    if (eventType === 'thread.started') return { eventText: 'Codex session started.' };
  } catch {
    return null;
  }

  return null;
}

export function buildAutonomousRuntimeStreamDisplay(
  run: Pick<AgentRun, 'runtime_kind' | 'invocation_mode' | 'status'> | null,
  artifacts: AgentRunArtifact[],
): AutonomousRuntimeStreamDisplay | null {
  if (!run || run.invocation_mode !== 'autonomous') return null;
  if (run.runtime_kind !== 'opencode' && run.runtime_kind !== 'codex') return null;

  const stdout = latestArtifactContent(artifacts, `${run.runtime_kind}_stdout`);
  const stderr = latestArtifactContent(artifacts, `${run.runtime_kind}_stderr`);

  if (run.runtime_kind === 'opencode') {
    if (!stdout && !stderr) return null;
    return {
      assistantText: '',
      processingText: stdout,
      errorText: stderr,
    };
  }

  const assistantParts: string[] = [];
  const eventParts: string[] = [];
  const errorParts: string[] = stderr ? [stderr] : [];

  for (const line of stdout.split('\n')) {
    const parsed = parseCodexChunkLine(line);
    if (!parsed) continue;
    if (parsed.assistantText) assistantParts.push(parsed.assistantText);
    if (parsed.eventText) eventParts.push(parsed.eventText);
    if (parsed.errorText) errorParts.push(parsed.errorText);
  }

  const unique = (values: string[]) =>
    values.filter((value, index) => value.trim().length > 0 && values.findIndex((candidate) => candidate.trim() === value.trim()) === index);

  const assistantText = unique(assistantParts).join('\n\n').trim();
  const processingText = unique(eventParts).join('\n').trim();
  const errorText = unique(errorParts).join('\n').trim();

  if (!assistantText && !processingText && !errorText && stdout.trim()) {
    return {
      assistantText: '',
      processingText: stdout.trim(),
      errorText,
    };
  }

  if (!assistantText && !processingText && !errorText) return null;

  return { assistantText, processingText, errorText };
}

function MarkdownContent({ content, className }: { content: string; className?: string }) {
  return (
    <div className={cn('text-[13px] leading-6 text-foreground', className)}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          p: ({ children }) => <p className="mb-3 last:mb-0">{children}</p>,
          ul: ({ children }) => <ul className="mb-3 list-disc space-y-1 pl-5 last:mb-0">{children}</ul>,
          ol: ({ children }) => <ol className="mb-3 list-decimal space-y-1 pl-5 last:mb-0">{children}</ol>,
          li: ({ children }) => <li>{children}</li>,
          h1: ({ children }) => <h1 className="mb-3 text-base font-semibold last:mb-0">{children}</h1>,
          h2: ({ children }) => <h2 className="mb-3 text-sm font-semibold last:mb-0">{children}</h2>,
          h3: ({ children }) => <h3 className="mb-2 text-sm font-semibold last:mb-0">{children}</h3>,
          table: ({ children }) => (
            <div className="mb-3 overflow-x-auto rounded-md border border-border/60 last:mb-0">
              <table className="min-w-full border-collapse">{children}</table>
            </div>
          ),
          thead: ({ children }) => <thead className="bg-muted/50">{children}</thead>,
          tbody: ({ children }) => <tbody>{children}</tbody>,
          tr: ({ children }) => <tr className="border-b border-border/60 last:border-b-0">{children}</tr>,
          th: ({ children }) => <th className="px-3 py-2 text-left text-[12px] font-semibold text-foreground">{children}</th>,
          td: ({ children }) => <td className="px-3 py-2 align-top text-[13px] leading-5 text-foreground">{children}</td>,
          blockquote: ({ children }) => (
            <blockquote className="mb-3 border-l-2 border-border pl-3 text-muted-foreground last:mb-0">
              {children}
            </blockquote>
          ),
          a: ({ children, href }) => (
            <a
              href={href}
              target="_blank"
              rel="noreferrer"
              className="text-primary underline underline-offset-2"
            >
              {children}
            </a>
          ),
          pre: ({ children }) => (
            <pre className="mb-3 overflow-x-auto rounded-md bg-zinc-950 px-3 py-2 text-[12px] leading-5 text-zinc-50 last:mb-0">
              {children}
            </pre>
          ),
          code: ({ inline, className: codeClassName, children, ...props }: ComponentPropsWithoutRef<'code'> & { inline?: boolean }) =>
            inline ? (
              <code className="rounded bg-muted px-1 py-0.5 text-[12px]" {...props}>
                {children}
              </code>
            ) : (
              <code className={cn('text-[12px]', codeClassName)} {...props}>
                {children}
              </code>
            ),
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}

function isInlineApprovalRun(run: AgentRun | null): boolean {
  return !!run && run.invocation_mode === 'interactive' && isPausedAgentRun(run);
}

const RUNTIME_STREAM_PREFIXES = ['opencode', 'codex'] as const;

function mergeArtifactsForDisplay(artifacts: AgentRunArtifact[]): AgentRunArtifact[] {
  const displayArtifacts = artifacts.filter(
    (artifact) =>
      artifact.artifact_type !== 'codex_session_state' &&
      !RUNTIME_STREAM_PREFIXES.some(
        (prefix) =>
          artifact.artifact_type === `${prefix}_stdout_chunk` ||
          artifact.artifact_type === `${prefix}_stderr_chunk`,
      ),
  );

  for (const [index, prefix] of RUNTIME_STREAM_PREFIXES.entries()) {
    const stdoutArtifact = displayArtifacts.find((artifact) => artifact.artifact_type === `${prefix}_stdout`);
    const stderrArtifact = displayArtifacts.find((artifact) => artifact.artifact_type === `${prefix}_stderr`);

    if (!stdoutArtifact) {
      const stdoutContent = artifacts
        .filter((artifact) => artifact.artifact_type === `${prefix}_stdout_chunk`)
        .map((artifact) => artifact.inline_content ?? '')
        .join('');
      if (stdoutContent) {
        displayArtifacts.unshift({
          id: `live-${prefix}-stdout`,
          workspace_id: artifacts[0]?.workspace_id ?? '',
          run_id: artifacts[0]?.run_id ?? '',
          artifact_type: `${prefix}_stdout`,
          format: 'text',
          storage_mode: 'inline',
          inline_content: stdoutContent,
          metadata: {},
          sequence_no: -2 - index * 2,
          created_at: artifacts[0]?.created_at ?? new Date().toISOString(),
        });
      }
    }

    if (!stderrArtifact) {
      const stderrContent = artifacts
        .filter((artifact) => artifact.artifact_type === `${prefix}_stderr_chunk`)
        .map((artifact) => artifact.inline_content ?? '')
        .join('');
      if (stderrContent) {
        displayArtifacts.unshift({
          id: `live-${prefix}-stderr`,
          workspace_id: artifacts[0]?.workspace_id ?? '',
          run_id: artifacts[0]?.run_id ?? '',
          artifact_type: `${prefix}_stderr`,
          format: 'text',
          storage_mode: 'inline',
          inline_content: stderrContent,
          metadata: {},
          sequence_no: -1 - index * 2,
          created_at: artifacts[0]?.created_at ?? new Date().toISOString(),
        });
      }
    }
  }

  return displayArtifacts;
}

/** Isolated reply form — local state prevents parent re-renders on every keystroke. */
function ReplyForm({ onSubmit }: { onSubmit: (content: string) => void }) {
  const [value, setValue] = useState('');
  const [sending, setSending] = useState(false);
  const handleSubmit = async () => {
    if (!value.trim() || sending) return;
    setSending(true);
    try {
      onSubmit(value.trim());
      setValue('');
    } finally {
      setSending(false);
    }
  };
  return (
    <div className="rounded-2xl border border-border/60 bg-background p-3 shadow-sm">
      <div className="mb-2 flex items-center justify-between gap-3">
        <Label className="text-xs font-medium text-muted-foreground">Reply to agent</Label>
        <span className="text-[11px] text-muted-foreground">Ctrl/Cmd + Enter to send</span>
      </div>
      <Textarea
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder="Clarify scope, answer a question, or request changes."
        rows={3}
        className="min-h-[84px] resize-none border-0 bg-transparent px-0 shadow-none focus-visible:ring-0"
      />
      <div className="mt-2 flex items-center justify-end">
        <Button onClick={() => void handleSubmit()} disabled={!value.trim() || sending} className="gap-1.5">
          {sending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
          Send reply
        </Button>
      </div>
    </div>
  );
}

/** Isolated approval actions — local textarea state prevents parent re-renders. */
function ApprovalActions({
  acting,
  onApprove,
  onRequestChanges,
}: {
  acting: boolean;
  onApprove: () => void;
  onRequestChanges: (comment: string) => void;
}) {
  const [comment, setComment] = useState('');
  const hasComment = comment.trim().length > 0;
  return (
    <div className="mt-3 space-y-2">
      <Textarea
        value={comment}
        onChange={(e) => setComment(e.target.value)}
        placeholder="Explain what needs to change before approval."
        rows={3}
      />
      <p className="text-[11px] text-muted-foreground">
        {hasComment
          ? 'Send the change request to keep this same session running and get a revised draft.'
          : 'Leave this blank only if you want to approve immediately.'}
      </p>
      <div className="flex gap-2">
        <Button
          size="sm"
          onClick={onApprove}
          disabled={acting || hasComment}
          className="gap-1.5"
        >
          {acting ? <Loader2 className="h-4 w-4 animate-spin" /> : <ShieldCheck className="h-4 w-4" />}
          Approve
        </Button>
        <Button
          size="sm"
          variant={hasComment ? 'default' : 'outline'}
          onClick={() => { if (comment.trim()) onRequestChanges(comment.trim()); setComment(''); }}
          disabled={acting || !comment.trim()}
        >
          Request changes
        </Button>
      </div>
    </div>
  );
}

/** Stable thinking block — single <details> that stays mounted, uncontrolled open state. */
function ThinkingBlock({ text, active }: { text: string; active: boolean }) {
  if (!text && !active) return null;
  return (
    <details
      className={cn(
        'rounded-lg border px-3 py-2 shadow-sm',
        active
          ? 'border-violet-200/80 bg-violet-50/80 dark:border-violet-900/70 dark:bg-violet-950/20'
          : 'border-border/60 bg-background/80',
      )}
    >
      <summary className={cn(
        'flex cursor-pointer list-none items-center gap-2 text-xs font-medium',
        active ? 'text-violet-900 dark:text-violet-200' : 'text-muted-foreground',
      )}>
        {active ? <Loader2 className="h-3 w-3 animate-spin" /> : <Bot className="h-3 w-3" />}
        {active ? 'Thinking…' : 'Thought for a moment'}
      </summary>
      {text ? (
        <pre className="mt-2 max-h-40 overflow-auto whitespace-pre-wrap text-[11px] leading-relaxed text-muted-foreground">
          {text}
        </pre>
      ) : null}
    </details>
  );
}

function ToolInlineBlock({
  name,
  content,
  status,
  durationMs,
  isError = false,
}: {
  name: string;
  content?: string;
  status: 'running' | 'completed';
  durationMs?: number;
  isError?: boolean;
}) {
  return (
    <details
      className={cn(
        'rounded-lg border px-3 py-2 shadow-sm',
        isError
          ? 'border-red-300/60 bg-red-50/70 dark:border-red-900/60 dark:bg-red-950/20'
          : 'border-border/60 bg-background/80',
      )}
    >
      <summary className="flex cursor-pointer list-none items-center gap-2 text-[11px] font-medium text-muted-foreground">
        {status === 'running' ? (
          <Loader2 className="h-3 w-3 shrink-0 animate-spin" />
        ) : (
          <Wrench className="h-3 w-3 shrink-0" />
        )}
        <span className="truncate">{name}</span>
        <span className="ml-auto text-[10px] uppercase tracking-wide">
          {status === 'running' ? 'Running' : 'Done'}
        </span>
      </summary>
      {content ? (
        <pre className="mt-2 max-h-40 overflow-auto whitespace-pre-wrap rounded-xl bg-muted/35 p-2 text-[10px] leading-4 text-muted-foreground">
          {content}
        </pre>
      ) : null}
      {status === 'completed' && durationMs ? (
        <p className="mt-2 text-[10px] text-muted-foreground">
          {(durationMs / 1000).toFixed(1)}s
        </p>
      ) : null}
    </details>
  );
}

function GenericPreviewPanel({ preview }: { preview: PublishedPreview }) {
  return (
    <div className="rounded-md border border-border/60 bg-background/80 p-3">
      <div className="mb-2 flex items-center gap-2">
        <FileText className="h-4 w-4 text-muted-foreground" />
        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{preview.title}</p>
      </div>
      <div className="max-h-[280px] overflow-auto rounded-md bg-muted/40 p-3">
        {preview.format === 'markdown' && typeof preview.content === 'string' ? (
          <MarkdownContent content={preview.content} className="text-[12px] leading-5" />
        ) : (
          <pre className="whitespace-pre-wrap text-[12px] leading-5 text-foreground">
            {JSON.stringify(preview.content, null, 2)}
          </pre>
        )}
      </div>
    </div>
  );
}

function StoryPlanPanel({
  title,
  preview,
}: {
  title: string;
  preview: StoryPlanPreviewModel;
}) {
  return (
    <div className="rounded-md border border-border/60 bg-background/80 p-3">
      <div className="mb-2 flex items-center gap-2">
        <Sparkles className="h-4 w-4 text-muted-foreground" />
        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{title}</p>
      </div>
      <div className="space-y-3">
        {preview.summary ? <MarkdownContent content={preview.summary} /> : null}

        <div className="flex flex-wrap gap-2 text-[11px] text-muted-foreground">
          <span>{preview.proposedStories.length} stories</span>
          {preview.risks.length ? <span>{preview.risks.length} risks</span> : null}
          {preview.openQuestions.length ? <span>{preview.openQuestions.length} open questions</span> : null}
        </div>

        <div className="space-y-1">
          {preview.proposedStories.map((story, index) => (
            <div
              key={`${story.ref ?? story.title}-${index}`}
              className="rounded-md px-1.5 py-1.5 transition-colors hover:bg-accent/30"
            >
              <div className="flex items-start gap-1.5">
                <Sparkles className="mt-0.5 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-1.5">
                    {story.type ? (
                      <span className="shrink-0 rounded-md bg-muted/60 px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                        {toTitleCase(story.type)}
                      </span>
                    ) : null}
                    {story.ref ? (
                      <Badge variant="outline" className="h-5 rounded-full px-1.5 text-[10px] font-medium text-muted-foreground">
                        {story.ref}
                      </Badge>
                    ) : null}
                    <p className="min-w-0 text-sm font-medium text-foreground">{story.title}</p>
                  </div>

                  {story.description ? (
                    <p className="mt-1 text-xs leading-5 text-muted-foreground">{story.description}</p>
                  ) : null}

                  <div className="mt-1.5 flex flex-wrap gap-1.5">
                    {story.acceptanceCriteria.length ? (
                      <span className="rounded-md bg-blue-50 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 dark:bg-blue-950/40 dark:text-blue-300">
                        {story.acceptanceCriteria.length} acceptance criteria
                      </span>
                    ) : null}
                    {story.dependencyRefs.length ? (
                      <span className="rounded-md bg-amber-50 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-950/40 dark:text-amber-300">
                        Depends on {story.dependencyRefs.join(', ')}
                      </span>
                    ) : null}
                    {story.filesToModify.length ? (
                      <span className="rounded-md bg-violet-50 px-1.5 py-0.5 text-[10px] font-medium text-violet-700 dark:bg-violet-950/40 dark:text-violet-300">
                        {story.filesToModify.length} files touched
                      </span>
                    ) : null}
                  </div>
                </div>
              </div>
            </div>
          ))}
        </div>

        {preview.risks.length ? (
          <div className="rounded-lg border border-amber-200/70 bg-amber-50/70 p-3 dark:border-amber-900/60 dark:bg-amber-950/20">
            <p className="text-[11px] font-semibold uppercase tracking-wide text-amber-800 dark:text-amber-200">Risks</p>
            <ul className="mt-2 space-y-1 text-xs leading-5 text-foreground">
              {preview.risks.map((risk, index) => (
                <li key={`${risk}-${index}`}>{risk}</li>
              ))}
            </ul>
          </div>
        ) : null}

        {preview.openQuestions.length ? (
          <div className="rounded-lg border border-border/60 bg-muted/30 p-3">
            <p className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">Open questions</p>
            <ul className="mt-2 space-y-1 text-xs leading-5 text-foreground">
              {preview.openQuestions.map((question, index) => (
                <li key={`${question}-${index}`}>{question}</li>
              ))}
            </ul>
          </div>
        ) : null}
      </div>
    </div>
  );
}

export function AgentRunDrawer({
  workspaceId,
  runId,
  open,
  onOpenChange,
  canEdit = false,
  title,
  description,
}: AgentRunDrawerProps) {
  const [run, setRun] = useState<AgentRun | null>(null);
  const [messages, setMessages] = useState<AgentRunMessage[]>([]);
  const [artifacts, setArtifacts] = useState<AgentRunArtifact[]>([]);
  const [loading, setLoading] = useState(false);
  const [actingOnRun, setActingOnRun] = useState<string | null>(null);
  const streamRouterRef = useRef(new StreamingTagRouter());
  const [liveSegments, setLiveSegments] = useState<StreamSegments>(INITIAL_SEGMENTS);
  const rafPendingRef = useRef(false);
  const [liveTools, setLiveTools] = useState<LiveToolEvent[]>([]);
  const [livePreviews, setLivePreviews] = useState<PublishedPreview[]>([]);
  const [rightPanelWidthPct, setRightPanelWidthPct] = useState(RIGHT_PANEL_DEFAULT_WIDTH_PCT);
  const chatEndRef = useRef<HTMLDivElement>(null);
  const splitLayoutRef = useRef<HTMLDivElement>(null);
  const isDraggingSplitRef = useRef(false);
  const reloadTimerRef = useRef<ReturnType<typeof window.setTimeout> | null>(null);
  const lastReloadRef = useRef(0);
  const runRef = useRef<AgentRun | null>(null);
  runRef.current = run;

  const upsertLiveTool = useCallback((nextTool: LiveToolEvent) => {
    setLiveTools((current) => {
      const index = current.findIndex((tool) => tool.id === nextTool.id);
      if (index === -1) return [...current, nextTool];
      const next = current.slice();
      next[index] = { ...next[index], ...nextTool };
      return next;
    });
  }, []);

  const upsertLivePreview = useCallback((nextPreview: PublishedPreview) => {
    setLivePreviews((current) => {
      const index = current.findIndex((preview) => preview.panelKey === nextPreview.panelKey);
      if (index === -1) return [...current, nextPreview];
      const next = current.slice();
      next[index] = nextPreview;
      return next;
    });
  }, []);

  const loadRun = useCallback(async (nextRunId: string) => {
    setLoading(true);
    try {
      const [runRes, messagesRes, artifactsRes] = await Promise.all([
        agentService.getRun(workspaceId, nextRunId),
        agentService.listRunMessages(workspaceId, nextRunId),
        agentService.listRunArtifacts(workspaceId, nextRunId),
      ]);
      startTransition(() => {
        setRun(runRes.data ?? null);
        setMessages(messagesRes.data ?? []);
        setArtifacts(artifactsRes.data ?? []);
      });
      lastReloadRef.current = Date.now();
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  /** Schedule a throttled reload. While the run is actively streaming (status=running),
   *  use a longer throttle window since live stream events already provide real-time UI updates. */
  const scheduleReload = useCallback((nextRunId: string, delayMs?: number) => {
    const isStreaming = runRef.current?.status === 'running';
    const throttleMs = delayMs ?? (isStreaming ? 5_000 : 500);
    const elapsed = Date.now() - lastReloadRef.current;

    if (reloadTimerRef.current) {
      window.clearTimeout(reloadTimerRef.current);
    }

    if (elapsed >= throttleMs) {
      reloadTimerRef.current = null;
      void loadRun(nextRunId);
    } else {
      reloadTimerRef.current = window.setTimeout(() => {
        reloadTimerRef.current = null;
        void loadRun(nextRunId);
      }, throttleMs - elapsed);
    }
  }, [loadRun]);

  useEffect(() => {
    if (!open || !runId) {
      if (reloadTimerRef.current) {
        window.clearTimeout(reloadTimerRef.current);
        reloadTimerRef.current = null;
      }
      return;
    }
    streamRouterRef.current.reset();
    setLiveSegments(INITIAL_SEGMENTS);
    setLiveTools([]);
    setLivePreviews([]);
    void loadRun(runId);
  }, [loadRun, open, runId]);

  useEffect(() => {
    if (!open || !runId) return;
    const handleRunEvent = (event: Event) => {
      const detail = (event as CustomEvent).detail as {
        entity_id?: string;
        parent_id?: string;
        data?: { status?: string; pause_reason?: string };
      } | undefined;
      if (!detail) return;
      if (detail.entity_id !== runId && detail.parent_id !== runId) return;

      const eventStatus = detail.data?.status;
      // Only reload on meaningful status transitions, not heartbeats/running updates.
      // Native interactive runs stream over dedicated websocket events, but
      // autonomous runtimes surface progress through artifacts/messages and
      // need periodic refetch while they are running.
      const shouldReloadWhileRunning =
        eventStatus === 'running' &&
        runRef.current?.invocation_mode === 'autonomous' &&
        (runRef.current?.runtime_kind === 'opencode' || runRef.current?.runtime_kind === 'codex');
      if (
        shouldReloadWhileRunning ||
        eventStatus === 'paused' ||
        eventStatus === 'completed' ||
        eventStatus === 'failed' ||
        eventStatus === 'cancelled'
      ) {
        scheduleReload(runId, shouldReloadWhileRunning ? 1_200 : 300);
      }
    };
    const handleMessageEvent = (event: Event) => {
      const detail = (event as CustomEvent).detail as { entity_id?: string; parent_id?: string } | undefined;
      if (!detail) return;
      if (detail.entity_id === runId || detail.parent_id === runId) {
        scheduleReload(runId, 300);
      }
    };
    window.addEventListener('agent_run-updated', handleRunEvent);
    window.addEventListener('agent_run_message-created', handleMessageEvent);
    return () => {
      window.removeEventListener('agent_run-updated', handleRunEvent);
      window.removeEventListener('agent_run_message-created', handleMessageEvent);
    };
  }, [open, runId, scheduleReload]);

  useEffect(() => {
    if (!open || !runId) return;
    const handleStreamEvent = (event: Event) => {
      const detail = (event as CustomEvent).detail as {
        entity_id?: string;
        parent_id?: string;
        data?: AgentRunStreamEvent;
      } | undefined;
      if (!detail || (detail.entity_id !== runId && detail.parent_id !== runId)) {
        return;
      }
      const stream = detail.data;
      if (!stream) return;

      switch (stream.type) {
        case 'assistant_message_started':
          streamRouterRef.current.reset();
          setLiveSegments(INITIAL_SEGMENTS);
          break;
        case 'assistant_message_delta':
          if (stream.text) {
            streamRouterRef.current.feed(stream.text);
            // Batch React state updates to animation frames for smooth rendering
            if (!rafPendingRef.current) {
              rafPendingRef.current = true;
              requestAnimationFrame(() => {
                rafPendingRef.current = false;
                setLiveSegments(streamRouterRef.current.getSegments());
              });
            }
          }
          break;
        case 'assistant_message_completed':
          setLiveSegments(streamRouterRef.current.finalize());
          break;
        case 'tool_call_started':
          upsertLiveTool({
            id: stream.tool_call_id || `${stream.tool_name}-${Date.now()}`,
            name: stream.tool_name || 'tool',
            input: stream.tool_input,
            status: 'running',
          });
          if (isPublishedPreviewToolName(stream.tool_name) && stream.tool_input) {
            const preview = parsePublishedPreviewRawInput(stream.tool_input);
            if (preview) {
              upsertLivePreview(preview);
            }
          }
          break;
        case 'tool_call_finished':
          upsertLiveTool({
            id: stream.tool_call_id || `${stream.tool_name}-${Date.now()}`,
            name: stream.tool_name || 'tool',
            output: stream.output_summary,
            durationMs: stream.duration_ms,
            status: 'completed',
          });
          break;
        default:
          break;
      }
    };

    window.addEventListener('agent_run_stream-updated', handleStreamEvent);
    return () => {
      window.removeEventListener('agent_run_stream-updated', handleStreamEvent);
    };
  }, [open, runId, upsertLivePreview, upsertLiveTool]);

  const hasLiveContent = liveSegments.chatText.trim();

  // Only poll while queued (waiting for worker pickup). During streaming,
  // websocket events provide real-time UI updates.
  useEffect(() => {
    if (!open || !run || run.status !== 'queued') return;
    const intervalId = window.setInterval(() => void loadRun(run.id), 3_000);
    return () => window.clearInterval(intervalId);
  }, [loadRun, open, run?.id, run?.status]);

  useEffect(() => {
    return () => {
      if (reloadTimerRef.current) {
        window.clearTimeout(reloadTimerRef.current);
      }
    };
  }, []);

  useEffect(() => {
    if (!run || !ACTIVE_RUN_STATUSES.has(run.status)) {
      streamRouterRef.current.reset();
      setLiveSegments(INITIAL_SEGMENTS);
      return;
    }
  }, [run]);

  useEffect(() => {
    const latestAssistant = [...messages].reverse().find((message) => message.role === 'assistant');
    if (!latestAssistant || !hasLiveContent) return;
    const persisted = latestAssistant.content?.trim() ?? '';
    const streamedChat = liveSegments.chatText.trim();
    if (persisted && streamedChat && (persisted.endsWith(streamedChat) || streamedChat.endsWith(persisted) || persisted === streamedChat)) {
      streamRouterRef.current.reset();
      setLiveSegments(INITIAL_SEGMENTS);
    }
  }, [messages, hasLiveContent, liveSegments.chatText]);

  // Auto-scroll chat to bottom when new content arrives, but only if user is near the bottom.
  const scrollChatToBottom = useCallback(() => {
    const el = chatEndRef.current;
    if (!el) return;
    const container = el.parentElement;
    if (!container) return;
    const isNearBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 150;
    if (isNearBottom) {
      el.scrollIntoView({ behavior: 'smooth' });
    }
  }, []);

  useEffect(() => {
    scrollChatToBottom();
  }, [messages, liveSegments.chatText, liveSegments.isThinking, liveTools, scrollChatToBottom]);

  const sendReplyContent = useCallback(async (content: string) => {
    if (!run || !content.trim()) return;
    try {
      await agentService.resumeRun(workspaceId, run.id, {
        intent: 'reply',
        content: content.trim(),
      });
      await loadRun(run.id);
    } catch { /* handled by caller */ }
  }, [loadRun, run, workspaceId]);

  const handleApprove = async (currentRunId: string) => {
    setActingOnRun(currentRunId);
    try {
      await agentService.resumeRun(workspaceId, currentRunId, {
        intent: 'approve',
        send_message: true,
      });
      await loadRun(currentRunId);
    } finally {
      setActingOnRun(null);
    }
  };

  const handleRequestChanges = async (currentRunId: string, content: string) => {
    if (!content) return;
    setActingOnRun(currentRunId);
    try {
      await agentService.resumeRun(workspaceId, currentRunId, {
        intent: 'request_changes',
        content,
      });
      await loadRun(currentRunId);
    } finally {
      setActingOnRun(null);
    }
  };

  const handleCancel = async (currentRunId: string) => {
    setActingOnRun(currentRunId);
    try {
      await agentService.cancelRun(workspaceId, currentRunId);
      await loadRun(currentRunId);
    } finally {
      setActingOnRun(null);
    }
  };

  const stopSplitDrag = useCallback(() => {
    if (!isDraggingSplitRef.current) return;
    isDraggingSplitRef.current = false;
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
  }, []);

  const handleSplitPointerDown = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
    event.preventDefault();
    isDraggingSplitRef.current = true;
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  }, []);

  useEffect(() => {
    const handlePointerMove = (event: PointerEvent) => {
      if (!isDraggingSplitRef.current) return;
      const layout = splitLayoutRef.current;
      if (!layout) return;

      const rect = layout.getBoundingClientRect();
      if (rect.width <= 0) return;

      const nextWidth = ((rect.right - event.clientX) / rect.width) * 100;
      const clampedWidth = Math.min(
        RIGHT_PANEL_MAX_WIDTH_PCT,
        Math.max(RIGHT_PANEL_MIN_WIDTH_PCT, nextWidth),
      );
      setRightPanelWidthPct(clampedWidth);
    };

    const handlePointerUp = () => {
      stopSplitDrag();
    };

    window.addEventListener('pointermove', handlePointerMove);
    window.addEventListener('pointerup', handlePointerUp);
    return () => {
      window.removeEventListener('pointermove', handlePointerMove);
      window.removeEventListener('pointerup', handlePointerUp);
      stopSplitDrag();
    };
  }, [stopSplitDrag]);

  const displayArtifacts = useMemo(() => mergeArtifactsForDisplay(artifacts), [artifacts]);
  const autonomousRuntimeStream = useMemo(
    () => buildAutonomousRuntimeStreamDisplay(run, displayArtifacts),
    [displayArtifacts, run],
  );

  const latestQuestionPrompt = useMemo(() => {
    if (!run || getAgentRunDisplayStatus(run) !== 'awaiting_input') return null;
    for (let index = messages.length - 1; index >= 0; index -= 1) {
      const message = messages[index];
      if (message.role !== 'assistant') continue;
      const parsed = parseMessageStructuredQuestions(message, artifacts);
      if (parsed) {
        return { messageId: message.id, ...parsed };
      }
    }
    return null;
  }, [artifacts, messages, run]);

  const persistedPreviewsByKey = useMemo(() => {
    const previews = new Map<string, PublishedPreview>();

    for (let index = artifacts.length - 1; index >= 0; index -= 1) {
      const preview = parseArtifactPublishedPreview(artifacts[index]);
      if (!preview || previews.has(preview.panelKey)) continue;
      previews.set(preview.panelKey, preview);
    }

    for (let index = messages.length - 1; index >= 0; index -= 1) {
      const preview = resolveMessagePublishedPreview(messages[index], artifacts);
      if (!preview || previews.has(preview.panelKey)) continue;
      previews.set(preview.panelKey, preview);
    }

    return previews;
  }, [artifacts, messages]);

  const previewsByKey = useMemo(() => {
    const previews = new Map(persistedPreviewsByKey);
    for (const preview of livePreviews) {
      previews.set(preview.panelKey, preview);
    }
    return previews;
  }, [livePreviews, persistedPreviewsByKey]);

  const visibleLiveTools = useMemo(() => getVisibleLiveTools(liveTools, messages), [liveTools, messages]);

  const latestSpecDraftPreview = useMemo(() => {
    const preview = previewsByKey.get('prd_draft');
    if (preview?.format === 'markdown' && typeof preview.content === 'string') {
      return {
        title: preview.title,
        markdown: preview.content,
      } satisfies MarkdownPanelPreview;
    }
    return null;
  }, [previewsByKey]);

  const latestStoryPlanPreview = useMemo(() => {
    return parseStoryPlanPreviewModel(previewsByKey.get('story_plan'));
  }, [previewsByKey]);

  const otherPreviewPanels = useMemo(() => {
    return Array.from(previewsByKey.values()).filter((preview) => {
      if (preview.panelKey === 'prd_draft' && latestSpecDraftPreview?.markdown) {
        return false;
      }
      if (preview.panelKey === 'story_plan' && latestStoryPlanPreview) {
        return false;
      }
      return true;
    });
  }, [latestSpecDraftPreview?.markdown, latestStoryPlanPreview, previewsByKey]);

  const latestApprovalRequest = useMemo<ParsedApprovalRequestWithMeta | null>(() => {
    for (let index = messages.length - 1; index >= 0; index -= 1) {
      if (messages[index].role !== 'assistant') continue;
      const parsed = parseMessageApprovalRequest(messages[index], artifacts);
      if (parsed) return { messageId: messages[index].id, index, ...parsed };
    }
    return null;
  }, [artifacts, messages]);

  const latestAssistantMessageId = useMemo(() => {
    for (let index = messages.length - 1; index >= 0; index -= 1) {
      if (messages[index].role === 'assistant') {
        return messages[index].id;
      }
    }
    return null;
  }, [messages]);

  const currentApprovalRequest = useMemo(() => {
    if (!latestApprovalRequest) return null;
    if (latestApprovalRequest.messageId !== latestAssistantMessageId) return null;
    return latestApprovalRequest;
  }, [latestApprovalRequest, latestAssistantMessageId]);

  const transcriptEntries = useMemo(() => buildTranscriptEntries(messages), [messages]);
  const runDisplayStatus = run ? getAgentRunDisplayStatus(run) : null;
  const runStatusMeta = runDisplayStatus ? STATUS_META[runDisplayStatus] ?? STATUS_META.queued : null;
  const inlineLiveToolHostMessageId =
    currentApprovalRequest?.messageId ??
    latestQuestionPrompt?.messageId ??
    ((runDisplayStatus === 'awaiting_input' || runDisplayStatus === 'awaiting_approval') ? latestAssistantMessageId : null);
  const trailingLiveTools =
    inlineLiveToolHostMessageId && visibleLiveTools.length > 0
      ? []
      : visibleLiveTools;

  const resolvedTitle = title ?? (run?.invocation_mode === 'interactive' ? 'Interactive Agent Run' : 'Agent Run');
  const resolvedDescription = description ?? (run ? `${run.target_type} · ${formatMessageTimestamp(run.created_at)}` : 'Run conversation and artifacts');

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full border-l sm:max-w-7xl">
        <SheetHeader className="border-b border-border/60 pr-12">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <SheetTitle className="flex items-center gap-2">
                <Bot className="h-4 w-4 text-muted-foreground" />
                <span className="truncate">{resolvedTitle}</span>
              </SheetTitle>
              <SheetDescription>{resolvedDescription}</SheetDescription>
            </div>
            {run ? <Badge variant="outline">{run.invocation_mode}</Badge> : null}
          </div>
        </SheetHeader>

        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
          {!runId ? (
            <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
              Select a run to open it.
            </div>
          ) : loading && !run ? (
            <div className="flex flex-1 items-center justify-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              Loading run...
            </div>
          ) : run ? (
            <div className="flex min-h-0 flex-1 flex-col">
              <div
                ref={splitLayoutRef}
                className="grid min-h-0 flex-1 gap-4 overflow-hidden p-4 lg:grid-cols-[minmax(0,calc(100%-var(--run-drawer-right-pane-width)-0.5rem))_0.5rem_minmax(280px,var(--run-drawer-right-pane-width))] lg:gap-0"
                style={{ '--run-drawer-right-pane-width': `${rightPanelWidthPct}%` } as CSSProperties}
              >
                <div className="flex min-h-0 flex-col lg:pr-4">
                  <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-2xl border border-border/60 bg-background shadow-sm">
                    <div className="relative z-10 border-b bg-background px-4 py-3">
                      <div className="absolute left-0 right-0 top-full h-2 bg-gradient-to-b from-black/[0.03] to-transparent pointer-events-none" />
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0">
                          <p className="text-sm font-semibold text-foreground">Run conversation</p>
                          <p className="text-xs text-muted-foreground">
                            {run.target_type} · {formatMessageTimestamp(run.created_at)}
                          </p>
                        </div>
                        <div className="flex items-center gap-2 shrink-0">
                          {runStatusMeta ? <Badge variant={runStatusMeta.variant}>{runStatusMeta.label}</Badge> : null}
                          <Badge variant="outline">{run.invocation_mode}</Badge>
                        </div>
                      </div>
                    </div>

                    <div className="min-h-0 flex-1 overflow-y-auto bg-muted/20">
                      <div className="px-4 pb-4 pt-2">
                        {messages.length === 0 && !hasLiveContent && !liveSegments.isThinking ? (
                          <div className="flex flex-col items-center justify-center gap-2 py-12 text-muted-foreground">
                            <Bot className="h-8 w-8 opacity-30" />
                            <p className="text-sm">
                              {run.status === 'queued' || run.status === 'running'
                                ? 'The agent is starting. The transcript, questions, and artifacts will appear here.'
                                : 'No run messages yet.'}
                            </p>
                          </div>
                        ) : null}

                        {transcriptEntries.map((entry, index) => {
                          if (entry.type === 'separator') {
                            return <TranscriptDaySeparator key={`sep-${entry.label}-${index}`} label={entry.label} />;
                          }

                          const message = entry.message;
                          let displayContent = message.content || '';
                          const parsedQuestions = message.role === 'assistant' ? parseMessageStructuredQuestions(message, artifacts) : null;
                          if (parsedQuestions) displayContent = parsedQuestions.surroundingText;
                          const parsedApproval = message.role === 'assistant'
                            ? parseMessageApprovalRequest({ ...message, content: displayContent }, artifacts)
                            : null;
                          if (parsedApproval) displayContent = parsedApproval.surroundingText;
                          const toolInvocations = Array.isArray(message.tool_invocations) ? message.tool_invocations : [];
                          const isLatestQuestion = latestQuestionPrompt?.messageId === message.id;
                          const isLatestApproval =
                            currentApprovalRequest?.messageId === message.id &&
                            (
                              getAgentRunDisplayStatus(run) === 'awaiting_approval' ||
                              isInlineApprovalRun(run)
                            );

                          if (message.role === 'tool') {
                            const toolMessage = parseToolMessage(message);
                            return (
                              <ProcessingBox key={message.id}>
                                <ToolInlineBlock
                                  name={toolMessage.name}
                                  content={toolMessage.content}
                                  status="completed"
                                  isError={toolMessage.isError}
                                />
                              </ProcessingBox>
                            );
                          }

                          const shouldRenderAssistantToolInvocations =
                            toolInvocations.length > 0 && messages[entry.messageIndex + 1]?.role !== 'tool';
                          const shouldRenderInlineLiveTools =
                            message.id === inlineLiveToolHostMessageId && visibleLiveTools.length > 0;

                          return (
                            <TranscriptBubble
                              key={message.id}
                              role={message.role}
                              timestamp={formatMessageTimestamp(message.created_at)}
                              isConsecutive={entry.isConsecutive}
                              isLastInGroup={entry.isLastInGroup}
                            >
                              {displayContent ? <MarkdownContent content={displayContent} /> : null}

                              {parsedApproval ? (
                                <div className="mt-3 rounded-xl border border-amber-300/50 bg-amber-50/70 p-3 dark:border-amber-800/60 dark:bg-amber-950/20">
                                  <div className="mb-2 flex items-center gap-2 text-xs font-medium text-amber-900 dark:text-amber-200">
                                    <ShieldCheck className="h-3.5 w-3.5" />
                                    Approval requested
                                  </div>
                                  <p className="text-sm font-medium text-foreground">{parsedApproval.title || 'Approval required'}</p>
                                  {parsedApproval.summary ? (
                                    <p className="mt-1 text-xs text-muted-foreground">{parsedApproval.summary}</p>
                                  ) : null}

                                  {isLatestApproval && canEdit ? (
                                    <ApprovalActions
                                      acting={actingOnRun === run.id}
                                      onApprove={() => void handleApprove(run.id)}
                                      onRequestChanges={(comment) => void handleRequestChanges(run.id, comment)}
                                    />
                                  ) : null}
                                </div>
                              ) : null}

                              {shouldRenderAssistantToolInvocations ? (
                                <ProcessingBox>
                                  {toolInvocations.map((invocation, toolIndex) => {
                                    const toolName = typeof invocation.tool_name === 'string' ? invocation.tool_name : 'tool';
                                    const outputSummary = typeof invocation.output_summary === 'string' ? invocation.output_summary : '';
                                    const durationMs = typeof invocation.duration_ms === 'number' ? invocation.duration_ms : undefined;
                                    return (
                                      <ToolInlineBlock
                                        key={`${message.id}-tool-${toolIndex}`}
                                        name={toolName}
                                        content={outputSummary}
                                        status="completed"
                                        durationMs={durationMs}
                                      />
                                    );
                                  })}
                                </ProcessingBox>
                              ) : null}

                              {shouldRenderInlineLiveTools ? (
                                <ProcessingBox active={visibleLiveTools.some((tool) => tool.status === 'running')}>
                                  {visibleLiveTools.map((tool) => (
                                    <ToolInlineBlock
                                      key={tool.id}
                                      name={tool.name}
                                      content={tool.status === 'running' ? tool.input : tool.output}
                                      status={tool.status}
                                      durationMs={tool.durationMs}
                                    />
                                  ))}
                                </ProcessingBox>
                              ) : null}

                              {parsedQuestions && isLatestQuestion ? (
                                <div className="mt-3">
                                  <StructuredQuestionCard
                                    questions={parsedQuestions.questions}
                                    onSubmit={(formattedAnswer) => void sendReplyContent(formattedAnswer)}
                                    disabled={!canEdit}
                                    readOnly={!canEdit}
                                  />
                                </div>
                              ) : null}
                            </TranscriptBubble>
                          );
                        })}

                        {liveSegments.chatText ? (
                          <TranscriptBubble
                            role="assistant"
                            timestamp="streaming…"
                            isConsecutive={false}
                            isLastInGroup
                          >
                            <MarkdownContent content={liveSegments.chatText} />
                          </TranscriptBubble>
                        ) : null}

                        {autonomousRuntimeStream?.assistantText ? (
                          <TranscriptBubble
                            role="assistant"
                            timestamp={run?.status === 'running' ? 'streaming…' : formatMessageTimestamp(run?.updated_at ?? run?.created_at ?? new Date().toISOString())}
                            isConsecutive={false}
                            isLastInGroup
                          >
                            <MarkdownContent content={autonomousRuntimeStream.assistantText} />
                          </TranscriptBubble>
                        ) : null}

                        {liveSegments.isThinking || liveSegments.thinkingText || trailingLiveTools.length > 0 || autonomousRuntimeStream?.processingText || autonomousRuntimeStream?.errorText ? (
                          <ProcessingBox active={liveSegments.isThinking || trailingLiveTools.some((tool) => tool.status === 'running') || run?.status === 'running'}>
                            <ThinkingBlock text={liveSegments.thinkingText} active={liveSegments.isThinking} />
                            {trailingLiveTools.map((tool) => (
                              <ToolInlineBlock
                                key={tool.id}
                                name={tool.name}
                                content={tool.status === 'running' ? tool.input : tool.output}
                                status={tool.status}
                                durationMs={tool.durationMs}
                              />
                            ))}
                            {autonomousRuntimeStream?.processingText ? (
                              <ToolInlineBlock
                                name={`${run?.runtime_kind === 'codex' ? 'Codex' : 'OpenCode'} stream`}
                                content={autonomousRuntimeStream.processingText}
                                status={run?.status === 'running' ? 'running' : 'completed'}
                              />
                            ) : null}
                            {autonomousRuntimeStream?.errorText ? (
                              <ToolInlineBlock
                                name={`${run?.runtime_kind === 'codex' ? 'Codex' : 'OpenCode'} stderr`}
                                content={autonomousRuntimeStream.errorText}
                                status="completed"
                                isError
                              />
                            ) : null}
                          </ProcessingBox>
                        ) : null}
                        <div ref={chatEndRef} />
                      </div>
                    </div>

                    {(run.invocation_mode === 'interactive' ? ACTIVE_RUN_STATUSES.has(run.status) : getAgentRunDisplayStatus(run) === 'awaiting_input') && canEdit ? (
                      <div className="border-t bg-background px-3 py-3">
                        <ReplyForm onSubmit={(content) => void sendReplyContent(content)} />
                      </div>
                    ) : null}
                  </div>
                </div>

                <div
                  role="separator"
                  aria-orientation="vertical"
                  aria-label="Resize transcript and details panels"
                  aria-valuemin={RIGHT_PANEL_MIN_WIDTH_PCT}
                  aria-valuemax={RIGHT_PANEL_MAX_WIDTH_PCT}
                  aria-valuenow={Math.round(rightPanelWidthPct)}
                  className="relative hidden cursor-col-resize touch-none select-none lg:block"
                  onPointerDown={handleSplitPointerDown}
                >
                  <div className="absolute inset-y-0 left-1/2 w-px -translate-x-1/2 bg-border/70" />
                </div>

                <div className="min-h-0 space-y-3 overflow-y-auto lg:pl-4">
                  {/* Run details (metadata + artifacts) */}
                  <AgentRunDetail
                    run={run}
                    artifacts={displayArtifacts}
                    actingOnRun={actingOnRun}
                    onCancel={handleCancel}
                    onApprove={handleApprove}
                    showArtifacts={false}
                  />

                  {getAgentRunDisplayStatus(run) === 'awaiting_approval' ? (
                    <div className="rounded-md border border-amber-300/50 bg-amber-50/70 p-3 dark:border-amber-800/60 dark:bg-amber-950/20">
                      <div className="mb-2 flex items-center gap-2">
                        <ShieldCheck className="h-4 w-4 text-amber-900 dark:text-amber-200" />
                        <p className="text-xs font-semibold uppercase tracking-wide text-amber-900 dark:text-amber-200">
                          Approval Required
                        </p>
                      </div>
                      <div className="space-y-3">
                        <div>
                          <p className="text-sm font-semibold text-foreground">
                            {currentApprovalRequest?.title || 'Review the latest proposed output'}
                          </p>
                          <p className="text-xs text-muted-foreground">
                            {currentApprovalRequest?.summary || 'Approve to continue this same run, or request changes with a required comment.'}
                          </p>
                        </div>
                        <p className="text-xs text-muted-foreground">
                          You can reply below, or use the latest approval card in the chat transcript for approve and request-changes shortcuts.
                        </p>
                      </div>
                    </div>
                  ) : null}

                  {latestSpecDraftPreview?.markdown ? (
                    <div className="rounded-md border border-border/60 bg-background/80 p-3">
                      <div className="mb-2 flex items-center gap-2">
                        <FileText className="h-4 w-4 text-muted-foreground" />
                        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                          {latestSpecDraftPreview.title || 'PRD Draft'}
                        </p>
                      </div>
                      <div className="space-y-2">
                        <div className="max-h-[280px] overflow-auto rounded-md bg-muted/40 p-3">
                          <MarkdownContent content={latestSpecDraftPreview.markdown} className="text-[12px] leading-5" />
                        </div>
                      </div>
                    </div>
                  ) : null}

                  {latestStoryPlanPreview ? (
                    <StoryPlanPanel
                      title={previewsByKey.get('story_plan')?.title || 'Story Plan'}
                      preview={latestStoryPlanPreview}
                    />
                  ) : null}

                  {otherPreviewPanels.map((preview) => (
                    <GenericPreviewPanel key={preview.panelKey} preview={preview} />
                  ))}

                </div>
              </div>
            </div>
          ) : (
            <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
              Run not found.
            </div>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}
