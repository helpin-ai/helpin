import { useEffect, useMemo, useState } from 'react';
import { differenceInDays, parseISO, format } from 'date-fns';
import { useLocation, useNavigate } from '@tanstack/react-router';
import {
  AlertCircleIcon,
  CancelCircleIcon,
  ChartColumnIcon,
  Calendar03Icon,
  CheckmarkCircle02Icon,
  Clock01Icon,
  Key01Icon,
  Loading01Icon,
  MessagePreview01Icon,
  PencilEdit02Icon,
  Timer01Icon,
  UserGroupIcon,
  ClipboardIcon,
  RecordIcon,
  SecurityCheckIcon,
} from '@/lib/icons';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { PRIORITY_BORDER_COLOR, PRIORITY_CONFIG, StateTypeIcon, PriorityIcon } from '@/lib/pmConstants';
import type { Task, StateType } from '@/lib/pmTypes';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';

type Mode = 'assigned' | 'requested';
type DeadlineStatus = 'overdue' | 'approaching' | 'normal';
type AgentRunEventDetail = {
  entity?: string;
  entity_id?: string;
  parent_type?: string;
  parent_id?: string;
  sent_at?: string;
  agent_id?: string;
  status?: string;
  pause_reason?: Task['latest_run_pause_reason'];
};

const DEADLINE_PILL_STYLE: Record<DeadlineStatus, string> = {
  overdue: 'border-red-300 bg-red-50 text-red-600 dark:border-red-800 dark:bg-red-950/50 dark:text-red-400',
  approaching: 'border-amber-300 bg-amber-50 text-amber-600 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-400',
  normal: 'border-border bg-muted/50 text-muted-foreground',
};

const DEADLINE_TOOLTIP: Record<DeadlineStatus, string> = {
  overdue: 'Overdue',
  approaching: 'Due soon',
  normal: 'Due date',
};

const AGENT_RUN_PILL_STYLE: Record<string, string> = {
  queued: 'border-border bg-muted/50 text-muted-foreground',
  running: 'border-foreground/20 bg-foreground/5 text-foreground',
  paused: 'border-amber-300 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-300',
  completed: 'border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-300',
  failed: 'border-red-300 bg-red-50 text-red-600 dark:border-red-800 dark:bg-red-950/50 dark:text-red-400',
  cancelled: 'border-border bg-muted/50 text-muted-foreground',
};

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
    return 'Agent needs input';
  }
  return AGENT_RUN_LABEL[task.latest_run_status] ?? `Agent ${task.latest_run_status}`;
}

function normalizePauseReason(value: AgentRunEventDetail['pause_reason']) {
  if (!value || value === 'none') return null;
  return value;
}

function isAgentRunEventDetail(value: unknown): value is AgentRunEventDetail {
  return typeof value === 'object' && value !== null;
}



export function MyWorkPage() {
  useTitle('My Work');
  const navigate = useNavigate();
  const location = useLocation();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const wsSlug = workspace?.slug ?? '';

  const { data: access } = useWorkspaceAccess(workspaceId);
  const memberId = access?.membership?.id;

  const { teams, hasTeams, isAdmin, findTeamName } = useAccessibleTeams(workspaceId);
  const showTeam = teams.length > 1;

  const [mode, setMode] = useState<Mode>('assigned');
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(false);
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    if (!workspaceId || !memberId) return;
    setLoading(true);
    const filters =
      mode === 'assigned'
        ? { owner_member_id: memberId, archived: false as const }
        : { requester_member_id: memberId, archived: false as const };

    pmTaskService
      .list(workspaceId, { ...filters, per_page: 200 })
      .then((res) => {
        if (res.data) {
          setTasks(res.data.data);
        }
      })
      .finally(() => setLoading(false));
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

        return {
          ...task,
          latest_run_id: detail.entity_id || task.latest_run_id,
          latest_run_agent_id: detail.agent_id || task.latest_run_agent_id,
          latest_run_status: detail.status || task.latest_run_status,
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
  }, [memberId]);

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

  return (
    <div className="max-w-4xl mx-auto">
      <header className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-semibold">My Work</h2>
          <p className="text-sm text-muted-foreground">
            {mode === 'assigned'
              ? `Tasks assigned to you across ${isAdmin ? 'all' : 'your'} teams.`
              : `Tasks you requested across ${isAdmin ? 'all' : 'your'} teams.`}
          </p>
        </div>

        {/* Mode toggle */}
        <div className="flex gap-0.5 rounded-lg bg-muted/60 p-0.5">
          {(['assigned', 'requested'] as const).map((m) => (
            <button
              key={m}
              type="button"
              onClick={() => setMode(m)}
              className={`rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
                mode === m
                  ? 'bg-background text-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground'
              }`}
            >
              {m === 'assigned' ? 'Assigned to me' : 'Requested by me'}
            </button>
          ))}
        </div>
      </header>

      {/* Summary cards */}
      {tasks.length > 0 && (
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-8">
          <SummaryCard icon={RecordIcon} iconColor="text-amber-500" label="In progress" value={counts.inProgress} />
          <SummaryCard icon={Clock01Icon} iconColor="text-blue-500" label="Due soon" value={counts.dueSoon} />
          <SummaryCard icon={Timer01Icon} iconColor="text-red-500" label="Overdue" value={counts.overdue} />
          <SummaryCard icon={AlertCircleIcon} iconColor="text-orange-500" label="Blocked" value={counts.blocked} />
        </div>
      )}

      {/* Content */}
      {loading ? null : !hasTeams && !isAdmin ? (
        <NoTeamEmptyState />
      ) : tasks.length === 0 ? (
        <MyWorkEmptyState mode={mode} />
      ) : (
        <div className="space-y-10">
          {focus.length > 0 && (
            <TaskSection title="Focus now" count={focus.length} tasks={focus} onClickTask={openTask} findTeamName={findTeamName} showTeam={showTeam} />
          )}
          {blockedTasks.length > 0 && (
            <TaskSection title="Blocked" count={blockedTasks.length} tasks={blockedTasks} onClickTask={openTask} findTeamName={findTeamName} showTeam={showTeam} />
          )}
          {rest.length > 0 && (
            <TaskSection title="Everything else" count={rest.length} tasks={rest} onClickTask={openTask} findTeamName={findTeamName} showTeam={showTeam} />
          )}
        </div>
      )}
    </div>
  );
}

// ── No team empty state ──────────────────────────────────────────

function NoTeamEmptyState() {
  return (
    <div className="flex flex-col items-center py-16 px-4">
      <div className="flex h-14 w-14 items-center justify-center rounded-full bg-muted/50 mb-5">
        <UserGroupIcon className="h-7 w-7 text-muted-foreground" />
      </div>
      <h3 className="text-base font-medium mb-1">No team assigned</h3>
      <p className="text-sm text-muted-foreground text-center max-w-md">
        You need to be added to a team to see work items. Ask a workspace admin to add you to a team.
      </p>
    </div>
  );
}

// ── Empty state ───────────────────────────────────────────────────

const WORKFLOW_STEPS = [
  { icon: PencilEdit02Icon, title: 'Create tasks', description: 'Describe work to be done — bugs, features, or tasks' },
  { icon: UserGroupIcon, title: 'Assign to team', description: 'Set an owner, priority, and deadline for each task' },
  { icon: ChartColumnIcon, title: 'Track progress', description: 'Tasks move through workflow states as work gets done' },
];

function MyWorkEmptyState({ mode }: { mode: Mode }) {
  return (
    <div className="flex flex-col items-center py-16 px-4">
      {/* Hero */}
      <div className="flex h-14 w-14 items-center justify-center rounded-full bg-blue-500/10 mb-5">
        <ClipboardIcon className="h-7 w-7 text-blue-500" />
      </div>
      <h3 className="text-base font-medium mb-1">
        {mode === 'assigned' ? 'No tasks assigned to you yet' : 'No tasks requested by you yet'}
      </h3>
      <p className="text-sm text-muted-foreground text-center max-w-md">
        {mode === 'assigned'
          ? 'When teammates assign tasks to you, they appear here — prioritized so you always know what to focus on first.'
          : 'Tasks you create or request will appear here so you can track their progress.'}
      </p>

      <div className="w-full max-w-4xl mt-10">
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          {WORKFLOW_STEPS.map(({ icon: Icon, title, description }) => (
            <div key={title} className="flex flex-col items-center text-center rounded-lg border border-border/50 bg-muted/30 p-6">
              <Icon className="h-5 w-5 text-muted-foreground mb-3" />
              <p className="text-sm font-medium mb-1">{title}</p>
              <p className="text-sm text-muted-foreground leading-relaxed">{description}</p>
            </div>
          ))}
        </div>
      </div>

    </div>
  );
}

// ── Summary card ──────────────────────────────────────────────────

function SummaryCard({ icon: Icon, iconColor, label, value }: {
  icon: React.ElementType;
  iconColor: string;
  label: string;
  value: number;
}) {
  return (
    <div className="flex items-center gap-3 rounded-lg border border-border/30 bg-muted/20 px-4 py-3">
      <Icon className={`h-4 w-4 shrink-0 ${iconColor}`} />
      <div className="min-w-0">
        <p className="text-lg font-semibold leading-none tabular-nums">{value}</p>
        <p className="text-[11px] text-muted-foreground/70 mt-0.5">{label}</p>
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
    <div>
      <div className="flex items-center gap-2 mb-1 px-1">
        <h3 className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
          {title}
        </h3>
        <span className="text-xs text-muted-foreground/50 tabular-nums">{count}</span>
      </div>
      <div className="divide-y divide-border/40">
        {visible.map((task) => (
          <TaskRow
            key={task.id}
            task={task}
            onClick={() => onClickTask(task)}
            teamName={showTeam ? findTeamName(task.team_id) : undefined}
          />
        ))}
      </div>
      {collapsible && (
        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
          className="mt-1 px-2 py-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
        >
          {expanded ? 'Show less' : `Show ${hiddenCount} more`}
        </button>
      )}
    </div>
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

  return (
    <button
      type="button"
      onClick={onClick}
      className="flex items-center gap-2.5 px-2 py-2.5 w-full text-left rounded-md hover:bg-muted/40 transition-colors group"
    >
      <span className="text-xs text-muted-foreground/50 font-mono shrink-0 w-14 text-right tabular-nums">
        {task.task_key}
      </span>

      <span className={`text-sm truncate flex-1 min-w-0 ${task.completed ? 'line-through text-muted-foreground/60' : 'text-foreground'}`}>
        {task.name}
      </span>

      {/* Metadata pills — matches TaskCard style */}
      <div className="flex items-center gap-1.5 shrink-0">
        {task.priority !== 'none' && (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={`flex h-5 shrink-0 items-center rounded-sm border-[0.5px] bg-muted/50 px-1 ${PRIORITY_BORDER_COLOR[task.priority]}`}>
                <PriorityIcon priority={task.priority} className="h-3.5 w-3.5" />
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">Priority: {PRIORITY_CONFIG[task.priority].label}</TooltipContent>
          </Tooltip>
        )}

        {task.state_name && task.state_type && (
          <span className="flex h-5 items-center gap-1 rounded-sm border-[0.5px] border-border bg-muted/50 px-2 text-[11px] font-medium text-muted-foreground shrink-0 hidden md:flex">
            <StateTypeIcon stateType={task.state_type as StateType} className="h-3 w-3" />
            {task.state_name}
          </span>
        )}

        {task.blocked && (
          <span className="flex h-5 items-center gap-1 rounded-sm border-[0.5px] border-red-300 bg-red-50 px-2 text-[11px] font-medium text-red-600 dark:border-red-800 dark:bg-red-950/50 dark:text-red-400 shrink-0">
            Blocked
          </span>
        )}

        {agentRunLabel && (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={`flex h-5 items-center gap-1 rounded-sm border-[0.5px] px-2 text-[11px] font-medium shrink-0 ${AGENT_RUN_PILL_STYLE[agentRunStatus] ?? AGENT_RUN_PILL_STYLE.queued}`}>
                <AgentRunIcon className={`h-3 w-3 ${agentRunStatus === 'running' ? 'animate-spin' : ''}`} />
                <span className="hidden sm:inline">{agentRunLabel}</span>
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">{agentRunLabel}</TooltipContent>
          </Tooltip>
        )}

        {deadlineInfo && (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={`flex h-5 items-center gap-1 rounded-sm border-[0.5px] px-2 text-[11px] font-medium shrink-0 ${DEADLINE_PILL_STYLE[deadlineInfo.status]}`}>
                <Calendar03Icon className="h-3 w-3" />
                <span className="hidden sm:inline">{deadlineInfo.label}</span>
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">
              {DEADLINE_TOOLTIP[deadlineInfo.status]}: {deadlineInfo.label}
            </TooltipContent>
          </Tooltip>
        )}

        {teamName && (
          <span className="flex h-5 items-center rounded-sm border-[0.5px] border-border bg-muted/50 px-2 text-[11px] font-medium text-muted-foreground shrink-0 hidden lg:flex truncate max-w-[100px]">
            {teamName}
          </span>
        )}
      </div>
    </button>
  );
}
