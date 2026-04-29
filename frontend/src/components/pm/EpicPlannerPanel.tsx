import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { formatDistanceToNow, parseISO } from 'date-fns';
import { ArrowDown01Icon, ArrowRight01Icon, Loading01Icon, MessagePreview01Icon, PlayIcon } from '@/lib/icons';
import { toast } from 'sonner';

import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { CodingSessionDrawer } from '@/components/pm/CodingSession/CodingSessionDrawer';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
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
import { ACTIVE_RUN_STATUSES, STATUS_META, getAgentRunDisplayStatus } from './agentRunConstants';

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

export function shouldReloadEpicPlannerRuns(
  detail: { parent_type?: string; parent_id?: string; data?: { status?: string } } | undefined,
  epicId: string,
) {
  return detail?.parent_type === 'epic' && detail.parent_id === epicId;
}

export function shouldShowRunsLoading(loadingRuns: boolean, refreshingRuns: boolean, runs: AgentRun[]) {
  return loadingRuns && !refreshingRuns && runs.length === 0;
}

type EpicPlannerPrimaryActionKind = 'start' | 'open';

interface EpicPlannerPrimaryAction {
  kind: EpicPlannerPrimaryActionKind;
  label: string;
  status: string;
  runId: string | null;
  secondaryActionLabel: string | null;
}

function displayAgentName(name: string | null | undefined) {
  const trimmed = name?.trim();
  return trimmed || 'AI planner';
}

function agentRunStatusLabel(run: Pick<AgentRun, 'status' | 'pause_reason' | 'approval_state'>) {
  const displayStatus = getAgentRunDisplayStatus(run);
  return STATUS_META[displayStatus]?.label ?? displayStatus.replaceAll('_', ' ');
}

export function getEpicPlannerPrimaryAction({
  selectedAgentName,
  activeRun,
  activeRunAgentName,
  latestRun,
  latestRunAgentName,
  starting,
}: {
  selectedAgentName: string | null | undefined;
  activeRun: Pick<AgentRun, 'id' | 'status' | 'pause_reason' | 'approval_state'> | null | undefined;
  activeRunAgentName?: string | null;
  latestRun: Pick<AgentRun, 'id' | 'status' | 'pause_reason' | 'approval_state'> | null | undefined;
  latestRunAgentName: string | null | undefined;
  starting: boolean;
}): EpicPlannerPrimaryAction {
  const selectedName = displayAgentName(selectedAgentName);

  if (starting) {
    return {
      kind: 'start',
      label: `Starting ${selectedName}...`,
      status: `${selectedName} is starting a planning run.`,
      runId: null,
      secondaryActionLabel: null,
    };
  }

  if (activeRun) {
    const activeName = displayAgentName(activeRunAgentName ?? latestRunAgentName ?? selectedName);
    const displayStatus = getAgentRunDisplayStatus(activeRun);
    if (displayStatus === 'awaiting_input') {
      return {
        kind: 'open',
        label: `Reply to ${activeName}`,
        status: `${activeName} is waiting for input.`,
        runId: activeRun.id,
        secondaryActionLabel: null,
      };
    }
    if (displayStatus === 'awaiting_approval') {
      return {
        kind: 'open',
        label: `Review ${activeName} request`,
        status: `${activeName} needs review before continuing.`,
        runId: activeRun.id,
        secondaryActionLabel: null,
      };
    }
    if (displayStatus === 'awaiting_auth') {
      return {
        kind: 'open',
        label: `Complete ${activeName} sign-in`,
        status: `${activeName} needs sign-in before continuing.`,
        runId: activeRun.id,
        secondaryActionLabel: null,
      };
    }
    const statusVerb = activeRun.status === 'queued' ? 'is queued' : 'is running';
    return {
      kind: 'open',
      label: `Open ${activeName} run`,
      status: `${activeName} ${statusVerb}.`,
      runId: activeRun.id,
      secondaryActionLabel: null,
    };
  }

  if (latestRun) {
    const latestName = displayAgentName(latestRunAgentName ?? selectedName);
    if (latestRun.status === 'completed') {
      return {
        kind: 'open',
        label: `View ${latestName} run`,
        status: `${latestName} completed a planning run.`,
        runId: latestRun.id,
        secondaryActionLabel: `Run ${selectedName}`,
      };
    }
    if (latestRun.status === 'failed') {
      return {
        kind: 'open',
        label: `Open failed ${latestName} run`,
        status: `${latestName} could not complete the last planning run.`,
        runId: latestRun.id,
        secondaryActionLabel: `Run ${selectedName}`,
      };
    }
    if (latestRun.status === 'cancelled') {
      return {
        kind: 'open',
        label: `Open cancelled ${latestName} run`,
        status: `${latestName} was cancelled.`,
        runId: latestRun.id,
        secondaryActionLabel: `Run ${selectedName}`,
      };
    }
  }

  return {
    kind: 'start',
    label: `Run ${selectedName}`,
    status: 'Choose an AI planning agent to plan this epic.',
    runId: null,
    secondaryActionLabel: null,
  };
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
  const [refreshingRuns, setRefreshingRuns] = useState(false);
  const [starting, setStarting] = useState(false);
  const lastReportedCompletedRunIdRef = useRef<string | null>(null);
  const { teams: accessibleTeams } = useAccessibleTeams(workspaceId);
  const accessibleTeamIds = useMemo(
    () => new Set(accessibleTeams.map((team) => team.id)),
    [accessibleTeams],
  );

  const loadAgents = useCallback(async () => {
    const res = await agentService.list(workspaceId);
    if (res.error) {
      toast.error(res.error);
      return;
    }
    setAgents(res.data ?? []);
  }, [workspaceId]);

  const loadRuns = useCallback(async (options?: { silent?: boolean }) => {
    const silent = options?.silent === true;
    if (silent) {
      setRefreshingRuns(true);
    } else {
      setLoadingRuns(true);
    }
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
      if (silent) {
        setRefreshingRuns(false);
      } else {
        setLoadingRuns(false);
      }
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
      if (!shouldReloadEpicPlannerRuns(detail, epicId)) return;

      void loadRuns({ silent: true });
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
      void loadRuns({ silent: true });
    }, 5_000);
    return () => {
      window.clearInterval(intervalId);
    };
  }, [loadRuns, runs]);

  const plannerAgents = useMemo(() => {
    return agents.filter((agent) => {
      if (!agent.allowed_targets.includes('epic')) {
        return false;
      }
      if (!agent.team_id) {
        return true;
      }
      return accessibleTeamIds.has(agent.team_id);
    });
  }, [accessibleTeamIds, agents]);

  const selectedPlanner = useMemo(
    () => plannerAgents.find((agent) => agent.id === selectedAgentId) ?? null,
    [plannerAgents, selectedAgentId],
  );

  const preferredPlanner = useMemo(
    () => plannerAgents.find((agent) => agent.preset_key === 'epic_planner' && agent.is_system)
      ?? plannerAgents.find((agent) => agent.preset_key === 'epic_planner')
      ?? plannerAgents[0]
      ?? null,
    [plannerAgents],
  );

  useEffect(() => {
    if (!selectedAgentId && preferredPlanner) {
      setSelectedAgentId(preferredPlanner.id);
    }
  }, [preferredPlanner, selectedAgentId]);

  useEffect(() => {
    if (!selectedAgentId) {
      return;
    }
    if (plannerAgents.some((agent) => agent.id === selectedAgentId)) {
      return;
    }
    setSelectedAgentId(preferredPlanner?.id ?? '');
  }, [plannerAgents, preferredPlanner, selectedAgentId]);

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
      await loadRuns({ silent: true });
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

  const latestRun = runs[0] ?? null;
  const activeRun = useMemo(
    () => runs.find((run) => ACTIVE_RUN_STATUSES.has(run.status)) ?? null,
    [runs],
  );
  const activeRunAgentName = activeRun ? agentNameById[activeRun.agent_id] ?? null : null;
  const latestRunAgentName = latestRun ? agentNameById[latestRun.agent_id] ?? null : null;
  const selectedPlannerName = selectedPlanner?.name ?? preferredPlanner?.name ?? null;
  const primaryAction = getEpicPlannerPrimaryAction({
    selectedAgentName: selectedPlannerName,
    activeRun,
    activeRunAgentName,
    latestRun,
    latestRunAgentName,
    starting,
  });
  const showVisibleRunsLoading = shouldShowRunsLoading(loadingRuns, refreshingRuns, runs);
  const plannerSelectionDisabled = !!activeRun || starting;

  const openRun = (runId: string) => {
    setSelectedRunId(runId);
    setDrawerOpen(true);
  };

  const handlePrimaryAction = () => {
    if (primaryAction.kind === 'open' && primaryAction.runId) {
      openRun(primaryAction.runId);
      return;
    }
    void handleStart();
  };

  return (
    <div className="space-y-4 rounded-lg border border-border/60 px-4 py-3">
      {canEdit ? (
        <div className="space-y-2.5">
          <p className="text-xs text-muted-foreground">
            {primaryAction.status}
          </p>

          <div className="grid gap-2 md:grid-cols-[minmax(0,1fr)_auto] md:items-end">
            <div className="space-y-1">
              <Select
                value={selectedAgentId || '__none__'}
                onValueChange={(value) => setSelectedAgentId(value === '__none__' ? '' : value)}
                disabled={plannerSelectionDisabled}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select an AI planning agent..." />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__none__">No agent selected</SelectItem>
                  {plannerAgents.map((agent) => (
                    <SelectItem key={agent.id} value={agent.id}>
                      <div className="flex items-center gap-2">
                        <AgentAvatar agent={agent} className="h-5 w-5" />
                        <span>{agent.name}{agent.role ? ` · ${agent.role}` : ''}</span>
                      </div>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {selectedPlanner && !selectedPlanner.system_prompt ? (
                <p className="text-[11px] text-muted-foreground">
                  This agent is missing system instructions.
                </p>
              ) : null}
            </div>
            <div className="flex flex-col gap-2 md:items-end">
              <Button
                onClick={handlePrimaryAction}
                disabled={primaryAction.kind === 'start' && (!selectedAgentId || starting)}
                className="w-full gap-1.5 md:w-auto md:min-w-44"
              >
                {starting ? (
                  <Loading01Icon className="h-4 w-4 animate-spin" />
                ) : primaryAction.kind === 'open' ? (
                  <MessagePreview01Icon className="h-4 w-4" />
                ) : (
                  <PlayIcon className="h-4 w-4" />
                )}
                {primaryAction.label}
              </Button>
              {primaryAction.secondaryActionLabel ? (
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => void handleStart()}
                  disabled={!selectedAgentId || starting}
                  className="w-full gap-1.5 md:w-auto md:min-w-44"
                >
                  {starting ? <Loading01Icon className="h-4 w-4 animate-spin" /> : <PlayIcon className="h-4 w-4" />}
                  {primaryAction.secondaryActionLabel}
                </Button>
              ) : null}
            </div>
          </div>

          {!activeRun ? (
            <div className="space-y-2 pt-2">
              <button
                type="button"
                className="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground transition-colors hover:text-foreground"
                onClick={() => setAdditionalContextOpen((open) => !open)}
              >
                {additionalContextOpen ? (
                  <ArrowDown01Icon className="h-3.5 w-3.5" />
                ) : (
                  <ArrowRight01Icon className="h-3.5 w-3.5" />
                )}
                {additionalContext.trim() ? 'Edit special instructions' : 'Add special instructions'}
              </button>

              {additionalContextOpen ? (
                <div className="space-y-1.5">
                  <Textarea
                    value={additionalContext}
                    onChange={(event) => setAdditionalContext(event.target.value)}
                    placeholder="Add anything not already captured in the epic description, such as constraints, priorities, or decisions the agent should account for."
                    rows={3}
                  />
                  <p className="text-[11px] text-muted-foreground">
                    Sent with the first message to help the agent scope the run.
                  </p>
                </div>
              ) : null}
            </div>
          ) : null}
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">You do not have permission to start or reply to epic planner runs.</p>
      )}

      <Separator className="my-1" />

      <div className="space-y-2.5 pt-5">
        <div className="flex items-center justify-between gap-2">
          <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Agent runs</p>
          {showVisibleRunsLoading ? (
            <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
              <Loading01Icon className="h-3 w-3 animate-spin" />
              Loading
            </span>
          ) : null}
        </div>

        {runs.length === 0 && !loadingRuns ? (
          <div className="pb-14 pt-1">
            <p className="text-sm text-muted-foreground">No runs yet.</p>
          </div>
        ) : (
          <div className="space-y-2">
            {runs.map((run) => {
              const agentName = agentNameById[run.agent_id] ?? 'Agent';
              const agent = agents.find((candidate) => candidate.id === run.agent_id) ?? null;
              return (
                <button
                  key={run.id}
                  type="button"
                  className="w-full rounded-lg border border-border/60 bg-input/50 px-3 py-2 text-left transition-colors hover:bg-accent"
                  onClick={() => {
                    setSelectedRunId(run.id);
                    setDrawerOpen(true);
                  }}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        {agent ? <AgentAvatar agent={agent} className="h-6 w-6" /> : null}
                        <p className="truncate text-sm font-medium">{agentName}</p>
                        {run.invocation_mode === 'interactive' ? (
                          <Badge variant="secondary" className="gap-1">
                            <MessagePreview01Icon className="h-3 w-3" />
                            Interactive
                          </Badge>
                        ) : null}
                      </div>
                      <p className="mt-1 text-xs text-muted-foreground">
                        {agentRunStatusLabel(run)} · {formatDistanceToNow(parseISO(run.created_at), { addSuffix: true })}
                      </p>
                    </div>
                    <Badge variant="outline">{agentRunStatusLabel(run)}</Badge>
                  </div>
                </button>
              );
            })}
          </div>
        )}
      </div>

      <CodingSessionDrawer
        sessionId={selectedRunId}
        open={drawerOpen && !!selectedRunId}
        onOpenChange={setDrawerOpen}
        title={selectedRunId ? `${agentNameById[runs.find((run) => run.id === selectedRunId)?.agent_id ?? ''] ?? 'Epic Agent'} Run` : 'Epic Agent Run'}
        description="Interactive agent chat, artifacts, and live tool activity for this epic."
      />
    </div>
  );
}
