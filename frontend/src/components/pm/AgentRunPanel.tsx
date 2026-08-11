import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { BotIcon, GitBranchIcon, Loading01Icon, PlayIcon } from '@/lib/icons';
import { toast } from 'sonner';

import { AgentAvatar, resolveAgentPersonaKey, type AgentPersonaKey } from '@/components/agents/AgentAvatar';
import { UpgradeRequiredDialog } from '@/components/billing/UpgradeRequiredDialog';
import { NextAgentHint } from '@/components/agents/NextAgentHint';
import { CodingSessionDrawer } from '@/components/pm/CodingSession/CodingSessionDrawer';
import { AgentRunTable } from '@/components/pm/AgentRunTable';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { isAgentAvailableForTarget } from '@/lib/agentAccess';
import { agentService } from '@/lib/services/agentService';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import type { Agent, AgentPresetKey, AgentRun, GitRepository, TaskDeliveryTarget } from '@/lib/pmTypes';
import { ACTIVE_RUN_STATUSES, getAgentRunDisplayStatus } from './agentRunConstants';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@/lib/upgradeRequired';
import { isAgentRunLifecycleEvent } from '@/lib/agentRunRealtime';

interface Props {
  taskId: string;
  workspaceId: string;
  taskTeamId?: string | null;
  latestRunAgentId?: string | null;
  delivery?: AgentRunDeliveryContext;
}

export interface AgentRunDeliveryContext {
  target: TaskDeliveryTarget | null;
  loading: boolean;
  resolvedBaseBranch: string;
  branchPreview: string;
  selectedRepository: GitRepository | null;
}

type TaskAgentRunPrimaryActionKind = 'start' | 'open';

interface TaskAgentRunPrimaryAction {
  kind: TaskAgentRunPrimaryActionKind;
  label: string;
  status: string;
  runId: string | null;
}

interface TaskAgentRunLaunchState {
  label: string;
  disabled: boolean;
  disabledReason: string | null;
}

function displayAgentName(name: string | null | undefined) {
  const trimmed = name?.trim();
  return trimmed || 'agent';
}

const TASK_AGENT_PIPELINE: AgentPresetKey[] = ['task_planner', 'code_builder', 'review_agent'];

export function getTaskAgentRunPrimaryAction({
  selectedAgentName,
  activeRun,
  activeRunAgentName,
  triggering,
}: {
  selectedAgentName: string | null | undefined;
  activeRun: Pick<AgentRun, 'id' | 'status' | 'pause_reason' | 'approval_state'> | null | undefined;
  activeRunAgentName?: string | null;
  triggering: boolean;
}): TaskAgentRunPrimaryAction {
  const selectedName = displayAgentName(selectedAgentName);

  if (triggering) {
    return {
      kind: 'start',
      label: 'Starting...',
      status: `${selectedName} is starting a task run.`,
      runId: null,
    };
  }

  if (activeRun) {
    const activeName = displayAgentName(activeRunAgentName ?? selectedAgentName);
    const displayStatus = getAgentRunDisplayStatus(activeRun);
    if (displayStatus === 'awaiting_input') {
      return {
        kind: 'open',
        label: `Reply to ${activeName}`,
        status: `${activeName} is waiting for input.`,
        runId: activeRun.id,
      };
    }
    if (displayStatus === 'awaiting_approval') {
      return {
        kind: 'open',
        label: `Review ${activeName} request`,
        status: `${activeName} needs review before continuing.`,
        runId: activeRun.id,
      };
    }
    if (displayStatus === 'awaiting_auth') {
      return {
        kind: 'open',
        label: `Complete ${activeName} sign-in`,
        status: `${activeName} needs sign-in before continuing.`,
        runId: activeRun.id,
      };
    }
    const statusVerb = activeRun.status === 'queued' ? 'is queued' : 'is running';
    return {
      kind: 'open',
      label: `Open ${activeName} run`,
      status: `${activeName} ${statusVerb}.`,
      runId: activeRun.id,
    };
  }

  return {
    kind: 'start',
    label: 'Run',
    status: 'Choose an agent to run on this task.',
    runId: null,
  };
}

export function getTaskAgentRunLaunchState({
  activeRun,
  triggering,
}: {
  activeRun: Pick<AgentRun, 'id' | 'status' | 'pause_reason' | 'approval_state'> | null | undefined;
  triggering: boolean;
}): TaskAgentRunLaunchState {
  if (triggering) {
    return {
      label: 'Starting...',
      disabled: true,
      disabledReason: 'The current run is starting.',
    };
  }

  if (activeRun) {
    const displayStatus = getAgentRunDisplayStatus(activeRun);
    if (displayStatus === 'awaiting_input') {
      return {
        label: 'Run',
        disabled: true,
        disabledReason: 'The current run is waiting for your response.',
      };
    }
    if (displayStatus === 'awaiting_approval') {
      return {
        label: 'Run',
        disabled: true,
        disabledReason: 'The current run is waiting for approval.',
      };
    }
    if (displayStatus === 'awaiting_auth') {
      return {
        label: 'Run',
        disabled: true,
        disabledReason: 'The current run is waiting for sign-in.',
      };
    }
    if (activeRun.status === 'queued') {
      return {
        label: 'Run',
        disabled: true,
        disabledReason: 'The current run is queued.',
      };
    }
    return {
      label: 'Run',
      disabled: true,
      disabledReason: 'The current run is still in progress.',
    };
  }

  return {
    label: 'Run',
    disabled: false,
    disabledReason: null,
  };
}

export function getTaskAgentRunSuggestedAgent<TAgent extends Pick<Agent, 'id' | 'preset_key'>>({
  agents,
  runs,
  activeRun,
}: {
  agents: TAgent[];
  runs: Pick<AgentRun, 'agent_id' | 'status'>[];
  activeRun: Pick<AgentRun, 'agent_id'> | null | undefined;
}) {
  if (activeRun) {
    return agents.find((agent) => agent.id === activeRun.agent_id) ?? null;
  }

  const completedPresetKeys = new Set(
    runs
      .filter((run) => run.status === 'completed')
      .map((run) => agents.find((agent) => agent.id === run.agent_id)?.preset_key)
      .filter(Boolean),
  );
  const nextPresetKey = TASK_AGENT_PIPELINE.find((presetKey) => !completedPresetKeys.has(presetKey)) ?? 'review_agent';
  return agents.find((agent) => agent.preset_key === nextPresetKey)
    ?? agents.find((agent) => agent.preset_key === 'task_planner')
    ?? agents.find((agent) => agent.preset_key === 'code_builder')
    ?? agents.find((agent) => agent.preset_key === 'review_agent')
    ?? agents[0]
    ?? null;
}

export function getTaskAgentRunPickerLabel(_args: {
  activeRun: Pick<AgentRun, 'status' | 'pause_reason' | 'approval_state'> | null | undefined;
  suggestedAgent: Pick<Agent, 'preset_key'> | null | undefined;
}) {
  return 'Agent';
}

export function getTaskAgentRunSelectedAgentId<TAgent extends Pick<Agent, 'id'>>({
  selectedAgentId,
  agents,
  suggestedAgent,
  selectionLocked,
}: {
  selectedAgentId: string;
  agents: TAgent[];
  suggestedAgent: TAgent | null | undefined;
  selectionLocked: boolean;
}) {
  if (!suggestedAgent) return '';
  const selectedAgentExists = selectedAgentId && agents.some((agent) => agent.id === selectedAgentId);
  return !selectedAgentExists || selectionLocked ? suggestedAgent.id : selectedAgentId;
}

export function getTaskAgentRunExecutionContextLockReason({
  activeRun,
  activeRunAgentName,
}: {
  activeRun: Pick<AgentRun, 'status' | 'pause_reason' | 'approval_state'> | null | undefined;
  activeRunAgentName?: string | null;
}) {
  if (!activeRun) return null;
  const activeName = displayAgentName(activeRunAgentName);
  const displayStatus = getAgentRunDisplayStatus(activeRun);
  const stateText = displayStatus === 'awaiting_input'
    ? 'waiting for input'
    : displayStatus === 'awaiting_approval'
      ? 'awaiting approval'
      : displayStatus === 'awaiting_auth'
        ? 'waiting for sign-in'
        : activeRun.status === 'queued'
          ? 'queued'
          : 'running';
  return `${activeName} is ${stateText}. Repository and branch can be changed after this run finishes.`;
}

export function AgentRunPanel({ taskId, workspaceId, taskTeamId, latestRunAgentId, delivery }: Props) {
  const navigate = useNavigate();
  const search = useSearch({ strict: false }) as { run?: string };
  const urlRunId = search.run ?? null;
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { isAdmin } = usePermissions(access);
  const accessibleTeamIds = useMemo(
    () => new Set((access?.team_memberships ?? []).map((team) => team.team_id)),
    [access?.team_memberships],
  );

  const [agents, setAgents] = useState<Agent[]>([]);
  const [runs, setRuns] = useState<AgentRun[]>([]);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(urlRunId);
  const [selectedAgentId, setSelectedAgentId] = useState('');
  const [drawerOpen, setDrawerOpen] = useState<boolean>(Boolean(urlRunId));
  const [triggering, setTriggering] = useState(false);
  const [loadingAgents, setLoadingAgents] = useState(true);
  const [loading, setLoading] = useState(true);
  const [upgradeDialogReason, setUpgradeDialogReason] = useState<UpgradeRequiredReason | null>(null);

  const setRunInUrl = useCallback(
    (runId: string | null) => {
      navigate({
        to: '.',
        search: (prev) => {
          const next = { ...(prev as Record<string, unknown>) };
          delete next.task;
          return { ...next, run: runId ?? undefined };
        },
        replace: true,
      });
    },
    [navigate],
  );

  useEffect(() => {
    if (urlRunId) {
      setSelectedRunId(urlRunId);
      setDrawerOpen(true);
    } else {
      setDrawerOpen(false);
    }
  }, [urlRunId]);

  const fetchAgents = useCallback(async () => {
    setLoadingAgents(true);
    try {
      const res = await agentService.list(workspaceId);
      if (res.error) {
        toast.error(res.error);
        return;
      }
      setAgents(res.data ?? []);
    } finally {
      setLoadingAgents(false);
    }
  }, [workspaceId]);

  const fetchRuns = useCallback(async () => {
    try {
      const res = await agentService.listTargetRuns(workspaceId, 'task', taskId);
      setRuns(res.data ?? []);
      setSelectedRunId((current) => current && (res.data ?? []).some((run) => run.id === current) ? current : (res.data?.[0]?.id ?? null));
    } finally {
      setLoading(false);
    }
  }, [taskId, workspaceId]);

  useEffect(() => {
    void fetchAgents();
  }, [fetchAgents]);

  useEffect(() => {
    void fetchRuns();
  }, [fetchRuns]);

  const taskRunnableAgents = useMemo(
    () => agents.filter((agent) => isAgentAvailableForTarget(agent, {
      targetType: 'task',
      targetTeamId: taskTeamId,
      accessibleTeamIds,
      canSeeAllAgents: isAdmin,
    })),
    [accessibleTeamIds, agents, isAdmin, taskTeamId],
  );
  useEffect(() => {
    const handler = (event: Event) => {
      if (!isAgentRunLifecycleEvent(event)) return;
      const detail = (event as CustomEvent).detail as { parent_type?: string; parent_id?: string } | undefined;
      if (detail?.parent_type === 'task' && detail.parent_id === taskId) {
        void fetchRuns();
      }
    };
    window.addEventListener('agent_run-updated', handler);
    window.addEventListener('agent_run-created', handler);
    return () => {
      window.removeEventListener('agent_run-updated', handler);
      window.removeEventListener('agent_run-created', handler);
    };
  }, [fetchRuns, taskId]);

  const startRun = useCallback(async (agentId: string) => {
    const res = await agentService.runTask(workspaceId, taskId, { agent_id: agentId });
    if (res.error) {
      const reason = getUpgradeRequiredReason(res.error);
      if (reason) {
        setUpgradeDialogReason(reason);
        return;
      }
      toast.error(res.error);
      return;
    }
    await fetchRuns();
    if (res.data?.id) {
      setRunInUrl(res.data.id);
    }
  }, [fetchRuns, setRunInUrl, taskId, workspaceId]);

  const agentNameById = useMemo(
    () => Object.fromEntries(agents.map((agent) => [agent.id, agent.name])),
    [agents],
  );
  const activeRun = useMemo(
    () => runs.find((run) => ACTIVE_RUN_STATUSES.has(run.status)) ?? null,
    [runs],
  );
  const suggestedAgent = useMemo(
    () => {
      const suggested = getTaskAgentRunSuggestedAgent({ agents: taskRunnableAgents, runs, activeRun });
      if (suggested) return suggested;
      return latestRunAgentId
        ? taskRunnableAgents.find((agent) => agent.id === latestRunAgentId) ?? null
        : null;
    },
    [activeRun, latestRunAgentId, runs, taskRunnableAgents],
  );
  const latestRun = runs[0];
  const latestRunBlocksSelection = Boolean(latestRun && ACTIVE_RUN_STATUSES.has(latestRun.status));
  useEffect(() => {
    const nextAgentId = getTaskAgentRunSelectedAgentId({
      selectedAgentId,
      agents: taskRunnableAgents,
      suggestedAgent,
      selectionLocked: latestRunBlocksSelection,
    });
    if (nextAgentId !== selectedAgentId) setSelectedAgentId(nextAgentId);
  }, [latestRunBlocksSelection, selectedAgentId, suggestedAgent, taskRunnableAgents]);
  const selectedAgent = useMemo(
    () => agents.find((agent) => agent.id === selectedAgentId) ?? suggestedAgent,
    [agents, suggestedAgent, selectedAgentId],
  );
  const activeRunAgentName = activeRun ? agentNameById[activeRun.agent_id] ?? null : null;
  const primaryAction = getTaskAgentRunPrimaryAction({
    selectedAgentName: selectedAgent?.name ?? null,
    activeRun,
    activeRunAgentName,
    triggering,
  });
  const launchState = getTaskAgentRunLaunchState({ activeRun, triggering });
  const pickerLabel = getTaskAgentRunPickerLabel({ activeRun, suggestedAgent: selectedAgent });
  const agentSelectionDisabled = !!activeRun || triggering;
  const actionDisabledReason = primaryAction.kind === 'open'
    ? null
    : loadingAgents
      ? 'Loading agents...'
      : !selectedAgentId
        ? 'Choose an agent to run.'
        : launchState.disabledReason;

  const handleRunAgent = async () => {
    if (primaryAction.kind === 'open' && primaryAction.runId) {
      setRunInUrl(primaryAction.runId);
      return;
    }
    if (!selectedAgentId) return;
    setTriggering(true);
    try {
      await startRun(selectedAgentId);
    } finally {
      setTriggering(false);
    }
  };

  const latestCompletedAgent = useMemo(() => {
    if (!latestRun || latestRun.status !== 'completed') return null;
    return agents.find((agent) => agent.id === latestRun.agent_id) ?? null;
  }, [agents, latestRun]);
  const completedPersonaKeys = useMemo(() => {
    const agentById = new Map(agents.map((agent) => [agent.id, agent]));
    const keys = new Set<AgentPersonaKey>();
    for (const run of runs) {
      const agent = agentById.get(run.agent_id);
      if (agent) {
        keys.add(resolveAgentPersonaKey({ agent }));
      }
    }
    return keys;
  }, [agents, runs]);
  if (taskRunnableAgents.length === 0 && runs.length === 0 && !loading && !loadingAgents) return null;

  return (
    <div className="mt-6">
      <div className="overflow-hidden rounded-md border border-border/60 bg-card">
        <div className="flex items-center gap-2 px-3 py-2">
          <BotIcon className="h-3.5 w-3.5 text-muted-foreground" />
          <span className="text-xs font-semibold uppercase tracking-wide text-foreground/70">
            Agent Runs
          </span>
          {runs.length > 0 && (
            <span className="inline-flex h-5 min-w-5 items-center justify-center rounded border border-border/60 bg-muted px-1.5 text-[11px] font-medium text-muted-foreground">
              {runs.length}
            </span>
          )}
        </div>

        {delivery ? (
          <AgentRunExecutionContext delivery={delivery} />
        ) : null}

        <div className="flex items-center justify-between gap-2 border-t border-border/60 px-3 py-2">
          <div className="flex min-w-0 items-center gap-2">
            <span className="shrink-0 text-xs text-muted-foreground">{pickerLabel}</span>
            <Select
              value={selectedAgentId || '__none__'}
              onValueChange={(value) => setSelectedAgentId(value === '__none__' ? '' : value)}
              disabled={agentSelectionDisabled}
            >
              <SelectTrigger size="sm" className="h-7 w-auto min-w-0 gap-1.5 text-xs">
                <SelectValue placeholder={loadingAgents ? 'Loading agents...' : 'Select agent'} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__none__" className="text-xs">No agent selected</SelectItem>
                {taskRunnableAgents.map((agent) => (
                  <SelectItem key={agent.id} value={agent.id} className="text-xs">
                    <div className="flex items-center gap-1.5">
                      <AgentAvatar agent={agent} className="h-5 w-5 rounded-none border-0 bg-transparent shadow-none" genericBare />
                      <span>{agent.name}{agent.role ? ` · ${agent.role}` : ''}</span>
                    </div>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {actionDisabledReason ? (
            <p className="ml-auto min-w-0 truncate text-right text-[11px] text-muted-foreground">
              {actionDisabledReason}
            </p>
          ) : null}
          <Tooltip>
            <TooltipTrigger asChild>
              <span>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={handleRunAgent}
                  disabled={loadingAgents || (primaryAction.kind === 'start' && (!selectedAgentId || launchState.disabled))}
                  title={primaryAction.status}
                  className="h-7 gap-1 px-2.5 text-xs"
                >
                  {triggering ? <Loading01Icon className="h-3 w-3 animate-spin" /> : <PlayIcon className="h-3 w-3" />}
                  {primaryAction.label}
                </Button>
              </span>
            </TooltipTrigger>
            {actionDisabledReason ? (
              <TooltipContent side="top">{actionDisabledReason}</TooltipContent>
            ) : null}
          </Tooltip>
        </div>

        {latestCompletedAgent ? (
          <NextAgentHint
            completedAgent={latestCompletedAgent}
            candidates={taskRunnableAgents}
            completedPersonaKeys={completedPersonaKeys}
            onRun={(agent) => startRun(agent.id)}
          />
        ) : null}

        <div className="border-t border-border/60">
          <AgentRunTable
            runs={runs}
            agents={agents}
            selectedRunId={selectedRunId}
            onSelectRun={(run) => setRunInUrl(run.id)}
            loading={loading}
          />
        </div>
      </div>

      <CodingSessionDrawer
        sessionId={selectedRunId}
        open={drawerOpen && !!selectedRunId}
        onOpenChange={(open) => {
          if (!open) setRunInUrl(null);
        }}
        title="Task Agent Run"
      />
      <UpgradeRequiredDialog
        open={upgradeDialogReason !== null}
        onOpenChange={(open) => {
          if (!open) setUpgradeDialogReason(null);
        }}
        reason={upgradeDialogReason}
      />
    </div>
  );
}

function AgentRunExecutionContext({
  delivery,
}: {
  delivery: AgentRunDeliveryContext;
}) {
  const repositoryName = delivery.selectedRepository?.full_name ?? delivery.target?.repo_full_name ?? '';
  const contextText = repositoryName
    ? `${repositoryName} · ${delivery.resolvedBaseBranch} -> ${delivery.branchPreview}`
    : 'Repository not configured';

  return (
    <div className="flex items-center gap-2 border-t border-border/60 px-3 py-2 text-xs">
      <GitBranchIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
      <span className="shrink-0 text-muted-foreground">Runs on</span>
      {delivery.loading ? (
        <span className="min-w-0 text-muted-foreground">Loading execution context...</span>
      ) : (
        <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-foreground">
          {contextText}
        </span>
      )}
    </div>
  );
}
