import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { formatDistanceToNow, parseISO } from 'date-fns';
import { Bot, ChevronDown, ChevronRight, Loader2, MessageSquareMore, Play } from 'lucide-react';
import { toast } from 'sonner';

import { AgentRunDrawer } from '@/components/pm/AgentRunDrawer';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { Textarea } from '@/components/ui/textarea';
import type { Agent, AgentRun } from '@/lib/pmTypes';
import { agentService } from '@/lib/services/agentService';

interface EpicPlannerPanelProps {
  workspaceId: string;
  epicId: string;
  lastRunId?: string;
  canEdit: boolean;
  onRunCompleted?: () => void;
}

export function nextCompletedRunNotificationId(
  latestRun: Pick<AgentRun, 'id' | 'status'> | null | undefined,
  lastReportedCompletedRunId: string | null,
) {
  if (!latestRun || latestRun.status !== 'completed') return null;
  if (lastReportedCompletedRunId === latestRun.id) return null;
  return latestRun.id;
}

export function EpicPlannerPanel({
  workspaceId,
  epicId,
  lastRunId,
  canEdit,
  onRunCompleted,
}: EpicPlannerPanelProps) {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [runs, setRuns] = useState<AgentRun[]>([]);
  const [selectedAgentId, setSelectedAgentId] = useState('');
  const [selectedRunId, setSelectedRunId] = useState<string | null>(lastRunId ?? null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [additionalContext, setAdditionalContext] = useState('');
  const [additionalContextOpen, setAdditionalContextOpen] = useState(false);
  const [loadingRuns, setLoadingRuns] = useState(true);
  const [starting, setStarting] = useState(false);
  const lastReportedCompletedRunIdRef = useRef<string | null>(null);

  const loadAgents = useCallback(async () => {
    const res = await agentService.list(workspaceId);
    if (res.error) {
      toast.error(res.error);
      return;
    }
    setAgents(res.data ?? []);
  }, [workspaceId]);

  const loadRuns = useCallback(async () => {
    setLoadingRuns(true);
    try {
      const res = await agentService.listTargetRuns(workspaceId, 'epic', epicId);
      const nextRuns = res.data ?? [];
      setRuns(nextRuns);
      setSelectedRunId((current) => {
        if (current && nextRuns.some((run) => run.id === current)) return current;
        if (lastRunId && nextRuns.some((run) => run.id === lastRunId)) return lastRunId;
        return nextRuns[0]?.id ?? null;
      });
    } finally {
      setLoadingRuns(false);
    }
  }, [epicId, lastRunId, workspaceId]);

  useEffect(() => {
    void loadAgents();
  }, [loadAgents]);

  useEffect(() => {
    void loadRuns();
  }, [loadRuns]);

  useEffect(() => {
    const handler = (event: Event) => {
      const detail = (event as CustomEvent).detail as {
        parent_type?: string;
        parent_id?: string;
        data?: { status?: string };
      } | undefined;
      if (detail?.parent_type !== 'epic' || detail.parent_id !== epicId) return;

      const eventStatus = detail.data?.status;
      // While the run is actively executing, no need to reload the run list —
      // the list only changes on status transitions (completed, failed, awaiting_*).
      if (eventStatus === 'running' || eventStatus === 'queued') return;

      void loadRuns();
    };
    window.addEventListener('agent_run-created', handler);
    window.addEventListener('agent_run-updated', handler);
    return () => {
      window.removeEventListener('agent_run-created', handler);
      window.removeEventListener('agent_run-updated', handler);
    };
  }, [epicId, loadRuns]);

  useEffect(() => {
    if (!runs.some((run) => run.status === 'queued' || run.status === 'running')) {
      return;
    }
    const intervalId = window.setInterval(() => {
      void loadRuns();
    }, 5_000);
    return () => {
      window.clearInterval(intervalId);
    };
  }, [loadRuns, runs]);

  const plannerAgents = useMemo(() => {
    let filtered = agents.filter((agent) =>
      agent.allowed_targets?.includes('epic') ||
      agent.preset_key === 'epic_planner',
    );
    const preferredId = selectedAgentId;
    if (preferredId && !filtered.some((agent) => agent.id === preferredId)) {
      const preferredAgent = agents.find((agent) => agent.id === preferredId);
      if (preferredAgent) {
        filtered = [preferredAgent, ...filtered];
      }
    }
    return filtered;
  }, [agents, selectedAgentId]);

  const selectedPlanner = useMemo(
    () => plannerAgents.find((agent) => agent.id === selectedAgentId) ?? agents.find((agent) => agent.id === selectedAgentId) ?? null,
    [agents, plannerAgents, selectedAgentId],
  );

  const preferredPlanner = useMemo(
    () => plannerAgents.find((agent) => agent.is_system) ?? plannerAgents[0] ?? null,
    [plannerAgents],
  );

  useEffect(() => {
    if (!selectedAgentId && preferredPlanner) {
      setSelectedAgentId(preferredPlanner.id);
    }
  }, [preferredPlanner, selectedAgentId]);

  useEffect(() => {
    const latestRun = runs[0];
    if (!latestRun) {
      lastReportedCompletedRunIdRef.current = null;
      return;
    }
    if (latestRun.status !== 'completed') {
      if (lastReportedCompletedRunIdRef.current === latestRun.id) {
        lastReportedCompletedRunIdRef.current = null;
      }
      return;
    }
    const nextNotificationId = nextCompletedRunNotificationId(latestRun, lastReportedCompletedRunIdRef.current);
    if (!nextNotificationId) return;
    lastReportedCompletedRunIdRef.current = nextNotificationId;
    onRunCompleted?.();
  }, [onRunCompleted, runs]);

  const handleStart = async () => {
    if (!selectedAgentId) return;
    setStarting(true);
    try {
      const res = await agentService.runEpic(workspaceId, epicId, {
        agent_id: selectedAgentId,
        additional_context: additionalContext.trim() || undefined,
      });
      if (res.error) {
        toast.error(res.error);
        return;
      }
      await loadRuns();
      if (res.data?.id) {
        setSelectedRunId(res.data.id);
        setDrawerOpen(true);
      }
      setAdditionalContext('');
      setAdditionalContextOpen(false);
    } finally {
      setStarting(false);
    }
  };

  const agentNameById = useMemo(
    () => Object.fromEntries(agents.map((agent) => [agent.id, agent.name])),
    [agents],
  );

  return (
    <div className="space-y-3 rounded-lg border border-border/60 p-3">
      <div className="flex items-center gap-2">
        <Bot className="h-4 w-4 text-muted-foreground" />
        <span className="text-sm font-medium">Epic Agent Runs</span>
      </div>

      {canEdit ? (
        <div className="space-y-3">
          <div className="grid gap-2 md:grid-cols-[minmax(0,1fr)_auto] md:items-end">
            <div className="space-y-1.5">
              <Label className="text-xs">Planner</Label>
              <Select value={selectedAgentId || '__none__'} onValueChange={(value) => setSelectedAgentId(value === '__none__' ? '' : value)}>
                <SelectTrigger>
                  <SelectValue placeholder="Select a product planner..." />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__none__">No planner selected</SelectItem>
                  {plannerAgents.map((agent) => (
                    <SelectItem key={agent.id} value={agent.id}>
                      {agent.is_system ? `${agent.name} (System)` : agent.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {selectedPlanner && !selectedPlanner.system_prompt ? (
                <p className="text-[11px] text-muted-foreground">
                  This planner is missing system instructions on the agent.
                </p>
              ) : null}
            </div>
            <Button onClick={handleStart} disabled={!selectedAgentId || starting} className="w-full gap-1.5 md:w-auto md:min-w-44">
              {starting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
              Start Planner Run
            </Button>
          </div>

          <div className="space-y-1.5">
            <button
              type="button"
              className="flex w-full items-center justify-between rounded-md border border-border/60 px-3 py-2 text-left transition-colors hover:bg-accent/30"
              onClick={() => setAdditionalContextOpen((open) => !open)}
            >
              <div>
                <p className="text-xs font-medium">Additional Context (Optional)</p>
                <p className="text-[11px] text-muted-foreground">
                  {additionalContextOpen
                    ? 'Add constraints, priorities, or business context for this run.'
                    : additionalContext.trim()
                      ? 'Context attached for this run. Expand to edit.'
                      : 'Collapsed by default. Expand to add guidance for the planner.'}
                </p>
              </div>
              {additionalContextOpen ? (
                <ChevronDown className="h-4 w-4 text-muted-foreground" />
              ) : (
                <ChevronRight className="h-4 w-4 text-muted-foreground" />
              )}
            </button>

            {additionalContextOpen ? (
              <div className="space-y-1.5 rounded-md border border-border/60 p-2.5">
                <Textarea
                  value={additionalContext}
                  onChange={(event) => setAdditionalContext(event.target.value)}
                  placeholder="Add constraints, priorities, or business context before starting this run."
                  rows={3}
                />
                <p className="text-[11px] text-muted-foreground">
                  This context is sent with the first message so the planner can scope work without extra back-and-forth.
                </p>
              </div>
            ) : null}
          </div>
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">You do not have permission to start or reply to epic planner runs.</p>
      )}

      <Separator />

      <div className="space-y-2">
        <div className="flex items-center justify-between gap-2">
          <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Runs On This Epic</p>
          {loadingRuns ? (
            <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
              <Loader2 className="h-3 w-3 animate-spin" />
              Loading
            </span>
          ) : null}
        </div>

        {runs.length === 0 && !loadingRuns ? (
          <p className="text-sm text-muted-foreground">No agent runs yet.</p>
        ) : (
          <div className="space-y-2">
            {runs.map((run) => {
              const agentName = agentNameById[run.agent_id] ?? 'Agent';
              return (
                <button
                  key={run.id}
                  type="button"
                  className="w-full rounded-lg border border-border/60 bg-background/70 px-3 py-2 text-left transition-colors hover:bg-accent/40"
                  onClick={() => {
                    setSelectedRunId(run.id);
                    setDrawerOpen(true);
                  }}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <p className="truncate text-sm font-medium">{agentName}</p>
                        {run.invocation_mode === 'interactive' ? (
                          <Badge variant="secondary" className="gap-1">
                            <MessageSquareMore className="h-3 w-3" />
                            Interactive
                          </Badge>
                        ) : null}
                      </div>
                      <p className="mt-1 text-xs text-muted-foreground">
                        {run.status} · {formatDistanceToNow(parseISO(run.created_at), { addSuffix: true })}
                      </p>
                    </div>
                    <Badge variant="outline">{run.status}</Badge>
                  </div>
                </button>
              );
            })}
          </div>
        )}
      </div>

      <AgentRunDrawer
        workspaceId={workspaceId}
        runId={selectedRunId}
        open={drawerOpen && !!selectedRunId}
        onOpenChange={setDrawerOpen}
        canEdit={canEdit}
        title={selectedRunId ? `${agentNameById[runs.find((run) => run.id === selectedRunId)?.agent_id ?? ''] ?? 'Epic Agent'} Run` : 'Epic Agent Run'}
        description="Interactive agent chat, artifacts, and live tool activity for this epic."
      />
    </div>
  );
}
