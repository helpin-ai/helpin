import { TaskListGroupingDropdown } from './TaskListGroupingDropdown';
import { memo, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useNavigate } from '@tanstack/react-router';
import {
  DndContext,
  DragOverlay,
  MeasuringStrategy,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragOverEvent,
  type DragStartEvent,
  useDroppable,
} from '@dnd-kit/core';
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { BotIcon, CollapseIcon, ExpandIcon, LayoutTwoColumnIcon, LayoutTable01Icon, Loading01Icon, PlusSignIcon, UserIcon } from '@/lib/icons';
import { ChartColumnIcon, StickyNote01Icon } from '@/lib/pmIcons';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { usePMBoardStore, type BoardFilters } from '@/stores/pmBoardStore';
import type { CreateTaskRequest, Task, TaskMemberColumn, TaskStateColumn } from '@/lib/pmTypes';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { StateTypeIcon } from '@/lib/pmConstants';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { useAuthStore } from '@/stores/authStore';
import { useAgents, useSession, useAutomationRulesByWorkflow, useTeamFieldVisibilityForTeam, useLabels, useEpics, useSprints } from '@/hooks/queries';
import { UserAvatar } from './UserAvatar';
import { TaskCard } from './TaskCard';
import { CreateTaskModal } from './CreateTaskModal';
import { TaskFilterProvider, TaskFilterTrigger, TaskFilterBar, TaskOwnerAvatarFilterRow } from './TaskFilters';
import { TaskListView } from './TaskListView';
import { ViewBar } from './ViewBar';
import { BoardDisplayMenu } from './BoardDisplayMenu';
import { ListDisplayMenu } from './ListDisplayMenu';
import { BoardToolbarSlot } from './BoardToolbarSlot';
import { useBoardDisplayStore, type DisplayPropertyKey } from '@/stores/boardDisplayStore';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
import { createPMDnDTraceID } from '@/lib/pmDnDDebug';
import { DragPreviewManager, PM_BOARD_DRAG_ACTIVATION_DISTANCE, PM_BOARD_DRAG_OVER_THROTTLE_MS, useActiveTask, useColumnDragPreview, useColumnDropPlaceholderIndex, useDropPlaceholderHeight, commitDropBeforeClearingPreview, getBaseDragSourceColumnId, getDragStartTaskRect, getPMBoardScrollContainerClassName, getStateColumnTaskGroupsForRender, getTaskDropPlaceholderId, getTaskDropPlaceholderPreview, hasDragPreviewChanged, resolveBoardDropTarget } from './KanbanBoard.dnd';
import { BoardDataContext, BoardCallbacksContext, DragPreviewContext } from './KanbanBoard.contexts';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { ACTIVE_RUN_STATUSES } from '@/components/pm/agentRunConstants';
import { getVisibleTaskListGroupOptions, type TaskListGroupByOption } from '@/components/pm/task-detail/taskListGrouping';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { QuietPageHeader, QuietPrimaryAction } from '@/components/design-system/quiet';
import { toast } from 'sonner';
import {
  getAgentAutoRunStateChangeMessage,
  getAgentAutoRunStateChangeToastId,
  shouldNotifyAgentAutoRunStateChange,
} from '@/components/pm/agentAutoRunNotification';

// ── Helpers ─────────────────────────────────────────────────────────

interface KanbanBoardProps {
  workspaceId: string;
  teamId?: string;
  initialFilters?: BoardFilters;
}

interface ColumnProps {
  column: TaskStateColumn;
  collapsed: boolean;
  isLoadingMore: boolean;
}

const PM_BOARD_DND_MEASURING = {
  droppable: {
    strategy: MeasuringStrategy.Always,
  },
};

function TaskDropPlaceholder({ columnId, index, height }: { columnId: string; index: number; height: number }) {
  const { setNodeRef } = useDroppable({ id: getTaskDropPlaceholderId(columnId, index),
  });

  return <div
      ref={setNodeRef}
      data-pm-board-drop-placeholder="true"
      style={{ height }}
      className="shrink-0 rounded-lg border border-dashed border-sky-500/40 bg-sky-500/10 shadow-inner dark:border-sky-400/35 dark:bg-sky-400/10"
    />;
}

function renderTaskCardsWithPlaceholder({
  columnId,
  tasks,
  placeholderIndex,
  placeholderHeight,
  renderTask }: {
  columnId: string;
  tasks: Task[];
  placeholderIndex: number | null;
  placeholderHeight: number;
  renderTask: (task: Task) => ReactNode }) {
  const boundedIndex = placeholderIndex == null
    ? null
    : Math.max(0, Math.min(placeholderIndex, tasks.length));
  const rendered: ReactNode[] = [];
  for (let index = 0; index <= tasks.length; index++) {
    if (boundedIndex === index) {
      rendered.push(
        <TaskDropPlaceholder
          key="__pm-board-drop-placeholder"
          columnId={columnId}
          index={index}
          height={placeholderHeight}
        />);
    }
    if (index < tasks.length) {
      rendered.push(renderTask(tasks[index]!));
    }
  }
  return rendered;
}

function AutomatedStateIndicator({ compact = false }: { compact?: boolean }) {
  const label = 'Tasks moved here automatically start an agent run';

  if (compact) {
    return (
      <QuickTooltip label={label}>
        <span
          aria-label={label}
          className="mb-1 inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md border border-emerald-300/70 bg-emerald-50 text-emerald-700 shadow-sm dark:border-emerald-700/70 dark:bg-emerald-950/50 dark:text-emerald-300"
          tabIndex={0}
        >
          <BotIcon className="h-3.5 w-3.5" />
        </span>
      </QuickTooltip>
    );
  }

  return (
    <QuickTooltip label={label}>
      <span
        aria-label={label}
        className="inline-flex h-5 shrink-0 items-center gap-1 rounded-md border border-emerald-300/70 bg-emerald-50 px-1.5 text-[10px] font-semibold leading-none text-emerald-700 shadow-sm dark:border-emerald-700/70 dark:bg-emerald-950/50 dark:text-emerald-300"
        tabIndex={0}
      >
        <BotIcon className="h-3 w-3" />
        Auto-run
      </span>
    </QuickTooltip>
  );
}

const Column = memo(function Column({ column, collapsed, isLoadingMore }: ColumnProps) {
  const { automatedStateIds, findTeamName } = useContext(BoardDataContext)!;
  const callbacksRef = useContext(BoardCallbacksContext)!;
  const dragManager = useContext(DragPreviewContext)!;

  // Subscribe to drag preview for this column only
  const tasks = useColumnDragPreview(dragManager, column.state.id, column.tasks);
  const dropPlaceholderIndex = useColumnDropPlaceholderIndex(dragManager, column.state.id, tasks.length);
  const dropPlaceholderHeight = useDropPlaceholderHeight(dragManager);
  const hasPreviewOverride = tasks !== column.tasks;

  const { setNodeRef, isOver } = useDroppable({ id: column.state.id });
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const loadMoreRef = useRef<HTMLDivElement | null>(null);
  const groupedTasks = getStateColumnTaskGroupsForRender({
    stateType: column.state.state_type,
    hasPreviewOverride,
    taskGroups: column.task_groups,
  });

  // Memoize sortable items from preview tasks
  const sortableItems = useMemo(() => tasks.map((s) => s.id), [tasks]);

  useEffect(() => {
    if (!column.has_more || isLoadingMore || !scrollRef.current || !loadMoreRef.current) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          callbacksRef.current.onLoadMore(column.state.id);
        }
      },
      {
        root: scrollRef.current,
        rootMargin: '0px 0px 160px 0px',
      },
    );

    observer.observe(loadMoreRef.current);
    return () => observer.disconnect();
  }, [column.has_more, column.state.id, column.tasks.length, isLoadingMore, callbacksRef]);

  if (collapsed) {
    return (
      <QuickTooltip label={`Expand ${column.state.name}`}>
        <section ref={setNodeRef} className={cn('relative flex h-full w-[44px] shrink-0 cursor-pointer flex-col items-center rounded-md border border-border/50 bg-muted/30 transition-colors hover:bg-muted/50', isOver && 'bg-accent ring-1 ring-inset ring-border')} onClick={() => callbacksRef.current.onToggleCollapse(column.state.id)}>
          {column.state.color && <div className="absolute top-0 left-2 right-2 h-[3px] rounded-b-full" style={{ backgroundColor: column.state.color }} />}
          <ExpandIcon className="mt-3 mb-2 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          <StateTypeIcon stateType={column.state.state_type} className="mb-2 h-4 w-4 shrink-0" />
          {automatedStateIds?.has(column.state.id) && <AutomatedStateIndicator compact />}
          <span className="text-xs font-medium text-muted-foreground">{column.task_count}</span>
          <div className="mt-3 flex flex-1 items-start">
            <span className="text-xs font-semibold whitespace-nowrap" style={{ writingMode: 'vertical-rl', textOrientation: 'mixed' }}>
              {column.state.name}
            </span>
          </div>
        </section>
      </QuickTooltip>
    );
  }

  return (
    <section className="flex h-full w-[300px] shrink-0 flex-col">
      <header className="group/header flex items-center justify-between px-3 pt-4 pb-3 relative">
        {column.state.color && <div className="absolute top-0 left-3 right-3 h-[3px] rounded-b-full" style={{ backgroundColor: column.state.color }} />}
        <div className="min-w-0">
          {column.state.description ? (
            <div className="flex min-w-0 items-center gap-1.5 text-sm font-semibold">
              <StateTypeIcon stateType={column.state.state_type} className="h-4 w-4 shrink-0" />
              <QuickTooltip label={column.state.description}>
                <span className="min-w-0 truncate cursor-default">{column.state.name}</span>
              </QuickTooltip>
              {automatedStateIds?.has(column.state.id) && <AutomatedStateIndicator />}
            </div>
          ) : (
            <div className="flex min-w-0 items-center gap-1.5 text-sm font-semibold">
              <StateTypeIcon stateType={column.state.state_type} className="h-4 w-4 shrink-0" />
              <span className="min-w-0 truncate">{column.state.name}</span>
              {automatedStateIds?.has(column.state.id) && <AutomatedStateIndicator />}
            </div>
          )}
          <p className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
            <QuickTooltip label={`${column.task_count} ${column.task_count === 1 ? 'task' : 'tasks'}`}>
              <span className="inline-flex items-center gap-1.5">
                <StickyNote01Icon className="h-3 w-3" />
                {column.task_count}
              </span>
            </QuickTooltip>
            <QuickTooltip label={`${column.point_total} estimate ${column.point_total === 1 ? 'point' : 'points'}`}>
              <span className="inline-flex items-center gap-1.5">
                <ChartColumnIcon className="h-3 w-3" />
                {column.point_total}
              </span>
            </QuickTooltip>
          </p>
        </div>
        <div className="flex items-center gap-0.5">
          <QuickTooltip label="Collapse column">
            <Button variant="ghost" size="icon" className="h-7 w-7 opacity-0 transition-opacity group-hover/header:opacity-100" onClick={() => callbacksRef.current.onToggleCollapse(column.state.id)}>
              <CollapseIcon className="h-3.5 w-3.5" />
            </Button>
          </QuickTooltip>
          <QuickTooltip label="Create task">
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => callbacksRef.current.onCreate(column.state.id)}>
              <PlusSignIcon className="h-4 w-4" />
            </Button>
          </QuickTooltip>
        </div>
      </header>

      <SortableContext items={sortableItems} strategy={verticalListSortingStrategy}>
        <div
          ref={(node) => {
            setNodeRef(node);
            scrollRef.current = node;
          }}
          className={getPMBoardScrollContainerClassName({
            variant: 'state',
            isOver,
          })}
        >
          {groupedTasks.length > 0
            ? groupedTasks.map((group) => (
                <div key={group.key} className="space-y-2">
                  <div className="rounded-md bg-muted px-3 py-1 text-center text-xs font-semibold text-muted-foreground">{group.label}</div>
                  {group.tasks.map((task) => (
                    <TaskCard key={task.id} task={task} teamName={findTeamName(task.team_id)} />
                  ))}
                </div>
              ))
            : renderTaskCardsWithPlaceholder({
                columnId: column.state.id,
                tasks,
                placeholderIndex: dropPlaceholderIndex,
                placeholderHeight: dropPlaceholderHeight,
                renderTask: (task) => <TaskCard key={task.id} task={task} teamName={findTeamName(task.team_id)} />,
              })}

          {column.has_more ? (
            <div ref={loadMoreRef} className="flex h-8 items-center justify-center text-xs text-muted-foreground">
              {isLoadingMore ? (
                <>
                  <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                  Loading more...
                </>
              ) : (
                <span>{column.task_count - column.tasks.length} remaining</span>
              )}
            </div>
          ) : null}

          <Button variant="ghost" className="w-full justify-center text-xs text-muted-foreground" onClick={() => callbacksRef.current.onCreate(column.state.id)}>
            <PlusSignIcon className="h-3.5 w-3.5" />
            Add task
          </Button>
        </div>
      </SortableContext>
    </section>
  );
});
Column.displayName = 'Column';

interface MemberColumnProps {
  column: TaskMemberColumn;
  collapsed: boolean;
  isLoadingMore: boolean;
}

const MemberColumn = memo(function MemberColumn({ column, collapsed, isLoadingMore }: MemberColumnProps) {
  const { findTeamName } = useContext(BoardDataContext)!;
  const callbacksRef = useContext(BoardCallbacksContext)!;
  const dragManager = useContext(DragPreviewContext)!;

  const colKey = column.member?.id ?? '__unassigned__';

  // Subscribe to drag preview for this column only
  const tasks = useColumnDragPreview(dragManager, colKey, column.tasks);
  const dropPlaceholderIndex = useColumnDropPlaceholderIndex(dragManager, colKey, tasks.length);
  const dropPlaceholderHeight = useDropPlaceholderHeight(dragManager);

  const { setNodeRef, isOver } = useDroppable({ id: colKey });
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const loadMoreRef = useRef<HTMLDivElement | null>(null);
  const displayName = column.member?.display_name ?? 'Unassigned';

  // Memoize sortable items from preview tasks
  const sortableItems = useMemo(() => tasks.map((s) => s.id), [tasks]);

  useEffect(() => {
    if (!column.has_more || isLoadingMore || !scrollRef.current || !loadMoreRef.current) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          callbacksRef.current.onLoadMoreMember(column.member?.id ?? null);
        }
      },
      { root: scrollRef.current, rootMargin: '0px 0px 160px 0px' },
    );
    observer.observe(loadMoreRef.current);
    return () => observer.disconnect();
  }, [column.has_more, column.member?.id, column.tasks.length, isLoadingMore, callbacksRef]);

  if (collapsed) {
    return (
      <QuickTooltip label={`Expand ${displayName}`}>
        <section ref={setNodeRef} className={cn('relative flex h-full w-[44px] shrink-0 cursor-pointer flex-col items-center rounded-md border border-border/50 bg-muted/30 pt-3 transition-colors hover:bg-muted/50', isOver && 'bg-accent ring-1 ring-inset ring-border')} onClick={() => callbacksRef.current.onToggleCollapse(colKey)}>
          <ExpandIcon className="mt-3 mb-2 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          {column.member ? <UserAvatar name={displayName} avatarUrl={column.member.avatar_url} avatarStyle={column.member.avatar_style} avatarSeed={column.member.avatar_seed} avatarBackgroundMode={column.member.avatar_background_mode} avatarBackgroundColor={column.member.avatar_background_color} className="h-5 w-5 text-[10px]" /> : <UserIcon className="h-4 w-4 shrink-0 text-muted-foreground" />}
          <span className="mt-2 text-xs font-medium text-muted-foreground">{column.task_count}</span>
          <div className="mt-3 flex flex-1 items-start">
            <span className="text-xs font-semibold whitespace-nowrap" style={{ writingMode: 'vertical-rl', textOrientation: 'mixed' }}>
              {displayName}
            </span>
          </div>
        </section>
      </QuickTooltip>
    );
  }

  return (
    <section className="flex h-full w-[300px] shrink-0 flex-col">
      <header className="group/header flex items-center justify-between px-3 pt-4 pb-3">
        <div className="min-w-0">
          <p className="flex items-center gap-1.5 truncate text-sm font-semibold">
            {column.member ? <UserAvatar name={displayName} avatarUrl={column.member.avatar_url} avatarStyle={column.member.avatar_style} avatarSeed={column.member.avatar_seed} avatarBackgroundMode={column.member.avatar_background_mode} avatarBackgroundColor={column.member.avatar_background_color} className="h-5 w-5 text-[10px]" /> : <UserIcon className="h-4 w-4 shrink-0 text-muted-foreground" />}
            {displayName}
          </p>
          <p className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
            <QuickTooltip label={`${column.task_count} ${column.task_count === 1 ? 'task' : 'tasks'}`}>
              <span className="inline-flex items-center gap-1.5">
                <StickyNote01Icon className="h-3 w-3" />
                {column.task_count}
              </span>
            </QuickTooltip>
            <QuickTooltip label={`${column.point_total} estimate ${column.point_total === 1 ? 'point' : 'points'}`}>
              <span className="inline-flex items-center gap-1.5">
                <ChartColumnIcon className="h-3 w-3" />
                {column.point_total}
              </span>
            </QuickTooltip>
          </p>
        </div>
        <div className="flex items-center gap-0.5">
          <QuickTooltip label="Collapse column">
            <Button variant="ghost" size="icon" className="h-7 w-7 opacity-0 group-hover/header:opacity-100 transition-opacity" onClick={() => callbacksRef.current.onToggleCollapse(colKey)}>
              <CollapseIcon className="h-3.5 w-3.5" />
            </Button>
          </QuickTooltip>
          <QuickTooltip label="Create task">
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => callbacksRef.current.onCreateForMember(column.member?.id ?? null)}>
              <PlusSignIcon className="h-4 w-4" />
            </Button>
          </QuickTooltip>
        </div>
      </header>

      <SortableContext items={sortableItems} strategy={verticalListSortingStrategy}>
        <div
          ref={(node) => {
            setNodeRef(node);
            scrollRef.current = node;
          }}
          className={getPMBoardScrollContainerClassName({
            variant: 'member',
            isOver,
          })}
        >
          {renderTaskCardsWithPlaceholder({
            columnId: colKey,
            tasks,
            placeholderIndex: dropPlaceholderIndex,
            placeholderHeight: dropPlaceholderHeight,
            renderTask: (task) => <TaskCard key={task.id} task={task} teamName={findTeamName(task.team_id)} showStateBadge />,
          })}

          {column.has_more ? (
            <div ref={loadMoreRef} className="flex h-8 items-center justify-center text-xs text-muted-foreground">
              {isLoadingMore ? (
                <>
                  <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                  Loading more...
                </>
              ) : (
                <span>{column.task_count - column.tasks.length} remaining</span>
              )}
            </div>
          ) : null}

          <Button variant="ghost" className="w-full justify-center text-xs text-muted-foreground" onClick={() => callbacksRef.current.onCreateForMember(column.member?.id ?? null)}>
            <PlusSignIcon className="h-3.5 w-3.5" />
            Add task
          </Button>
        </div>
      </SortableContext>
    </section>
  );
});
MemberColumn.displayName = 'MemberColumn';

const DragOverlayCard = memo(function DragOverlayCard({ manager, resolveTeamName, groupBy }: { manager: DragPreviewManager; resolveTeamName: (id?: string) => string | undefined; groupBy: string }) {
  const activeTask = useActiveTask(manager);
  if (!activeTask) return null;
  return <TaskCard task={activeTask} isOverlay teamName={resolveTeamName(activeTask.team_id)} showStateBadge={groupBy === 'members'} />;
});
DragOverlayCard.displayName = 'DragOverlayCard';

export function KanbanBoard({ workspaceId, teamId, initialFilters }: KanbanBoardProps) {
  const navigate = useNavigate();
  const workspaceSlug = useWorkspaceStore((s) => s.currentWorkspace?.slug ?? '');
  const workflow = usePMBoardStore((state) => state.workflow);
  const columns = usePMBoardStore((state) => state.columns);
  const loading = usePMBoardStore((state) => state.loading);
  const error = usePMBoardStore((state) => state.error);
  const storeTeamId = usePMBoardStore((state) => state.teamId);
  const filters = usePMBoardStore((state) => state.filters);
  const columnLoading = usePMBoardStore((state) => state.columnLoading);
  const loadBoard = usePMBoardStore((state) => state.loadBoard);
  const setTeamFilter = usePMBoardStore((state) => state.setTeamFilter);
  const setFilters = usePMBoardStore((state) => state.setFilters);
  const createTask = usePMBoardStore((state) => state.createTask);
  const moveTask = usePMBoardStore((state) => state.moveTask);
  const patchTask = usePMBoardStore((state) => state.patchTask);
  const refreshBoard = usePMBoardStore((state) => state.refreshBoard);
  const loadMoreColumn = usePMBoardStore((state) => state.loadMoreColumn);
  const loadViews = usePMBoardStore((state) => state.loadViews);
  const memberColumns = usePMBoardStore((state) => state.memberColumns);
  const memberColumnLoading = usePMBoardStore((state) => state.memberColumnLoading);
  const loadMemberBoard = usePMBoardStore((state) => state.loadMemberBoard);
  const loadMoreMemberColumn = usePMBoardStore((state) => state.loadMoreMemberColumn);
  const moveMemberTask = usePMBoardStore((state) => state.moveMemberTask);
  const allWorkflows = usePMBoardStore((state) => state.workflows);

  const currentUser = useAuthStore((s) => s.user);
  const { data: sessionMembership } = useSession(workspaceId);
  const currentMemberId = sessionMembership?.id;
  const { teams, findTeamName } = useAccessibleTeams(workspaceId);
  const teamLabel = teamId ? findTeamName(teamId) : null;
  const { userMemberships } = useWorkspaceTeams(workspaceId);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const listFieldVis = useTeamFieldVisibilityForTeam(workspaceId, storeTeamId);
  const ownerNameMap = useMemo(() => buildAssignableMemberNameMap(assignableMembers), [assignableMembers]);
  const showEmptyColumns = useBoardDisplayStore((s) => s.showEmptyColumns);
  const groupBy = useBoardDisplayStore((s) => s.groupBy);
  const setGroupBy = useBoardDisplayStore((s) => s.setGroupBy);
  const initDisplay = useBoardDisplayStore((s) => s.init);
  const initialFiltersKey = useMemo(() => JSON.stringify(initialFilters ?? {}), [initialFilters]);
  const appliedInitialFiltersRef = useRef<string | null>(null);

  useEffect(() => {
    initDisplay(workspaceId);
  }, [workspaceId, initDisplay]);

  // Fetch automation rules to show bot icons on columns with direct agent-run automation.
  const { data: automationRules } = useAutomationRulesByWorkflow(workspaceId, workflow?.workflow.id);
  const automatedStateIds = useMemo(() => {
    const ids = new Set<string>();
    if (!automationRules) return ids;
    for (const rule of automationRules) {
      if (rule.enabled && rule.trigger_type === 'task.state_entered' && rule.action_type === 'start_agent_run') {
        const stateId = rule.trigger_config?.state_id;
        if (stateId) ids.add(stateId);
      }
    }
    return ids;
  }, [automationRules]);

  const notifyAgentAutoRunStateChange = useCallback(
    (fromStateId: string | null | undefined, toStateId: string | null | undefined) => {
      if (
        !shouldNotifyAgentAutoRunStateChange({
          fromStateId,
          toStateId,
          automatedStateIds,
        })
      )
        return;
      if (!toStateId) return;
      const stateName = columns.find((column) => column.state.id === toStateId)?.state.name ?? 'this state';
      toast.info(getAgentAutoRunStateChangeMessage(stateName), {
        id: getAgentAutoRunStateChangeToastId(toStateId),
      });
    },
    [automatedStateIds, columns],
  );

  // Load member board when groupBy switches to 'members'
  const activeMemberIds = useMemo(() => assignableMembers.filter((m) => m.status === 'active').map((m) => m.id), [assignableMembers]);
  useEffect(() => {
    if (groupBy === 'members' && workflow) {
      loadMemberBoard(showEmptyColumns ? activeMemberIds : undefined, showEmptyColumns);
    }
  }, [groupBy, workflow, showEmptyColumns, activeMemberIds, loadMemberBoard]);

  // Sync URL team param → store on mount / prop change.
  // No team in the URL means "all work", not "first team".
  useEffect(() => {
    setTeamFilter(teamId ?? null);
  }, [teamId, setTeamFilter]);

  useEffect(() => {
    if (!initialFilters || Object.keys(initialFilters).length === 0) return;
    if (appliedInitialFiltersRef.current === initialFiltersKey) return;

    appliedInitialFiltersRef.current = initialFiltersKey;
    void setFilters(initialFilters);
  }, [initialFilters, initialFiltersKey, setFilters]);

  const { data: refLabels = [] } = useLabels(workspaceId);
  const { data: refEpics = [] } = useEpics(workspaceId);
  const { data: refSprints = [] } = useSprints(workspaceId);
  const { data: agents = [] } = useAgents(workspaceId);
  const agentById = useMemo(() => new Map(agents.map((agent) => [agent.id, agent])), [agents]);

  const dragManager = useRef(new DragPreviewManager()).current;
  const isDragging = useActiveTask(dragManager) !== null;
  const lastDragOverTime = useRef(0);
  const activeDragTraceID = useRef<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [createStateId, setCreateStateId] = useState<string>('');
  const [createOwnerMemberId, setCreateOwnerMemberId] = useState<string | undefined>(undefined);
  const [seedConfirmOpen, setSeedConfirmOpen] = useState(false);
  const [isSeeding, setIsSeeding] = useState(false);
  const showSeedButton = import.meta.env.DEV;
  const VIEW_MODE_KEY = `pm_view_mode_${workspaceId}`;
  const [bulkTriggerSlot, setBulkTriggerSlot] = useState<HTMLSpanElement | null>(null);
  const [viewMode, setViewModeState] = useState<'board' | 'list'>(() => {
    try {
      const saved = localStorage.getItem(VIEW_MODE_KEY);
      return saved === 'list' ? 'list' : 'board';
    } catch {
      return 'board';
    }
  });
  const setViewMode = useCallback(
    (mode: 'board' | 'list') => {
      setViewModeState(mode);
      try {
        localStorage.setItem(VIEW_MODE_KEY, mode);
      } catch {}
    },
    [VIEW_MODE_KEY],
  );
  const [listGroupBy, setListGroupBy] = useState<TaskListGroupByOption>('workflow_state');

  const COLLAPSED_KEY = `pm_kanban_collapsed_${workspaceId}`;
  const [collapsedColumns, setCollapsedColumnsState] = useState<Set<string>>(() => {
    try {
      const saved = localStorage.getItem(COLLAPSED_KEY);
      return saved ? new Set(JSON.parse(saved) as string[]) : new Set();
    } catch {
      return new Set();
    }
  });
  const toggleCollapse = useCallback(
    (stateId: string) => {
      setCollapsedColumnsState((prev) => {
        const next = new Set(prev);
        if (next.has(stateId)) next.delete(stateId);
        else next.add(stateId);
        try {
          localStorage.setItem(COLLAPSED_KEY, JSON.stringify([...next]));
        } catch {}
        return next;
      });
    },
    [COLLAPSED_KEY],
  );

  const listGroupOptions = useMemo(
    () =>
      getVisibleTaskListGroupOptions({
        task_type: listFieldVis.task_type,
        priority: listFieldVis.priority,
        severity: listFieldVis.severity,
        epic: listFieldVis.epic,
        sprint: listFieldVis.sprint,
      }),
    [listFieldVis.epic, listFieldVis.priority, listFieldVis.severity, listFieldVis.sprint, listFieldVis.task_type],
  );

  const listDisabledKeys = useMemo(() => {
    const keys = new Set<DisplayPropertyKey>();
    if (!listFieldVis.priority) keys.add('priority');
    if (!listFieldVis.severity) keys.add('severity');
    if (!listFieldVis.task_type) keys.add('task_type');
    if (!listFieldVis.estimate) keys.add('estimate');
    if (!listFieldVis.epic) keys.add('epic');
    if (!listFieldVis.sprint) keys.add('sprint');
    if (!listFieldVis.due_date) keys.add('due_date');
    if (!listFieldVis.labels) keys.add('labels');
    if (teamId) keys.add('team');
    return keys;
  }, [listFieldVis, teamId]);

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: PM_BOARD_DRAG_ACTIVATION_DISTANCE },
    }),
  );

  useEffect(() => {
    loadBoard(workspaceId);
  }, [workspaceId, loadBoard]);

  // Load views once board and user are ready
  useEffect(() => {
    if (currentMemberId) {
      loadViews(workspaceId, currentMemberId);
    }
  }, [workspaceId, currentMemberId, loadViews]);

  // Refresh board when a task is created via the global modal
  useEffect(() => {
    const handler = () => {
      if (groupBy === 'members') {
        loadMemberBoard(showEmptyColumns ? activeMemberIds : undefined, showEmptyColumns);
      } else {
        refreshBoard();
      }
    };
    window.addEventListener('task-created', handler);
    return () => window.removeEventListener('task-created', handler);
  }, [refreshBoard, groupBy, loadMemberBoard, showEmptyColumns, activeMemberIds]);

  const openTask = useCallback(
    (task: Task) => {
      if (!workspaceSlug) return;
      openTaskRoute(navigate as never, { pathname: window.location.pathname } as never, workspaceSlug, task.id);
    },
    [navigate, workspaceSlug],
  );
  const openAgentRun = useCallback(
    (task: Task) => {
      if (!workspaceSlug) return;
      const hasActiveRun = !!task.latest_run_id && !!task.latest_run_status && ACTIVE_RUN_STATUSES.has(task.latest_run_status);
      openTaskRoute(navigate as never, { pathname: window.location.pathname } as never, workspaceSlug, task.id, hasActiveRun ? { run: task.latest_run_id! } : undefined);
    },
    [navigate, workspaceSlug],
  );
  const resolveTeamName = useCallback(
    (taskTeamId: string | undefined) => {
      if (storeTeamId) return undefined;
      return findTeamName(taskTeamId);
    },
    [storeTeamId, findTeamName],
  );
  const handleCreateForState = useCallback((stateId: string) => {
    setCreateOwnerMemberId(undefined);
    setCreateStateId(stateId);
    setCreateOpen(true);
  }, []);
  const handleCreateForMember = useCallback(
    (memberId: string | null) => {
      setCreateOwnerMemberId(memberId ?? undefined);
      setCreateStateId(workflow?.states[0]?.id || '');
      setCreateOpen(true);
    },
    [workflow],
  );

  // Listen for global panel events to patch board state
  useEffect(() => {
    const onUpdated = (e: Event) => {
      const updated = (e as CustomEvent)?.detail?.task;
      if (!updated) return;
      const task = { ...updated.task };
      if (groupBy === 'members') {
        const stateCol = columns.find((c) => c.state.id === task.workflow_state_id);
        if (stateCol) {
          task.state_name = stateCol.state.name;
          task.state_type = stateCol.state.state_type;
          task.state_color = stateCol.state.color;
        }
        const cols = usePMBoardStore.getState().memberColumns;
        const patched = cols.map((col: TaskMemberColumn) => {
          const idx = col.tasks.findIndex((s) => s.id === task.id);
          if (idx < 0) return col;
          const tasks = [...col.tasks];
          tasks[idx] = { ...tasks[idx], ...task };
          return { ...col, tasks };
        });
        usePMBoardStore.setState({ memberColumns: patched });
      } else {
        if (!patchTask('updated', task.id, task)) refreshBoard();
      }
    };
    const onArchived = (e: Event) => {
      const taskId = (e as CustomEvent)?.detail?.taskId;
      if (!taskId) return;
      if (groupBy === 'members') {
        const cols = usePMBoardStore.getState().memberColumns;
        const updated = cols.map((col: TaskMemberColumn) => {
          const idx = col.tasks.findIndex((s) => s.id === taskId);
          if (idx < 0) return col;
          return {
            ...col,
            tasks: col.tasks.filter((s) => s.id !== taskId),
            task_count: col.task_count - 1,
          };
        });
        usePMBoardStore.setState({ memberColumns: updated });
      } else {
        if (!patchTask('deleted', taskId)) refreshBoard();
      }
    };
    window.addEventListener('task-panel-updated', onUpdated);
    window.addEventListener('task-panel-archived', onArchived);
    return () => {
      window.removeEventListener('task-panel-updated', onUpdated);
      window.removeEventListener('task-panel-archived', onArchived);
    };
  }, [groupBy, columns, patchTask, refreshBoard]);

  const onDragStart = useCallback(
    (event: DragStartEvent) => {
      const traceID = createPMDnDTraceID();
      const activeId = String(event.active.id);
      activeDragTraceID.current = traceID;
      dragManager.clearColumnOverrides();
      dragManager.setDropPlaceholderRect(
        getDragStartTaskRect({
          activeId,
          activatorEvent: event.activatorEvent,
          dndRect: event.active.rect.current.initial,
        }),
      );
      const allTasks = groupBy === 'members' ? memberColumns.flatMap((col) => col.tasks) : columns.flatMap((column) => column.tasks);
      const task = allTasks.find((item) => item.id === activeId);
      dragManager.setActiveTask(task ?? null);
      if (task) {
        if (groupBy === 'members') {
          const sourceColumn = memberColumns.find((column) => column.tasks.some((candidate) => candidate.id === activeId));
          const sourceColumnId = sourceColumn?.member?.id ?? (sourceColumn ? '__unassigned__' : null);
          const sourceIndex = sourceColumn?.tasks.findIndex((candidate) => candidate.id === activeId) ?? -1;
          if (sourceColumn && sourceColumnId && sourceIndex >= 0) {
            const preview = getTaskDropPlaceholderPreview({
              activeId,
              fromColumnId: sourceColumnId,
              toColumnId: sourceColumnId,
              fromTasks: sourceColumn.tasks,
              toTasks: sourceColumn.tasks,
              toIndex: sourceIndex,
            });
            dragManager.updatePreview(sourceColumnId, sourceColumnId, preview.fromTasks, preview.toTasks, preview.dropTarget.toIndex);
          }
        } else {
          const sourceColumn = columns.find((column) => column.tasks.some((candidate) => candidate.id === activeId));
          const sourceColumnId = sourceColumn?.state.id;
          const sourceIndex = sourceColumn?.tasks.findIndex((candidate) => candidate.id === activeId) ?? -1;
          if (sourceColumn && sourceColumnId && sourceIndex >= 0) {
            const preview = getTaskDropPlaceholderPreview({
              activeId,
              fromColumnId: sourceColumnId,
              toColumnId: sourceColumnId,
              fromTasks: sourceColumn.tasks,
              toTasks: sourceColumn.tasks,
              toIndex: sourceIndex,
            });
            dragManager.updatePreview(sourceColumnId, sourceColumnId, preview.fromTasks, preview.toTasks, preview.dropTarget.toIndex);
          }
        }
      }
    },
    [columns, memberColumns, groupBy, dragManager],
  );

  const onDragOver = useCallback(
    (event: DragOverEvent) => {
      // Keep preview work capped near frame cadence while avoiding visible lag.
      const now = performance.now();
      if (now - lastDragOverTime.current < PM_BOARD_DRAG_OVER_THROTTLE_MS) return;
      lastDragOverTime.current = now;

      const { active, over } = event;
      if (!over) return;
      const activeId = String(active.id);
      const overId = String(over.id);
      if (activeId === overId) return;

      if (groupBy === 'members') {
        const fromKey = getBaseDragSourceColumnId({
          activeId,
          columns: memberColumns.map((col) => ({
            id: col.member?.id ?? '__unassigned__',
            tasks: col.tasks,
          })),
        });
        const previousTarget = dragManager.getDropTarget();
        const r = active.rect.current.translated;
        const belowMid = r ? r.top + r.height / 2 > over.rect.top + over.rect.height / 2 : false;
        const target = fromKey
          ? resolveBoardDropTarget({
              activeId,
              fromColumnId: fromKey,
              overId,
              pointerBelowMid: belowMid,
              previewTarget: previousTarget,
              columns: memberColumns.map((col) => ({
                id: col.member?.id ?? '__unassigned__',
                tasks: col.tasks,
              })),
            })
          : null;
        if (!fromKey || !target) {
          dragManager.clearColumnOverrides();
          return;
        }

        const toKey = target.toColumnId;
        const fromBase = memberColumns.find((c) => (c.member?.id ?? '__unassigned__') === fromKey);
        const toBase = memberColumns.find((c) => (c.member?.id ?? '__unassigned__') === toKey);
        if (!fromBase || !toBase) return;

        const preview = getTaskDropPlaceholderPreview({
          activeId,
          fromColumnId: fromKey,
          toColumnId: toKey,
          fromTasks: fromBase.tasks,
          toTasks: toBase.tasks,
          toIndex: target.toIndex,
        });

        // Skip no-op updates
        const currentFrom = dragManager.getColumnTasks(fromKey);
        const currentTo = dragManager.getColumnTasks(toKey);
        if (
          !hasDragPreviewChanged({
            currentFrom,
            nextFrom: preview.fromTasks,
            currentTo,
            nextTo: preview.toTasks,
            currentDropTarget: previousTarget,
            nextDropTarget: preview.dropTarget,
          })
        )
          return;

        dragManager.updatePreview(fromKey, toKey, preview.fromTasks, preview.toTasks, preview.dropTarget.toIndex);
      } else {
        // State board path
        const fromStateId = getBaseDragSourceColumnId({
          activeId,
          columns: columns.map((col) => ({
            id: col.state.id,
            tasks: col.tasks,
          })),
        });
        const previousTarget = dragManager.getDropTarget();
        const r = active.rect.current.translated;
        const belowMid = r ? r.top + r.height / 2 > over.rect.top + over.rect.height / 2 : false;
        const target = fromStateId
          ? resolveBoardDropTarget({
              activeId,
              fromColumnId: fromStateId,
              overId,
              pointerBelowMid: belowMid,
              previewTarget: previousTarget,
              columns: columns.map((col) => ({
                id: col.state.id,
                stateType: col.state.state_type,
                tasks: col.tasks,
              })),
            })
          : null;
        if (!fromStateId || !target) {
          dragManager.clearColumnOverrides();
          return;
        }

        const toStateId = target.toColumnId;
        const fromCol = columns.find((c) => c.state.id === fromStateId)!;
        const toCol = columns.find((c) => c.state.id === toStateId)!;
        const preview = getTaskDropPlaceholderPreview({
          activeId,
          fromColumnId: fromStateId,
          toColumnId: toStateId,
          fromTasks: fromCol.tasks,
          toTasks: toCol.tasks,
          toIndex: target.toIndex,
        });

        // Skip no-op updates
        const currentFrom = dragManager.getColumnTasks(fromStateId);
        const currentTo = dragManager.getColumnTasks(toStateId);
        if (
          !hasDragPreviewChanged({
            currentFrom,
            nextFrom: preview.fromTasks,
            currentTo,
            nextTo: preview.toTasks,
            currentDropTarget: previousTarget,
            nextDropTarget: preview.dropTarget,
          })
        )
          return;

        dragManager.updatePreview(fromStateId, toStateId, preview.fromTasks, preview.toTasks, preview.dropTarget.toIndex);
      }
    },
    [groupBy, columns, memberColumns, dragManager],
  );

  const clearDragPreview = useCallback(() => {
    dragManager.clear();
    activeDragTraceID.current = null;
  }, [dragManager]);

  const onDragEnd = useCallback(
    async (event: DragEndEvent) => {
      const { active, over } = event;
      if (!over) {
        clearDragPreview();
        return;
      }
      const activeId = String(active.id);
      const overId = String(over.id);

      if (groupBy === 'members') {
        const fromKey = getBaseDragSourceColumnId({
          activeId,
          columns: memberColumns.map((col) => ({
            id: col.member?.id ?? '__unassigned__',
            tasks: col.tasks,
          })),
        });
        if (!fromKey) {
          clearDragPreview();
          return;
        }

        const overColumn = memberColumns.find((col) => {
          const key = col.member?.id ?? '__unassigned__';
          return key === overId || col.tasks.some((s) => s.id === overId);
        });
        const overIdx = overColumn?.tasks.findIndex((s) => s.id === overId) ?? -1;
        const r = active.rect.current.translated;
        const belowMid = overIdx >= 0 && r ? r.top + r.height / 2 > over.rect.top + over.rect.height / 2 : false;
        const finalTarget = resolveBoardDropTarget({
          activeId,
          previewTarget: dragManager.getDropTarget(),
          fromColumnId: fromKey,
          overId,
          pointerBelowMid: belowMid,
          columns: memberColumns.map((col) => ({
            id: col.member?.id ?? '__unassigned__',
            tasks: col.tasks,
          })),
        });
        const toKey = finalTarget?.toColumnId ?? null;
        const toIdx = finalTarget?.toIndex ?? 0;

        if (toKey) {
          if (fromKey === toKey) {
            clearDragPreview();
            return;
          }
          const fromColumn = memberColumns.find((c) => (c.member?.id ?? '__unassigned__') === fromKey);
          const toColumn = memberColumns.find((c) => (c.member?.id ?? '__unassigned__') === toKey);
          if (!fromColumn || !toColumn) {
            clearDragPreview();
            return;
          }
          await commitDropBeforeClearingPreview({
            commit: () =>
              moveMemberTask({
                workspaceId,
                taskId: activeId,
                fromMemberId: fromColumn.member?.id ?? null,
                toMemberId: toColumn.member?.id ?? null,
                toIndex: toIdx,
              }),
            clearPreview: clearDragPreview,
          });
        } else {
          clearDragPreview();
        }
        return;
      }

      // ── State board ──
      // Find original column from BASE store data
      const fromStateId = getBaseDragSourceColumnId({
        activeId,
        columns: columns.map((col) => ({
          id: col.state.id,
          tasks: col.tasks,
        })),
      });
      if (!fromStateId) {
        clearDragPreview();
        return;
      }
      const debugTraceID = activeDragTraceID.current ?? createPMDnDTraceID();

      const overColumn = columns.find((col) => col.state.id === overId || col.tasks.some((s) => s.id === overId));
      const overIdx = overColumn?.tasks.findIndex((s) => s.id === overId) ?? -1;
      const r = active.rect.current.translated;
      const belowMid = overIdx >= 0 && r ? r.top + r.height / 2 > over.rect.top + over.rect.height / 2 : false;
      const finalTarget = resolveBoardDropTarget({
        activeId,
        previewTarget: dragManager.getDropTarget(),
        fromColumnId: fromStateId,
        overId,
        pointerBelowMid: belowMid,
        columns: columns.map((col) => ({
          id: col.state.id,
          stateType: col.state.state_type,
          tasks: col.tasks,
        })),
      });

      if (!finalTarget) {
        clearDragPreview();
        return;
      }

      const toStateId = finalTarget.toColumnId;
      const toIndex = finalTarget.toIndex;
      const toColumn = columns.find((c) => c.state.id === toStateId);
      if (!toColumn) {
        clearDragPreview();
        return;
      }

      if (fromStateId !== toStateId) {
        await commitDropBeforeClearingPreview({
          commit: async () => {
            const moved = await moveTask({
              workspaceId,
              taskId: activeId,
              fromStateId,
              toStateId,
              toIndex,
              debugTraceID,
            });
            if (moved) notifyAgentAutoRunStateChange(fromStateId, toStateId);
          },
          clearPreview: clearDragPreview,
        });
      } else {
        if (toColumn.state.state_type === 'done') {
          clearDragPreview();
          return;
        }
        const fromIndex = toColumn.tasks.findIndex((s) => s.id === activeId);
        if (fromIndex < 0 || fromIndex === toIndex) {
          clearDragPreview();
          return;
        }
        await commitDropBeforeClearingPreview({
          commit: async () => {
            const moved = await moveTask({
              workspaceId,
              taskId: activeId,
              fromStateId,
              toStateId,
              toIndex,
              debugTraceID,
            });
            if (moved) notifyAgentAutoRunStateChange(fromStateId, toStateId);
          },
          clearPreview: clearDragPreview,
        });
      }
    },
    [columns, memberColumns, groupBy, moveTask, moveMemberTask, workspaceId, clearDragPreview, dragManager, notifyAgentAutoRunStateChange],
  );

  const handleCreate = useCallback(
    async (payload: CreateTaskRequest) => {
      const task = await createTask(payload);
      return task
        ? {
            id: task.id,
            task: {
              id: task.id,
              name: task.name,
              display_id: task.display_id,
              task_key: task.task_key,
            },
          }
        : undefined;
    },
    [createTask],
  );

  const handleSeedTasks = useCallback(async () => {
    if (!workspaceId || isSeeding) return;
    setIsSeeding(true);
    try {
      const { data: result, error } = await pmTaskService.seed({
        workspace_id: workspaceId,
        count: 500,
      });
      if (error) {
        toast.error(error);
        return;
      }
      await loadBoard(workspaceId);
      if (groupBy === 'members') {
        await loadMemberBoard(showEmptyColumns ? activeMemberIds : undefined, showEmptyColumns);
      }
      toast.success(`Created ${result?.created ?? 500} test tasks`);
      setSeedConfirmOpen(false);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to seed tasks');
    } finally {
      setIsSeeding(false);
    }
  }, [workspaceId, isSeeding, loadBoard, groupBy, loadMemberBoard, showEmptyColumns, activeMemberIds]);

  const handleTaskPatched = useCallback(
    (task: Task) => {
      if (groupBy === 'members') {
        // Enrich with state info from workflow columns (read from store directly to avoid dep)
        const stateColumns = usePMBoardStore.getState().columns;
        const stateCol = stateColumns.find((c) => c.state.id === task.workflow_state_id);
        if (stateCol) {
          task = {
            ...task,
            state_name: stateCol.state.name,
            state_type: stateCol.state.state_type,
            state_color: stateCol.state.color,
          };
        }
        // Optimistically patch the task in member columns
        const cols = usePMBoardStore.getState().memberColumns;
        let previousStateId: string | undefined;
        const updated = cols.map((col) => {
          const idx = col.tasks.findIndex((s) => s.id === task.id);
          if (idx < 0) return col;
          previousStateId = col.tasks[idx]?.workflow_state_id;
          const tasks = [...col.tasks];
          tasks[idx] = { ...tasks[idx], ...task };
          return { ...col, tasks };
        });
        usePMBoardStore.setState({ memberColumns: updated });
        notifyAgentAutoRunStateChange(previousStateId, task.workflow_state_id);
      } else {
        const previousStateId = usePMBoardStore
          .getState()
          .columns.flatMap((column) => column.tasks)
          .find((candidate) => candidate.id === task.id)?.workflow_state_id;
        const patched = patchTask('updated', task.id, task);
        if (!patched) {
          refreshBoard();
        }
        notifyAgentAutoRunStateChange(previousStateId, task.workflow_state_id);
      }
    },
    [patchTask, refreshBoard, groupBy, notifyAgentAutoRunStateChange],
  );

  // Memoize context values to avoid re-rendering all consumers
  const boardData = useMemo<import('./KanbanBoard.contexts').BoardDataContextValue>(
    () => ({
      workspaceId,
      ownerNameMap,
      epicById: new Map(refEpics.map(({ epic }) => [epic.id, epic])),
      agentById,
      assignableMembers,
      automatedStateIds,
      findTeamName: resolveTeamName,
    }),
    [workspaceId, ownerNameMap, agentById, assignableMembers, automatedStateIds, resolveTeamName, refEpics],
  );

  // Use a ref so the context value identity never changes — consumers never
  // re-render from callback identity shifts (e.g. ownerNameMap refetch).
  const boardCallbacksRef = useRef<import('./KanbanBoard.contexts').BoardCallbacksContextValue>({
    onTaskPatched: handleTaskPatched,
    onOpen: openTask,
    onOpenAgentRun: openAgentRun,
    onCreate: handleCreateForState,
    onCreateForMember: handleCreateForMember,
    onToggleCollapse: toggleCollapse,
    onLoadMore: loadMoreColumn,
    onLoadMoreMember: loadMoreMemberColumn,
  });
  boardCallbacksRef.current = {
    onTaskPatched: handleTaskPatched,
    onOpen: openTask,
    onOpenAgentRun: openAgentRun,
    onCreate: handleCreateForState,
    onCreateForMember: handleCreateForMember,
    onToggleCollapse: toggleCollapse,
    onLoadMore: loadMoreColumn,
    onLoadMoreMember: loadMoreMemberColumn,
  };

  return (
    <TaskFilterProvider workspaceId={workspaceId} assignableMembers={assignableMembers} activeTeamId={storeTeamId} userMemberships={userMemberships} labels={refLabels} epics={refEpics} sprints={refSprints} onChange={setFilters} externalFilters={filters}>
      <div className="flex h-full min-h-0 flex-col">
        <QuietPageHeader
          variant="shell"
          title="Tasks"
          context={teamLabel}
          actions={
            <QuietPrimaryAction className="gap-1.5" disabled={!workflow} onClick={() => handleCreateForState(workflow?.states[0]?.id ?? '')}>
              <PlusSignIcon className="h-4 w-4" />
              Add task
            </QuietPrimaryAction>
          }
        />
        {currentUser && <ViewBar workspaceId={workspaceId} currentUserId={currentUser.id} />}
        <header className="ui-divider-bottom-fade flex min-h-11 flex-wrap items-center gap-2 px-3 py-2">
          <span ref={setBulkTriggerSlot} className="contents" />
          <TaskFilterTrigger />
          <TaskOwnerAvatarFilterRow />

          {/* Team selector — only shown when no team is pre-selected via URL */}
          {!teamId && teams.length > 0 && (
            <Select value={storeTeamId ?? '__all__'} onValueChange={(value) => setTeamFilter(value === '__all__' ? null : value)}>
              <SelectTrigger className="h-7 w-auto gap-1.5 text-xs px-2.5">
                <span className="text-muted-foreground">Team:</span>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__all__">All teams</SelectItem>
                {teams.map((t) => (
                  <SelectItem key={t.id} value={t.id}>
                    {t.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
          <div className="ml-auto flex items-center gap-1 self-center">
            <BoardToolbarSlot>
              {viewMode === 'board' ? (
                <div className="inline-flex h-7 items-center rounded-md border border-input bg-muted/40 p-0.5 text-xs">
                  <button className={`rounded px-2 py-0.5 transition-colors ${groupBy === 'status' ? 'bg-background font-medium text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}`} onClick={() => setGroupBy('status')}>
                    By States
                  </button>
                  <button className={`rounded px-2 py-0.5 transition-colors ${groupBy === 'members' ? 'bg-background font-medium text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}`} onClick={() => setGroupBy('members')}>
                    By Members
                  </button>
                </div>
              ) : (
                <TaskListGroupingDropdown
                  value={listGroupBy}
                  options={listGroupOptions}
                  onChange={setListGroupBy}
                />
              )}
            </BoardToolbarSlot>
            <BoardToolbarSlot className="gap-1">
              {showSeedButton ? (
                <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => setSeedConfirmOpen(true)} disabled={!workspaceId || isSeeding}>
                  {isSeeding ? <Loading01Icon className="mr-1 h-3.5 w-3.5 animate-spin" /> : <StickyNote01Icon className="mr-1 h-3.5 w-3.5" />}
                  Seed 500
                </Button>
              ) : null}
              <span className="inline-flex h-7 items-center gap-0.5 rounded-md border border-border/70 bg-muted/30 p-0.5">
                <QuickTooltip label="Board view">
                  <Button variant="ghost" size="icon" aria-pressed={viewMode === 'board'} className={`h-6 w-6 rounded-sm ${viewMode === 'board'
                    ? 'bg-background text-foreground shadow-sm hover:bg-background'
                    : 'text-muted-foreground hover:text-foreground'
                  }`} onClick={() => setViewMode('board')}>
                    <LayoutTwoColumnIcon className="h-3.5 w-3.5" />
                  </Button>
                </QuickTooltip>
                <QuickTooltip label="List view">
                  <Button variant="ghost" size="icon" aria-pressed={viewMode === 'list'} className={`h-6 w-6 rounded-sm ${viewMode === 'list'
                    ? 'bg-background text-foreground shadow-sm hover:bg-background'
                    : 'text-muted-foreground hover:text-foreground'
                  }`} onClick={() => setViewMode('list')}>
                    <LayoutTable01Icon className="h-3.5 w-3.5" />
                  </Button>
                </QuickTooltip>
              </span>
            </BoardToolbarSlot>
            <BoardToolbarSlot>{viewMode === 'board' ? <BoardDisplayMenu /> : <ListDisplayMenu disabledKeys={listDisabledKeys} />}</BoardToolbarSlot>
          </div>
        </header>

        <TaskFilterBar />

        {error ? <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">{error}</div> : null}

        {loading ? (
          <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
            <Loading01Icon className="mr-2 h-4 w-4 animate-spin" />
            Loading board...
          </div>
        ) : null}

        {!loading && viewMode === 'board' ? (
          <BoardDataContext.Provider value={boardData}>
            <BoardCallbacksContext.Provider value={boardCallbacksRef}>
              <DragPreviewContext.Provider value={dragManager}>
                <DndContext sensors={sensors} collisionDetection={closestCenter} measuring={PM_BOARD_DND_MEASURING} onDragStart={onDragStart} onDragOver={onDragOver} onDragEnd={onDragEnd} onDragCancel={clearDragPreview}>
                  <div className="min-h-0 flex-1 overflow-x-auto">
                    <div className="flex h-full min-w-full gap-1.5 pb-2">
                      {groupBy === 'members'
                        ? memberColumns
                            .filter((col) => showEmptyColumns || isDragging || col.task_count > 0)
                            .map((col) => {
                              const colKey = col.member?.id ?? '__unassigned__';
                              return <MemberColumn key={colKey} column={col} collapsed={collapsedColumns.has(colKey)} isLoadingMore={!!memberColumnLoading[colKey]} />;
                            })
                        : columns.filter((column) => showEmptyColumns || isDragging || column.task_count > 0).map((column) => <Column key={column.state.id} column={column} collapsed={collapsedColumns.has(column.state.id)} isLoadingMore={!!columnLoading[column.state.id]} />)}
                    </div>
                  </div>

                  <DragOverlay>
                    <DragOverlayCard manager={dragManager} resolveTeamName={resolveTeamName} groupBy={groupBy} />
                  </DragOverlay>
                </DndContext>
              </DragPreviewContext.Provider>
            </BoardCallbacksContext.Provider>
          </BoardDataContext.Provider>
        ) : null}

        {!loading && viewMode === 'list' && workflow ? <TaskListView workspaceId={workspaceId} workflow={workflow} workflows={allWorkflows} teams={teams} assignableMembers={assignableMembers} epics={refEpics} sprints={refSprints} filters={filters} teamId={storeTeamId} onOpenTask={openTask} groupBy={listGroupBy} onGroupByChange={setListGroupBy} showToolbar={false} bulkTriggerContainer={bulkTriggerSlot} /> : null}

        {workflow ? <CreateTaskModal open={createOpen} onOpenChange={setCreateOpen} workspaceId={workspaceId} workflow={workflow} initialStateId={createStateId || workflow.states[0]?.id || ''} initialTeamId={storeTeamId ?? undefined} initialOwnerMemberId={createOwnerMemberId} onCreate={handleCreate} /> : null}

        <ConfirmDialog
          open={seedConfirmOpen}
          onOpenChange={(open) => {
            if (!isSeeding) setSeedConfirmOpen(open);
          }}
          title="Seed 500 test tasks"
          description="This will create 500 synthetic tasks across the default workflow so you can test the board, grouping, and list performance."
          confirmLabel={isSeeding ? 'Seeding...' : 'Seed tasks'}
          variant="default"
          onConfirm={handleSeedTasks}
        />
      </div>
    </TaskFilterProvider>
  );
}
