import { type ReactNode, useCallback, useEffect, useMemo, useState } from 'react';
import { formatDistanceToNow, parseISO } from 'date-fns';
import {
  Bot,
  CheckCircle2,
  Clock,
  KeyRound,
  Loader2,
  MessageSquareMore,
  RefreshCw,
  ShieldCheck,
  XCircle,
} from 'lucide-react';

import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { AgentRunDrawer } from '@/components/pm/AgentRunDrawer';
import { ACTIVE_RUN_STATUSES, getAgentRunDisplayStatus } from '@/components/pm/agentRunConstants';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries/useSession';
import { buildCodingSessionPath, canOpenCodingSession } from '@/lib/codingSessionSurface';
import { useTitle } from '@/hooks/useTitle';
import type { Agent, AgentRun } from '@/lib/pmTypes';
import { agentService } from '@/lib/services/agentService';
import { useWorkspaceStore } from '@/stores/workspaceStore';

const STATUS_ICONS: Record<string, ReactNode> = {
  queued: <Clock className="h-3 w-3" />,
  running: <Loader2 className="h-3 w-3 animate-spin" />,
  awaiting_input: <MessageSquareMore className="h-3 w-3" />,
  awaiting_approval: <ShieldCheck className="h-3 w-3" />,
  awaiting_auth: <KeyRound className="h-3 w-3" />,
  completed: <CheckCircle2 className="h-3 w-3" />,
  failed: <XCircle className="h-3 w-3" />,
  cancelled: <XCircle className="h-3 w-3" />,
};

const STATUS_LABELS: Record<string, string> = {
  queued: 'Queued',
  running: 'Running',
  awaiting_input: 'Awaiting input',
  awaiting_approval: 'Awaiting approval',
  awaiting_auth: 'Awaiting sign-in',
  completed: 'Completed',
  failed: 'Failed',
  cancelled: 'Cancelled',
};

const TARGET_LABELS: Record<string, string> = {
  epic: 'Epic',
  story: 'Story',
  support_conversation: 'Support',
  document: 'Document',
  crm_deal: 'Deal',
};

function formatTimestamp(value?: string) {
  if (!value) return 'Unknown time';
  try {
    return formatDistanceToNow(parseISO(value), { addSuffix: true });
  } catch {
    return value;
  }
}

function formatTarget(run: AgentRun) {
  const targetLabel = TARGET_LABELS[run.target_type] ?? run.target_type;
  return `${targetLabel} ${run.target_id.slice(0, 8)}`;
}

function statusVariant(status: AgentRun['status']): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (status) {
    case 'running':
      return 'default';
    case 'failed':
      return 'destructive';
    case 'completed':
      return 'outline';
    default:
      return 'secondary';
  }
}

function AgentRunRow({
  run,
  agent,
  onOpen,
  sessionPath,
}: {
  run: AgentRun;
  agent?: Agent | null;
  onOpen: (run: AgentRun) => void;
  sessionPath?: string;
}) {
  const agentName = agent?.name ?? 'Agent';
  const displayStatus = getAgentRunDisplayStatus(run);
  const statusLabel = STATUS_LABELS[displayStatus] ?? displayStatus;

  return (
    <div className="rounded-lg border border-border/60 bg-background/80">
      <button
        type="button"
        className="w-full px-4 py-3 text-left transition-colors hover:bg-accent/40"
        onClick={() => onOpen(run)}
      >
        <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div className="space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              {agent ? <AgentAvatar agent={agent} className="h-7 w-7" /> : null}
              <p className="text-sm font-medium">{agentName}</p>
              <Badge variant={statusVariant(run.status)} className="gap-1 px-1.5 py-0 text-[10px]">
                {STATUS_ICONS[displayStatus]}
                {statusLabel}
              </Badge>
              <Badge variant="outline" className="text-[10px]">
                {run.invocation_mode}
              </Badge>
            </div>
            <p className="text-xs text-muted-foreground">
              {formatTarget(run)} • {run.id.slice(0, 8)}
              {run.execution_stage ? ` • ${run.execution_stage}` : ''}
              {run.working_branch ? ` • ${run.working_branch}` : run.base_branch ? ` • ${run.base_branch}` : ''}
            </p>
          </div>

          <div className="text-xs text-muted-foreground md:text-right">
            <p>{formatTimestamp(run.created_at)}</p>
            <p>
              {run.tokens_used > 0 ? `${(run.tokens_used / 1000).toFixed(1)}k tokens` : 'No tokens yet'}
            </p>
          </div>
        </div>
      </button>
      {sessionPath ? (
        <div className="border-t border-border/60 px-4 py-2">
          <Button asChild variant="outline" size="sm">
            <a href={sessionPath}>Open session</a>
          </Button>
        </div>
      ) : null}
    </div>
  );
}

export function AgentRunsPage() {
  useTitle('Agent Runs');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id;
  const { data: access } = useWorkspaceAccess(workspaceId ?? '');
  const { canEdit } = usePermissions(access);

  const [runs, setRuns] = useState<AgentRun[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [totalRuns, setTotalRuns] = useState(0);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const loadData = useCallback(async (isRefresh = false) => {
    if (!workspaceId) return;
    if (isRefresh) {
      setRefreshing(true);
    } else {
      setLoading(true);
    }
    setError(null);

    try {
      const [runsRes, agentsRes] = await Promise.all([
        agentService.listWorkspaceRuns(workspaceId, 1, 100),
        agentService.list(workspaceId),
      ]);

      if (runsRes.error) {
        setError(runsRes.error);
      } else {
        setRuns(runsRes.data?.data ?? []);
        setTotalRuns(runsRes.data?.total ?? 0);
      }

      if (agentsRes.error) {
        setError((current) => current ?? agentsRes.error ?? 'Failed to load agents');
      } else {
        setAgents(agentsRes.data ?? []);
      }
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  useEffect(() => {
    const handler = () => {
      void loadData(true);
    };
    window.addEventListener('agent_run-created', handler);
    window.addEventListener('agent_run-updated', handler);
    return () => {
      window.removeEventListener('agent_run-created', handler);
      window.removeEventListener('agent_run-updated', handler);
    };
  }, [loadData]);

  const agentNameById = useMemo(
    () => Object.fromEntries(agents.map((agent) => [agent.id, agent.name])),
    [agents],
  );
  const agentById = useMemo(
    () => Object.fromEntries(agents.map((agent) => [agent.id, agent])),
    [agents],
  );

  const activeRuns = useMemo(
    () => runs.filter((run) => ACTIVE_RUN_STATUSES.has(run.status)),
    [runs],
  );
  const previousRuns = useMemo(
    () => runs.filter((run) => !ACTIVE_RUN_STATUSES.has(run.status)),
    [runs],
  );
  const selectedRun = useMemo(
    () => runs.find((run) => run.id === selectedRunId) ?? null,
    [runs, selectedRunId],
  );

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }
  const activeWorkspaceId = workspace.id;

  return (
    <div className="mx-auto max-w-5xl space-y-4">
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div className="space-y-1">
          <h1 className="text-xl font-semibold">Agent Runs</h1>
          <p className="text-sm text-muted-foreground">
            Current and previous runs across this workspace. Interactive runs open in the shared chat drawer.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant="outline">{activeRuns.length} active</Badge>
          <Badge variant="outline">{runs.filter((run) => run.invocation_mode === 'interactive').length} interactive</Badge>
          <Button variant="outline" size="sm" onClick={() => void loadData(true)} disabled={refreshing}>
            {refreshing ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="mr-1.5 h-3.5 w-3.5" />}
            Refresh
          </Button>
        </div>
      </div>

      {totalRuns > runs.length ? (
        <p className="text-xs text-muted-foreground">Showing the latest {runs.length} runs out of {totalRuns} total.</p>
      ) : null}

      {loading ? (
        <div className="flex items-center gap-2 rounded-lg border border-border/60 px-4 py-6 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading agent runs...
        </div>
      ) : error ? (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      ) : runs.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border/70 px-6 py-10 text-center">
          <Bot className="mx-auto mb-3 h-6 w-6 text-muted-foreground" />
          <p className="text-sm font-medium">No agent runs yet</p>
          <p className="mt-1 text-sm text-muted-foreground">
            Start an agent from a story, epic, or support conversation and it will appear here.
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          <section className="space-y-2">
            <div className="flex items-center gap-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Current</p>
              <Badge variant="outline" className="text-[10px]">{activeRuns.length}</Badge>
            </div>
            {activeRuns.length === 0 ? (
              <div className="rounded-lg border border-border/60 px-4 py-3 text-sm text-muted-foreground">
                No active runs right now.
              </div>
            ) : (
              <div className="space-y-2">
                {activeRuns.map((run) => (
                  <AgentRunRow
                    key={run.id}
                    run={run}
                    agent={agentById[run.agent_id] ?? null}
                    onOpen={(nextRun) => {
                      setSelectedRunId(nextRun.id);
                      setDrawerOpen(true);
                    }}
                    sessionPath={canOpenCodingSession(run) ? (buildCodingSessionPath(workspace.slug, run.id) ?? undefined) : undefined}
                  />
                ))}
              </div>
            )}
          </section>

          <section className="space-y-2">
            <div className="flex items-center gap-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">History</p>
              <Badge variant="outline" className="text-[10px]">{previousRuns.length}</Badge>
            </div>
            {previousRuns.length === 0 ? (
              <div className="rounded-lg border border-border/60 px-4 py-3 text-sm text-muted-foreground">
                No completed or failed runs yet.
              </div>
            ) : (
              <div className="space-y-2">
                {previousRuns.map((run) => (
                  <AgentRunRow
                    key={run.id}
                    run={run}
                    agent={agentById[run.agent_id] ?? null}
                    onOpen={(nextRun) => {
                      setSelectedRunId(nextRun.id);
                      setDrawerOpen(true);
                    }}
                    sessionPath={canOpenCodingSession(run) ? (buildCodingSessionPath(workspace.slug, run.id) ?? undefined) : undefined}
                  />
                ))}
              </div>
            )}
          </section>
        </div>
      )}

      <AgentRunDrawer
        workspaceId={activeWorkspaceId}
        runId={selectedRunId}
        open={drawerOpen && !!selectedRunId}
        onOpenChange={setDrawerOpen}
        canEdit={canEdit}
        title={selectedRun ? `${agentNameById[selectedRun.agent_id] ?? 'Agent'} Run` : 'Agent Run'}
        description={selectedRun ? `${formatTarget(selectedRun)} • ${formatTimestamp(selectedRun.created_at)}` : undefined}
      />
    </div>
  );
}
