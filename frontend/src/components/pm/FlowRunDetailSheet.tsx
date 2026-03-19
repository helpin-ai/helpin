import { useEffect, useMemo, useRef, useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import Markdown from 'react-markdown';
import {
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Loader2,
  RotateCcw,
  Send,
  Wrench,
  X,
  XCircle,
} from 'lucide-react';

import {
  useFlowRun,
  useFlowNodeMessages,
  useSendFlowNodeMessage,
  useSendFlowNodeAction,
  useCancelFlowRun,
  useCancelActiveFlowRunByTarget,
} from '@/hooks/queries/useFlow';
import {
  planningPendingLabel,
  shouldShowPlanningEmptyState,
  usePlanningStream,
} from '@/hooks/usePlanningStream';
import { parseStructuredQuestions } from '@/components/pm/parseStructuredQuestions';
import { extractArtifacts, type ChatArtifact } from '@/components/pm/parseArtifacts';
import { StructuredQuestionCard } from '@/components/pm/StructuredQuestionCard';
import { FlowPipeline } from '@/components/pm/FlowPipeline';
import {
  RUN_STATUS_CONFIG,
  ACTIVE_FLOW_STATUSES,
  templateLabel,
  nodeLabel,
} from '@/components/pm/flowConstants';
import type {
  FlowRunView,
  PlanningSessionMessage,
  ToolInvocation,
} from '@/lib/pmTypes';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { Textarea } from '@/components/ui/textarea';

// ---------------------------------------------------------------------------
// XML tag strippers
// ---------------------------------------------------------------------------

function stripXmlTags(content: string): string {
  return content
    .replace(/<spec_draft>[\s\S]*?<\/spec_draft>/g, '')
    .replace(/<story_plan>[\s\S]*?<\/story_plan>/g, '')
    .replace(/<questions>[\s\S]*?<\/questions>/g, '')
    .trim();
}

function stripXmlTagsStreaming(content: string): string {
  return content
    .replace(/<spec_draft>[\s\S]*?<\/spec_draft>/g, '')
    .replace(/<story_plan>[\s\S]*?<\/story_plan>/g, '')
    .replace(/<questions>[\s\S]*?<\/questions>/g, '')
    .replace(/<spec_draft>[\s\S]*$/g, '')
    .replace(/<story_plan>[\s\S]*$/g, '')
    .replace(/<questions>[\s\S]*$/g, '')
    .trim();
}

// ---------------------------------------------------------------------------
// ToolCallBadge
// ---------------------------------------------------------------------------

function ToolCallBadge({ invocation }: { invocation: ToolInvocation }) {
  const [expanded, setExpanded] = useState(false);
  return (
    <div className="inline-block">
      <button
        type="button"
        className="inline-flex items-center gap-1 rounded-md border border-border/60 bg-muted/40 px-2 py-0.5 text-[11px] text-muted-foreground hover:bg-muted transition-colors"
        onClick={() => setExpanded(!expanded)}
      >
        <Wrench className="h-3 w-3 shrink-0" />
        <span className="font-medium">{invocation.tool_name}</span>
        {invocation.duration_ms > 0 && (
          <span className="text-muted-foreground/70">{(invocation.duration_ms / 1000).toFixed(1)}s</span>
        )}
        {invocation.output_summary && (
          expanded ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />
        )}
      </button>
      {expanded && invocation.output_summary && (
        <div className="mt-1 rounded-md border border-border/40 bg-muted/20 px-2.5 py-1.5 text-[11px] text-muted-foreground whitespace-pre-wrap">
          {invocation.output_summary}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// ArtifactView
// ---------------------------------------------------------------------------

function ArtifactView({ artifact }: { artifact: ChatArtifact }) {
  return (
    <div className="flex-1 overflow-y-auto p-4">
      <div className="prose prose-sm dark:prose-invert max-w-none">
        <Markdown>{artifact.content}</Markdown>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// InteractiveChat
// ---------------------------------------------------------------------------

function InteractiveChat({
  wsId,
  flowRunId,
  nodeRunId,
  sessionId,
}: {
  wsId: string;
  flowRunId: string;
  nodeRunId: string;
  sessionId?: string;
}) {
  const [message, setMessage] = useState('');
  const [activeTab, setActiveTab] = useState<'chat' | 'spec_draft' | 'story_plan'>('chat');
  const scrollRef = useRef<HTMLDivElement>(null);
  const { data: messages } = useFlowNodeMessages(wsId, flowRunId, nodeRunId);
  const sendMessage = useSendFlowNodeMessage(wsId, flowRunId, nodeRunId);

  const {
    isStreaming,
    turnPending,
    pendingState,
    markTurnPending,
    activeToolCall,
    toolResults,
    error: streamError,
    streamingText,
  } = usePlanningStream(wsId, sessionId);

  const artifacts = useMemo(() => extractArtifacts(messages ?? []), [messages]);
  const activeArtifact = artifacts.find((a) => a.type === activeTab);

  useEffect(() => {
    if (activeTab !== 'chat') return;
    const el = scrollRef.current;
    if (!el) return;
    const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 150;
    if (isNearBottom) {
      el.scrollTop = el.scrollHeight;
    }
  }, [messages, isStreaming, activeToolCall, toolResults, streamingText, activeTab]);

  const handleSend = (content?: string) => {
    const text = content ?? message.trim();
    if (!text) return;
    markTurnPending();
    sendMessage.mutate(text);
    if (!content) setMessage('');
  };

  return (
    <div className="flex flex-col h-full">
      {/* Tab bar — only when artifacts exist */}
      {artifacts.length > 0 && (
        <div className="flex items-center gap-1 border-b border-border/60 px-3 pt-2">
          <button
            onClick={() => setActiveTab('chat')}
            className={`px-2.5 py-1.5 text-xs font-medium rounded-t-md transition-colors ${
              activeTab === 'chat'
                ? 'bg-background text-foreground border border-b-0 border-border/60'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            Chat
          </button>
          {artifacts.map((a) => (
            <button
              key={a.type}
              onClick={() => setActiveTab(a.type)}
              className={`px-2.5 py-1.5 text-xs font-medium rounded-t-md transition-colors ${
                activeTab === a.type
                  ? 'bg-background text-foreground border border-b-0 border-border/60'
                  : 'text-muted-foreground hover:text-foreground'
              }`}
            >
              {a.label}
            </button>
          ))}
        </div>
      )}

      {/* Artifact view */}
      {activeArtifact && <ArtifactView artifact={activeArtifact} />}

      {/* Chat messages */}
      <div ref={scrollRef} className={`flex-1 overflow-y-auto p-3 ${activeArtifact ? 'hidden' : ''}`}>
        <div className="space-y-3">
          {(messages ?? []).map((msg: PlanningSessionMessage) => {
            const isUser = msg.role === 'user';

            if (isUser) {
              return (
                <div key={msg.id} className="flex justify-end">
                  <div className="max-w-[85%] rounded-lg px-3 py-2 text-sm bg-primary text-primary-foreground">
                    <p className="whitespace-pre-wrap break-words">{msg.content}</p>
                  </div>
                </div>
              );
            }

            const parsed = parseStructuredQuestions(msg.content);
            const displayContent = stripXmlTags(msg.content);
            const toolInvocations = msg.tool_invocations ?? [];

            return (
              <div key={msg.id} className="flex justify-start">
                <div className="max-w-[85%] rounded-lg px-3 py-2 text-sm bg-muted">
                  {toolInvocations.length > 0 && (
                    <div className="flex flex-wrap gap-1 mb-2">
                      {toolInvocations.map((inv, i) => (
                        <ToolCallBadge key={`${msg.id}-tool-${i}`} invocation={inv} />
                      ))}
                    </div>
                  )}

                  {displayContent && (
                    <div className="prose prose-sm dark:prose-invert max-w-none break-words [&>*:first-child]:mt-0 [&>*:last-child]:mb-0">
                      <Markdown>{displayContent}</Markdown>
                    </div>
                  )}

                  {parsed && (
                    <StructuredQuestionCard
                      questions={parsed.questions}
                      onSubmit={(answer) => handleSend(answer)}
                      disabled={sendMessage.isPending || isStreaming}
                    />
                  )}
                </div>
              </div>
            );
          })}

          {/* Error indicator */}
          {streamError && !isStreaming && !turnPending && (
            <div className="flex justify-start">
              <div className="max-w-[85%] rounded-lg px-3 py-2 text-sm bg-destructive/10 text-destructive border border-destructive/20">
                <p className="font-medium text-[12px] mb-0.5">Agent error</p>
                <p className="whitespace-pre-wrap break-words">{streamError}</p>
              </div>
            </div>
          )}

          {/* Empty state — no messages yet and not streaming */}
          {shouldShowPlanningEmptyState({
            messageCount: (messages ?? []).length,
            isStreaming,
            turnPending,
            streamError,
          }) && (
            <div className="flex items-center justify-center py-8 text-muted-foreground">
              <div className="text-center">
                <Loader2 className="h-5 w-5 animate-spin mx-auto mb-2 text-muted-foreground/60" />
                <p className="text-[12px]">Starting planner...</p>
              </div>
            </div>
          )}

          {/* Live streaming indicators */}
          {(turnPending || isStreaming) && (
            <div className="flex justify-start">
              <div className="max-w-[85%] rounded-lg px-3 py-2 text-sm bg-muted space-y-2">
                {toolResults.length > 0 && (
                  <div className="flex flex-wrap gap-1">
                    {toolResults.map((inv, i) => (
                      <ToolCallBadge key={`live-result-${i}`} invocation={inv} />
                    ))}
                  </div>
                )}

                {activeToolCall && (
                  <div className="inline-flex items-center gap-1.5 rounded-md border border-blue-300/50 bg-blue-50/50 dark:border-blue-700/50 dark:bg-blue-950/30 px-2 py-1 text-[11px] text-blue-600 dark:text-blue-400">
                    <Loader2 className="h-3 w-3 animate-spin" />
                    <Wrench className="h-3 w-3" />
                    <span className="font-medium">{activeToolCall.tool_name}</span>
                  </div>
                )}

                {(() => {
                  const visibleText = stripXmlTagsStreaming(streamingText);
                  if (visibleText) {
                    return (
                      <div className="prose prose-sm dark:prose-invert max-w-none break-words">
                        <Markdown>{visibleText}</Markdown>
                      </div>
                    );
                  }
                  return (
                    <div className="flex items-center gap-1.5 text-[12px] text-muted-foreground">
                      <Loader2 className="h-3 w-3 animate-spin" />
                      <span>{turnPending ? planningPendingLabel(pendingState) : 'Thinking...'}</span>
                    </div>
                  );
                })()}
              </div>
            </div>
          )}
        </div>
      </div>
      <div className="border-t border-border/60 p-3 flex gap-2">
        <Textarea
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          placeholder="Type a message..."
          rows={1}
          className="min-h-[36px] resize-none"
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend(); }
          }}
        />
        <Button
          size="sm"
          className="shrink-0 h-9"
          onClick={() => handleSend()}
          disabled={!message.trim() || sendMessage.isPending || isStreaming}
        >
          <Send className="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Action Buttons
// ---------------------------------------------------------------------------

function FinalizeButton({ wsId, flowRunId, nodeRunId }: { wsId: string; flowRunId: string; nodeRunId: string }) {
  const sendAction = useSendFlowNodeAction(wsId, flowRunId, nodeRunId);
  return (
    <Button
      size="sm"
      variant="outline"
      className="w-full"
      onClick={() => sendAction.mutate({ actionType: 'finalize' })}
      disabled={sendAction.isPending}
    >
      {sendAction.isPending ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <CheckCircle2 className="mr-1.5 h-3.5 w-3.5" />}
      Finalize & Continue
    </Button>
  );
}

function ApprovalButtons({
  wsId,
  flowRunId,
  nodeRunId,
  actions,
}: {
  wsId: string;
  flowRunId: string;
  nodeRunId: string;
  actions: string[];
}) {
  const sendAction = useSendFlowNodeAction(wsId, flowRunId, nodeRunId);
  const [overrideText, setOverrideText] = useState('');
  const [requestComment, setRequestComment] = useState('');
  const [feedbackText, setFeedbackText] = useState('');
  const [error, setError] = useState<string | null>(null);

  const parseOptionalJSON = (value: string, label: string) => {
    const trimmed = value.trim();
    if (!trimmed) return undefined;
    try {
      return JSON.parse(trimmed) as Record<string, unknown>;
    } catch {
      throw new Error(`${label} must be valid JSON`);
    }
  };

  const handleApprove = () => {
    try {
      const payload = parseOptionalJSON(overrideText, 'Override payload');
      setError(null);
      sendAction.mutate({ actionType: 'approve', payload });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Invalid approve payload');
    }
  };

  const handleRequestChanges = () => {
    if (!requestComment.trim()) {
      setError('A reviewer comment is required to request changes.');
      return;
    }
    try {
      const structuredFeedback = parseOptionalJSON(feedbackText, 'Structured feedback');
      setError(null);
      sendAction.mutate({
        actionType: 'request_changes',
        payload: {
          comment: requestComment.trim(),
          ...(structuredFeedback ? { structured_feedback: structuredFeedback } : {}),
        },
      });
      setRequestComment('');
      setFeedbackText('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Invalid request changes payload');
    }
  };

  const handleReject = () => {
    setError(null);
    sendAction.mutate({ actionType: 'reject' });
  };

  return (
    <div className="space-y-3">
      <div className="space-y-1.5">
        <Label>Approve Override Payload <span className="text-muted-foreground font-normal">(optional JSON)</span></Label>
        <Textarea
          value={overrideText}
          onChange={(event) => setOverrideText(event.target.value)}
          rows={4}
          placeholder='{"proposed_stories":[...]}'
        />
      </div>

      {actions.includes('request_changes') && (
        <>
          <div className="space-y-1.5">
            <Label>Request Changes Comment</Label>
            <Textarea
              value={requestComment}
              onChange={(event) => setRequestComment(event.target.value)}
              rows={3}
              placeholder="Explain what needs to change before approval..."
            />
          </div>
          <div className="space-y-1.5">
            <Label>Structured Feedback <span className="text-muted-foreground font-normal">(optional JSON)</span></Label>
            <Textarea
              value={feedbackText}
              onChange={(event) => setFeedbackText(event.target.value)}
              rows={4}
              placeholder='{"focus":"trim scope"}'
            />
          </div>
        </>
      )}

      {error && <p className="text-xs text-destructive">{error}</p>}

      <div className="flex flex-wrap gap-2">
        {actions.includes('approve') && (
          <Button
            size="sm"
            onClick={handleApprove}
            disabled={sendAction.isPending}
            className="flex-1 min-w-[140px]"
          >
            {sendAction.isPending ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <CheckCircle2 className="mr-1.5 h-3.5 w-3.5" />}
            Approve
          </Button>
        )}
        {actions.includes('request_changes') && (
          <Button
            size="sm"
            variant="outline"
            onClick={handleRequestChanges}
            disabled={sendAction.isPending}
            className="flex-1 min-w-[140px]"
          >
            Request Changes
          </Button>
        )}
        {actions.includes('reject') && (
          <Button
            size="sm"
            variant="outline"
            className="flex-1 min-w-[140px] text-destructive hover:text-destructive"
            onClick={handleReject}
            disabled={sendAction.isPending}
          >
            <XCircle className="mr-1.5 h-3.5 w-3.5" />
            Reject
          </Button>
        )}
      </div>
    </div>
  );
}

function RetryButton({ wsId, flowRunId, nodeRunId }: { wsId: string; flowRunId: string; nodeRunId: string }) {
  const sendAction = useSendFlowNodeAction(wsId, flowRunId, nodeRunId);
  return (
    <Button
      size="sm"
      onClick={() => sendAction.mutate({ actionType: 'retry' })}
      disabled={sendAction.isPending}
    >
      {sendAction.isPending ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <RotateCcw className="mr-1.5 h-3.5 w-3.5" />}
      Retry
    </Button>
  );
}

// ---------------------------------------------------------------------------
// FlowRunDetailSheet
// ---------------------------------------------------------------------------

export function FlowRunDetailSheet({
  open,
  onOpenChange,
  runView,
  workspaceId,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  runView: FlowRunView | null;
  workspaceId: string;
}) {
  const flowRunId = runView?.run.id ?? '';
  const { data: liveData } = useFlowRun(workspaceId, open ? flowRunId : undefined);
  const cancelRun = useCancelFlowRun(workspaceId);
  const cancelByTarget = useCancelActiveFlowRunByTarget(workspaceId);

  const data = liveData ?? runView;
  if (!data) return null;

  const status = RUN_STATUS_CONFIG[data.run.status] ?? RUN_STATUS_CONFIG.running;
  const templateName = templateLabel(data.spec.template_id, data.spec.name);
  const isActive = ACTIVE_FLOW_STATUSES.has(data.run.status);

  const activeNodeRun = data.node_runs.find(
    (nr) => nr.status === 'awaiting_input' || nr.status === 'awaiting_approval' || nr.status === 'running',
  );
  const activeNodeSpec = activeNodeRun
    ? data.spec.nodes.find((n) => n.id === activeNodeRun.node_id)
    : undefined;

  const failedNodeRun = data.node_runs.find((nr) => nr.status === 'failed');

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="sm:max-w-xl w-full flex flex-col gap-0 p-0" showCloseButton={false}>
        <SheetHeader className="px-5 py-4 border-b border-border/60">
          <div className="flex items-center gap-2">
            <SheetTitle className="flex items-center gap-2 text-base flex-1 min-w-0">
              <span className="truncate">{templateName}</span>
              <Badge variant="secondary" className={`text-[11px] shrink-0 ${status.className}`}>
                {status.label}
              </Badge>
            </SheetTitle>
            <Button
              size="sm"
              variant="ghost"
              className="h-7 w-7 p-0 text-muted-foreground hover:text-foreground shrink-0"
              onClick={() => onOpenChange(false)}
            >
              <span className="sr-only">Close</span>
              <X className="h-4 w-4" />
            </Button>
          </div>
          <div className="flex items-center gap-3 text-[12px] text-muted-foreground mt-1">
            <span>Started {formatDistanceToNow(new Date(data.run.created_at), { addSuffix: true })}</span>
            {data.run.completed_at && (
              <span>Completed {formatDistanceToNow(new Date(data.run.completed_at), { addSuffix: true })}</span>
            )}
          </div>
        </SheetHeader>

        <div className="flex-1 flex flex-col min-h-0 overflow-hidden">
          {/* Pipeline visualization */}
          <div className="px-5 py-4 border-b border-border/60">
            <h4 className="text-[11px] font-medium text-muted-foreground uppercase tracking-wide mb-3">Pipeline</h4>
            <FlowPipeline nodes={data.spec.nodes} nodeRuns={data.node_runs} compact />
          </div>

          {/* Active node actions */}
          <div className="flex-1 overflow-auto">
            {activeNodeRun && activeNodeSpec?.type === 'interactive_agent' && activeNodeRun.status === 'awaiting_input' && (
              <div className="flex flex-col h-full">
                <div className="px-5 pt-3 pb-1">
                  <h4 className="text-[11px] font-medium text-muted-foreground uppercase tracking-wide">
                    {nodeLabel(activeNodeRun.node_id, activeNodeSpec?.label)} — Chat
                  </h4>
                </div>
                <div className="flex-1 min-h-0">
                  <InteractiveChat wsId={workspaceId} flowRunId={flowRunId} nodeRunId={activeNodeRun.id} sessionId={activeNodeRun.child_session_id} />
                </div>
                <div className="px-5 pb-3">
                  <FinalizeButton wsId={workspaceId} flowRunId={flowRunId} nodeRunId={activeNodeRun.id} />
                </div>
              </div>
            )}

            {activeNodeRun && activeNodeSpec?.type === 'approval_gate' && activeNodeRun.status === 'awaiting_approval' && (
              <div className="px-5 py-4">
                <h4 className="text-[11px] font-medium text-muted-foreground uppercase tracking-wide mb-3">
                  {nodeLabel(activeNodeRun.node_id, activeNodeSpec?.label)}
                </h4>
                <p className="text-sm text-muted-foreground mb-4">
                  Review the output and approve, request changes, or reject to continue the flow.
                </p>
                <ApprovalButtons
                  wsId={workspaceId}
                  flowRunId={flowRunId}
                  nodeRunId={activeNodeRun.id}
                  actions={activeNodeSpec.actions ?? []}
                />
              </div>
            )}

            {failedNodeRun && !activeNodeRun && (
              <div className="px-5 py-4">
                <h4 className="text-[11px] font-medium text-muted-foreground uppercase tracking-wide mb-3">
                  Failed: {nodeLabel(failedNodeRun.node_id, data.spec.nodes.find((n) => n.id === failedNodeRun.node_id)?.label)}
                </h4>
                {failedNodeRun.error_message && (
                  <p className="text-sm text-destructive mb-3">{failedNodeRun.error_message}</p>
                )}
                <div className="flex gap-2">
                  <RetryButton wsId={workspaceId} flowRunId={flowRunId} nodeRunId={failedNodeRun.id} />
                  {failedNodeRun.error_message?.includes('active planning session already exists') && (
                    <Button
                      size="sm"
                      variant="outline"
                      className="text-destructive hover:text-destructive"
                      onClick={() => cancelByTarget.mutate({
                        targetType: data.run.target_type,
                        targetId: data.run.target_id,
                      })}
                      disabled={cancelByTarget.isPending}
                    >
                      {cancelByTarget.isPending ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <X className="mr-1.5 h-3.5 w-3.5" />}
                      Cancel Existing Run
                    </Button>
                  )}
                </div>
              </div>
            )}

            {!activeNodeRun && !failedNodeRun && data.run.status === 'completed' && (
              <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
                <CheckCircle2 className="h-8 w-8 text-green-500 mb-2" />
                <p className="text-sm font-medium text-foreground">Flow completed</p>
              </div>
            )}

            {!activeNodeRun && !failedNodeRun && data.run.status === 'cancelled' && (
              <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
                <XCircle className="h-8 w-8 mb-2" />
                <p className="text-sm font-medium text-foreground">Flow cancelled</p>
                {data.run.cancellation_reason && (
                  <p className="text-sm mt-1">{data.run.cancellation_reason}</p>
                )}
              </div>
            )}

            {activeNodeRun && activeNodeRun.status === 'running' && (
              <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
                <Loader2 className="h-8 w-8 animate-spin text-blue-500 mb-2" />
                <p className="text-sm font-medium text-foreground">
                  {nodeLabel(activeNodeRun.node_id, activeNodeSpec?.label)}
                </p>
                <p className="text-[12px] mt-1">Processing...</p>
              </div>
            )}
          </div>

          {/* Cancel button */}
          {isActive && (
            <div className="px-5 py-3 border-t border-border/60">
              <Button
                variant="outline"
                size="sm"
                className="w-full text-destructive hover:text-destructive"
                onClick={() => cancelRun.mutate(flowRunId)}
                disabled={cancelRun.isPending}
              >
                {cancelRun.isPending ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <X className="mr-1.5 h-3.5 w-3.5" />}
                Cancel Flow
              </Button>
            </div>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}
