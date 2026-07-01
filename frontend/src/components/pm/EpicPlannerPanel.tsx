import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { formatDistanceToNow, parseISO } from 'date-fns';
import { ArrowDown01Icon, ArrowRight01Icon, BotIcon, Loading01Icon, MessagePreview01Icon, PlayIcon } from '@/lib/icons';
import { toast } from 'sonner';

import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { UpgradeRequiredDialog } from '@/components/billing/UpgradeRequiredDialog';
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
import { Textarea } from '@/components/ui/textarea';
import type { Agent, AgentRun } from '@/lib/pmTypes';
import { isAgentAvailableForTarget } from '@/lib/agentAccess';
import { agentService } from '@/lib/services/agentService';
import { cn } from '@/lib/utils';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@/lib/upgradeRequired';
import { ACTIVE_RUN_STATUSES, STATUS_META, getAgentRunDisplayStatus } from './agentRunConstants';
import {
  HISTORY_VISIBLE_ROW_LIMIT,
  groupHistoryRuns,
  historyGroupTimeLabel,
  isCommandBarRun,
  isDecayedRun,
  compactRelativeAge,
  runResultSummary,
  type HistoryRunGroup,
} from './epicPlannerRunHistory';

interface EpicPlannerPanelProps {
  workspaceId: string;
  epicId: string;
  epicTeamId?: string | null;
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

function displayAgentName(name: string | null | undefined) {
  const trimmed = name?.trim();
  return trimmed || 'AI planner';
}

function agentRunStatusLabel(run: Pick<AgentRun, 'status' | 'pause_reason' | 'approval_state'>) {
  const displayStatus = getAgentRunDisplayStatus(run);
  return STATUS_META[displayStatus]?.label ?? displayStatus.replaceAll('_', ' ');
}

export interface EpicPlannerFeaturedAction {
  label: string;
  runId: string;
  emphasis: 'prominent' | 'quiet';
}

export function getEpicPlannerFeaturedAction(
  run: Pick<AgentRun, 'id' | 'status' | 'pause_reason' | 'approval_state'>,
  agentName: string | null | undefined,
): EpicPlannerFeaturedAction {
  const name = displayAgentName(agentName);
  const displayStatus = getAgentRunDisplayStatus(run);
  if (displayStatus === 'awaiting_input') {
    return { label: `Reply to ${name}`, runId: run.id, emphasis: 'prominent' };
  }
  if (displayStatus === 'awaiting_approval') {
    return { label: `Review ${name} request`, runId: run.id, emphasis: 'prominent' };
  }
  if (displayStatus === 'awaiting_auth') {
    return { label: `Complete ${name} sign-in`, runId: run.id, emphasis: 'prominent' };
  }
  if (displayStatus === 'queued' || displayStatus === 'running') {
    return { label: 'Open run', runId: run.id, emphasis: 'prominent' };
  }
  return { label: 'View run', runId: run.id, emphasis: 'quiet' };
}

function featuredRunTimeLabel(run: AgentRun) {
  try {
    return formatDistanceToNow(parseISO(run.created_at), { addSuffix: true });
  } catch {
    return '';
  }
}

function FeaturedRunCard({
  run,
  agent,
  agentName,
  action,
  onOpen,
}: {
  run: AgentRun;
  agent: Agent | null;
  agentName: string;
  action: EpicPlannerFeaturedAction;
  onOpen: (runId: string) => void;
}) {
  const displayStatus = getAgentRunDisplayStatus(run);
  const statusMeta = STATUS_META[displayStatus];
  const summary = runResultSummary(run);
  return (
    <div
      role="button"
      tabIndex={0}
      className="cursor-pointer rounded-md border border-border/60 bg-muted/30 px-3 py-2.5 transition-colors hover:bg-muted/60"
      onClick={() => onOpen(run.id)}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onOpen(run.id);
        }
      }}
    >
      <div className="flex items-start gap-2.5">
        {agent ? (
          <AgentAvatar
            agent={agent}
            className="h-8 w-8 rounded-none border-0 bg-transparent shadow-none"
            genericBare
          />
        ) : null}
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="truncate text-sm font-medium">{agentName}</span>
            <Badge variant={statusMeta?.variant ?? 'outline'} className={statusMeta?.className}>
              {agentRunStatusLabel(run)}
            </Badge>
            {run.invocation_mode === 'interactive' ? (
              <Badge variant="secondary" className="gap-1">
                <MessagePreview01Icon className="h-3 w-3" />
                Interactive
              </Badge>
            ) : null}
            <span className="ml-auto shrink-0 text-[11px] text-muted-foreground">
              {featuredRunTimeLabel(run)}
            </span>
          </div>
          {summary ? (
            <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">{summary}</p>
          ) : null}
          <Button
            size="sm"
            variant={action.emphasis === 'prominent' ? 'default' : 'outline'}
            className="mt-2 gap-1.5"
            onClick={(event) => {
              event.stopPropagation();
              onOpen(action.runId);
            }}
          >
            <MessagePreview01Icon className="h-3.5 w-3.5" />
            {action.label}
          </Button>
        </div>
      </div>
    </div>
  );
}

function HistoryRunRow({
  run,
  agent,
  agentName,
  onClick,
}: {
  run: AgentRun;
  agent: Agent | null;
  agentName: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'flex h-7 w-full items-center gap-2 rounded-md px-2 text-left transition-colors hover:bg-accent',
        isDecayedRun(run) && 'opacity-60',
      )}
    >
      {agent ? (
        <AgentAvatar
          agent={agent}
          className="h-4 w-4 shrink-0 rounded-none border-0 bg-transparent shadow-none"
          genericBare
        />
      ) : null}
      <span className="truncate text-xs font-medium">{agentName}</span>
      <span
        className={cn(
          'shrink-0 text-[11px] text-muted-foreground',
          run.status === 'failed' && 'text-destructive',
        )}
      >
        {agentRunStatusLabel(run)}
      </span>
      <span className="ml-auto shrink-0 text-[11px] text-muted-foreground/70">
        {compactRelativeAge(run.created_at)} ago
      </span>
    </button>
  );
}

function CollapsedGroupRow({
  group,
  agent,
  agentName,
  onExpand,
}: {
  group: HistoryRunGroup;
  agent: Agent | null;
  agentName: string;
  onExpand: () => void;
}) {
  const allDecayed = group.runs.every((run) => isDecayedRun(run));
  return (
    <button
      type="button"
      onClick={onExpand}
      className={cn(
        'flex h-7 w-full items-center gap-2 rounded-md px-2 text-left transition-colors hover:bg-accent',
        allDecayed && 'opacity-60',
      )}
    >
      <ArrowRight01Icon className="h-3 w-3 shrink-0 text-muted-foreground" />
      {agent ? (
        <AgentAvatar
          agent={agent}
          className="h-4 w-4 shrink-0 rounded-none border-0 bg-transparent shadow-none"
          genericBare
        />
      ) : null}
      <span className="truncate text-xs font-medium">{agentName}</span>
      <span className="shrink-0 text-[11px] text-muted-foreground">{group.runs.length} runs</span>
      <span className="ml-auto shrink-0 text-[11px] text-muted-foreground/70">
        {historyGroupTimeLabel(group)}
      </span>
    </button>
  );
}

export function EpicPlannerPanel({
  workspaceId,
  epicId,
  epicTeamId,
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
  const [showAllHistory, setShowAllHistory] = useState(false);
  const [expandedGroupIds, setExpandedGroupIds] = useState<Set<string>>(new Set());
  const [upgradeDialogReason, setUpgradeDialogReason] = useState<UpgradeRequiredReason | null>(null);
  const lastReportedCompletedRunIdRef = useRef<string | null>(null);
  const { teams: accessibleTeams, isAdmin } = useAccessibleTeams(workspaceId);
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
      // Command-bar / DAG orchestration runs target this epic but are not
      // planner runs (no transcript of their own); they live in the Ask-agents
      // dock, so keep them out of the planner list entirely.
      const nextRuns = (res.data ?? []).filter((run) => !isCommandBarRun(run));
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
    return agents.filter((agent) => isAgentAvailableForTarget(agent, {
      targetType: 'epic',
      targetTeamId: epicTeamId,
      accessibleTeamIds,
      canSeeAllAgents: isAdmin,
    }));
  }, [accessibleTeamIds, agents, epicTeamId, isAdmin]);

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
        const reason = getUpgradeRequiredReason(res.error);
        if (reason) {
          setUpgradeDialogReason(reason);
          return;
        }
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
  const agentById = useMemo(
    () => Object.fromEntries(agents.map((agent) => [agent.id, agent])),
    [agents],
  );

  const latestRun = runs[0] ?? null;
  const activeRun = useMemo(
    () => runs.find((run) => ACTIVE_RUN_STATUSES.has(run.status)) ?? null,
    [runs],
  );
  const featuredRun = activeRun ?? latestRun;
  const featuredAgentName = featuredRun ? agentNameById[featuredRun.agent_id] ?? 'Agent' : null;
  const historyRuns = useMemo(
    () => (featuredRun ? runs.filter((run) => run.id !== featuredRun.id) : runs),
    [featuredRun, runs],
  );
  const historyGroups = useMemo(() => groupHistoryRuns(historyRuns), [historyRuns]);
  const visibleGroups = showAllHistory
    ? historyGroups
    : historyGroups.slice(0, HISTORY_VISIBLE_ROW_LIMIT);
  const showVisibleRunsLoading = shouldShowRunsLoading(loadingRuns, refreshingRuns, runs);
  const plannerSelectionDisabled = !!activeRun || starting;

  const openRun = (runId: string) => {
    setSelectedRunId(runId);
    setDrawerOpen(true);
  };

  const expandGroup = (groupId: string) => {
    setExpandedGroupIds((current) => {
      const next = new Set(current);
      next.add(groupId);
      return next;
    });
  };

  const launcherDisabledReason = activeRun
    ? 'A run is in progress.'
    : !selectedAgentId
      ? 'Choose an agent to run.'
      : null;

  return (
    <div className="overflow-hidden rounded-md border border-border/60 bg-card">
      <div className="flex items-center gap-2 px-3 py-2">
        <BotIcon className="h-3.5 w-3.5 text-muted-foreground" />
        <span className="text-xs font-semibold uppercase tracking-wide text-foreground/70">
          Agent runs
        </span>
        {runs.length > 0 ? (
          <span className="inline-flex h-5 min-w-5 items-center justify-center rounded border border-border/60 bg-muted px-1.5 text-[11px] font-medium text-muted-foreground">
            {runs.length}
          </span>
        ) : null}
      </div>

      {canEdit ? (
        <div className="flex items-center justify-between gap-2 border-t border-border/60 px-3 py-2">
          <div className="flex min-w-0 items-center gap-2">
            <span className="shrink-0 text-xs text-muted-foreground">Agent</span>
            <Select
              value={selectedAgentId || '__none__'}
              onValueChange={(value) => setSelectedAgentId(value === '__none__' ? '' : value)}
              disabled={plannerSelectionDisabled}
            >
              <SelectTrigger size="sm" className="h-7 w-auto min-w-0 gap-1.5 text-xs">
                <SelectValue placeholder="Select agent" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__none__" className="text-xs">No agent selected</SelectItem>
                {plannerAgents.map((agent) => (
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
          {launcherDisabledReason ? (
            <p className="ml-auto min-w-0 truncate text-right text-[11px] text-muted-foreground">
              {launcherDisabledReason}
            </p>
          ) : null}
          <Button
            size="sm"
            variant="outline"
            className="h-7 gap-1 px-2.5 text-xs"
            onClick={() => void handleStart()}
            disabled={!selectedAgentId || starting || !!activeRun}
          >
            {starting ? (
              <Loading01Icon className="h-3 w-3 animate-spin" />
            ) : (
              <PlayIcon className="h-3 w-3" />
            )}
            {starting ? 'Starting...' : 'Run'}
          </Button>
        </div>
      ) : (
        <p className="border-t border-border/60 px-3 py-2 text-xs text-muted-foreground">
          You do not have permission to start or reply to epic planner runs.
        </p>
      )}

      {canEdit && selectedPlanner && !selectedPlanner.system_prompt ? (
        <p className="border-t border-border/60 px-3 py-2 text-[11px] text-muted-foreground">
          This agent is missing system instructions.
        </p>
      ) : null}

      {canEdit && !activeRun ? (
        <div className="space-y-2 border-t border-border/60 px-3 py-2">
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

      <div className="space-y-2 border-t border-border/60 px-3 py-2.5">
        {showVisibleRunsLoading ? (
          <p className="inline-flex items-center gap-1.5 py-1 text-xs text-muted-foreground">
            <Loading01Icon className="h-3 w-3 animate-spin" />
            Loading runs
          </p>
        ) : !featuredRun ? (
          <p className="py-1 text-xs text-muted-foreground">No runs yet. Choose an agent and click Run.</p>
        ) : (
          <FeaturedRunCard
            run={featuredRun}
            agent={agentById[featuredRun.agent_id] ?? null}
            agentName={featuredAgentName ?? 'Agent'}
            action={getEpicPlannerFeaturedAction(featuredRun, featuredAgentName)}
            onOpen={openRun}
          />
        )}

        {historyGroups.length > 0 ? (
          <div className="space-y-0.5">
            {visibleGroups.map((group) => {
              const groupId = group.runs[0].id;
              const agentName = agentNameById[group.agentId] ?? 'Agent';
              const groupAgent = agentById[group.agentId] ?? null;
              if (group.kind === 'collapsed' && !expandedGroupIds.has(groupId)) {
                return (
                  <CollapsedGroupRow
                    key={groupId}
                    group={group}
                    agent={groupAgent}
                    agentName={agentName}
                    onExpand={() => expandGroup(groupId)}
                  />
                );
              }
              return group.runs.map((run) => (
                <HistoryRunRow
                  key={run.id}
                  run={run}
                  agent={agentById[run.agent_id] ?? null}
                  agentName={agentName}
                  onClick={() => openRun(run.id)}
                />
              ));
            })}
            {!showAllHistory && historyGroups.length > HISTORY_VISIBLE_ROW_LIMIT ? (
              <button
                type="button"
                onClick={() => setShowAllHistory(true)}
                className="px-2 pt-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
              >
                Show all {historyRuns.length} runs
              </button>
            ) : null}
          </div>
        ) : null}
      </div>

      <CodingSessionDrawer
        sessionId={selectedRunId}
        open={drawerOpen && !!selectedRunId}
        onOpenChange={setDrawerOpen}
        title={selectedRunId ? `${agentNameById[runs.find((run) => run.id === selectedRunId)?.agent_id ?? ''] ?? 'Epic Agent'} Run` : 'Epic Agent Run'}
        description="Interactive agent chat, artifacts, and live tool activity for this epic."
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
