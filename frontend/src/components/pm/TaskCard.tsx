import type React from 'react';
import { memo, useCallback, useContext, useMemo, useState } from 'react';
import { useSortable } from '@dnd-kit/sortable';
import {
  Alert01Icon,
  Layers01Icon,
} from '@/lib/icons';
import { Calendar03Icon, Tick01Icon, UserAdd01Icon } from '@/lib/pmIcons';
import { AgentAvatar, resolveAgentPersonaKey } from '@/components/agents/AgentAvatar';
import { differenceInDays, format, formatDistanceToNow, isBefore, parseISO, startOfDay } from 'date-fns';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { cn } from '@/lib/utils';
import { PRIORITY_BORDER_COLOR, PRIORITY_CONFIG, PriorityIcon, SEVERITY_CONFIG, SeverityIcon, SprintIcon, StateTypeIcon, TASK_TYPE_CONFIG, TaskTypeIcon } from '@/lib/pmConstants';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { MultiMemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { OwnerAvatarStack } from '@/components/pm/OwnerAvatarStack';
import { RecurringTemplateBadge } from '@/components/pm/RecurringTemplateBadge';
import { UserAvatar } from './UserAvatar';
import { getSortableTaskCardStyle, animateCardLayoutChanges, shouldIgnoreTaskCardDrag } from './TaskCard.sortable';
import type { Agent, Priority, Severity, Task } from '@/lib/pmTypes';
import type { AssignableMember } from '@/lib/types';
import { EstimatePicker, formatEstimateDisplay } from '@/components/pm/EstimatePicker';
import { LabelBadge } from '@/components/pm/LabelPicker';
import { useTeamFieldVisibilityForTeam } from '@/hooks/queries';
import { useBoardDisplayStore } from '@/stores/boardDisplayStore';
import { BoardDataContext, BoardCallbacksContext } from './KanbanBoard.contexts';
import { ACTIVE_RUN_STATUSES } from './agentRunConstants';

// ── Shared constants ────────────────────────────────────────────────

const pillBase = 'flex h-5 items-center gap-1 rounded-sm border-[0.5px] px-2 text-[11px] font-medium';


const ALL_PRIORITIES: Priority[] = ['urgent', 'high', 'medium', 'low', 'none'];
const ALL_SEVERITIES: Severity[] = ['critical', 'major', 'minor', 'none'];

// ── Component ───────────────────────────────────────────────────────

interface TaskCardProps {
  task: Task;
  /** @deprecated Use BoardCallbacksContext instead. Kept for backward compat outside KanbanBoard. */
  onOpen?: (task: Task) => void;
  /** @deprecated Use BoardCallbacksContext instead. Kept for backward compat outside KanbanBoard. */
  onOpenAgentRun?: (task: Task) => void;
  isOverlay?: boolean;
  teamName?: string;
  /** @deprecated Use BoardDataContext instead */
  workspaceId?: string;
  /** @deprecated Use BoardDataContext instead */
  assignableMembers?: AssignableMember[];
  /** @deprecated Use BoardDataContext instead */
  ownerNameMap?: Map<string, string>;
  /** @deprecated Use BoardCallbacksContext.onTaskPatched instead */
  onOwnerChanged?: (task: Task) => void;
  /** @deprecated Use BoardCallbacksContext.onTaskPatched instead */
  onPriorityChanged?: (task: Task) => void;
  /** @deprecated Use BoardCallbacksContext.onTaskPatched instead */
  onSeverityChanged?: (task: Task) => void;
  /** @deprecated Use BoardCallbacksContext.onTaskPatched instead */
  onEstimateChanged?: (task: Task) => void;
  showStateBadge?: boolean;
}

const AGENT_OCTAGON_POINTS = '30,2 70,2 98,30 98,70 70,98 30,98 2,70 2,30';

const AWAITING_INPUT_LABELS: Record<string, string> = {
  human_input: 'Awaiting your input',
  human_approval: 'Awaiting approval',
  authentication: 'Needs auth',
};

function getAwaitingLabel(pauseReason?: string | null): string | null {
  if (!pauseReason || pauseReason === 'none') return null;
  return AWAITING_INPUT_LABELS[pauseReason] ?? 'Awaiting your input';
}

function TaskCardAgentBadge({
  agent,
  isWorking = false,
  latestRunStatus,
}: {
  agent?: Pick<Agent, 'id' | 'name' | 'preset_key' | 'status'> | null;
  isWorking?: boolean;
  latestRunStatus?: string | null;
}) {
  const isGenericAgent = resolveAgentPersonaKey({ agent }) === 'generic';
  const statusDotClassName = latestRunStatus === 'completed'
    ? 'bg-emerald-500 dark:bg-emerald-400'
    : latestRunStatus === 'failed'
      ? 'bg-red-500 dark:bg-red-400'
      : null;

  return (
    <span className="relative block h-7 w-7 shrink-0">
      {isWorking ? (
        <>
          <svg
            viewBox="0 0 100 100"
            aria-hidden="true"
            className="absolute inset-0 h-full w-full overflow-visible motion-safe:animate-spin motion-safe:[animation-duration:2.4s]"
          >
            <polygon
              points={AGENT_OCTAGON_POINTS}
              fill="none"
              className="stroke-foreground/80"
              strokeWidth={4}
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
          <span
            className="absolute inset-[2px] overflow-hidden bg-background/95"
            style={{ clipPath: 'polygon(31% 4%, 69% 4%, 96% 31%, 96% 69%, 69% 96%, 31% 96%, 4% 69%, 4% 31%)' }}
          >
            <AgentAvatar
              agent={agent}
              className="h-full w-full rounded-none border-0 bg-transparent shadow-none"
              genericBare={isGenericAgent}
            />
          </span>
        </>
      ) : (
        <AgentAvatar
          agent={agent}
          className="h-7 w-7 rounded-none border-0 bg-transparent shadow-none"
          genericBare={isGenericAgent}
        />
      )}
      {statusDotClassName ? (
        <span
          aria-hidden="true"
          className={cn(
            'absolute bottom-0 right-0 h-2 w-2 rounded-full ring-1 ring-background',
            statusDotClassName,
          )}
        />
      ) : null}
    </span>
  );
}

function TaskCardComponent({
  task,
  onOpen: onOpenProp,
  onOpenAgentRun: onOpenAgentRunProp,
  isOverlay = false,
  teamName,
  workspaceId: workspaceIdProp,
  assignableMembers: assignableMembersProp,
  ownerNameMap: ownerNameMapProp,
  onOwnerChanged,
  onPriorityChanged,
  onSeverityChanged,
  onEstimateChanged,
  showStateBadge = false,
}: TaskCardProps) {
  // Consume board contexts (null when used outside KanbanBoard)
  const boardData = useContext(BoardDataContext);
  const callbacksRef = useContext(BoardCallbacksContext);

  // Resolve values: context first, then prop fallback
  const workspaceId = boardData?.workspaceId ?? workspaceIdProp;
  const assignableMembers = boardData?.assignableMembers ?? assignableMembersProp;
  const ownerNameMap = boardData?.ownerNameMap ?? ownerNameMapProp;
  const agentById = boardData?.agentById;
  const latestRunAgent = agentById && task.latest_run_agent_id ? agentById.get(task.latest_run_agent_id) ?? null : null;
  const onOpen = callbacksRef?.current.onOpen ?? onOpenProp;
  const onOpenAgentRun = callbacksRef?.current.onOpenAgentRun ?? onOpenAgentRunProp ?? onOpen;
  const onTaskPatched = callbacksRef?.current.onTaskPatched;

  const {
    attributes,
    listeners,
    setNodeRef,
    setActivatorNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: task.id, disabled: isOverlay, animateLayoutChanges: animateCardLayoutChanges });

  const style = getSortableTaskCardStyle({
    transform,
    transition,
    isDragging,
  });
  const setCardNodeRef = useCallback(
    (node: HTMLElement | null) => {
      setNodeRef(node);
      if (!isOverlay) {
        setActivatorNodeRef(node);
      }
    },
    [isOverlay, setActivatorNodeRef, setNodeRef],
  );
  const cardDragListeners = useMemo(() => {
    if (isOverlay || !listeners) return undefined;
    const pointerDown = listeners.onPointerDown as ((event: React.PointerEvent<HTMLElement>) => void) | undefined;
    return {
      ...listeners,
      onPointerDown: (event: React.PointerEvent<HTMLElement>) => {
        if (shouldIgnoreTaskCardDrag(event.target, event.currentTarget)) {
          return;
        }
        pointerDown?.(event);
      },
    };
  }, [isOverlay, listeners]);

  const [priorityOpen, setPriorityOpen] = useState(false);
  const [severityOpen, setSeverityOpen] = useState(false);
  const fieldVis = useTeamFieldVisibilityForTeam(workspaceId ?? '', task.team_id);
  const displayProps = useBoardDisplayStore((s) => s.properties);
  const vis = useMemo(() => ({
    task_id: displayProps.task_id,
    task_type: fieldVis.task_type && displayProps.task_type,
    priority: fieldVis.priority && displayProps.priority,
    severity: fieldVis.severity && displayProps.severity,
    agent: displayProps.agent,
    epic: fieldVis.epic && displayProps.epic,
    sprint: (fieldVis.sprint ?? true) && (displayProps.sprint ?? true),
    labels: (fieldVis.labels ?? true) && displayProps.labels,
    estimate: fieldVis.estimate && displayProps.estimate,
    due_date: fieldVis.due_date && displayProps.due_date,
    blocked: fieldVis.blocked && displayProps.blocked,
    assignee: displayProps.assignee,
  }), [fieldVis, displayProps]);

  const due = useMemo(() => {
    if (!task.deadline) return null;
    const date = parseISO(task.deadline);
    const today = startOfDay(new Date());
    const overdue = isBefore(date, today) && !task.completed;
    const daysAway = differenceInDays(date, today);
    const approaching = !task.completed && !overdue && daysAway <= 3;
    return {
      label: format(date, 'MMM d'),
      overdue,
      approaching,
    };
  }, [task.deadline, task.completed]);

  const severityCfg = task.severity !== 'none' && task.severity in SEVERITY_CONFIG
    ? SEVERITY_CONFIG[task.severity]
    : null;
  const blockedLabel = useMemo(() => {
    if (!task.blocked) return null;
    if (task.blocked_by_count && task.blocked_by_count > 0) {
      if (task.blocked_by_count === 1 && task.blocked_by_tasks?.[0]) {
        return `Blocked by ${task.blocked_by_tasks[0].task_key}`;
      }
      return `Blocked by ${task.blocked_by_count} tasks`;
    }
    if (task.blocker?.trim()) {
      return 'External blocker';
    }
    return 'Blocked';
  }, [task.blocked, task.blocked_by_count, task.blocked_by_tasks, task.blocker]);

  const priorityCfg = PRIORITY_CONFIG[task.priority];
  const taskTypeCfg = TASK_TYPE_CONFIG[task.task_type];
  const ownerMemberIds = task.owner_member_ids ?? [];
  const currentOwnerName = useMemo(() => {
    if (ownerMemberIds.length === 0) return null;
    return ownerMemberIds
      .map((ownerId) => ownerNameMap?.get(ownerId) ?? ownerId)
      .join(', ');
  }, [ownerMemberIds, ownerNameMap]);
  const shouldShowAgentRow = vis.agent && !!task.latest_run_agent_id;
  const hasActiveRun =
    shouldShowAgentRow
    && !isOverlay
    && !!task.latest_run_id
    && !!task.latest_run_status
    && ACTIVE_RUN_STATUSES.has(task.latest_run_status);
  const awaitingLabel = task.latest_run_status === 'paused'
    ? getAwaitingLabel(task.latest_run_pause_reason)
    : null;
  const runTimeLabel = task.latest_run_at
    ? `Last run ${formatDistanceToNow(new Date(task.latest_run_at), { addSuffix: true })}`
    : 'Last run';
  const agentStatusLabel = awaitingLabel
    ?? (task.latest_run_status === 'running'
      ? 'Running'
      : task.latest_run_status === 'queued'
        ? 'Queued'
        : task.latest_run_status === 'paused'
          ? 'Paused'
          : task.latest_run_status === 'failed'
            ? 'Failed'
            : task.latest_run_status === 'completed'
              ? 'Completed'
              : 'Last run');
  const agentTooltipLabel = awaitingLabel
    ? `${awaitingLabel}${latestRunAgent?.name ? ` · ${latestRunAgent.name}` : ''} · Open run`
    : hasActiveRun
      ? `${runTimeLabel}${latestRunAgent?.name ? `: ${latestRunAgent.name}` : ''} · Open run`
      : `${runTimeLabel}${latestRunAgent?.name ? `: ${latestRunAgent.name}` : ''}`;
  const handleAgentClick = useCallback(
    (e: React.MouseEvent | React.KeyboardEvent) => {
      e.stopPropagation();
      e.preventDefault();
      onOpenAgentRun?.(task);
    },
    [onOpenAgentRun, task],
  );
  const handleAssignOwner = useCallback(
    async (ownerIds: string[]) => {
      if (!workspaceId) return;
      try {
        const result = await pmTaskService.update(workspaceId, task.id, { owner_member_ids: ownerIds });
        if (result.data?.task) {
          (onTaskPatched ?? onOwnerChanged)?.(result.data.task);
        }
      } catch {
        // Board will show stale data until next refresh
      }
    },
    [workspaceId, task.id, onTaskPatched, onOwnerChanged],
  );

  const handleChangePriority = useCallback(
    async (priority: Priority) => {
      if (!workspaceId || priority === task.priority) {
        setPriorityOpen(false);
        return;
      }
      try {
        const result = await pmTaskService.update(workspaceId, task.id, { priority });
        if (result.data?.task) {
          (onTaskPatched ?? onPriorityChanged)?.(result.data.task);
        }
      } catch {
        // Board will show stale data until next refresh
      }
      setPriorityOpen(false);
    },
    [workspaceId, task.id, task.priority, onTaskPatched, onPriorityChanged],
  );

  const handleChangeSeverity = useCallback(
    async (severity: Severity) => {
      if (!workspaceId || severity === task.severity) {
        setSeverityOpen(false);
        return;
      }
      try {
        const result = await pmTaskService.update(workspaceId, task.id, { severity });
        if (result.data?.task) {
          (onTaskPatched ?? onSeverityChanged)?.(result.data.task);
        }
      } catch {
        // Board will show stale data until next refresh
      }
      setSeverityOpen(false);
    },
    [workspaceId, task.id, task.severity, onTaskPatched, onSeverityChanged],
  );

  const handleChangeEstimate = useCallback(
    async (_display: string, apiValue: number | undefined) => {
      if (!workspaceId || apiValue === task.estimate) return;
      try {
        const result = await pmTaskService.update(workspaceId, task.id, { estimate: apiValue ?? 0 });
        if (result.data?.task) {
          (onTaskPatched ?? onEstimateChanged)?.(result.data.task);
        }
      } catch {
        // Board will show stale data until next refresh
      }
    },
    [workspaceId, task.id, task.estimate, onTaskPatched, onEstimateChanged],
  );

  const shouldShowTaskKey = vis.task_id && !!task.task_key;
  const taskTitleText = shouldShowTaskKey ? `${task.task_key}: ${task.name}` : task.name;
  const titleIsLong = taskTitleText.length > 60;
  const taskTitleContent = (
    <>
      {shouldShowTaskKey ? (
        <>
          <span className="font-mono text-[13px] text-muted-foreground">{task.task_key}:</span>{' '}
        </>
      ) : null}
      {task.name}
    </>
  );

  return (
    <article
      ref={setCardNodeRef}
      style={style}
      data-pm-task-card="true"
      data-pm-task-card-id={task.id}
      {...(!isOverlay ? attributes : {})}
      {...(cardDragListeners ?? {})}
      role="button"
      tabIndex={0}
      onClick={() => onOpen?.(task)}
      onKeyDown={(event) => {
        if (event.target !== event.currentTarget) return;
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onOpen?.(task);
        }
      }}
      className={cn(
        'group/card relative shrink-0 rounded-lg border border-border/60 bg-card shadow-sm transition-all overflow-hidden',
        !isOverlay && 'cursor-grab touch-none select-none active:cursor-grabbing',
        'hover:border-border hover:shadow-md',
        isDragging && 'opacity-50',
        isOverlay && 'opacity-80 ring-1 ring-primary/30 shadow-2xl',
        showStateBadge && task.state_color && 'flex flex-row',
      )}
    >
      {/* State color accent bar (member board only) */}
      {showStateBadge && task.state_color && (
        <div className="w-0.5 shrink-0 self-stretch rounded-l-lg" style={{ backgroundColor: task.state_color }} />
      )}
      <div className="p-3 flex-1 min-w-0">
      {/* Row 1: State badge + Team */}
      {((showStateBadge && task.state_name) || teamName) && (
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        {showStateBadge && task.state_name && (
          <span className={cn(pillBase, 'shrink-0 border-border bg-muted/50 text-muted-foreground')}>
            <StateTypeIcon stateType={(task.state_type as import('@/lib/pmTypes').StateType) ?? 'unstarted'} className="h-3 w-3" />
            {task.state_name}
          </span>
        )}

        <span className="flex-1" />

        {teamName && (
              <span className={cn(pillBase, 'shrink-0 border-border bg-muted/50 text-muted-foreground')}>
                {teamName}
              </span>
        )}
      </div>
      )}

      {/* Row 2: Title + Task-type icon (right) */}
      {task.recurring_template_id ? (
        <div className="mt-3">
          <RecurringTemplateBadge compact occurrenceNumber={task.recurring_occurrence_number} />
        </div>
      ) : null}
      <div className="mb-3.5 mt-2 flex items-start gap-2">
        {titleIsLong ? (
          <Tooltip>
            <TooltipTrigger asChild>
              <h4 className="m-0 flex-1 line-clamp-2 text-sm font-medium leading-snug text-foreground">
                {taskTitleContent}
              </h4>
            </TooltipTrigger>
            <TooltipContent side="bottom" className="max-w-[300px]">{taskTitleText}</TooltipContent>
          </Tooltip>
        ) : (
          <h4 className="m-0 flex-1 line-clamp-2 text-sm font-medium leading-snug text-foreground">
            {taskTitleContent}
          </h4>
        )}
        {vis.task_type && (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="shrink-0">
                <TaskTypeIcon taskType={task.task_type} className="h-[18px] w-[18px]" />
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">{taskTypeCfg.label}</TooltipContent>
          </Tooltip>
        )}
      </div>

      {/* Epic row */}
      {vis.epic && task.epic_name && (
        <div className="mt-1.5 flex items-center gap-1.5 text-xs text-muted-foreground">
          <Layers01Icon className="h-3 w-3 shrink-0" />
          <span className="truncate">{task.epic_name}</span>
        </div>
      )}

      {/* Sprint row */}
      {vis.sprint && task.sprint_name && (
        <div className="mt-1 flex items-center gap-1.5 text-xs text-muted-foreground">
          <SprintIcon className="h-3 w-3 shrink-0 text-muted-foreground" />
          <span className="truncate">{task.sprint_name}</span>
        </div>
      )}

      {/* Row 3: Property pills */}
      <div className="mt-2 flex flex-wrap items-center gap-1.5">
        {/* Severity pill — clickable dropdown */}
        {vis.severity && (severityCfg && workspaceId ? (
          <Popover open={severityOpen} onOpenChange={setSeverityOpen}>
                <PopoverTrigger asChild>
                  <button
                    type="button"
                    className={cn(pillBase, 'border-border bg-muted/50 transition-colors hover:bg-muted', severityCfg.color)}
                    onClick={(e) => { e.stopPropagation(); setSeverityOpen(true); }}
                  >
                    <SeverityIcon severity={task.severity} className="h-3 w-3" />
                    {severityCfg.label}
                  </button>
                </PopoverTrigger>
            {severityOpen && (
              <PopoverContent
                className="w-[180px] p-0"
                align="start"
                side="bottom"
                onClick={(e) => e.stopPropagation()}
                onKeyDown={(e) => e.stopPropagation()}
              >
                <Command>
                  <CommandInput placeholder="Search..." className="h-8 text-xs" />
                  <CommandList>
                    <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No match</CommandEmpty>
                    <CommandGroup>
                      {ALL_SEVERITIES.map((sev) => {
                        const cfg = SEVERITY_CONFIG[sev];
                        return (
                          <CommandItem
                            key={sev}
                            value={cfg.label}
                            onSelect={() => handleChangeSeverity(sev)}
                            className="flex items-center gap-2 text-xs"
                          >
                            <SeverityIcon severity={sev} className="h-3.5 w-3.5" />
                            <span>{cfg.label}</span>
                            {task.severity === sev && <Tick01Icon className="ml-auto h-3.5 w-3.5 text-primary" />}
                          </CommandItem>
                        );
                      })}
                    </CommandGroup>
                  </CommandList>
                </Command>
              </PopoverContent>
            )}
          </Popover>
        ) : severityCfg ? (
              <span className={cn(pillBase, 'border-border bg-muted/50', severityCfg.color)}>
                <SeverityIcon severity={task.severity} className="h-3 w-3" />
                {severityCfg.label}
              </span>
        ) : null)}

        {vis.blocked && task.blocked && blockedLabel && (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={cn(pillBase, 'border-red-300 bg-red-50 text-red-600 dark:border-red-800 dark:bg-red-950/50 dark:text-red-400')}>
                <Alert01Icon className="h-3 w-3" />
                {blockedLabel}
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">{blockedLabel}</TooltipContent>
          </Tooltip>
        )}

        {/* Labels */}
        {vis.labels && task.labels && task.labels.length > 0 && task.labels.map((label) => (
          <LabelBadge key={label.id} label={label} />
        ))}
      </div>

      {/* Row 4: Footer - priority, due date, estimate + assignee */}
      <div data-task-card-footer="true" className="mt-2 flex items-center gap-1.5">
        <div data-task-card-footer-metadata="true" className="flex min-w-0 items-center gap-1.5">
          {/* Priority pill — clickable dropdown (hidden when 'none') */}
          {vis.priority && task.priority !== 'none' && (workspaceId ? (
            <Popover open={priorityOpen} onOpenChange={setPriorityOpen}>
              <Tooltip open={priorityOpen ? false : undefined}>
                <TooltipTrigger asChild>
                  <PopoverTrigger asChild>
                    <button
                      type="button"
                      className={cn(
                        'flex h-6 shrink-0 items-center rounded-sm border-[0.5px] bg-muted/50 px-1 transition-colors hover:bg-muted',
                        PRIORITY_BORDER_COLOR[task.priority],
                      )}
                      onClick={(e) => { e.stopPropagation(); setPriorityOpen(true); }}
                    >
                      <PriorityIcon priority={task.priority} className="h-4 w-4" />
                    </button>
                  </PopoverTrigger>
                </TooltipTrigger>
                <TooltipContent side="top">Priority: {priorityCfg.label}</TooltipContent>
              </Tooltip>
              {priorityOpen && (
                <PopoverContent
                  className="w-[180px] p-0"
                  align="start"
                  side="bottom"
                  onClick={(e) => e.stopPropagation()}
                  onKeyDown={(e) => e.stopPropagation()}
                >
                  <Command>
                    <CommandInput placeholder="Search..." className="h-8 text-xs" />
                    <CommandList>
                      <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No match</CommandEmpty>
                      <CommandGroup>
                        {ALL_PRIORITIES.map((p) => {
                          const cfg = PRIORITY_CONFIG[p];
                          return (
                            <CommandItem
                              key={p}
                              value={cfg.label}
                              onSelect={() => handleChangePriority(p)}
                              className="flex items-center gap-2 text-xs"
                            >
                              <PriorityIcon priority={p} className="h-3.5 w-3.5" />
                              <span>{cfg.label}</span>
                              {task.priority === p && <Tick01Icon className="ml-auto h-3.5 w-3.5 text-primary" />}
                            </CommandItem>
                          );
                        })}
                      </CommandGroup>
                    </CommandList>
                  </Command>
                </PopoverContent>
              )}
            </Popover>
          ) : (
            <Tooltip>
              <TooltipTrigger asChild>
                <span className={cn(
                  'flex h-6 shrink-0 items-center rounded-sm border-[0.5px] bg-muted/50 px-1',
                  PRIORITY_BORDER_COLOR[task.priority],
                )}>
                  <PriorityIcon priority={task.priority} className="h-4 w-4" />
                </span>
              </TooltipTrigger>
              <TooltipContent side="top">Priority: {priorityCfg.label}</TooltipContent>
            </Tooltip>
          ))}
          {vis.due_date && due && (
            <Tooltip>
              <TooltipTrigger asChild>
                <span className={cn(
                  pillBase,
                  due.overdue
                    ? 'border-red-300 bg-red-50 text-red-600 dark:border-red-800 dark:bg-red-950/50 dark:text-red-400'
                    : due.approaching
                      ? 'border-amber-300 bg-amber-50 text-amber-600 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-400'
                      : 'border-border bg-muted/50 text-muted-foreground',
                )}>
                  <Calendar03Icon className="h-3 w-3 shrink-0" />
                  {due.label}
                </span>
              </TooltipTrigger>
              <TooltipContent side="top">
                {due.overdue ? 'Overdue' : due.approaching ? 'Due soon' : 'Due date'}: {due.label}
              </TooltipContent>
            </Tooltip>
          )}
          {vis.estimate && task.estimate != null && (workspaceId ? (
            <span data-no-task-card-drag="true" onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()}>
              <EstimatePicker
                value={task.estimate != null ? String(task.estimate) : ''}
                teamId={task.team_id}
                onChange={handleChangeEstimate}
                className={cn(pillBase, 'border-border bg-muted/50 text-muted-foreground hover:bg-muted cursor-pointer')}
              />
            </span>
          ) : task.estimate != null ? (
            <span className={cn(pillBase, 'border-border bg-muted/50 text-muted-foreground')}>
              {formatEstimateDisplay(task.estimate, task.team_id)}
            </span>
          ) : null)}
        </div>
        <span className="flex-1" />
        <div data-task-card-footer-owners="true" data-no-task-card-drag="true" className="flex min-w-0 items-center gap-1.5">
          {/* Assignee avatar / assign button */}
          {vis.assignee && (assignableMembers && workspaceId ? (
            <MultiMemberPickerPopover
              values={ownerMemberIds}
              members={assignableMembers}
              onChange={(nextOwnerIds) => {
                void handleAssignOwner(nextOwnerIds);
              }}
              align="end"
              triggerClassName="group shrink-0 overflow-visible rounded-full px-0 py-0 hover:bg-transparent focus-visible:ring-2 focus-visible:ring-primary/40"
              triggerLabel={ownerMemberIds.length > 0 ? undefined : 'Assign owner'}
              contentClassName="w-[220px]"
              renderTrigger={() => {
                return ownerMemberIds.length > 0 ? (
                  <OwnerAvatarStack
                    memberIds={ownerMemberIds}
                    nameMap={ownerNameMap}
                    members={assignableMembers}
                    size="sm"
                    max={3}
                    showSingleName={false}
                  />
                ) : (
                  <span className="flex h-7 w-7 items-center justify-center rounded-full border border-dashed border-border bg-muted/40 text-muted-foreground transition-colors group-hover:border-solid group-hover:bg-accent group-hover:text-foreground">
                    <UserAdd01Icon className="h-3.5 w-3.5" />
                  </span>
                );
              }}
            />
          ) : (
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="shrink-0">
                  {currentOwnerName ? (
                    <UserAvatar name={currentOwnerName} className="h-7 w-7" />
                  ) : (
                    <span className="flex h-7 w-7 items-center justify-center rounded-full border border-dashed border-border bg-muted/40 text-muted-foreground">
                      <UserAdd01Icon className="h-3.5 w-3.5" />
                    </span>
                  )}
                </span>
              </TooltipTrigger>
              <TooltipContent side="top">{currentOwnerName || 'Unassigned'}</TooltipContent>
            </Tooltip>
          ))}
        </div>
      </div>
      {/* Agent row: fixed placement for latest/relevant agent run state */}
      {shouldShowAgentRow && (
        <div
          data-task-card-agent-row="true"
          className="mt-2 flex min-w-0 items-center"
        >
          <Tooltip>
            <TooltipTrigger asChild>
              {isOverlay ? (
                <span
                  className={cn(
                    'flex h-7 min-w-0 flex-1 items-center gap-2 rounded-sm text-xs',
                    awaitingLabel
                      ? 'text-amber-700 dark:text-amber-300'
                      : task.latest_run_status === 'failed'
                        ? 'text-red-600 dark:text-red-400'
                        : task.latest_run_status === 'running'
                          ? 'text-emerald-700 dark:text-emerald-400'
                        : 'text-muted-foreground',
                  )}
                  aria-label={agentTooltipLabel}
                >
                  <TaskCardAgentBadge
                    agent={latestRunAgent}
                    isWorking={hasActiveRun}
                    latestRunStatus={task.latest_run_status}
                  />
                  <span data-task-card-agent-label="true" className="min-w-0 flex-1 truncate font-medium">
                    {agentStatusLabel}
                    {latestRunAgent?.name ? ` · ${latestRunAgent.name}` : ''}
                  </span>
                </span>
              ) : (
                <button
                  type="button"
                  onClick={handleAgentClick}
                  onPointerDown={(e) => e.stopPropagation()}
                  onMouseDown={(e) => e.stopPropagation()}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      handleAgentClick(e);
                    }
                  }}
                  className={cn(
                    'flex h-7 min-w-0 flex-1 items-center gap-2 rounded-sm text-left text-xs transition-colors hover:bg-muted/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40',
                    awaitingLabel
                      ? 'text-amber-700 dark:text-amber-300'
                      : task.latest_run_status === 'failed'
                        ? 'text-red-600 dark:text-red-400'
                        : task.latest_run_status === 'running'
                          ? 'text-emerald-700 dark:text-emerald-400'
                        : 'text-muted-foreground',
                  )}
                  aria-label={agentTooltipLabel}
                >
                  <TaskCardAgentBadge
                    agent={latestRunAgent}
                    isWorking={hasActiveRun}
                    latestRunStatus={task.latest_run_status}
                  />
                  <span data-task-card-agent-label="true" className="min-w-0 flex-1 truncate font-medium">
                    {agentStatusLabel}
                    {latestRunAgent?.name ? ` · ${latestRunAgent.name}` : ''}
                  </span>
                </button>
              )}
            </TooltipTrigger>
            <TooltipContent side="top">{agentTooltipLabel}</TooltipContent>
          </Tooltip>
        </div>
      )}
      </div>
    </article>
  );
}

function stringArrayEqual(prev: string[] | undefined, next: string[] | undefined) {
  if (prev === next) return true;
  if (!prev || !next) return prev === next;
  if (prev.length !== next.length) return false;
  return prev.every((value, index) => value === next[index]);
}

function labelsEqual(prev: Task['labels'], next: Task['labels']) {
  if (prev === next) return true;
  if (!prev || !next) return prev === next;
  if (prev.length !== next.length) return false;
  return prev.every((label, index) => {
    const nextLabel = next[index];
    return label.id === nextLabel.id
      && label.name === nextLabel.name
      && label.color === nextLabel.color
      && label.archived === nextLabel.archived;
  });
}

function dependencyTasksEqual(prev: Task['blocked_by_tasks'], next: Task['blocked_by_tasks']) {
  if (prev === next) return true;
  if (!prev || !next) return prev === next;
  if (prev.length !== next.length) return false;
  return prev.every((task, index) => {
    const nextTask = next[index];
    return task.id === nextTask.id
      && task.task_key === nextTask.task_key
      && task.name === nextTask.name
      && task.completed === nextTask.completed;
  });
}

function renderedTaskFieldsEqual(prev: Task, next: Task) {
  if (prev === next) return true;

  return prev.id === next.id
    && prev.updated_at === next.updated_at
    && prev.display_id === next.display_id
    && prev.task_key === next.task_key
    && prev.name === next.name
    && prev.task_type === next.task_type
    && prev.deadline === next.deadline
    && prev.completed === next.completed
    && prev.severity === next.severity
    && prev.priority === next.priority
    && prev.estimate === next.estimate
    && prev.team_id === next.team_id
    && stringArrayEqual(prev.owner_member_ids, next.owner_member_ids)
    && prev.blocked === next.blocked
    && prev.blocker === next.blocker
    && prev.blocked_by_count === next.blocked_by_count
    && dependencyTasksEqual(prev.blocked_by_tasks, next.blocked_by_tasks)
    && prev.recurring_template_id === next.recurring_template_id
    && prev.recurring_occurrence_number === next.recurring_occurrence_number
    && prev.state_color === next.state_color
    && prev.state_name === next.state_name
    && prev.state_type === next.state_type
    && prev.epic_name === next.epic_name
    && prev.sprint_name === next.sprint_name
    && labelsEqual(prev.labels, next.labels)
    && prev.latest_run_id === next.latest_run_id
    && prev.latest_run_agent_id === next.latest_run_agent_id
    && prev.latest_run_status === next.latest_run_status
    && prev.latest_run_pause_reason === next.latest_run_pause_reason
    && prev.latest_run_at === next.latest_run_at;
}

export const TaskCard = memo(TaskCardComponent, (prev, next) => {
  return renderedTaskFieldsEqual(prev.task, next.task)
    && prev.isOverlay === next.isOverlay
    && prev.teamName === next.teamName
    && prev.showStateBadge === next.showStateBadge;
});
TaskCard.displayName = 'TaskCard';
