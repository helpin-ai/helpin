import { useCallback, useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { BotIcon, GitBranchIcon, Loading01Icon, PlayIcon } from '@/lib/icons';
import { toast } from 'sonner';

import { AgentAvatar, resolveAgentPersonaKey, type AgentPersonaKey } from '@/components/agents/AgentAvatar';
import { UpgradeRequiredDialog } from '@/components/billing/UpgradeRequiredDialog';
import { NextAgentHint } from '@/components/agents/NextAgentHint';
import { CodingSessionDrawer } from '@/components/pm/CodingSession/CodingSessionDrawer';
import { TaskDeliveryTimeline } from '@/components/pm/TaskDeliveryTimeline';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/design-system/quiet-dropdown-select';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { isAgentAvailableForTarget } from '@/lib/agentAccess';
import { agentService } from '@/lib/services/agentService';
import { gitService } from '@/lib/services/gitService';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import type { Agent, AgentPresetKey, AgentRun, GitRepository, TaskDeliveryTarget } from '@/lib/pmTypes';
import { ACTIVE_RUN_STATUSES, getAgentRunDisplayStatus } from './agentRunConstants';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@/lib/upgradeRequired';
import { isAgentRunLifecycleEvent } from '@/lib/agentRunRealtime';
import { queryKeys } from '@/lib/queryKeys';

interface Props {
  taskId: string;
  workspaceId: string;
  taskTeamId?: string | null;
  latestRunAgentId?: string | null;
  delivery?: AgentRunDeliveryContext;
  showDevelopmentHistory?: boolean;
  onEditDeliveryContext?: () => void;
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

export function AgentRunPanel({
  taskId,
  workspaceId,
  taskTeamId,
  latestRunAgentId,
  delivery,
  showDevelopmentHistory = false,
  onEditDeliveryContext,
}: Props) {
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
  const gitLinksQuery = useQuery({
    queryKey: queryKeys.git.taskLinks(workspaceId, taskId),
    queryFn: async () => {
      const response = await gitService.getTaskGitLinks(workspaceId, taskId);
      if (response.error) throw new Error(response.error);
      return response.data ?? [];
    },
    enabled: showDevelopmentHistory,
  });

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
  const gitLinks = showDevelopmentHistory ? (gitLinksQuery.data ?? []) : [];
  const latestRunFailed = !activeRun && latestRun?.status === 'failed';
  const latestRunAgent = latestRun ? agents.find((agent) => agent.id === latestRun.agent_id) : null;

  const retryLatestRun = async () => {
    if (!latestRun) return;
    setTriggering(true);
    try {
      await startRun(latestRun.agent_id);
    } finally {
      setTriggering(false);
    }
  };

  if (
    taskRunnableAgents.length === 0
    && runs.length === 0
    && gitLinks.length === 0
    && !loading
    && !loadingAgents
    && !(showDevelopmentHistory && gitLinksQuery.isPending)
    && !gitLinksQuery.error
  ) return null;

  return (
    <div className="mt-6 space-y-7">
      {delivery ? (
        <AgentRunExecutionContext delivery={delivery} onEdit={onEditDeliveryContext} />
      ) : null}

      <section aria-label="Next delivery action">
        <h2 className="mb-3 text-xs font-semibold uppercase tracking-wide text-foreground/70">Next action</h2>
        {latestRunFailed && latestRun ? (
          <div className="flex flex-wrap items-center gap-3 rounded-md bg-destructive/5 px-4 py-3 ring-1 ring-inset ring-destructive/15">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 text-sm font-medium text-foreground">
                <BotIcon className="h-4 w-4 text-destructive" />
                Last run failed
              </div>
              <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">
                {latestRun.error_message || `${latestRunAgent?.name ?? 'The agent'} could not finish this task.`}
              </p>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <Button type="button" variant="outline" size="sm" onClick={() => setRunInUrl(latestRun.id)}>
                Review failure
              </Button>
              <Button type="button" size="sm" onClick={() => void retryLatestRun()} disabled={triggering}>
                {triggering ? <Loading01Icon className="animate-spin" /> : <PlayIcon />}
                Retry
              </Button>
            </div>
          </div>
        ) : activeRun ? (
          <div className="flex flex-wrap items-center gap-3 rounded-md bg-muted/35 px-4 py-3 ring-1 ring-inset ring-border/50">
            <AgentAvatar
              agent={agents.find((agent) => agent.id === activeRun.agent_id)}
              className="h-7 w-7 rounded-none border-0 bg-transparent shadow-none"
              genericBare
            />
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium text-foreground">{primaryAction.status}</p>
              <p className="mt-0.5 text-xs text-muted-foreground">Open the run to follow progress or respond.</p>
            </div>
            <Button type="button" size="sm" onClick={handleRunAgent}>{primaryAction.label}</Button>
          </div>
        ) : (
          <div className="flex flex-wrap items-center gap-3 rounded-md bg-muted/35 px-4 py-3 ring-1 ring-inset ring-border/50">
            <div className="flex min-w-0 flex-1 items-center gap-2">
              <span className="shrink-0 text-sm text-muted-foreground">{pickerLabel}</span>
              <Select
                value={selectedAgentId || '__none__'}
                onValueChange={(value) => setSelectedAgentId(value === '__none__' ? '' : value)}
                disabled={agentSelectionDisabled}
              >
                <SelectTrigger className="min-w-48 max-w-full">
                  <SelectValue placeholder={loadingAgents ? 'Loading agents...' : 'Select agent'} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__none__">No agent selected</SelectItem>
                  {taskRunnableAgents.map((agent) => (
                    <SelectItem key={agent.id} value={agent.id}>
                      <div className="flex items-center gap-1.5">
                        <AgentAvatar agent={agent} className="h-5 w-5 rounded-none border-0 bg-transparent shadow-none" genericBare />
                        <span>{agent.name}{agent.role ? ` · ${agent.role}` : ''}</span>
                      </div>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {actionDisabledReason ? <p className="min-w-0 text-xs text-muted-foreground">{actionDisabledReason}</p> : null}
            <Tooltip>
              <TooltipTrigger asChild>
                <span>
                  <Button
                    type="button"
                    onClick={handleRunAgent}
                    disabled={loadingAgents || !selectedAgentId || launchState.disabled}
                    title={primaryAction.status}
                  >
                    {triggering ? <Loading01Icon className="animate-spin" /> : <PlayIcon />}
                    Run agent
                  </Button>
                </span>
              </TooltipTrigger>
              {actionDisabledReason ? <TooltipContent side="top">{actionDisabledReason}</TooltipContent> : null}
            </Tooltip>
          </div>
        )}

        {latestCompletedAgent ? (
          <NextAgentHint
            completedAgent={latestCompletedAgent}
            candidates={taskRunnableAgents}
            completedPersonaKeys={completedPersonaKeys}
            onRun={(agent) => startRun(agent.id)}
          />
        ) : null}
      </section>

      <TaskDeliveryTimeline
        runs={runs}
        agents={agents}
        links={gitLinks}
        deliveryTarget={delivery?.target}
        loading={loading || (showDevelopmentHistory && gitLinksQuery.isPending)}
        error={showDevelopmentHistory && gitLinksQuery.error instanceof Error ? gitLinksQuery.error.message : null}
        onOpenRun={setRunInUrl}
      />

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
  onEdit,
}: {
  delivery: AgentRunDeliveryContext;
  onEdit?: () => void;
}) {
  const repositoryName = delivery.selectedRepository?.full_name ?? delivery.target?.repo_full_name ?? '';

  if (!delivery.loading && !repositoryName) return null;

  return (
    <section className="border-y border-border/60 py-4" aria-label="Execution context">
      <div className="mb-3 flex items-center justify-between gap-3">
        <h2 className="text-xs font-semibold uppercase tracking-wide text-foreground/70">Execution context</h2>
        {onEdit ? <Button type="button" variant="outline" size="sm" onClick={onEdit}>Edit context</Button> : null}
      </div>
      {delivery.loading ? (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loading01Icon className="h-4 w-4 animate-spin" />
          Loading execution context…
        </div>
      ) : (
        <div className="grid min-w-0 gap-4 sm:grid-cols-3">
          <ExecutionContextValue label="Repository" value={repositoryName} icon />
          <ExecutionContextValue label="Base branch" value={delivery.resolvedBaseBranch || 'Not set'} />
          <ExecutionContextValue label="Working branch" value={delivery.branchPreview || 'Not set'} />
        </div>
      )}
    </section>
  );
}

function ExecutionContextValue({ label, value, icon = false }: { label: string; value: string; icon?: boolean }) {
  return (
    <div className="min-w-0">
      <span className="block text-[11px] text-muted-foreground">{label}</span>
      <span className="mt-1 flex min-w-0 items-center gap-1.5 font-mono text-xs text-foreground">
        {icon ? <GitBranchIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" /> : null}
        <span className="truncate">{value}</span>
      </span>
    </div>
  );
}
