import { useCallback, useEffect, useMemo, useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import {
  Bot,
  CheckCircle2,
  ChevronRight,
  ClipboardCheck,
  Flag,
  Loader2,
  MessageSquare,
  Play,
  RotateCcw,
  Send,
  Wrench,
  X,
  XCircle,
} from 'lucide-react';

import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceAccess, usePermissions, useEpics } from '@/hooks/queries';
import {
  useFlowRuns,
  useFlowTemplates,
  useStartFlowRun,
  useFlowRun,
  useFlowNodeMessages,
  useSendFlowNodeMessage,
  useSendFlowNodeAction,
  useCancelFlowRun,
} from '@/hooks/queries/useFlow';
import { agentService } from '@/lib/services/agentService';
import type {
  FlowSpec,
  FlowNodeSpec,
  FlowRunView,
  FlowNodeRun,
  FlowStatus,
  FlowNodeStatus,
  FlowNodeType,
  Agent,
  EpicWithStats,
  StartFlowRunRequest,
  StartEpicPlanningFlowInput,
  PlanningSessionMessage,
} from '@/lib/pmTypes';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { Textarea } from '@/components/ui/textarea';
import { ScrollArea } from '@/components/ui/scroll-area';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const NODE_LABELS: Record<string, string> = {
  ensure_spec_doc: 'Ensure Spec Document',
  spec_planning: 'Plan Specification',
  spec_approval: 'Approve Specification',
  story_planning: 'Plan Stories',
  plan_approval: 'Approve Story Plan',
  create_stories: 'Create Stories',
  done: 'Complete',
};

const NODE_ICONS: Record<FlowNodeType, typeof Wrench> = {
  system_action: Wrench,
  interactive_agent: MessageSquare,
  agent_task: Bot,
  approval_gate: ClipboardCheck,
  terminal: Flag,
};

const NODE_TYPE_LABELS: Record<FlowNodeType, string> = {
  system_action: 'System',
  interactive_agent: 'Interactive',
  agent_task: 'Autonomous',
  approval_gate: 'Approval',
  terminal: 'Terminal',
};

const RUN_STATUS_CONFIG: Record<FlowStatus, { label: string; className: string }> = {
  running: { label: 'Running', className: 'bg-blue-500/10 text-blue-600 dark:text-blue-400' },
  awaiting_input: { label: 'Awaiting Input', className: 'bg-amber-500/10 text-amber-600 dark:text-amber-400' },
  awaiting_approval: { label: 'Awaiting Approval', className: 'bg-amber-500/10 text-amber-600 dark:text-amber-400' },
  completed: { label: 'Completed', className: 'bg-green-500/10 text-green-600 dark:text-green-400' },
  failed: { label: 'Failed', className: 'bg-red-500/10 text-red-600 dark:text-red-400' },
  cancelled: { label: 'Cancelled', className: 'bg-zinc-500/10 text-zinc-500' },
};

const NODE_STATUS_DOT: Record<FlowNodeStatus, string> = {
  queued: 'bg-zinc-300 dark:bg-zinc-600',
  running: 'bg-blue-500',
  awaiting_input: 'bg-amber-500',
  awaiting_approval: 'bg-amber-500',
  completed: 'bg-green-500',
  failed: 'bg-red-500',
  cancelled: 'bg-zinc-400',
  skipped: 'bg-zinc-300 dark:bg-zinc-600',
};

const ACTIVE_FLOW_STATUSES = new Set<FlowStatus>(['running', 'awaiting_input', 'awaiting_approval']);

// ---------------------------------------------------------------------------
// FlowPipeline — Vertical timeline of flow nodes
// ---------------------------------------------------------------------------

function FlowPipeline({
  nodes,
  nodeRuns,
  compact = false,
  onNodeClick,
}: {
  nodes: FlowNodeSpec[];
  nodeRuns?: FlowNodeRun[];
  compact?: boolean;
  onNodeClick?: (nodeId: string, nodeRun?: FlowNodeRun) => void;
}) {
  const getNodeRun = (nodeId: string): FlowNodeRun | undefined => {
    if (!nodeRuns) return undefined;
    for (let i = nodeRuns.length - 1; i >= 0; i--) {
      if (nodeRuns[i].node_id === nodeId) return nodeRuns[i];
    }
    return undefined;
  };

  return (
    <div className="flex flex-col">
      {nodes.map((node, idx) => {
        const nr = getNodeRun(node.id);
        const Icon = NODE_ICONS[node.type] ?? Wrench;
        const isLast = idx === nodes.length - 1;
        const label = NODE_LABELS[node.id] ?? node.id;
        const statusDot = nr ? NODE_STATUS_DOT[nr.status] : 'bg-zinc-200 dark:bg-zinc-700';
        const isClickable = !!onNodeClick && !!nr;

        return (
          <div key={node.id} className="flex gap-3">
            {/* Timeline column */}
            <div className="flex flex-col items-center w-5 shrink-0">
              <div
                className={`h-5 w-5 rounded-full flex items-center justify-center ring-2 ring-background ${statusDot}`}
              >
                {nr?.status === 'completed' && <CheckCircle2 className="h-3 w-3 text-white" />}
                {nr?.status === 'running' && <Loader2 className="h-3 w-3 text-white animate-spin" />}
                {(nr?.status === 'awaiting_input' || nr?.status === 'awaiting_approval') && (
                  <div className="h-2 w-2 rounded-full bg-white" />
                )}
                {nr?.status === 'failed' && <XCircle className="h-3 w-3 text-white" />}
              </div>
              {!isLast && (
                <div className={`w-px flex-1 min-h-4 ${nr?.status === 'completed' ? 'bg-green-500/40' : 'bg-border'}`} />
              )}
            </div>

            {/* Content */}
            <div
              className={`pb-${compact ? '2' : '3'} flex-1 min-w-0 ${isClickable ? 'cursor-pointer hover:bg-muted/50 -mx-1 px-1 rounded' : ''}`}
              onClick={isClickable ? () => onNodeClick(node.id, nr) : undefined}
            >
              <div className="flex items-center gap-2">
                <Icon className={`h-3.5 w-3.5 shrink-0 ${nr ? 'text-foreground' : 'text-muted-foreground'}`} />
                <span className={`text-sm font-medium truncate ${nr ? '' : 'text-muted-foreground'}`}>
                  {label}
                </span>
              </div>
              {!compact && (
                <div className="flex items-center gap-1.5 mt-0.5 ml-5.5">
                  <span className="text-[11px] text-muted-foreground">
                    {NODE_TYPE_LABELS[node.type]}
                    {node.required_mode && ` · ${node.required_mode}`}
                  </span>
                  {nr && nr.status !== 'queued' && (
                    <Badge variant="secondary" className="text-[10px] h-4 px-1">
                      {nr.status.replace(/_/g, ' ')}
                    </Badge>
                  )}
                </div>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}

// ---------------------------------------------------------------------------
// FlowTemplateCard
// ---------------------------------------------------------------------------

function FlowTemplateCard({ spec, onStart }: { spec: FlowSpec; onStart: () => void }) {
  const templateName = spec.template_id === 'pm.epic_planning_v1' ? 'Epic Planning' : spec.template_id;

  return (
    <Card className="overflow-hidden">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-sm font-semibold">{templateName}</h3>
            <p className="text-[12px] text-muted-foreground mt-0.5">
              Target: {spec.target_type} · Triggers: {spec.supported_triggers.join(', ')}
            </p>
          </div>
          <Badge variant="outline" className="text-[11px] shrink-0">
            v{spec.template_version}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="pt-0">
        <div className="rounded-md border border-border/60 bg-muted/20 p-3">
          <FlowPipeline nodes={spec.nodes} />
        </div>
        <Button size="sm" className="mt-3 gap-1.5" onClick={onStart}>
          <Play className="h-3.5 w-3.5" />
          Start Flow
        </Button>
      </CardContent>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// StartFlowDialog
// ---------------------------------------------------------------------------

function StartFlowDialog({
  open,
  onOpenChange,
  workspaceId,
  templates,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  templates: FlowSpec[];
}) {
  const [templateId, setTemplateId] = useState(templates[0]?.template_id ?? '');
  const [targetEpicId, setTargetEpicId] = useState('');
  const [specPlannerId, setSpecPlannerId] = useState('');
  const [storyPlannerId, setStoryPlannerId] = useState('');
  const [context, setContext] = useState('');
  const [agents, setAgents] = useState<Agent[]>([]);

  const { data: epics } = useEpics(workspaceId);
  const startMutation = useStartFlowRun(workspaceId);

  useEffect(() => {
    if (!open || !workspaceId) return;
    agentService.list(workspaceId).then((res) => {
      if (res.data) setAgents(res.data);
    });
  }, [open, workspaceId]);

  const plannerAgents = useMemo(
    () => agents.filter((a) => a.agent_class === 'product_planner'),
    [agents],
  );

  const handleStart = async () => {
    if (!targetEpicId || !specPlannerId) return;
    const input: StartEpicPlanningFlowInput = {
      spec_planner_agent_id: specPlannerId,
      ...(storyPlannerId ? { story_planner_agent_id: storyPlannerId } : {}),
      ...(context.trim() ? { additional_context: context.trim() } : {}),
    };
    const req: StartFlowRunRequest = {
      template_id: templateId,
      target_type: 'epic',
      target_id: targetEpicId,
      input: input as unknown as Record<string, unknown>,
    };
    await startMutation.mutateAsync(req);
    onOpenChange(false);
    // Reset
    setTargetEpicId('');
    setContext('');
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Start Flow</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          {templates.length > 1 && (
            <div className="space-y-1.5">
              <Label>Template</Label>
              <Select value={templateId} onValueChange={setTemplateId}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {templates.map((t) => (
                    <SelectItem key={t.template_id} value={t.template_id}>
                      {t.template_id === 'pm.epic_planning_v1' ? 'Epic Planning v1' : t.template_id}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          <div className="space-y-1.5">
            <Label>Target Epic</Label>
            <Select value={targetEpicId} onValueChange={setTargetEpicId}>
              <SelectTrigger><SelectValue placeholder="Select an epic..." /></SelectTrigger>
              <SelectContent>
                {(epics ?? []).map((e: EpicWithStats) => (
                  <SelectItem key={e.epic.id} value={e.epic.id}>{e.epic.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1.5">
            <Label>Spec Planner Agent</Label>
            <Select value={specPlannerId} onValueChange={setSpecPlannerId}>
              <SelectTrigger><SelectValue placeholder="Select a planner agent..." /></SelectTrigger>
              <SelectContent>
                {plannerAgents.map((a) => (
                  <SelectItem key={a.id} value={a.id}>{a.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1.5">
            <Label>Story Planner Agent <span className="text-muted-foreground font-normal">(optional)</span></Label>
            <Select value={storyPlannerId || '_none'} onValueChange={(v) => setStoryPlannerId(v === '_none' ? '' : v)}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="_none">Same as spec planner</SelectItem>
                {plannerAgents.map((a) => (
                  <SelectItem key={a.id} value={a.id}>{a.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1.5">
            <Label>Additional Context <span className="text-muted-foreground font-normal">(optional)</span></Label>
            <Textarea
              value={context}
              onChange={(e) => setContext(e.target.value)}
              placeholder="Any extra instructions for the planning agents..."
              rows={3}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button
            onClick={handleStart}
            disabled={!targetEpicId || !specPlannerId || startMutation.isPending}
          >
            {startMutation.isPending ? (
              <><Loader2 className="mr-1.5 h-4 w-4 animate-spin" /> Starting...</>
            ) : (
              <><Play className="mr-1.5 h-4 w-4" /> Start</>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// FlowRunRow
// ---------------------------------------------------------------------------

function FlowRunRow({
  run,
  onClick,
}: {
  run: FlowRunView;
  onClick: (run: FlowRunView) => void;
}) {
  const status = RUN_STATUS_CONFIG[run.run.status] ?? RUN_STATUS_CONFIG.running;
  const templateName = run.spec.template_id === 'pm.epic_planning_v1' ? 'Epic Planning' : run.spec.template_id;

  // Find current node label
  const currentNodeLabel = run.run.current_node_id
    ? (NODE_LABELS[run.run.current_node_id] ?? run.run.current_node_id)
    : '—';

  return (
    <button
      type="button"
      className="flex items-center gap-3 px-4 py-2.5 w-full text-left border-b border-border/60 last:border-b-0 hover:bg-muted/40 transition-colors"
      onClick={() => onClick(run)}
    >
      <Badge variant="secondary" className={`text-[11px] shrink-0 ${status.className}`}>
        {status.label}
      </Badge>
      <span className="text-sm font-medium min-w-[100px] shrink-0">{templateName}</span>
      <span className="text-sm text-muted-foreground truncate flex-1">
        {currentNodeLabel}
      </span>
      <span className="text-[12px] text-muted-foreground shrink-0">
        {formatDistanceToNow(new Date(run.run.created_at), { addSuffix: true })}
      </span>
      <ChevronRight className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
    </button>
  );
}

// ---------------------------------------------------------------------------
// InteractiveChat — Chat interface for interactive nodes
// ---------------------------------------------------------------------------

function InteractiveChat({
  wsId,
  flowRunId,
  nodeRunId,
}: {
  wsId: string;
  flowRunId: string;
  nodeRunId: string;
}) {
  const [message, setMessage] = useState('');
  const { data: messages } = useFlowNodeMessages(wsId, flowRunId, nodeRunId);
  const sendMessage = useSendFlowNodeMessage(wsId, flowRunId, nodeRunId);

  const handleSend = () => {
    if (!message.trim()) return;
    sendMessage.mutate(message.trim());
    setMessage('');
  };

  return (
    <div className="flex flex-col h-full">
      <ScrollArea className="flex-1 p-3">
        <div className="space-y-3">
          {(messages ?? []).map((msg: PlanningSessionMessage) => (
            <div
              key={msg.id}
              className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}
            >
              <div
                className={`max-w-[85%] rounded-lg px-3 py-2 text-sm ${
                  msg.role === 'user'
                    ? 'bg-primary text-primary-foreground'
                    : 'bg-muted'
                }`}
              >
                <p className="whitespace-pre-wrap break-words">{msg.content}</p>
              </div>
            </div>
          ))}
        </div>
      </ScrollArea>
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
          onClick={handleSend}
          disabled={!message.trim() || sendMessage.isPending}
        >
          <Send className="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// FlowRunDetailSheet
// ---------------------------------------------------------------------------

function FlowRunDetailSheet({
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

  const data = liveData ?? runView;
  if (!data) return null;

  const status = RUN_STATUS_CONFIG[data.run.status] ?? RUN_STATUS_CONFIG.running;
  const templateName = data.spec.template_id === 'pm.epic_planning_v1' ? 'Epic Planning' : data.spec.template_id;
  const isActive = ACTIVE_FLOW_STATUSES.has(data.run.status);

  // Find the active node run for interactive/approval actions
  const activeNodeRun = data.node_runs.find(
    (nr) => nr.status === 'awaiting_input' || nr.status === 'awaiting_approval' || nr.status === 'running',
  );
  const activeNodeSpec = activeNodeRun
    ? data.spec.nodes.find((n) => n.id === activeNodeRun.node_id)
    : undefined;

  // Find failed node for retry
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
                    {NODE_LABELS[activeNodeRun.node_id] ?? activeNodeRun.node_id} — Chat
                  </h4>
                </div>
                <div className="flex-1 min-h-0">
                  <InteractiveChat wsId={workspaceId} flowRunId={flowRunId} nodeRunId={activeNodeRun.id} />
                </div>
                <div className="px-5 pb-3">
                  <FinalizeButton wsId={workspaceId} flowRunId={flowRunId} nodeRunId={activeNodeRun.id} />
                </div>
              </div>
            )}

            {activeNodeRun && activeNodeSpec?.type === 'approval_gate' && activeNodeRun.status === 'awaiting_approval' && (
              <div className="px-5 py-4">
                <h4 className="text-[11px] font-medium text-muted-foreground uppercase tracking-wide mb-3">
                  {NODE_LABELS[activeNodeRun.node_id] ?? activeNodeRun.node_id}
                </h4>
                <p className="text-sm text-muted-foreground mb-4">
                  Review the output and approve or reject to continue the flow.
                </p>
                <ApprovalButtons wsId={workspaceId} flowRunId={flowRunId} nodeRunId={activeNodeRun.id} />
              </div>
            )}

            {failedNodeRun && !activeNodeRun && (
              <div className="px-5 py-4">
                <h4 className="text-[11px] font-medium text-muted-foreground uppercase tracking-wide mb-3">
                  Failed: {NODE_LABELS[failedNodeRun.node_id] ?? failedNodeRun.node_id}
                </h4>
                {failedNodeRun.error_message && (
                  <p className="text-sm text-destructive mb-3">{failedNodeRun.error_message}</p>
                )}
                <RetryButton wsId={workspaceId} flowRunId={flowRunId} nodeRunId={failedNodeRun.id} />
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
                  {NODE_LABELS[activeNodeRun.node_id] ?? activeNodeRun.node_id}
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

// ---------------------------------------------------------------------------
// Action Buttons (extracted to avoid hook rules in conditionals)
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

function ApprovalButtons({ wsId, flowRunId, nodeRunId }: { wsId: string; flowRunId: string; nodeRunId: string }) {
  const sendAction = useSendFlowNodeAction(wsId, flowRunId, nodeRunId);
  return (
    <div className="flex gap-2">
      <Button
        size="sm"
        onClick={() => sendAction.mutate({ actionType: 'approve' })}
        disabled={sendAction.isPending}
        className="flex-1"
      >
        {sendAction.isPending ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <CheckCircle2 className="mr-1.5 h-3.5 w-3.5" />}
        Approve
      </Button>
      <Button
        size="sm"
        variant="outline"
        className="flex-1 text-destructive hover:text-destructive"
        onClick={() => sendAction.mutate({ actionType: 'reject' })}
        disabled={sendAction.isPending}
      >
        <XCircle className="mr-1.5 h-3.5 w-3.5" />
        Reject
      </Button>
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
// FlowsPage
// ---------------------------------------------------------------------------

export function FlowsPage() {
  useTitle('Flows');
  const { currentWorkspace: workspace } = useWorkspaceStore();
  const workspaceId = workspace?.id ?? '';
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { has } = usePermissions(access);
  const canEdit = has('pm.edit');

  const { data: templates } = useFlowTemplates(workspaceId);
  const { data: runsData, isLoading: runsLoading } = useFlowRuns(workspaceId);

  const [startDialogOpen, setStartDialogOpen] = useState(false);
  const [selectedRun, setSelectedRun] = useState<FlowRunView | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);

  const runs = runsData?.data ?? [];

  const handleRunClick = useCallback((run: FlowRunView) => {
    setSelectedRun(run);
    setSheetOpen(true);
  }, []);

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <div className="max-w-5xl mx-auto space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Flows</h1>
        {canEdit && (
          <Button size="sm" onClick={() => setStartDialogOpen(true)}>
            <Play className="mr-1.5 h-4 w-4" />
            Start Flow
          </Button>
        )}
      </div>

      {/* Templates */}
      {templates && templates.length > 0 && (
        <section>
          <h2 className="text-[11px] font-medium text-muted-foreground uppercase tracking-wide mb-3">
            Flow Templates
          </h2>
          <div className="grid gap-4 sm:grid-cols-1 lg:grid-cols-2">
            {templates.map((spec) => (
              <FlowTemplateCard
                key={spec.template_id}
                spec={spec}
                onStart={() => setStartDialogOpen(true)}
              />
            ))}
          </div>
        </section>
      )}

      {/* Recent Runs */}
      <section>
        <h2 className="text-[11px] font-medium text-muted-foreground uppercase tracking-wide mb-3">
          Recent Runs
        </h2>
        {runsLoading && (
          <p className="text-sm text-muted-foreground">Loading runs...</p>
        )}
        {!runsLoading && runs.length === 0 && (
          <div className="rounded-lg border border-dashed border-border/60 p-8 flex flex-col items-center text-center">
            <Play className="h-6 w-6 text-muted-foreground/50 mb-2" />
            <p className="text-sm text-muted-foreground">No flow runs yet.</p>
            <p className="text-[12px] text-muted-foreground/70 mt-1">
              Start a flow from a template above to begin orchestrating work.
            </p>
          </div>
        )}
        {runs.length > 0 && (
          <div className="rounded-lg border border-border overflow-hidden">
            {runs.map((run) => (
              <FlowRunRow key={run.run.id} run={run} onClick={handleRunClick} />
            ))}
          </div>
        )}
      </section>

      {/* Start Dialog */}
      {templates && (
        <StartFlowDialog
          open={startDialogOpen}
          onOpenChange={setStartDialogOpen}
          workspaceId={workspaceId}
          templates={templates}
        />
      )}

      {/* Run Detail Sheet */}
      <FlowRunDetailSheet
        open={sheetOpen}
        onOpenChange={setSheetOpen}
        runView={selectedRun}
        workspaceId={workspaceId}
      />
    </div>
  );
}
