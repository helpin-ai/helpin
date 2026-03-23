import { type ComponentPropsWithoutRef, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { formatDistanceToNow, parseISO } from 'date-fns';
import { Bot, FileText, Loader2, Send, ShieldCheck, Sparkles } from 'lucide-react';
import ReactMarkdown from 'react-markdown';

import { AgentRunDetail } from '@/components/pm/AgentRunDetail';
import { StructuredQuestionCard } from '@/components/pm/StructuredQuestionCard';
import { parseApprovalRequest, parseSpecDraft, parseStoryPlan, type ParsedApprovalRequest } from '@/components/pm/agentRunMarkup';
import { ACTIVE_RUN_STATUSES } from '@/components/pm/agentRunConstants';
import { parseStructuredQuestions } from '@/components/pm/parseStructuredQuestions';
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
  OrchestrationProposal,
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

interface ProductSpecDraftArtifact {
  title?: string;
  summary?: string;
  spec_markdown?: string;
  risks?: string[];
  assumptions?: string[];
  open_questions?: string[];
}

interface LiveToolEvent {
  id: string;
  name: string;
  input?: string;
  output?: string;
  durationMs?: number;
  status: 'running' | 'completed';
}

interface ParsedApprovalRequestWithMeta extends ParsedApprovalRequest {
  messageId: string;
  index: number;
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

function MarkdownContent({ content, className }: { content: string; className?: string }) {
  return (
    <div className={cn('text-sm leading-6 text-foreground', className)}>
      <ReactMarkdown
        components={{
          p: ({ children }) => <p className="mb-3 last:mb-0">{children}</p>,
          ul: ({ children }) => <ul className="mb-3 list-disc space-y-1 pl-5 last:mb-0">{children}</ul>,
          ol: ({ children }) => <ol className="mb-3 list-decimal space-y-1 pl-5 last:mb-0">{children}</ol>,
          li: ({ children }) => <li>{children}</li>,
          h1: ({ children }) => <h1 className="mb-3 text-base font-semibold last:mb-0">{children}</h1>,
          h2: ({ children }) => <h2 className="mb-3 text-sm font-semibold last:mb-0">{children}</h2>,
          h3: ({ children }) => <h3 className="mb-2 text-sm font-semibold last:mb-0">{children}</h3>,
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

function parseArtifactJSON<T>(artifact: AgentRunArtifact | null | undefined): T | null {
  if (!artifact?.inline_content) return null;
  try {
    return JSON.parse(artifact.inline_content) as T;
  } catch {
    return null;
  }
}

function findLatestArtifact(artifacts: AgentRunArtifact[], types: string[]): AgentRunArtifact | null {
  for (let index = artifacts.length - 1; index >= 0; index -= 1) {
    if (types.includes(artifacts[index].artifact_type)) {
      return artifacts[index];
    }
  }
  return null;
}

function isInlineApprovalRun(run: AgentRun | null): boolean {
  return !!run && run.invocation_mode === 'interactive' && run.status === 'awaiting_input';
}

function isApprovalActionableRun(run: AgentRun | null): boolean {
  return !!run && (run.status === 'awaiting_approval' || isInlineApprovalRun(run));
}

function mergeArtifactsForDisplay(artifacts: AgentRunArtifact[]): AgentRunArtifact[] {
  const displayArtifacts = artifacts.filter(
    (artifact) => artifact.artifact_type !== 'opencode_stdout_chunk' && artifact.artifact_type !== 'opencode_stderr_chunk',
  );

  const stdoutArtifact = displayArtifacts.find((artifact) => artifact.artifact_type === 'opencode_stdout');
  const stderrArtifact = displayArtifacts.find((artifact) => artifact.artifact_type === 'opencode_stderr');

  if (!stdoutArtifact) {
    const stdoutContent = artifacts
      .filter((artifact) => artifact.artifact_type === 'opencode_stdout_chunk')
      .map((artifact) => artifact.inline_content ?? '')
      .join('');
    if (stdoutContent) {
      displayArtifacts.unshift({
        id: 'live-opencode-stdout',
        workspace_id: artifacts[0]?.workspace_id ?? '',
        run_id: artifacts[0]?.run_id ?? '',
        artifact_type: 'opencode_stdout',
        format: 'text',
        storage_mode: 'inline',
        inline_content: stdoutContent,
        metadata: {},
        sequence_no: -2,
        created_at: artifacts[0]?.created_at ?? new Date().toISOString(),
      });
    }
  }

  if (!stderrArtifact) {
    const stderrContent = artifacts
      .filter((artifact) => artifact.artifact_type === 'opencode_stderr_chunk')
      .map((artifact) => artifact.inline_content ?? '')
      .join('');
    if (stderrContent) {
      displayArtifacts.unshift({
        id: 'live-opencode-stderr',
        workspace_id: artifacts[0]?.workspace_id ?? '',
        run_id: artifacts[0]?.run_id ?? '',
        artifact_type: 'opencode_stderr',
        format: 'text',
        storage_mode: 'inline',
        inline_content: stderrContent,
        metadata: {},
        sequence_no: -1,
        created_at: artifacts[0]?.created_at ?? new Date().toISOString(),
      });
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
    <div className="space-y-2 rounded-md border border-border/60 bg-background/80 p-3">
      <Label className="text-xs">Reply To Agent</Label>
      <Textarea
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder="Clarify scope, answer a question, or request changes."
        rows={3}
      />
      <Button onClick={() => void handleSubmit()} disabled={!value.trim() || sending} className="gap-1.5">
        {sending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
        Send Reply
      </Button>
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
  return (
    <div className="mt-3 space-y-2">
      <Textarea
        value={comment}
        onChange={(e) => setComment(e.target.value)}
        placeholder="Explain what needs to change before approval."
        rows={3}
      />
      <div className="flex gap-2">
        <Button
          size="sm"
          onClick={onApprove}
          disabled={acting}
          className="gap-1.5"
        >
          {acting ? <Loader2 className="h-4 w-4 animate-spin" /> : <ShieldCheck className="h-4 w-4" />}
          Approve
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={() => { if (comment.trim()) onRequestChanges(comment.trim()); setComment(''); }}
          disabled={acting || !comment.trim()}
        >
          Request changes
        </Button>
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
  const [latestTool, setLatestTool] = useState<LiveToolEvent | null>(null);
  const chatEndRef = useRef<HTMLDivElement>(null);
  const reloadTimerRef = useRef<ReturnType<typeof window.setTimeout> | null>(null);
  const lastReloadRef = useRef(0);
  const runRef = useRef<AgentRun | null>(null);
  runRef.current = run;

  const loadRun = useCallback(async (nextRunId: string) => {
    setLoading(true);
    try {
      const [runRes, messagesRes, artifactsRes] = await Promise.all([
        agentService.getRun(workspaceId, nextRunId),
        agentService.listRunMessages(workspaceId, nextRunId),
        agentService.listRunArtifacts(workspaceId, nextRunId),
      ]);
      setRun(runRes.data ?? null);
      setMessages(messagesRes.data ?? []);
      setArtifacts(artifactsRes.data ?? []);
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
      if (!open) {
        setReply('');
        setRequestChangesComment('');
      }
      if (reloadTimerRef.current) {
        window.clearTimeout(reloadTimerRef.current);
        reloadTimerRef.current = null;
      }
      return;
    }
    streamRouterRef.current.reset();
    setLiveSegments(INITIAL_SEGMENTS);
    setLatestTool(null);
    void loadRun(runId);
  }, [loadRun, open, runId]);

  useEffect(() => {
    if (!open || !runId) return;
    const handleRunEvent = (event: Event) => {
      const detail = (event as CustomEvent).detail as {
        entity_id?: string;
        parent_id?: string;
        data?: { status?: string };
      } | undefined;
      if (!detail) return;
      if (detail.entity_id !== runId && detail.parent_id !== runId) return;

      const eventStatus = detail.data?.status;
      if (
        eventStatus === 'awaiting_input' ||
        eventStatus === 'awaiting_approval' ||
        eventStatus === 'completed' ||
        eventStatus === 'failed' ||
        eventStatus === 'cancelled'
      ) {
        scheduleReload(runId, 300);
        return;
      }
      scheduleReload(runId);
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
          setLiveSegments(streamRouterRef.current.finalize(stream.text));
          break;
        case 'tool_call_started':
          setLatestTool({
            id: stream.tool_call_id || `${stream.tool_name}-${Date.now()}`,
            name: stream.tool_name || 'tool',
            input: stream.tool_input,
            status: 'running',
          });
          break;
        case 'tool_call_finished':
          setLatestTool({
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
  }, [open, runId]);

  useEffect(() => {
    if (!open || !run || (run.status !== 'queued' && run.status !== 'running')) {
      return;
    }
    const intervalMs = run.status === 'queued' ? 3_000 : 5_000;
    const intervalId = window.setInterval(() => {
      void loadRun(run.id);
    }, intervalMs);
    return () => {
      window.clearInterval(intervalId);
    };
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
      setLatestTool(null);
    }
    if (!isApprovalActionableRun(run)) {
      setRequestChangesComment('');
    }
  }, [run]);

  const hasLiveContent = liveSegments.chatText.trim() || liveSegments.specDraftText.trim() || liveSegments.storyPlanText.trim() || liveSegments.questionsText.trim();

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
  }, [messages, liveSegments.chatText, liveSegments.isThinking, latestTool, scrollChatToBottom]);

  const sendReplyContent = useCallback(async (content: string) => {
    if (!run || !content.trim()) return;
    try {
      await agentService.sendRunMessage(workspaceId, run.id, { content: content.trim() });
      await loadRun(run.id);
    } catch { /* handled by caller */ }
  }, [loadRun, run, workspaceId]);

  const handleApprove = async (currentRunId: string) => {
    setActingOnRun(currentRunId);
    try {
      if (run?.id === currentRunId && isInlineApprovalRun(run)) {
        await agentService.sendRunMessage(workspaceId, currentRunId, { content: 'approve' });
      } else {
        await agentService.approveRun(workspaceId, currentRunId, { send_message: true });
      }
      await loadRun(currentRunId);
    } finally {
      setActingOnRun(null);
    }
  };

  const handleRequestChanges = async (currentRunId: string, content: string) => {
    if (!content) return;
    setActingOnRun(currentRunId);
    try {
      if (run?.id === currentRunId && isInlineApprovalRun(run)) {
        await agentService.sendRunMessage(workspaceId, currentRunId, { content });
      } else {
        await agentService.requestRunChanges(workspaceId, currentRunId, { content });
      }
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

  const displayArtifacts = useMemo(() => mergeArtifactsForDisplay(artifacts), [artifacts]);

  const latestQuestionPrompt = useMemo(() => {
    if (run?.status !== 'awaiting_input') return null;
    for (let index = messages.length - 1; index >= 0; index -= 1) {
      const message = messages[index];
      if (message.role !== 'assistant') continue;
      const parsed = parseStructuredQuestions(message.content || '');
      if (parsed) {
        return { messageId: message.id, ...parsed };
      }
    }
    return null;
  }, [messages, run?.status]);

  const latestSpecDraftPreview = useMemo(() => {
    const draftArtifact = parseArtifactJSON<ProductSpecDraftArtifact>(
      findLatestArtifact(artifacts, ['product_spec_draft']),
    );
    if (draftArtifact) return draftArtifact;

    for (let index = messages.length - 1; index >= 0; index -= 1) {
      const parsed = parseSpecDraft(messages[index].content || '');
      if (parsed) {
        return {
          summary: parsed.surroundingText || undefined,
          spec_markdown: parsed.draft,
        } satisfies ProductSpecDraftArtifact;
      }
    }
    return null;
  }, [artifacts, messages]);

  const latestStoryPlanPreview = useMemo(() => {
    for (let index = messages.length - 1; index >= 0; index -= 1) {
      const parsed = parseStoryPlan(messages[index].content || '');
      if (parsed) return parsed.plan;
    }
    return parseArtifactJSON<OrchestrationProposal>(findLatestArtifact(artifacts, ['story_plan_proposal', 'orchestration_proposal']));
  }, [artifacts, messages]);

  const latestApprovalRequest = useMemo<ParsedApprovalRequestWithMeta | null>(() => {
    for (let index = messages.length - 1; index >= 0; index -= 1) {
      if (messages[index].role !== 'assistant') continue;
      const parsed = parseApprovalRequest(messages[index].content || '');
      if (parsed) return { messageId: messages[index].id, index, ...parsed };
    }
    return null;
  }, [messages]);

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

  const resolvedTitle = title ?? (run?.invocation_mode === 'interactive' ? 'Interactive Agent Run' : 'Agent Run');
  const resolvedDescription = description ?? (run ? `${run.target_type} · ${formatMessageTimestamp(run.created_at)}` : 'Run conversation and artifacts');

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full border-l sm:max-w-5xl">
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
              <div className="border-b border-border/60 p-4">
                <AgentRunDetail
                  run={run}
                  artifacts={displayArtifacts}
                  actingOnRun={actingOnRun}
                  onCancel={handleCancel}
                  onApprove={handleApprove}
                  showArtifacts={false}
                />
              </div>

              <div className="grid min-h-0 flex-1 gap-4 overflow-hidden p-4 lg:grid-cols-[minmax(0,1.35fr)_minmax(280px,0.65fr)]">
                <div className="flex min-h-0 flex-col space-y-2">
                  <div className="flex items-center justify-between gap-2">
                    <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Run Chat</p>
                    {run.status === 'running' ? (
                      <Badge variant="secondary" className="gap-1">
                        <Loader2 className="h-3 w-3 animate-spin" />
                        Streaming
                      </Badge>
                    ) : null}
                  </div>

                  <div className="min-h-0 flex-1 space-y-3 overflow-y-auto rounded-md border border-border/60 bg-muted/15 p-3">
                    {messages.length === 0 && !hasLiveContent && !liveSegments.isThinking ? (
                      <p className="text-sm text-muted-foreground">
                        {run.status === 'queued' || run.status === 'running'
                          ? 'The agent is starting. The transcript, questions, and artifacts will appear here.'
                          : 'No run messages yet.'}
                      </p>
                    ) : null}

                    {messages.map((message) => {
                      let displayContent = message.content || '';
                      const parsedQuestions = message.role === 'assistant' ? parseStructuredQuestions(displayContent) : null;
                      if (parsedQuestions) displayContent = parsedQuestions.surroundingText;
                      const parsedSpec = message.role === 'assistant' ? parseSpecDraft(displayContent) : null;
                      if (parsedSpec) displayContent = parsedSpec.surroundingText;
                      const parsedStoryPlan = message.role === 'assistant' ? parseStoryPlan(displayContent) : null;
                      if (parsedStoryPlan) displayContent = parsedStoryPlan.surroundingText;
                      const parsedApproval = message.role === 'assistant' ? parseApprovalRequest(displayContent) : null;
                      if (parsedApproval) displayContent = parsedApproval.surroundingText;
                      const toolInvocations = Array.isArray(message.tool_invocations) ? message.tool_invocations : [];
                      const isLatestQuestion = latestQuestionPrompt?.messageId === message.id;
                      const isLatestApproval =
                        currentApprovalRequest?.messageId === message.id &&
                        (
                          run.status === 'awaiting_approval' ||
                          (
                            isInlineApprovalRun(run)
                          )
                        );

                      return (
                        <div
                          key={message.id}
                          className={cn(
                            message.role === 'user'
                              ? 'rounded-xl border border-blue-200/70 bg-blue-50/70 p-3 shadow-sm dark:border-blue-900/60 dark:bg-blue-950/20'
                              : message.role === 'tool'
                                ? 'rounded-xl border border-border/60 bg-background/90 p-3 shadow-sm'
                                : '',
                          )}
                        >
                          <div className={cn(
                            'flex items-center justify-between gap-2',
                            message.role === 'assistant' ? 'mb-1' : 'mb-2',
                          )}>
                            <span className={cn(
                              'text-xs font-medium',
                              message.role === 'assistant' ? 'text-muted-foreground' : 'text-foreground',
                            )}>{roleLabel(message.role)}</span>
                            <span className="text-[11px] text-muted-foreground">{formatMessageTimestamp(message.created_at)}</span>
                          </div>

                          {displayContent ? <MarkdownContent content={displayContent} /> : null}

                          {parsedSpec?.draft ? (
                            <div className="mt-3 rounded-lg border border-border/50 bg-background/80 p-3">
                              <div className="mb-2 flex items-center gap-2 text-xs font-medium text-muted-foreground">
                                <FileText className="h-3.5 w-3.5" />
                                Live Draft
                              </div>
                              <pre className="max-h-64 overflow-auto whitespace-pre-wrap text-[12px] leading-5 text-foreground">
                                {parsedSpec.draft}
                              </pre>
                            </div>
                          ) : null}

                          {parsedStoryPlan?.plan ? (
                            <div className="mt-3 rounded-lg border border-border/50 bg-background/80 p-3">
                              <div className="mb-2 flex items-center gap-2 text-xs font-medium text-muted-foreground">
                                <Sparkles className="h-3.5 w-3.5" />
                                Story Plan
                              </div>
                              <div className="space-y-2 text-sm">
                                {parsedStoryPlan.plan.summary ? <MarkdownContent content={parsedStoryPlan.plan.summary} className="text-[12px] leading-5" /> : null}
                                <p className="text-xs text-muted-foreground">
                                  {parsedStoryPlan.plan.proposed_stories?.length ?? 0} proposed stories
                                </p>
                              </div>
                            </div>
                          ) : null}

                          {parsedApproval ? (
                            <div className="mt-3 rounded-lg border border-amber-300/50 bg-amber-50/70 p-3 dark:border-amber-800/60 dark:bg-amber-950/20">
                              <div className="mb-2 flex items-center gap-2 text-xs font-medium text-amber-900 dark:text-amber-200">
                                <ShieldCheck className="h-3.5 w-3.5" />
                                Approval Requested
                              </div>
                              <p className="text-sm font-medium text-foreground">{parsedApproval.title || 'Approval required'}</p>
                              {parsedApproval.summary ? (
                                <p className="mt-1 text-xs text-muted-foreground">{parsedApproval.summary}</p>
                              ) : null}

                              {isLatestApproval && canEdit ? (
                                <div className="mt-3 space-y-2">
                                  <Textarea
                                    value={requestChangesComment}
                                    onChange={(event) => setRequestChangesComment(event.target.value)}
                                    placeholder="Explain what needs to change before approval."
                                    rows={3}
                                  />
                                  <div className="flex gap-2">
                                    <Button
                                      size="sm"
                                      onClick={() => void handleApprove(run.id)}
                                      disabled={actingOnRun === run.id}
                                      className="gap-1.5"
                                    >
                                      {actingOnRun === run.id ? <Loader2 className="h-4 w-4 animate-spin" /> : <ShieldCheck className="h-4 w-4" />}
                                      Approve
                                    </Button>
                                    <Button
                                      size="sm"
                                      variant="outline"
                                      onClick={() => void handleRequestChanges(run.id)}
                                      disabled={actingOnRun === run.id || !requestChangesComment.trim()}
                                    >
                                      Request changes
                                    </Button>
                                  </div>
                                </div>
                              ) : null}
                            </div>
                          ) : null}

                          {toolInvocations.length > 0 ? (
                            <details className="mt-3 rounded-md border border-border/40 bg-background/60 px-2.5 py-2 text-[11px] text-muted-foreground">
                              <summary className="cursor-pointer list-none text-[11px] font-medium">
                                Tool calls ({toolInvocations.length})
                              </summary>
                              <div className="mt-2 space-y-2">
                                {toolInvocations.map((invocation, index) => {
                                  const toolName = typeof invocation.tool_name === 'string' ? invocation.tool_name : 'tool';
                                  const outputSummary = typeof invocation.output_summary === 'string' ? invocation.output_summary : '';
                                  const durationMs = typeof invocation.duration_ms === 'number' ? invocation.duration_ms : null;
                                  return (
                                    <div key={`${message.id}-tool-${index}`} className="space-y-1 border-t border-border/30 pt-2 first:border-t-0 first:pt-0">
                                      <div className="flex items-center justify-between gap-2">
                                        <span className="font-medium text-foreground/80">{toolName}</span>
                                        {durationMs !== null ? <span>{durationMs}ms</span> : null}
                                      </div>
                                      {outputSummary ? <p className="whitespace-pre-wrap">{outputSummary}</p> : null}
                                    </div>
                                  );
                                })}
                              </div>
                            </details>
                          ) : null}

                          {parsedQuestions && isLatestQuestion ? (
                            <StructuredQuestionCard
                              questions={parsedQuestions.questions}
                              onSubmit={(formattedAnswer) => void sendReplyContent(formattedAnswer)}
                              disabled={sendingReply || !canEdit}
                              readOnly={!canEdit}
                            />
                          ) : null}
                        </div>
                      );
                    })}

                    {/* Live thinking indicator */}
                    {liveSegments.isThinking ? (
                      <details
                        className="rounded-xl border border-purple-200/70 bg-purple-50/70 p-3 shadow-sm dark:border-purple-900/60 dark:bg-purple-950/20"
                        open={false}
                      >
                        <summary className="flex cursor-pointer list-none items-center gap-2 text-xs font-medium text-purple-900 dark:text-purple-200">
                          <Loader2 className="h-3 w-3 animate-spin" />
                          Thinking...
                        </summary>
                        {liveSegments.thinkingText ? (
                          <div className="mt-2 max-h-40 overflow-auto text-xs text-muted-foreground">
                            <MarkdownContent content={liveSegments.thinkingText} className="text-[11px]" />
                          </div>
                        ) : null}
                      </details>
                    ) : liveSegments.thinkingText ? (
                      <details className="rounded-xl border border-purple-200/50 bg-purple-50/40 p-3 shadow-sm dark:border-purple-900/40 dark:bg-purple-950/10">
                        <summary className="flex cursor-pointer list-none items-center gap-2 text-xs font-medium text-muted-foreground">
                          Thought for a moment
                        </summary>
                        <div className="mt-2 max-h-40 overflow-auto text-xs text-muted-foreground">
                          <MarkdownContent content={liveSegments.thinkingText} className="text-[11px]" />
                        </div>
                      </details>
                    ) : null}

                    {/* Live chat text (everything outside known XML tags) */}
                    {liveSegments.chatText ? (
                      <MarkdownContent content={liveSegments.chatText} />
                    ) : null}

                    {run.status === 'running' && latestTool ? (
                      <div className="flex items-center gap-2 py-1 text-[11px] text-muted-foreground">
                        {latestTool.status === 'running' ? (
                          <Loader2 className="h-3 w-3 animate-spin shrink-0" />
                        ) : (
                          <span className="h-3 w-3 shrink-0" />
                        )}
                        <span className="truncate font-medium">{latestTool.name}</span>
                        <span className="shrink-0">
                          {latestTool.status === 'running'
                            ? ''
                            : latestTool.durationMs
                              ? `${(latestTool.durationMs / 1000).toFixed(1)}s`
                              : 'done'}
                        </span>
                      </div>
                    ) : null}
                    <div ref={chatEndRef} />
                  </div>

                  {(run.invocation_mode === 'interactive' ? ACTIVE_RUN_STATUSES.has(run.status) : run.status === 'awaiting_input') && canEdit ? (
                    <div className="space-y-2 rounded-md border border-border/60 bg-background/80 p-3">
                      <Label className="text-xs">Reply To Agent</Label>
                      <Textarea
                        value={reply}
                        onChange={(event) => setReply(event.target.value)}
                        placeholder="Clarify scope, answer a question, or request changes."
                        rows={3}
                      />
                      <Button onClick={() => void sendReplyContent(reply)} disabled={!reply.trim() || sendingReply} className="gap-1.5">
                        {sendingReply ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
                        Send Reply
                      </Button>
                    </div>
                  ) : null}
                </div>

                <div className="min-h-0 space-y-3 overflow-y-auto">
                  {run.status === 'awaiting_approval' ? (
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
                          Use the approval card in the chat transcript to approve or request changes.
                        </p>
                      </div>
                    </div>
                  ) : null}

                  {/* Live streaming spec draft (shown while streaming, before persisted artifact) */}
                  {(liveSegments.isStreamingSpecDraft || liveSegments.specDraftText) && !latestSpecDraftPreview ? (
                    <div className="animate-in fade-in-0 rounded-md border border-border/60 bg-background/80 p-3 duration-200">
                      <div className="mb-2 flex items-center gap-2">
                        <FileText className="h-4 w-4 text-muted-foreground" />
                        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                          Live Draft
                        </p>
                        {liveSegments.isStreamingSpecDraft ? (
                          <Loader2 className="h-3 w-3 animate-spin text-muted-foreground" />
                        ) : null}
                      </div>
                      <div className="max-h-[280px] overflow-auto rounded-md bg-muted/40 p-3">
                        <MarkdownContent content={liveSegments.specDraftText} className="text-[12px] leading-5" />
                      </div>
                    </div>
                  ) : latestSpecDraftPreview?.spec_markdown ? (
                    <div className="rounded-md border border-border/60 bg-background/80 p-3">
                      <div className="mb-2 flex items-center gap-2">
                        <FileText className="h-4 w-4 text-muted-foreground" />
                        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Live Draft</p>
                      </div>
                      <div className="space-y-2">
                        {latestSpecDraftPreview.title ? (
                          <div>
                            <p className="text-sm font-semibold">{latestSpecDraftPreview.title}</p>
                            {latestSpecDraftPreview.summary ? (
                              <p className="text-xs text-muted-foreground">{latestSpecDraftPreview.summary}</p>
                            ) : null}
                          </div>
                        ) : null}
                        <div className="max-h-[280px] overflow-auto rounded-md bg-muted/40 p-3">
                          <MarkdownContent content={latestSpecDraftPreview.spec_markdown} className="text-[12px] leading-5" />
                        </div>
                      </div>
                    </div>
                  ) : null}

                  {/* Live streaming story plan (shown while streaming, before persisted artifact) */}
                  {(liveSegments.isStreamingStoryPlan || liveSegments.storyPlanText) && !latestStoryPlanPreview ? (
                    <div className="animate-in fade-in-0 rounded-md border border-border/60 bg-background/80 p-3 duration-200">
                      <div className="mb-2 flex items-center gap-2">
                        <Sparkles className="h-4 w-4 text-muted-foreground" />
                        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                          Story Plan
                        </p>
                        {liveSegments.isStreamingStoryPlan ? (
                          <Loader2 className="h-3 w-3 animate-spin text-muted-foreground" />
                        ) : null}
                      </div>
                      <div className="max-h-[280px] overflow-auto rounded-md bg-muted/40 p-3">
                        <pre className="whitespace-pre-wrap text-[12px] leading-5 text-foreground">
                          {liveSegments.storyPlanText}
                        </pre>
                      </div>
                    </div>
                  ) : latestStoryPlanPreview ? (
                    <div className="rounded-md border border-border/60 bg-background/80 p-3">
                      <div className="mb-2 flex items-center gap-2">
                        <Sparkles className="h-4 w-4 text-muted-foreground" />
                        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Current Plan</p>
                      </div>
                      <div className="space-y-3">
                        {latestStoryPlanPreview.summary ? <MarkdownContent content={latestStoryPlanPreview.summary} /> : null}
                        <div className="flex flex-wrap gap-2 text-[11px] text-muted-foreground">
                          <span>{latestStoryPlanPreview.proposed_stories?.length ?? 0} stories</span>
                          {latestStoryPlanPreview.risks?.length ? <span>{latestStoryPlanPreview.risks.length} risks</span> : null}
                          {latestStoryPlanPreview.open_questions?.length ? <span>{latestStoryPlanPreview.open_questions.length} open questions</span> : null}
                        </div>
                        {latestStoryPlanPreview.proposed_stories?.length ? (
                          <div className="space-y-1">
                            {latestStoryPlanPreview.proposed_stories.slice(0, 5).map((story, index) => (
                              <p key={`${story.ref ?? story.name}-${index}`} className="text-xs text-foreground/80">
                                {story.ref ? `${story.ref}: ` : ''}{story.name}
                              </p>
                            ))}
                          </div>
                        ) : null}
                      </div>
                    </div>
                  ) : null}

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
