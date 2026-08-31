import { useEffect, useMemo, useState } from 'react';
import { differenceInDays, parseISO, format } from 'date-fns';
import { useLocation, useNavigate } from '@tanstack/react-router';
import {
  CancelCircleIcon,
  Calendar03Icon,
  CheckmarkCircle02Icon,
  Clock01Icon,
  Key01Icon,
  Loading01Icon,
  MessagePreview01Icon,
  SecurityCheckIcon,
} from '@/lib/icons';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { PRIORITY_CONFIG, StateTypeIcon, PriorityIcon } from '@/lib/pmConstants';
import type { Task, StateType } from '@/lib/pmTypes';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { isAgentRunEventDetail, type AgentRunEventDetail } from '@/lib/agentRunRealtime';
import {
  QuietEmptyState,
  QuietListRow,
  QuietMetaLine,
  QuietMetricBlock,
  QuietMetricGrid,
  QuietPageHeader,
  QuietPageViewport,
  QuietSection,
  QuietStatusText,
  QuietTextAction,
} from '@/components/design-system/quiet';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';

type Mode = 'assigned' | 'requested';

const AGENT_RUN_LABEL: Record<string, string> = {
  queued: 'Agent queued',
  running: 'Agent running',
  completed: 'Agent completed',
  failed: 'Agent failed',
  cancelled: 'Agent cancelled',
};

function getAgentRunLabel(task: Task) {
  if (!task.latest_run_status) return null;
  if (task.latest_run_status === 'paused') {
    if (task.latest_run_pause_reason === 'human_approval') return 'Agent needs approval';
    if (task.latest_run_pause_reason === 'authentication') return 'Agent needs auth';
    if (task.latest_run_pause_reason === 'awaiting_user_message') return 'Agent awaiting reply';
    return 'Agent needs input';
  }
  return AGENT_RUN_LABEL[task.latest_run_status] ?? `Agent ${task.latest_run_status}`;
}

function normalizePauseReason(value: AgentRunEventDetail['pause_reason']) {
  if (!value || value === 'none') return null;
  return value as Task['latest_run_pause_reason'];
}

export function MyWorkPage() {
  useTitle('My Work');
  const navigate = useNavigate();
  const location = useLocation();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const wsSlug = workspace?.slug ?? '';

  const { data: access, isLoading: accessLoading } = useWorkspaceAccess(workspaceId);
  const memberId = access?.membership?.id;

  const { teams, hasTeams, isAdmin, findTeamName, isLoading: teamsLoading } = useAccessibleTeams(workspaceId);
  const showTeam = teams.length > 1;

  const [mode, setMode] = useState<Mode>('assigned');
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loadedMode, setLoadedMode] = useState<Mode | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    if (!workspaceId || !memberId) return;
    let ignore = false;
    const filters =
      mode === 'assigned'
        ? { owner_member_ids: memberId, archived: false as const }
        : { requester_member_id: memberId, archived: false as const };

    pmTaskService
      .list(workspaceId, { ...filters, per_page: 200 })
      .then((res) => {
        if (ignore) return;
        if (res.error) {
          setError(res.error);
          setLoadedMode(mode);
          return;
        }
        setError(null);
        setTasks(res.data?.data ?? []);
        setLoadedMode(mode);
      })
      .catch((requestError: unknown) => {
        if (ignore) return;
        setError(requestError instanceof Error ? requestError.message : 'Unable to load your work.');
        setLoadedMode(mode);
      });
    return () => {
      ignore = true;
    };
  }, [workspaceId, memberId, mode, refreshKey]);

  useEffect(() => {
    if (!workspaceId) return;
    const handleAgentRunEvent = (event: Event) => {
      const detail = (event as CustomEvent<unknown>).detail;
      if (!isAgentRunEventDetail(detail) || detail.entity !== 'agent_run' || detail.parent_type !== 'task' || !detail.parent_id) return;

      setTasks((currentTasks) => currentTasks.map((task) => {
        if (task.id !== detail.parent_id) return task;
        const nextPauseReason =
          detail.status && detail.status !== 'paused'
            ? null
            : Object.prototype.hasOwnProperty.call(detail, 'pause_reason')
              ? normalizePauseReason(detail.pause_reason)
              : task.latest_run_pause_reason;

        const nextRunId = detail.entity_id || task.latest_run_id;
        const nextAgentId = detail.agent_id || task.latest_run_agent_id;
        const nextStatus = detail.status || task.latest_run_status;
        if (
          nextRunId === task.latest_run_id
          && nextAgentId === task.latest_run_agent_id
          && nextStatus === task.latest_run_status
          && nextPauseReason === task.latest_run_pause_reason
        ) return task;

        return {
          ...task,
          latest_run_id: nextRunId,
          latest_run_agent_id: nextAgentId,
          latest_run_status: nextStatus,
          latest_run_pause_reason: nextPauseReason,
          latest_run_at: detail.sent_at || new Date().toISOString(),
        };
      }));
    };

    window.addEventListener('agent_run-updated', handleAgentRunEvent);
    return () => window.removeEventListener('agent_run-updated', handleAgentRunEvent);
  }, [workspaceId]);

    // Refresh list when a task is updated or archived via the global panel
  useEffect(() => {
    const refresh = () => setRefreshKey((k) => k + 1);
    const handleTaskCreated = (e: Event) => {
      const detail = (e as CustomEvent).detail;
      // Navigate to the team's task board so the user sees their new task
      if (detail?.teamId && wsSlug) {
        navigate({ to: '/w/$slug/pm/tasks', params: { slug: wsSlug }, search: { team: detail.teamId } });
        return;
      }
      setRefreshKey((k) => k + 1);
    };
    window.addEventListener('task-panel-updated', refresh);
    window.addEventListener('task-panel-archived', refresh);
    window.addEventListener('task-created', handleTaskCreated);
    return () => {
      window.removeEventListener('task-panel-updated', refresh);
      window.removeEventListener('task-panel-archived', refresh);
      window.removeEventListener('task-created', handleTaskCreated);
    };
  }, [navigate, wsSlug]);

  // ── Derived data ──────────────────────────────────────────────────

  const counts = useMemo(() => {
    const now = new Date();
    let inProgress = 0;
    let dueSoon = 0;
    let overdue = 0;
    let blocked = 0;

    for (const s of tasks) {
      if (s.completed) continue;
      if (s.state_type === 'started') inProgress++;
      if (s.blocked) blocked++;
      if (s.deadline) {
        const days = differenceInDays(parseISO(s.deadline), now);
        if (days < 0) overdue++;
        else if (days <= 3) dueSoon++;
      }
    }
    return { inProgress, dueSoon, overdue, blocked };
  }, [tasks]);

  const { focus, blockedTasks, rest } = useMemo(() => {
    const now = new Date();
    const active = tasks.filter((s) => !s.completed);
    const done = tasks.filter((s) => s.completed);

    const scoreFn = (s: Task): number => {
      let score = 0;
      if (s.deadline) {
        const days = differenceInDays(parseISO(s.deadline), now);
        if (days < 0) score += 1000 + Math.abs(days);
        else if (days <= 3) score += 500;
      }
      if (s.blocked) score += 400;
      if (s.priority === 'urgent') score += 300;
      else if (s.priority === 'high') score += 200;
      if (s.state_type === 'started') score += 100;
      const updatedDaysAgo = differenceInDays(now, parseISO(s.updated_at));
      if (updatedDaysAgo <= 1) score += 50;
      return score;
    };

    const sorted = [...active].sort((a, b) => scoreFn(b) - scoreFn(a));

    const blocked: Task[] = [];
    const focusItems: Task[] = [];
    const restItems: Task[] = [];

    for (const s of sorted) {
      if (s.blocked) {
        blocked.push(s);
      } else if (scoreFn(s) >= 100) {
        focusItems.push(s);
      } else {
        restItems.push(s);
      }
    }

    const recentDone = done
      .sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
      .slice(0, 10);

    return { focus: focusItems, blockedTasks: blocked, rest: [...restItems, ...recentDone] };
  }, [tasks]);

  // ── Render ────────────────────────────────────────────────────────

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  const openTask = (task: Task) => {
    if (!wsSlug) return;
    openTaskRoute(navigate as never, location as never, wsSlug, task.id);
  };

  const showingLoading = accessLoading || teamsLoading || loadedMode !== mode;

  return (
    <div className="flex h-full min-h-0 flex-col">
      <QuietPageHeader
        variant="shell"
        title="My Work"
        description={`Tasks assigned to you and requested by you across ${isAdmin ? 'all' : 'your'} teams.`}
      />

      <Tabs
        value={mode}
        onValueChange={(value) => {
          setError(null);
          setMode(value as Mode);
        }}
        className="min-h-0 flex-1 gap-0"
      >
        <TabsList variant="quiet" aria-label="My Work views" className="w-full shrink-0 justify-start px-4 sm:px-6 lg:px-8">
          <TabsTrigger value="assigned">Assigned to me</TabsTrigger>
          <TabsTrigger value="requested">Requested by me</TabsTrigger>
        </TabsList>

        <QuietPageViewport className="min-h-0 flex-1" contentClassName="max-w-4xl">
          {showingLoading ? (
            <MyWorkLoadingState />
          ) : !hasTeams && !isAdmin ? (
            <NoTeamEmptyState />
          ) : error ? (
            <QuietEmptyState
              title="Unable to load your work"
              description={error}
              action={(
                <QuietTextAction
                  onClick={() => {
                    setError(null);
                    setLoadedMode(null);
                    setRefreshKey((key) => key + 1);
                  }}
                >
                  Try again
                </QuietTextAction>
              )}
            />
          ) : tasks.length === 0 ? (
            <MyWorkEmptyState mode={mode} />
          ) : (
            <div className="space-y-5">
              <QuietMetricGrid>
                <QuietMetricBlock label="In progress" value={counts.inProgress} description="Tasks currently underway" />
                <QuietMetricBlock
                  label="Due soon"
                  value={counts.dueSoon}
                  description="Due within the next three days"
                  tone={counts.dueSoon > 0 ? 'warning' : 'neutral'}
                />
                <QuietMetricBlock
                  label="Overdue"
                  value={counts.overdue}
                  description="Past their due date"
                  tone={counts.overdue > 0 ? 'danger' : 'neutral'}
                />
                <QuietMetricBlock
                  label="Blocked"
                  value={counts.blocked}
                  description="Waiting on another dependency"
                  tone={counts.blocked > 0 ? 'danger' : 'neutral'}
                />
              </QuietMetricGrid>

              <p className="px-1 text-[11.5px] leading-5 text-quiet-muted">
                Focus order is based on due dates, blockers, priority, active state, and recent updates.
              </p>

              <div className="border-t border-quiet-divider-strong">
                {focus.length > 0 ? (
                  <TaskSection title="Focus now" count={focus.length} tasks={focus} onClickTask={openTask} findTeamName={findTeamName} showTeam={showTeam} />
                ) : null}
                {blockedTasks.length > 0 ? (
                  <TaskSection title="Blocked" count={blockedTasks.length} tasks={blockedTasks} onClickTask={openTask} findTeamName={findTeamName} showTeam={showTeam} />
                ) : null}
                {rest.length > 0 ? (
                  <TaskSection title="Everything else" count={rest.length} tasks={rest} onClickTask={openTask} findTeamName={findTeamName} showTeam={showTeam} />
                ) : null}
              </div>
            </div>
          )}
        </QuietPageViewport>
      </Tabs>
    </div>
  );
}

// ── No team empty state ──────────────────────────────────────────

function NoTeamEmptyState() {
  return (
    <QuietEmptyState
      title="No team assigned"
      description="You need to be added to a team to see work items. Ask a workspace administrator to add you to a team."
    >
      <QuietStatusText tone="blocker">Team access required</QuietStatusText>
    </QuietEmptyState>
  );
}

// ── Empty state ───────────────────────────────────────────────────

const WORKFLOW_STEPS = [
  { title: 'Create tasks', description: 'Describe work to be done — bugs, features, or tasks.' },
  { title: 'Assign the work', description: 'Set an owner, priority, and deadline for each task.' },
  { title: 'Track progress', description: 'Tasks move through workflow states as work gets done.' },
];

function MyWorkEmptyState({ mode }: { mode: Mode }) {
  return (
    <QuietEmptyState
      title={mode === 'assigned' ? 'No tasks assigned to you yet' : 'No tasks requested by you yet'}
      description={
        mode === 'assigned'
          ? 'When teammates assign tasks to you, they appear here in focus order.'
          : 'Tasks you create or request will appear here so you can track their progress.'
      }
    >
      <div className="border-t border-quiet-divider-light">
        {WORKFLOW_STEPS.map((step) => (
          <QuietListRow key={step.title} title={step.title} detail={step.description} className="px-0" />
        ))}
      </div>
    </QuietEmptyState>
  );
}

function MyWorkLoadingState() {
  return (
    <div className="space-y-5" aria-live="polite" aria-label="Loading your work">
      <QuietMetricGrid>
        {Array.from({ length: 4 }).map((_, index) => (
          <div key={index} className="space-y-2 p-4 motion-safe:animate-pulse">
            <div className="h-2 w-20 bg-quiet-icon-well" />
            <div className="h-5 w-10 bg-quiet-icon-well" />
            <div className="h-2 w-32 max-w-full bg-quiet-icon-well" />
          </div>
        ))}
      </QuietMetricGrid>
      <div className="border-t border-quiet-divider-strong">
        {Array.from({ length: 5 }).map((_, index) => (
          <div key={index} className="space-y-2 border-b border-quiet-divider-light px-5 py-3 motion-safe:animate-pulse">
            <div className="h-2 w-24 bg-quiet-icon-well" />
            <div className="h-3 w-3/5 bg-quiet-icon-well" />
            <div className="h-2 w-2/5 bg-quiet-icon-well" />
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Task section ─────────────────────────────────────────────────

const COLLAPSE_THRESHOLD = 5;

function TaskSection({ title, count, tasks, onClickTask, findTeamName, showTeam }: {
  title: string;
  count: number;
  tasks: Task[];
  onClickTask: (s: Task) => void;
  findTeamName: (id?: string) => string | undefined;
  showTeam: boolean;
}) {
  const collapsible = tasks.length > COLLAPSE_THRESHOLD;
  const [expanded, setExpanded] = useState(!collapsible);
  const visible = expanded ? tasks : tasks.slice(0, COLLAPSE_THRESHOLD);
  const hiddenCount = tasks.length - COLLAPSE_THRESHOLD;

  return (
    <QuietSection
      title={title}
      count={count}
      className="py-4"
      bodyClassName="-mx-4 -mb-4 sm:-mx-6 lg:-mx-8"
      action={collapsible ? (
        <QuietTextAction onClick={() => setExpanded((value) => !value)}>
          {expanded ? 'Show less' : `Show ${hiddenCount} more`}
        </QuietTextAction>
      ) : undefined}
    >
      <div>
        {visible.map((task) => (
          <TaskRow
            key={task.id}
            task={task}
            onClick={() => onClickTask(task)}
            teamName={showTeam ? findTeamName(task.team_id) : undefined}
          />
        ))}
      </div>
    </QuietSection>
  );
}

// ── Task row ─────────────────────────────────────────────────────

function TaskRow({ task, onClick, teamName }: {
  task: Task;
  onClick: () => void;
  teamName?: string;
}) {
  const deadlineInfo = useMemo(() => {
    if (!task.deadline) return null;
    const d = parseISO(task.deadline);
    const days = differenceInDays(d, new Date());
    const status: 'overdue' | 'approaching' | 'normal' =
      days < 0 ? 'overdue' : days <= 3 ? 'approaching' : 'normal';
    return { label: format(d, 'MMM d'), status };
  }, [task.deadline]);
  const agentRunLabel = getAgentRunLabel(task);
  const agentRunStatus = task.latest_run_status ?? 'queued';
  const AgentRunIcon =
    agentRunStatus === 'running' ? Loading01Icon
      : agentRunStatus === 'completed' ? CheckmarkCircle02Icon
        : agentRunStatus === 'failed' || agentRunStatus === 'cancelled' ? CancelCircleIcon
          : agentRunStatus === 'paused' && task.latest_run_pause_reason === 'human_approval' ? SecurityCheckIcon
            : agentRunStatus === 'paused' && task.latest_run_pause_reason === 'authentication' ? Key01Icon
              : agentRunStatus === 'paused' ? MessagePreview01Icon
                : Clock01Icon;

  const rowState = task.completed
    ? 'positive'
    : task.blocked || deadlineInfo?.status === 'overdue'
      ? 'blocker'
      : task.state_type === 'started'
        ? 'current'
        : 'none';
  const agentTone = agentRunStatus === 'completed'
    ? 'positive'
    : agentRunStatus === 'failed' || agentRunStatus === 'paused'
      ? 'blocker'
      : agentRunStatus === 'running'
        ? 'current'
        : 'neutral';

  const facts = [
    task.state_name && task.state_type ? (
      <span className="inline-flex items-center gap-1.5">
        <StateTypeIcon stateType={task.state_type as StateType} className="h-3.5 w-3.5" />
        {task.state_name}
      </span>
    ) : null,
    task.priority !== 'none' ? (
      <span className="inline-flex items-center gap-1.5">
        <PriorityIcon priority={task.priority} className="h-3.5 w-3.5" />
        {PRIORITY_CONFIG[task.priority].label} priority
      </span>
    ) : null,
    task.blocked ? <span className="font-medium text-quiet-accent">Blocked</span> : null,
    deadlineInfo ? (
      <span className={`inline-flex items-center gap-1.5 ${deadlineInfo.status !== 'normal' ? 'font-medium text-quiet-accent' : ''}`}>
        <Calendar03Icon className="h-3.5 w-3.5" />
        {deadlineInfo.status === 'overdue' ? 'Overdue' : deadlineInfo.status === 'approaching' ? 'Due soon' : 'Due'} {deadlineInfo.label}
      </span>
    ) : null,
    agentRunLabel ? (
      <QuietStatusText tone={agentTone} pulse={agentRunStatus === 'running'}>
        <AgentRunIcon className={`h-3 w-3 ${agentRunStatus === 'running' ? 'motion-safe:animate-spin' : ''}`} />
        {agentRunLabel}
      </QuietStatusText>
    ) : null,
  ];

  return (
    <QuietListRow
      onClick={onClick}
      actor={<span className="whitespace-nowrap font-mono text-[11.5px] tabular-nums text-quiet-muted">{task.task_key}</span>}
      meta={teamName}
      title={<span className={task.completed ? 'line-through text-quiet-text-tertiary' : undefined}>{task.name}</span>}
      detail={<QuietMetaLine items={facts} />}
      state={rowState}
      className="px-4 last:border-b-0 sm:px-6 lg:px-8"
    />
  );
}
