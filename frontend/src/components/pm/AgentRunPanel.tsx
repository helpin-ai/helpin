import { useCallback, useEffect, useState } from 'react';
import { Bot, Loader2, Play, Clock, CheckCircle2, XCircle, FileCode, FileText, GitPullRequest, StopCircle, ShieldCheck } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Separator } from '@/components/ui/separator';
import { agentService } from '@/lib/services/agentService';
import type { AgentRun, AgentRunArtifact } from '@/lib/pmTypes';
import { formatDistanceToNow, parseISO } from 'date-fns';

interface Props {
  storyId: string;
  workspaceId: string;
  assignedAgentId?: string;
}

const STATUS_CONFIG: Record<string, { label: string; icon: React.ReactNode; variant: 'default' | 'secondary' | 'destructive' | 'outline' }> = {
  queued: { label: 'Queued', icon: <Clock className="h-3 w-3" />, variant: 'secondary' },
  running: { label: 'Running', icon: <Loader2 className="h-3 w-3 animate-spin" />, variant: 'default' },
  awaiting_approval: { label: 'Awaiting approval', icon: <ShieldCheck className="h-3 w-3" />, variant: 'secondary' },
  completed: { label: 'Completed', icon: <CheckCircle2 className="h-3 w-3" />, variant: 'outline' },
  failed: { label: 'Failed', icon: <XCircle className="h-3 w-3" />, variant: 'destructive' },
  cancelled: { label: 'Cancelled', icon: <XCircle className="h-3 w-3" />, variant: 'secondary' },
};

const ARTIFACT_ICONS: Record<string, React.ReactNode> = {
  conversation_log: <FileText className="h-3.5 w-3.5" />,
  tool_log: <FileText className="h-3.5 w-3.5" />,
  diff: <FileCode className="h-3.5 w-3.5" />,
  test_report: <CheckCircle2 className="h-3.5 w-3.5" />,
  pr_metadata: <GitPullRequest className="h-3.5 w-3.5" />,
  agent_summary: <Bot className="h-3.5 w-3.5" />,
  file_bundle: <FileCode className="h-3.5 w-3.5" />,
  handoff_note: <FileText className="h-3.5 w-3.5" />,
};

export function AgentRunPanel({ storyId, workspaceId, assignedAgentId }: Props) {
  const [runs, setRuns] = useState<AgentRun[]>([]);
  const [selectedRun, setSelectedRun] = useState<AgentRun | null>(null);
  const [artifacts, setArtifacts] = useState<AgentRunArtifact[]>([]);
  const [triggering, setTriggering] = useState(false);
  const [actingOnRun, setActingOnRun] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchRuns = useCallback(async () => {
    if (!assignedAgentId) {
      setRuns([]);
      setLoading(false);
      return;
    }
    try {
      const res = await agentService.listRuns(workspaceId, assignedAgentId);
      const data = res.data?.data ?? [];
      const storyRuns = (Array.isArray(data) ? data : []).filter(
        (run: AgentRun) => run.target_type === 'story' && run.target_id === storyId
      );
      setRuns(storyRuns);
    } finally {
      setLoading(false);
    }
  }, [workspaceId, assignedAgentId, storyId]);

  const loadArtifacts = useCallback(async (runId: string) => {
    const res = await agentService.listRunArtifacts(workspaceId, runId);
    setArtifacts(res.data ?? []);
  }, [workspaceId]);

  useEffect(() => {
    fetchRuns();
  }, [fetchRuns]);

  useEffect(() => {
    const hasActive = runs.some(
      (run) => run.status === 'queued' || run.status === 'running' || run.status === 'awaiting_approval',
    );
    if (!hasActive) return;
    const interval = setInterval(fetchRuns, 5000);
    return () => clearInterval(interval);
  }, [runs, fetchRuns]);

  const handleRunAgent = async () => {
    setTriggering(true);
    try {
      await agentService.runAgent(workspaceId, storyId);
      await fetchRuns();
    } finally {
      setTriggering(false);
    }
  };

  const handleSelectRun = async (run: AgentRun) => {
    setSelectedRun(run);
    await loadArtifacts(run.id);
  };

  const handleCancelRun = async (runId: string) => {
    setActingOnRun(runId);
    try {
      await agentService.cancelRun(workspaceId, runId);
      await fetchRuns();
    } finally {
      setActingOnRun(null);
    }
  };

  const handleApproveRun = async (runId: string) => {
    setActingOnRun(runId);
    try {
      await agentService.approveRun(workspaceId, runId, { send_message: true });
      await fetchRuns();
      if (selectedRun?.id === runId) {
        await loadArtifacts(runId);
      }
    } finally {
      setActingOnRun(null);
    }
  };

  if (!assignedAgentId) return null;

  return (
    <div className="mt-6">
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Bot className="h-4 w-4 text-muted-foreground" />
          <h3 className="text-sm font-semibold">Agent Runs</h3>
        </div>
        <Button
          size="sm"
          variant="outline"
          onClick={handleRunAgent}
          disabled={triggering}
          className="gap-1.5"
        >
          {triggering ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Play className="h-3.5 w-3.5" />}
          Run Agent
        </Button>
      </div>

      {loading ? (
        <div className="flex items-center gap-2 py-4 text-xs text-muted-foreground">
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
          Loading runs...
        </div>
      ) : runs.length === 0 ? (
        <p className="py-2 text-xs text-muted-foreground">No runs yet. Click "Run Agent" to start.</p>
      ) : (
        <div className="space-y-2">
          {runs.map((run) => {
            const cfg = STATUS_CONFIG[run.status] ?? STATUS_CONFIG.queued;
            const isSelected = selectedRun?.id === run.id;
            return (
              <div key={run.id}>
                <button
                  onClick={() => handleSelectRun(run)}
                  className={`w-full rounded-md border px-3 py-2 text-left text-xs transition-colors hover:bg-accent/50 ${isSelected ? 'border-primary bg-accent/30' : 'border-border/60'}`}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Badge variant={cfg.variant} className="gap-1 px-1.5 py-0 text-[10px]">
                        {cfg.icon}
                        {cfg.label}
                      </Badge>
                      <span className="text-muted-foreground">
                        {formatDistanceToNow(parseISO(run.created_at), { addSuffix: true })}
                      </span>
                    </div>
                    <span className="text-muted-foreground">
                      {run.tokens_used > 0 && `${(run.tokens_used / 1000).toFixed(1)}k tokens`}
                    </span>
                  </div>
                  <div className="mt-1 flex flex-wrap items-center gap-2 text-[10px] text-muted-foreground">
                    <span>{run.runtime_kind}</span>
                    {run.runner_pool && <span>pool: {run.runner_pool}</span>}
                    {run.execution_stage && <span>stage: {run.execution_stage}</span>}
                    <span>approval: {run.approval_state}</span>
                    {run.handoff_state && <span>handoff: {run.handoff_state}</span>}
                  </div>
                  {(run.repo_full_name || run.working_branch || run.base_branch || run.last_heartbeat_at) && (
                    <div className="mt-1 flex flex-wrap items-center gap-2 text-[10px] text-muted-foreground">
                      {run.repo_full_name && <span>repo: {run.repo_full_name}</span>}
                      {run.base_branch && <span>base: {run.base_branch}</span>}
                      {run.working_branch && <span>branch: {run.working_branch}</span>}
                      {run.last_heartbeat_at && (
                        <span>
                          heartbeat {formatDistanceToNow(parseISO(run.last_heartbeat_at), { addSuffix: true })}
                        </span>
                      )}
                    </div>
                  )}
                  {run.error_message && (
                    <p className="mt-1 truncate text-destructive">{run.error_message}</p>
                  )}
                </button>

                {isSelected && (
                  <div className="ml-3 mb-1 mt-2 space-y-1.5 border-l-2 border-border pl-3">
                    <div className="flex flex-wrap gap-2">
                      {(run.status === 'queued' || run.status === 'running' || run.status === 'awaiting_approval') && (
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-7 gap-1 text-[11px]"
                          disabled={actingOnRun === run.id}
                          onClick={() => handleCancelRun(run.id)}
                        >
                          {actingOnRun === run.id ? <Loader2 className="h-3 w-3 animate-spin" /> : <StopCircle className="h-3 w-3" />}
                          Cancel
                        </Button>
                      )}
                      {run.approval_state === 'pending' && (
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-7 gap-1 text-[11px]"
                          disabled={actingOnRun === run.id}
                          onClick={() => handleApproveRun(run.id)}
                        >
                          {actingOnRun === run.id ? <Loader2 className="h-3 w-3 animate-spin" /> : <ShieldCheck className="h-3 w-3" />}
                          Approve
                        </Button>
                      )}
                    </div>

                    {artifacts.map((artifact) => (
                      <div key={artifact.id} className="rounded border border-border/60 bg-muted/30 p-2">
                        <div className="mb-1 flex items-center gap-1.5 text-[11px] font-medium">
                          {ARTIFACT_ICONS[artifact.artifact_type] ?? <FileText className="h-3.5 w-3.5" />}
                          <span className="capitalize">{artifact.artifact_type.replace(/_/g, ' ')}</span>
                          <span className="text-muted-foreground">({artifact.format})</span>
                        </div>
                        {artifact.inline_content && (
                          <pre className="max-h-32 overflow-auto whitespace-pre-wrap break-all text-[10px] text-muted-foreground">
                            {artifact.inline_content.slice(0, 2000)}
                            {artifact.inline_content.length > 2000 && '...'}
                          </pre>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      <Separator className="mt-4" />
    </div>
  );
}
