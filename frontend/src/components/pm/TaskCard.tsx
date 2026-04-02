import { memo, useCallback, useContext, useMemo, useState } from 'react';
import { useSortable } from '@dnd-kit/sortable';
import {
  AlertTriangle,
  CalendarDays,
  Check,
  Layers,
  UserPlus,
} from 'lucide-react';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { differenceInDays, format, isBefore, parseISO, startOfDay } from 'date-fns';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { cn } from '@/lib/utils';
import { PRIORITY_BORDER_COLOR, PRIORITY_CONFIG, PriorityIcon, SEVERITY_CONFIG, SeverityIcon, SprintIcon, StateTypeIcon, TASK_TYPE_CONFIG, TaskTypeIcon } from '@/lib/pmConstants';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { RecurringTemplateBadge } from '@/components/pm/RecurringTemplateBadge';
import { UserAvatar } from './UserAvatar';
import { getSortableTaskCardStyle } from './TaskCard.sortable';
import type { Agent, Priority, Severity, Task } from '@/lib/pmTypes';
import type { AssignableMember } from '@/lib/types';
import { EstimatePicker, formatEstimateDisplay } from '@/components/pm/EstimatePicker';
import { LabelBadge } from '@/components/pm/LabelPicker';
import { useTeamFieldVisibilityForTeam } from '@/hooks/queries';
import { useBoardDisplayStore } from '@/stores/boardDisplayStore';
import { findAssignableMember } from '@/lib/assignableMembers';
import { BoardDataContext, BoardCallbacksContext } from './KanbanBoard.contexts';

// ── Shared constants ────────────────────────────────────────────────

const pillBase = 'flex h-5 items-center gap-1 rounded-sm border-[0.5px] px-2 text-[11px] font-medium';


const ALL_PRIORITIES: Priority[] = ['urgent', 'high', 'medium', 'low', 'none'];
const ALL_SEVERITIES: Severity[] = ['critical', 'major', 'minor', 'none'];

// ── Component ───────────────────────────────────────────────────────

interface TaskCardProps {
  task: Task;
  /** @deprecated Use BoardCallbacksContext instead. Kept for backward compat outside KanbanBoard. */
  onOpen?: (task: Task) => void;
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
  assignedAgent?: Pick<Agent, 'id' | 'name' | 'preset_key' | 'status'> | null;
}

const AGENT_OCTAGON_POINTS = '30,2 70,2 98,30 98,70 70,98 30,98 2,70 2,30';

function TaskCardAgentBadge({
  agent,
}: {
  agent?: Pick<Agent, 'id' | 'name' | 'preset_key' | 'status'> | null;
}) {
  const isWorking = agent?.status === 'working';

  return (
    <span className="relative block h-6 w-6 shrink-0">
      <svg
        viewBox="0 0 100 100"
        aria-hidden="true"
        className={cn(
          'absolute inset-0 h-full w-full overflow-visible',
          isWorking && 'motion-safe:animate-spin motion-safe:[animation-duration:2.4s]',
        )}
      >
        <polygon
          points={AGENT_OCTAGON_POINTS}
          fill="none"
          className={cn(
            isWorking
              ? 'stroke-foreground/80'
              : 'stroke-muted-foreground/45 dark:stroke-muted-foreground/70',
          )}
          strokeWidth={5}
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeDasharray={isWorking ? undefined : '6 7'}
        />
      </svg>
      <span
        className="absolute inset-[2px] overflow-hidden bg-background/95"
        style={{ clipPath: 'polygon(31% 4%, 69% 4%, 96% 31%, 96% 69%, 69% 96%, 31% 96%, 4% 69%, 4% 31%)' }}
      >
        <AgentAvatar
          agent={agent}
          className="h-full w-full rounded-none border-0 bg-transparent shadow-none"
        />
      </span>
    </span>
  );
}

function TaskCardComponent({
  task,
  onOpen: onOpenProp,
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
  assignedAgent: assignedAgentProp,
}: TaskCardProps) {
  // Consume board contexts (null when used outside KanbanBoard)
  const boardData = useContext(BoardDataContext);
  const callbacksRef = useContext(BoardCallbacksContext);

  // Resolve values: context first, then prop fallback
  const workspaceId = boardData?.workspaceId ?? workspaceIdProp;
  const assignableMembers = boardData?.assignableMembers ?? assignableMembersProp;
  const ownerNameMap = boardData?.ownerNameMap ?? ownerNameMapProp;
  const agentById = boardData?.agentById;
  const assignedAgent = assignedAgentProp ?? (agentById && task.assigned_agent_id ? agentById.get(task.assigned_agent_id) ?? null : null);
  const onOpen = callbacksRef?.current.onOpen ?? onOpenProp;
  const onTaskPatched = callbacksRef?.current.onTaskPatched;

  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: task.id });

  const style = getSortableTaskCardStyle({
    transform,
    transition,
    isDragging,
  });

  const [priorityOpen, setPriorityOpen] = useState(false);
  const [severityOpen, setSeverityOpen] = useState(false);
  const fieldVis = useTeamFieldVisibilityForTeam(workspaceId ?? '', task.team_id);
  const displayProps = useBoardDisplayStore((s) => s.properties);
  const vis = useMemo(() => ({
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
        return `Blocked by ${task.blocked_by_tasks[0].display_id}`;
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
  const currentOwnerName = useMemo(() => {
    const ownerKey = task.owner_member_id;
    if (!ownerKey) return null;
    return ownerNameMap?.get(ownerKey) ?? task.owner_name ?? null;
  }, [task.owner_member_id, task.owner_name, ownerNameMap]);
  const handleAssignOwner = useCallback(
    async (value: string) => {
      if (!workspaceId) return;
      const newOwnerId = value === '__none__' ? '' : value;
      try {
        const result = await pmTaskService.update(workspaceId, task.id, { owner_member_id: newOwnerId });
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

  const titleIsLong = task.name.length > 60;

  return (
    <article
      ref={setNodeRef}
      style={style}
      {...attributes}
      {...listeners}
      role="button"
      tabIndex={0}
      onClick={() => onOpen?.(task)}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onOpen?.(task);
        }
      }}
      className={cn(
        'group/card relative shrink-0 cursor-pointer rounded-lg border border-border/60 bg-background shadow-sm transition-all overflow-hidden',
        'hover:border-border hover:shadow-md',
        isDragging && 'opacity-50',
        isOverlay && 'ring-1 ring-primary/30 shadow-lg',
        showStateBadge && task.state_color && 'flex flex-row',
      )}
    >
      {/* State color accent bar (member board only) */}
      {showStateBadge && task.state_color && (
        <div className="w-0.5 shrink-0 self-stretch rounded-l-lg" style={{ backgroundColor: task.state_color }} />
      )}
      <div className="p-3 flex-1 min-w-0">
      {/* Row 1: Task type + Epic + Team + Priority */}
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        {vis.task_type && (
        <Tooltip>
          <TooltipTrigger asChild>
            <span className="shrink-0">
              <TaskTypeIcon taskType={task.task_type} className="h-3.5 w-3.5" />
            </span>
          </TooltipTrigger>
          <TooltipContent side="top">{taskTypeCfg.label}</TooltipContent>
        </Tooltip>
        )}
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

        {/* Priority pill — clickable dropdown (hidden when 'none') */}
        {vis.priority && task.priority !== 'none' && (workspaceId ? (
          <Popover open={priorityOpen} onOpenChange={setPriorityOpen}>
            <Tooltip open={priorityOpen ? false : undefined}>
              <TooltipTrigger asChild>
                <PopoverTrigger asChild>
                  <button
                    type="button"
                    className={cn(
                      'flex h-5 shrink-0 items-center rounded-sm border-[0.5px] bg-muted/50 px-1 transition-colors hover:bg-muted',
                      PRIORITY_BORDER_COLOR[task.priority],
                    )}
                    onClick={(e) => { e.stopPropagation(); setPriorityOpen(true); }}
                  >
                    <PriorityIcon priority={task.priority} className="h-3.5 w-3.5" />
                  </button>
                </PopoverTrigger>
              </TooltipTrigger>
              <TooltipContent side="top">Priority: {priorityCfg.label}</TooltipContent>
            </Tooltip>
            {priorityOpen && (
              <PopoverContent
                className="w-[180px] p-0"
                align="end"
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
                            {task.priority === p && <Check className="ml-auto h-3.5 w-3.5 text-primary" />}
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
                'flex h-5 shrink-0 items-center rounded-sm border-[0.5px] bg-muted/50 px-1',
                PRIORITY_BORDER_COLOR[task.priority],
              )}>
                <PriorityIcon priority={task.priority} className="h-3.5 w-3.5" />
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">Priority: {priorityCfg.label}</TooltipContent>
          </Tooltip>
        ))}
      </div>

      {/* Row 2: Title */}
      {task.recurring_template_id ? (
        <div className="mt-3">
          <RecurringTemplateBadge compact occurrenceNumber={task.recurring_occurrence_number} />
        </div>
      ) : null}
      {titleIsLong ? (
        <Tooltip>
          <TooltipTrigger asChild>
            <h4 className="mb-3.5 mt-2 line-clamp-2 text-sm font-medium leading-snug text-foreground">
              {task.name}
            </h4>
          </TooltipTrigger>
          <TooltipContent side="bottom" className="max-w-[300px]">{task.name}</TooltipContent>
        </Tooltip>
      ) : (
        <h4 className="mb-3.5 mt-2 line-clamp-2 text-sm font-medium leading-snug text-foreground">
          {task.name}
        </h4>
      )}

      {/* Epic row */}
      {vis.epic && task.epic_name && (
        <div className="mt-1.5 flex items-center gap-1.5 text-xs text-muted-foreground">
          <Layers className="h-3 w-3 shrink-0" />
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
                            {task.severity === sev && <Check className="ml-auto h-3.5 w-3.5 text-primary" />}
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
                <AlertTriangle className="h-3 w-3" />
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

      {/* Row 4: Footer - due date, estimate + assignee */}
      <div className="mt-2 flex items-center gap-1.5">
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
                <CalendarDays className="h-3 w-3 shrink-0" />
                {due.label}
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">
              {due.overdue ? 'Overdue' : due.approaching ? 'Due soon' : 'Due date'}: {due.label}
            </TooltipContent>
          </Tooltip>
        )}
        {vis.estimate && task.estimate != null && (workspaceId ? (
          <span onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()}>
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
        <span className="flex-1" />
        <div className="flex items-center gap-1.5">
          {vis.agent && task.assigned_agent_id && (
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="shrink-0">
                  <TaskCardAgentBadge agent={assignedAgent} />
                </span>
              </TooltipTrigger>
              <TooltipContent side="top">{assignedAgent?.name ?? 'Agent assigned'}</TooltipContent>
            </Tooltip>
          )}
          {/* Assignee avatar / assign button */}
          {vis.assignee && (assignableMembers && workspaceId ? (
            <MemberPickerPopover
              value={task.owner_member_id || '__none__'}
              members={assignableMembers}
              noneLabel="Unassigned"
              onChange={(value) => {
                void handleAssignOwner(value);
              }}
              align="end"
              triggerClassName="shrink-0 rounded-full transition-opacity hover:opacity-80"
              contentClassName="w-[220px]"
              renderTrigger={() => {
                const selectedMember = findAssignableMember(assignableMembers, task.owner_member_id);
                return selectedMember ? (
                  <UserAvatar
                    name={selectedMember.display_name || selectedMember.email}
                    avatarUrl={selectedMember.avatar_url}
                    className="h-5 w-5"
                  />
                ) : (
                  <span className="flex h-5 w-5 items-center justify-center rounded-full border border-dashed border-border bg-muted/40 text-muted-foreground transition-colors hover:border-primary/40 hover:text-primary">
                    <UserPlus className="h-2.5 w-2.5" />
                  </span>
                );
              }}
            />
          ) : (
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="shrink-0">
                  {currentOwnerName ? (
                    <UserAvatar name={currentOwnerName} className="h-5 w-5" />
                  ) : (
                    <span className="flex h-5 w-5 items-center justify-center rounded-full border border-dashed border-border bg-muted/40 text-muted-foreground">
                      <UserPlus className="h-2.5 w-2.5" />
                    </span>
                  )}
                </span>
              </TooltipTrigger>
              <TooltipContent side="top">{currentOwnerName || 'Unassigned'}</TooltipContent>
            </Tooltip>
          ))}
        </div>
      </div>
      </div>
    </article>
  );
}

export const TaskCard = memo(TaskCardComponent, (prev, next) => {
  // Fast path: same object reference means no change
  if (prev.task !== next.task) {
    // Different reference — check if the task actually changed
    if (prev.task.id !== next.task.id || prev.task.updated_at !== next.task.updated_at) return false;
  }
  return prev.isOverlay === next.isOverlay
    && prev.teamName === next.teamName
    && prev.showStateBadge === next.showStateBadge
    && prev.assignedAgent === next.assignedAgent;
});
TaskCard.displayName = 'TaskCard';
