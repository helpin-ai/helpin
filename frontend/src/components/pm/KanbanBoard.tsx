import { memo, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import {
  DndContext,
  DragOverlay,
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
import { BarChart3, Bot, ChevronsLeftRight, ChevronsRightLeft, Columns2, LayoutList, Loader2, Plus, StickyNote, User } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { usePMBoardStore } from '@/stores/pmBoardStore';
import type { Agent, CreateTaskRequest, Task, TaskMemberColumn, TaskStateColumn, Label, EpicWithStats, SprintWithStats } from '@/lib/pmTypes';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { pmLabelService } from '@/lib/services/pmLabelService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmSprintService } from '@/lib/services/pmSprintService';
import { StateTypeIcon } from '@/lib/pmConstants';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { useAuthStore } from '@/stores/authStore';
import { useAgents, useSession, useAutomationRulesByWorkflow, useTeamFieldVisibilityForTeam } from '@/hooks/queries';
import { UserAvatar } from './UserAvatar';
import { TaskCard } from './TaskCard';
import { CreateTaskModal } from './CreateTaskModal';
import { StoryFilterProvider, StoryFilterTrigger, StoryFilterBar, StoryOwnerAvatarFilterRow } from './StoryFilters';
import { TaskListView } from './TaskListView';
import { ViewBar } from './ViewBar';
import { BoardDisplayMenu } from './BoardDisplayMenu';
import { ListDisplayMenu } from './ListDisplayMenu';
import { BoardToolbarSlot } from './BoardToolbarSlot';
import { useBoardDisplayStore, type DisplayPropertyKey } from '@/stores/boardDisplayStore';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
import { createPMDnDTraceID, logPMDnD } from '@/lib/pmDnDDebug';
import { DragPreviewManager, useActiveStory, useColumnDragPreview, commitDropBeforeClearingPreview, getSameStateBoardDropIndex, getStateBoardPreviewInsertIndex, getStoredCrossColumnDropTarget, getStableCrossColumnPreviewIndex } from './KanbanBoard.dnd';
import { BoardDataContext, BoardCallbacksContext, DragPreviewContext } from './KanbanBoard.contexts';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { getVisibleTaskListGroupOptions, type TaskListGroupByOption } from '@/components/pm/task-detail/taskListGrouping';
import { useWorkspaceStore } from '@/stores/workspaceStore';

// ── Helpers ─────────────────────────────────────────────────────────

function storyListChanged(a: Task[], b: Task[]): boolean {
  if (a.length !== b.length) return true;
  for (let i = 0; i < a.length; i++) {
    if (a[i].id !== b[i].id) return true;
  }
  return false;
}

interface KanbanBoardProps {
  workspaceId: string;
  teamId?: string;
}

interface ColumnProps {
  column: TaskStateColumn;
  collapsed: boolean;
  isLoadingMore: boolean;
}

const Column = memo(function Column({ column, collapsed, isLoadingMore }: ColumnProps) {
  const { automatedStateIds, findTeamName } = useContext(BoardDataContext)!;
  const callbacksRef = useContext(BoardCallbacksContext)!;
  const dragManager = useContext(DragPreviewContext)!;

  // Subscribe to drag preview for this column only
  const stories = useColumnDragPreview(dragManager, column.state.id, column.stories);

  const { setNodeRef, isOver } = useDroppable({ id: column.state.id });
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const loadMoreRef = useRef<HTMLDivElement | null>(null);
  const groupedStories = column.state.state_type === 'done' ? column.task_groups ?? [] : [];

  // Memoize sortable items from preview stories
  const sortableItems = useMemo(() => stories.map((s) => s.id), [stories]);

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
  }, [column.has_more, column.state.id, column.stories.length, isLoadingMore, callbacksRef]);

  if (collapsed) {
    return (
      <QuickTooltip label={`Expand ${column.state.name}`}>
      <section
        ref={setNodeRef}
        className={cn(
          'relative flex h-full w-[44px] shrink-0 cursor-pointer flex-col items-center rounded-md border border-border/50 bg-muted/30 transition-colors hover:bg-muted/50',
          isOver && 'bg-accent ring-1 ring-inset ring-border',
        )}
        onClick={() => callbacksRef.current.onToggleCollapse(column.state.id)}
      >
        {column.state.color && (
          <div className="absolute top-0 left-2 right-2 h-[3px] rounded-b-full" style={{ backgroundColor: column.state.color }} />
        )}
        <ChevronsLeftRight className="mt-3 mb-2 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        <StateTypeIcon stateType={column.state.state_type} className="mb-2 h-4 w-4 shrink-0" />
        {automatedStateIds?.has(column.state.id) && (
          <Bot className="mb-1 h-3.5 w-3.5 shrink-0 text-violet-500" />
        )}
        <span className="text-xs font-medium text-muted-foreground">{column.task_count}</span>
        <div className="mt-3 flex flex-1 items-start">
          <span
            className="text-xs font-semibold whitespace-nowrap"
            style={{ writingMode: 'vertical-rl', textOrientation: 'mixed' }}
          >
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
        {column.state.color && (
          <div className="absolute top-0 left-3 right-3 h-[3px] rounded-b-full" style={{ backgroundColor: column.state.color }} />
        )}
        <div className="min-w-0">
          {column.state.description ? (
            <QuickTooltip label={column.state.description}>
              <p className="flex items-center gap-1.5 truncate text-sm font-semibold cursor-default">
                <StateTypeIcon stateType={column.state.state_type} className="h-4 w-4 shrink-0" />
                {column.state.name}
                {automatedStateIds?.has(column.state.id) && (
                  <QuickTooltip label="Agent runs automatically on entry">
                    <Bot className="h-3.5 w-3.5 shrink-0 text-violet-500" />
                  </QuickTooltip>
                )}
              </p>
            </QuickTooltip>
          ) : (
            <p className="flex items-center gap-1.5 truncate text-sm font-semibold">
              <StateTypeIcon stateType={column.state.state_type} className="h-4 w-4 shrink-0" />
              {column.state.name}
              {automatedStateIds?.has(column.state.id) && (
                <QuickTooltip label="Agent runs automatically on entry">
                  <Bot className="h-3.5 w-3.5 shrink-0 text-violet-500" />
                </QuickTooltip>
              )}
            </p>
          )}
          <p className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
            <QuickTooltip label={`${column.task_count} ${column.task_count === 1 ? 'task' : 'tasks'}`}>
              <span className="inline-flex items-center gap-1.5">
                <StickyNote className="h-3 w-3" />
                {column.task_count}
              </span>
            </QuickTooltip>
            <QuickTooltip label={`${column.point_total} estimate ${column.point_total === 1 ? 'point' : 'points'}`}>
              <span className="inline-flex items-center gap-1.5">
                <BarChart3 className="h-3 w-3" />
                {column.point_total}
              </span>
            </QuickTooltip>
          </p>
        </div>
        <div className="flex items-center gap-0.5">
          <QuickTooltip label="Collapse column">
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 opacity-0 group-hover/header:opacity-100 transition-opacity"
              onClick={() => callbacksRef.current.onToggleCollapse(column.state.id)}
            >
              <ChevronsRightLeft className="h-3.5 w-3.5" />
            </Button>
          </QuickTooltip>
          <QuickTooltip label="Create story">
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => callbacksRef.current.onCreate(column.state.id)}>
              <Plus className="h-4 w-4" />
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
          className={`scrollbar-hover min-h-0 flex-1 overflow-y-auto p-2 flex flex-col rounded-md transition-all duration-200 ${
            isOver ? 'bg-accent ring-1 ring-inset ring-border gap-4' : 'gap-2'
          }`}
        >
          {groupedStories.length > 0 ? (
            groupedStories.map((group) => (
              <div key={group.key} className="space-y-2">
                <div className="rounded-md bg-muted px-3 py-1 text-center text-xs font-semibold text-muted-foreground">
                  {group.label}
                </div>
                {group.stories.map((story) => (
                  <TaskCard
                    key={story.id}
                    story={story}
                    teamName={findTeamName(story.team_id)}
                  />
                ))}
              </div>
            ))
          ) : (
            stories.map((story) => (
              <TaskCard
                key={story.id}
                story={story}
                teamName={findTeamName(story.team_id)}
              />
            ))
          )}

          {column.has_more ? (
            <div
              ref={loadMoreRef}
              className="flex h-8 items-center justify-center text-xs text-muted-foreground"
            >
              {isLoadingMore ? (
                <>
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                  Loading more...
                </>
              ) : (
                <span>{column.task_count - column.stories.length} remaining</span>
              )}
            </div>
          ) : null}

          <Button
            variant="ghost"
            className="w-full justify-start text-xs text-muted-foreground"
            onClick={() => callbacksRef.current.onCreate(column.state.id)}
          >
            <Plus className="h-3.5 w-3.5" />
            Add story
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
  const stories = useColumnDragPreview(dragManager, colKey, column.stories);

  const { setNodeRef, isOver } = useDroppable({ id: colKey });
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const loadMoreRef = useRef<HTMLDivElement | null>(null);
  const displayName = column.member?.display_name ?? 'Unassigned';

  // Memoize sortable items from preview stories
  const sortableItems = useMemo(() => stories.map((s) => s.id), [stories]);

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
  }, [column.has_more, column.member?.id, column.stories.length, isLoadingMore, callbacksRef]);

  if (collapsed) {
    return (
      <QuickTooltip label={`Expand ${displayName}`}>
        <section
          ref={setNodeRef}
          className={cn(
            'relative flex h-full w-[44px] shrink-0 cursor-pointer flex-col items-center rounded-md border border-border/50 bg-muted/30 pt-3 transition-colors hover:bg-muted/50',
            isOver && 'bg-accent ring-1 ring-inset ring-border',
          )}
          onClick={() => callbacksRef.current.onToggleCollapse(colKey)}
        >
          <ChevronsLeftRight className="mt-3 mb-2 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          {column.member ? (
            <UserAvatar name={displayName} avatarUrl={column.member.avatar_url} className="h-5 w-5 text-[10px]" />
          ) : (
            <User className="h-4 w-4 shrink-0 text-muted-foreground" />
          )}
          <span className="mt-2 text-xs font-medium text-muted-foreground">{column.story_count}</span>
          <div className="mt-3 flex flex-1 items-start">
            <span
              className="text-xs font-semibold whitespace-nowrap"
              style={{ writingMode: 'vertical-rl', textOrientation: 'mixed' }}
            >
              {displayName}
            </span>
          </div>
        </section>
      </QuickTooltip>
    );
  }

  return (
    <section className="flex h-full w-[340px] shrink-0 flex-col">
      <header className="group/header flex items-center justify-between px-3 pt-4 pb-3">
        <div className="min-w-0">
          <p className="flex items-center gap-1.5 truncate text-sm font-semibold">
            {column.member ? (
              <UserAvatar name={displayName} avatarUrl={column.member.avatar_url} className="h-5 w-5 text-[10px]" />
            ) : (
              <User className="h-4 w-4 shrink-0 text-muted-foreground" />
            )}
            {displayName}
          </p>
          <p className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
            <QuickTooltip label={`${column.story_count} ${column.story_count === 1 ? 'task' : 'tasks'}`}>
              <span className="inline-flex items-center gap-1.5">
                <StickyNote className="h-3 w-3" />
                {column.story_count}
              </span>
            </QuickTooltip>
            <QuickTooltip label={`${column.point_total} estimate ${column.point_total === 1 ? 'point' : 'points'}`}>
              <span className="inline-flex items-center gap-1.5">
                <BarChart3 className="h-3 w-3" />
                {column.point_total}
              </span>
            </QuickTooltip>
          </p>
        </div>
        <div className="flex items-center gap-0.5">
          <QuickTooltip label="Collapse column">
            <Button variant="ghost" size="icon" className="h-7 w-7 opacity-0 group-hover/header:opacity-100 transition-opacity" onClick={() => callbacksRef.current.onToggleCollapse(colKey)}>
              <ChevronsRightLeft className="h-3.5 w-3.5" />
            </Button>
          </QuickTooltip>
          <QuickTooltip label="Create story">
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => callbacksRef.current.onCreateForMember(column.member?.id ?? null)}>
              <Plus className="h-4 w-4" />
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
          className={`scrollbar-hover min-h-0 flex-1 overflow-y-auto p-2 flex flex-col rounded-md transition-all duration-200 ${isOver ? 'bg-accent ring-1 ring-inset ring-border gap-4' : 'gap-2'}`}
        >
          {stories.map((story) => (
            <TaskCard
              key={story.id}
              story={story}
              teamName={findTeamName(story.team_id)}
              showStateBadge
            />
          ))}

          {column.has_more ? (
            <div ref={loadMoreRef} className="flex h-8 items-center justify-center text-xs text-muted-foreground">
              {isLoadingMore ? (
                <><Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />Loading more...</>
              ) : (
                <span>{column.story_count - column.stories.length} remaining</span>
              )}
            </div>
          ) : null}

          <Button
            variant="ghost"
            className="w-full justify-start text-xs text-muted-foreground"
            onClick={() => callbacksRef.current.onCreateForMember(column.member?.id ?? null)}
          >
            <Plus className="h-3.5 w-3.5" />
            Add story
          </Button>
        </div>
      </SortableContext>
    </section>
  );
});
MemberColumn.displayName = 'MemberColumn';

const DragOverlayCard = memo(function DragOverlayCard({
  manager,
  resolveTeamName,
  agentById,
  groupBy,
}: {
  manager: DragPreviewManager;
  resolveTeamName: (id?: string) => string | undefined;
  agentById: Map<string, Agent>;
  groupBy: string;
}) {
  const activeStory = useActiveStory(manager);
  if (!activeStory) return null;
  return (
    <TaskCard
      story={activeStory}
      isOverlay
      teamName={resolveTeamName(activeStory.team_id)}
      assignedAgent={activeStory.assigned_agent_id ? agentById.get(activeStory.assigned_agent_id) ?? null : null}
      showStateBadge={groupBy === 'members'}
    />
  );
});
DragOverlayCard.displayName = 'DragOverlayCard';

export function KanbanBoard({ workspaceId, teamId }: KanbanBoardProps) {
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

  const currentUser = useAuthStore((s) => s.user);
  const { data: sessionMembership } = useSession(workspaceId);
  const currentMemberId = sessionMembership?.id;
  const { teams, findTeamName } = useAccessibleTeams(workspaceId);
  const { userMemberships } = useWorkspaceTeams(workspaceId);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const listFieldVis = useTeamFieldVisibilityForTeam(workspaceId, storeTeamId);
  const ownerNameMap = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );
  const showEmptyColumns = useBoardDisplayStore((s) => s.showEmptyColumns);
  const groupBy = useBoardDisplayStore((s) => s.groupBy);
  const setGroupBy = useBoardDisplayStore((s) => s.setGroupBy);
  const initDisplay = useBoardDisplayStore((s) => s.init);

  useEffect(() => { initDisplay(workspaceId); }, [workspaceId, initDisplay]);

  // Fetch automation rules to show bot icons on columns with direct agent-run automation.
  const { data: automationRules } = useAutomationRulesByWorkflow(workspaceId, workflow?.workflow.id);
  const automatedStateIds = useMemo(() => {
    const ids = new Set<string>();
    if (!automationRules) return ids;
    for (const rule of automationRules) {
      if (
        rule.enabled &&
        rule.trigger_type === 'story.state_entered' &&
        rule.action_type === 'start_agent_run'
      ) {
        const stateId = rule.trigger_config?.state_id;
        if (stateId) ids.add(stateId);
      }
    }
    return ids;
  }, [automationRules]);

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

  const [refLabels, setRefLabels] = useState<Label[]>([]);
  const [refEpics, setRefEpics] = useState<EpicWithStats[]>([]);
  const [refSprints, setRefSprints] = useState<SprintWithStats[]>([]);
  const { data: agents = [] } = useAgents(workspaceId);

  useEffect(() => {
    pmLabelService.list(workspaceId).then((r) => { if (r.data) setRefLabels(r.data); });
    pmEpicService.list(workspaceId).then((r) => { if (r.data) setRefEpics(r.data); });
    pmSprintService.list(workspaceId).then((r) => { if (r.data) setRefSprints(r.data); });
  }, [workspaceId]);
  const agentById = useMemo(
    () => new Map(agents.map((agent) => [agent.id, agent])),
    [agents],
  );

  const dragManager = useRef(new DragPreviewManager()).current;
  const isDragging = useActiveStory(dragManager) !== null;
  const lastDragOverTime = useRef(0);
  const [createOpen, setCreateOpen] = useState(false);
  const [createStateId, setCreateStateId] = useState<string>('');
  const [createOwnerMemberId, setCreateOwnerMemberId] = useState<string | undefined>(undefined);
  const VIEW_MODE_KEY = `pm_view_mode_${workspaceId}`;
  const [viewMode, setViewModeState] = useState<'board' | 'list'>(() => {
    try {
      const saved = localStorage.getItem(VIEW_MODE_KEY);
      return saved === 'list' ? 'list' : 'board';
    } catch { return 'board'; }
  });
  const setViewMode = useCallback((mode: 'board' | 'list') => {
    setViewModeState(mode);
    try { localStorage.setItem(VIEW_MODE_KEY, mode); } catch {}
  }, [VIEW_MODE_KEY]);
  const [listGroupBy, setListGroupBy] = useState<TaskListGroupByOption>('workflow_state');

  const COLLAPSED_KEY = `pm_kanban_collapsed_${workspaceId}`;
  const [collapsedColumns, setCollapsedColumnsState] = useState<Set<string>>(() => {
    try {
      const saved = localStorage.getItem(COLLAPSED_KEY);
      return saved ? new Set(JSON.parse(saved) as string[]) : new Set();
    } catch { return new Set(); }
  });
  const toggleCollapse = useCallback((stateId: string) => {
    setCollapsedColumnsState((prev) => {
      const next = new Set(prev);
      if (next.has(stateId)) next.delete(stateId);
      else next.add(stateId);
      try { localStorage.setItem(COLLAPSED_KEY, JSON.stringify([...next])); } catch {}
      return next;
    });
  }, [COLLAPSED_KEY]);

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
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
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

  // Refresh board when a story is created via the global modal
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

  // Open ?task= URL param in global panel on mount
  useEffect(() => {
    if (!workflow) return;
    const maybeTask = new URLSearchParams(window.location.search).get('task');
    if (!maybeTask) return;
    const match = maybeTask.match(/^(\d+)$/);
    if (!match) return;
    (async () => {
      const res = await pmTaskService.getByDisplayId(workspaceId, Number(match[1]));
      if (res.data && workspaceSlug) {
        openTaskRoute(navigate as never, { pathname: window.location.pathname } as never, workspaceSlug, res.data.task.id);
      }
    })();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspaceId, workflow, workspaceSlug]);
  const openStory = useCallback(
    (story: Task) => {
      if (!workspaceSlug) return;
      openTaskRoute(navigate as never, { pathname: window.location.pathname } as never, workspaceSlug, story.id);
    },
    [navigate, workspaceSlug],
  );
  const resolveTeamName = useCallback(
    (storyTeamId: string | undefined) => {
      if (storeTeamId) return undefined;
      return findTeamName(storyTeamId);
    },
    [storeTeamId, findTeamName],
  );
  const handleCreateForState = useCallback((stateId: string) => {
    setCreateOwnerMemberId(undefined);
    setCreateStateId(stateId);
    setCreateOpen(true);
  }, []);
  const handleCreateForMember = useCallback((memberId: string | null) => {
    setCreateOwnerMemberId(memberId ?? undefined);
    setCreateStateId(workflow?.states[0]?.id || '');
    setCreateOpen(true);
  }, [workflow]);

  // Listen for global panel events to patch board state
  useEffect(() => {
    const onUpdated = (e: Event) => {
      const updated = (e as CustomEvent)?.detail?.story;
      if (!updated) return;
      const story = { ...updated.task };
      const ownerKey = story.owner_member_id;
      if (ownerKey && !story.owner_name) {
        story.owner_name = updated.owner_member
          ? ownerNameMap.get(updated.owner_member.id) ?? updated.owner_member.display_name ?? updated.owner_member.email
          : ownerNameMap.get(ownerKey);
      }
      if (groupBy === 'members') {
        const stateCol = columns.find((c) => c.state.id === story.workflow_state_id);
        if (stateCol) {
          story.state_name = stateCol.state.name;
          story.state_type = stateCol.state.state_type;
          story.state_color = stateCol.state.color;
        }
        const cols = usePMBoardStore.getState().memberColumns;
        const patched = cols.map((col: TaskMemberColumn) => {
          const idx = col.stories.findIndex((s) => s.id === story.id);
          if (idx < 0) return col;
          const stories = [...col.stories];
          stories[idx] = { ...stories[idx], ...story };
          return { ...col, stories };
        });
        usePMBoardStore.setState({ memberColumns: patched });
      } else {
        if (!patchTask('updated', story.id, story)) refreshBoard();
      }
    };
    const onArchived = (e: Event) => {
      const storyId = (e as CustomEvent)?.detail?.storyId;
      if (!storyId) return;
      if (groupBy === 'members') {
        const cols = usePMBoardStore.getState().memberColumns;
        const updated = cols.map((col: TaskMemberColumn) => {
          const idx = col.stories.findIndex((s) => s.id === storyId);
          if (idx < 0) return col;
          return { ...col, stories: col.stories.filter((s) => s.id !== storyId), story_count: col.story_count - 1 };
        });
        usePMBoardStore.setState({ memberColumns: updated });
      } else {
        if (!patchTask('deleted', storyId)) refreshBoard();
      }
    };
    window.addEventListener('task-panel-updated', onUpdated);
    window.addEventListener('task-panel-archived', onArchived);
    return () => {
      window.removeEventListener('task-panel-updated', onUpdated);
      window.removeEventListener('task-panel-archived', onArchived);
    };
  }, [groupBy, columns, ownerNameMap, patchTask, refreshBoard]);

  const findStateIdByItemId = useCallback(
    (id: string) => {
      if (columns.some((column) => column.state.id === id)) return id;
      for (const column of columns) {
        if (column.stories.some((story) => story.id === id)) {
          return column.state.id;
        }
      }
      return null;
    },
    [columns]
  );

  const onDragStart = useCallback(
    (event: DragStartEvent) => {
      dragManager.clearColumnOverrides();
      const allStories = groupBy === 'members'
        ? memberColumns.flatMap((col) => col.stories)
        : columns.flatMap((column) => column.stories);
      const story = allStories.find((item) => item.id === String(event.active.id));
      dragManager.setActiveStory(story ?? null);
    },
    [columns, memberColumns, groupBy, dragManager]
  );

  const onDragOver = useCallback(
    (event: DragOverEvent) => {
      // Throttle to ~20fps
      const now = performance.now();
      if (now - lastDragOverTime.current < 50) return;
      lastDragOverTime.current = now;

      const { active, over } = event;
      if (!over) return;
      const activeId = String(active.id);
      const overId = String(over.id);
      if (activeId === overId) return;

      if (groupBy === 'members') {
        // Find source/target columns using manager overrides or base data
        let fromKey: string | null = null;
        let toKey: string | null = null;
        for (const col of memberColumns) {
          const key = col.member?.id ?? '__unassigned__';
          const colStories = dragManager.getColumnStories(key) ?? col.stories;
          if (colStories.some((s) => s.id === activeId)) fromKey = key;
          if (key === overId || colStories.some((s) => s.id === overId)) toKey = key;
        }
        if (!fromKey || !toKey || fromKey === toKey) {
          dragManager.clearColumnOverrides();
          return;
        }

        const fromBase = memberColumns.find((c) => (c.member?.id ?? '__unassigned__') === fromKey);
        const toBase = memberColumns.find((c) => (c.member?.id ?? '__unassigned__') === toKey);
        if (!fromBase || !toBase) return;

        const fromStories = [...(dragManager.getColumnStories(fromKey) ?? fromBase.stories)];
        const toStories = [...(dragManager.getColumnStories(toKey) ?? toBase.stories)];

        const idx = fromStories.findIndex((s) => s.id === activeId);
        if (idx < 0) return;
        const [story] = fromStories.splice(idx, 1);
        const previousTarget = dragManager.getDropTarget();
        let insertIdx: number;
        if (overId === toKey) {
          insertIdx = getStableCrossColumnPreviewIndex({
            previewTarget: previousTarget,
            fromColumnId: fromKey,
            toColumnId: toKey,
            overId,
            containerId: toKey,
            computedIndex: toStories.length,
            columnLength: toStories.length,
          });
        } else {
          const overIdx = toStories.findIndex((s) => s.id === overId);
          insertIdx = overIdx >= 0 ? overIdx : toStories.length;
          if (overIdx >= 0) {
            const r = active.rect.current.translated;
            const belowMid = r ? r.top + r.height / 2 > over.rect.top + over.rect.height / 2 : false;
            if (belowMid) insertIdx = overIdx + 1;
          }
        }
        toStories.splice(insertIdx, 0, story);

        // Skip no-op updates
        const currentFrom = dragManager.getColumnStories(fromKey);
        if (currentFrom && !storyListChanged(currentFrom, fromStories)) return;

        dragManager.updatePreview(fromKey, toKey, fromStories, toStories, insertIdx);
      } else {
        // State board path
        let fromStateId: string | null = null;
        let toStateId: string | null = null;
        for (const col of columns) {
          const colStories = dragManager.getColumnStories(col.state.id) ?? col.stories;
          if (colStories.some((s) => s.id === activeId)) fromStateId = col.state.id;
          if (col.state.id === overId || colStories.some((s) => s.id === overId)) toStateId = col.state.id;
        }
        if (!fromStateId || !toStateId || fromStateId === toStateId) {
          dragManager.clearColumnOverrides();
          return;
        }

        const fromStories = [...(dragManager.getColumnStories(fromStateId) ?? columns.find((c) => c.state.id === fromStateId)!.stories)];
        const toStories = [...(dragManager.getColumnStories(toStateId) ?? columns.find((c) => c.state.id === toStateId)!.stories)];

        const idx = fromStories.findIndex((s) => s.id === activeId);
        if (idx < 0) return;
        const [story] = fromStories.splice(idx, 1);
        const overIdx = toStories.findIndex((s) => s.id === overId);
        const r = active.rect.current.translated;
        const belowMid = overIdx >= 0 && r ? r.top + r.height / 2 > over.rect.top + over.rect.height / 2 : false;
        const toCol = columns.find((c) => c.state.id === toStateId)!;
        const computedInsertIdx = getStateBoardPreviewInsertIndex({
          toStateType: toCol.state.state_type,
          overId,
          toStateId,
          overIdx,
          columnLength: toStories.length,
          pointerBelowMid: belowMid,
        });
        const insertIdx = getStableCrossColumnPreviewIndex({
          previewTarget: dragManager.getDropTarget(),
          fromColumnId: fromStateId,
          toColumnId: toStateId,
          overId,
          containerId: toStateId,
          computedIndex: computedInsertIdx,
          columnLength: toStories.length,
        });
        toStories.splice(insertIdx, 0, story);

        // Skip no-op updates
        const currentFrom = dragManager.getColumnStories(fromStateId);
        if (currentFrom && !storyListChanged(currentFrom, fromStories)) return;

        dragManager.updatePreview(fromStateId, toStateId, fromStories, toStories, insertIdx);
      }
    },
    [groupBy, columns, memberColumns, dragManager],
  );

  const clearDragPreview = useCallback(() => {
    dragManager.clear();
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
        // Find original column from BASE store data
        let fromKey: string | null = null;
        for (const col of memberColumns) {
          const key = col.member?.id ?? '__unassigned__';
          if (col.stories.some((s) => s.id === activeId)) { fromKey = key; break; }
        }
        if (!fromKey) { clearDragPreview(); return; }

        // Find target column: first check preview, then use over event
        let toKey: string | null = null;
        let toIdx = 0;
        const storedMemberTarget = getStoredCrossColumnDropTarget({
          previewTarget: dragManager.getDropTarget(),
          fromColumnId: fromKey,
          validColumnIds: memberColumns.map((col) => col.member?.id ?? '__unassigned__'),
        });

        if (storedMemberTarget) {
          toKey = storedMemberTarget.toColumnId;
          toIdx = storedMemberTarget.toIndex;
        }

        // Check preview overrides for cross-column move
        if (!toKey) {
          for (const col of memberColumns) {
            const key = col.member?.id ?? '__unassigned__';
            const previewStories = dragManager.getColumnStories(key);
            if (previewStories) {
              const idx = previewStories.findIndex((s) => s.id === activeId);
              if (idx >= 0 && key !== fromKey) { toKey = key; toIdx = idx; break; }
            }
          }
        }

        // If preview didn't capture it (throttle), compute from the over event
        if (!toKey) {
          for (const col of memberColumns) {
            const key = col.member?.id ?? '__unassigned__';
            if (key === overId || col.stories.some((s) => s.id === overId)) {
              if (key !== fromKey) { toKey = key; toIdx = 0; }
              break;
            }
          }
        }

        if (toKey) {
          const fromColumn = memberColumns.find((c) => (c.member?.id ?? '__unassigned__') === fromKey);
          const toColumn = memberColumns.find((c) => (c.member?.id ?? '__unassigned__') === toKey);
          if (!fromColumn || !toColumn) { clearDragPreview(); return; }
          await commitDropBeforeClearingPreview({
            commit: () => moveMemberTask({
              workspaceId,
              storyId: activeId,
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
      let fromStateId: string | null = null;
      for (const col of columns) {
        if (col.stories.some((s) => s.id === activeId)) { fromStateId = col.state.id; break; }
      }
      if (!fromStateId) { clearDragPreview(); return; }
      const debugTraceID = createPMDnDTraceID();

        // Find target: first check preview, then use over event
        let crossStateId: string | null = null;
        let crossIdx = 0;
        const storedStateTarget = getStoredCrossColumnDropTarget({
          previewTarget: dragManager.getDropTarget(),
          fromColumnId: fromStateId,
          validColumnIds: columns.map((col) => col.state.id),
        });

        if (storedStateTarget) {
          crossStateId = storedStateTarget.toColumnId;
          crossIdx = storedStateTarget.toIndex;
        }

        // Check preview overrides for cross-column move
        if (!crossStateId) {
          for (const col of columns) {
            const previewStories = dragManager.getColumnStories(col.state.id);
            if (previewStories) {
              const idx = previewStories.findIndex((s) => s.id === activeId);
              if (idx >= 0 && col.state.id !== fromStateId) { crossStateId = col.state.id; crossIdx = idx; break; }
            }
          }
        }

      // If preview didn't capture it (throttle), compute from the over event
      if (!crossStateId) {
        for (const col of columns) {
          if (col.state.id === overId || col.stories.some((s) => s.id === overId)) {
            if (col.state.id !== fromStateId) {
              crossStateId = col.state.id;
              // Compute insert index from the over position
              const overIdx = col.stories.findIndex((s) => s.id === overId);
              const r = active.rect.current.translated;
              const belowMid = overIdx >= 0 && r ? r.top + r.height / 2 > over.rect.top + over.rect.height / 2 : false;
              crossIdx = getStateBoardPreviewInsertIndex({
                toStateType: col.state.state_type,
                overId,
                toStateId: col.state.id,
                overIdx,
                columnLength: col.stories.length,
                pointerBelowMid: belowMid,
              });
            }
            break;
          }
        }
      }

      if (crossStateId) {
        logPMDnD('drag_end_cross_state', {
          trace_id: debugTraceID,
          story_id: activeId,
          over_id: overId,
          from_state_id: fromStateId,
          to_state_id: crossStateId,
          to_index: crossIdx,
        });
        await commitDropBeforeClearingPreview({
          commit: () => moveTask({
            workspaceId,
            storyId: activeId,
            fromStateId,
            toStateId: crossStateId,
            toIndex: crossIdx,
            debugTraceID,
          }),
          clearPreview: clearDragPreview,
        });
      } else {
        // Same-column reorder
        if (activeId === overId) { clearDragPreview(); return; }
        const toStateId = findStateIdByItemId(overId);
        if (!toStateId || fromStateId !== toStateId) { clearDragPreview(); return; }
        const fromColumn = columns.find((c) => c.state.id === fromStateId);
        if (!fromColumn) { clearDragPreview(); return; }
        if (fromColumn.state.state_type === 'done') {
          logPMDnD('drag_end_done_column_noop', { trace_id: debugTraceID, story_id: activeId, state_id: fromStateId, over_id: overId });
          clearDragPreview();
          return;
        }
        const fromIndex = fromColumn.stories.findIndex((s) => s.id === activeId);
        const overIndex = overId === toStateId
          ? fromColumn.stories.length - 1
          : fromColumn.stories.findIndex((s) => s.id === overId);
        const toIndex = getSameStateBoardDropIndex({ overId, stateId: toStateId, overIndex, columnLength: fromColumn.stories.length });
        if (fromIndex < 0 || overIndex < 0 || fromIndex === toIndex) { clearDragPreview(); return; }
        logPMDnD('drag_end_same_state', { trace_id: debugTraceID, story_id: activeId, over_id: overId, state_id: toStateId, state_type: fromColumn.state.state_type, from_index: fromIndex, over_index: overIndex, to_index: toIndex });
        await commitDropBeforeClearingPreview({
          commit: () => moveTask({ workspaceId, storyId: activeId, fromStateId, toStateId, toIndex, debugTraceID }),
          clearPreview: clearDragPreview,
        });
      }
    },
    [columns, memberColumns, groupBy, findStateIdByItemId, moveTask, moveMemberTask, workspaceId, clearDragPreview, dragManager]
  );

  const handleCreate = useCallback(
    async (payload: CreateTaskRequest) => {
      const story = await createTask(payload);
      return story ? { id: story.id } : undefined;
    },
    [createTask]
  );

  const handleStoryPatched = useCallback((story: Task) => {
    // Enrich with owner_name for board display (update API doesn't include it)
    const ownerKey = story.owner_member_id;
    if (ownerKey && !story.owner_name) {
      const ownerName = ownerNameMap.get(ownerKey);
      if (ownerName) story = { ...story, owner_name: ownerName };
    }
    if (groupBy === 'members') {
      // Enrich with state info from workflow columns (read from store directly to avoid dep)
      const stateColumns = usePMBoardStore.getState().columns;
      const stateCol = stateColumns.find((c) => c.state.id === story.workflow_state_id);
      if (stateCol) {
        story = { ...story, state_name: stateCol.state.name, state_type: stateCol.state.state_type, state_color: stateCol.state.color };
      }
      // Optimistically patch the story in member columns
      const cols = usePMBoardStore.getState().memberColumns;
      const updated = cols.map((col) => {
        const idx = col.stories.findIndex((s) => s.id === story.id);
        if (idx < 0) return col;
        const stories = [...col.stories];
        stories[idx] = { ...stories[idx], ...story };
        return { ...col, stories };
      });
      usePMBoardStore.setState({ memberColumns: updated });
    } else {
      const patched = patchTask('updated', story.id, story);
      if (!patched) {
        refreshBoard();
      }
    }
  }, [patchTask, refreshBoard, ownerNameMap, groupBy]);

  // Memoize context values to avoid re-rendering all consumers
  const boardData = useMemo<import('./KanbanBoard.contexts').BoardDataContextValue>(() => ({
    workspaceId, ownerNameMap, agentById, assignableMembers, automatedStateIds, findTeamName: resolveTeamName,
  }), [workspaceId, ownerNameMap, agentById, assignableMembers, automatedStateIds, resolveTeamName]);

  // Use a ref so the context value identity never changes — consumers never
  // re-render from callback identity shifts (e.g. ownerNameMap refetch).
  const boardCallbacksRef = useRef<import('./KanbanBoard.contexts').BoardCallbacksContextValue>({
    onStoryPatched: handleStoryPatched,
    onOpen: openStory,
    onCreate: handleCreateForState,
    onCreateForMember: handleCreateForMember,
    onToggleCollapse: toggleCollapse,
    onLoadMore: loadMoreColumn,
    onLoadMoreMember: loadMoreMemberColumn,
  });
  boardCallbacksRef.current = {
    onStoryPatched: handleStoryPatched,
    onOpen: openStory,
    onCreate: handleCreateForState,
    onCreateForMember: handleCreateForMember,
    onToggleCollapse: toggleCollapse,
    onLoadMore: loadMoreColumn,
    onLoadMoreMember: loadMoreMemberColumn,
  };

  return (
    <StoryFilterProvider
      workspaceId={workspaceId}
      assignableMembers={assignableMembers}
      activeTeamId={storeTeamId}
      userMemberships={userMemberships}
      labels={refLabels}
      epics={refEpics}
      sprints={refSprints}
      onChange={setFilters}
      externalFilters={filters}
    >
    <div className="flex h-full min-h-0 flex-col">
      {currentUser && (
        <ViewBar workspaceId={workspaceId} currentUserId={currentUser.id} />
      )}
      <header className="ui-divider-bottom-fade flex min-h-11 flex-wrap items-center gap-2 px-3 py-2">
        <StoryFilterTrigger />
        <StoryOwnerAvatarFilterRow />

        {/* Team selector — only shown when no team is pre-selected via URL */}
        {!teamId && teams.length > 0 && (
          <Select
            value={storeTeamId ?? '__all__'}
            onValueChange={(value) => setTeamFilter(value === '__all__' ? null : value)}
          >
            <SelectTrigger className="h-7 w-auto gap-1.5 text-xs px-2.5">
              <span className="text-muted-foreground">Team:</span>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">All teams</SelectItem>
              {teams.map((t) => (
                <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
        <div className="ml-auto flex items-center gap-1 self-center">
          <BoardToolbarSlot>
            {viewMode === 'board' ? (
              <div className="inline-flex h-7 items-center rounded-md border border-input bg-muted/40 p-0.5 text-xs">
                <button
                  className={`rounded px-2 py-0.5 transition-colors ${groupBy === 'status' ? 'bg-background font-medium text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}`}
                  onClick={() => setGroupBy('status')}
                >
                  By States
                </button>
                <button
                  className={`rounded px-2 py-0.5 transition-colors ${groupBy === 'members' ? 'bg-background font-medium text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}`}
                  onClick={() => setGroupBy('members')}
                >
                  By Members
                </button>
              </div>
            ) : (
              <Select value={listGroupBy} onValueChange={(value) => setListGroupBy(value as TaskListGroupByOption)}>
                <SelectTrigger className="h-7 w-auto min-w-[150px] max-w-[190px] gap-1 border-0 bg-transparent px-1.5 text-xs shadow-none hover:bg-accent focus-visible:ring-0 focus-visible:border-transparent">
                  <span className="shrink-0 text-muted-foreground">Group by:</span>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {listGroupOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </BoardToolbarSlot>
          <BoardToolbarSlot>
            {viewMode === 'board' ? <BoardDisplayMenu /> : <ListDisplayMenu disabledKeys={listDisabledKeys} />}
          </BoardToolbarSlot>
          <BoardToolbarSlot className="gap-1">
            <QuickTooltip label="Board view">
              <Button
                variant={viewMode === 'board' ? 'default' : 'ghost'}
                size="icon"
                className="h-7 w-7"
                onClick={() => setViewMode('board')}
              >
                <Columns2 className="h-4 w-4" />
              </Button>
            </QuickTooltip>
            <QuickTooltip label="List view">
              <Button
                variant={viewMode === 'list' ? 'default' : 'ghost'}
                size="icon"
                className="h-7 w-7"
                onClick={() => setViewMode('list')}
              >
                <LayoutList className="h-4 w-4" />
              </Button>
            </QuickTooltip>
          </BoardToolbarSlot>
        </div>

      </header>

      <StoryFilterBar />

      {error ? (
        <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      {loading ? (
        <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          Loading board...
        </div>
      ) : null}

      {!loading && viewMode === 'board' ? (
        <BoardDataContext.Provider value={boardData}>
        <BoardCallbacksContext.Provider value={boardCallbacksRef}>
        <DragPreviewContext.Provider value={dragManager}>
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragStart={onDragStart}
          onDragOver={onDragOver}
          onDragEnd={onDragEnd}
          onDragCancel={clearDragPreview}
        >
          <div className="min-h-0 flex-1 overflow-x-auto">
            <div className="flex h-full min-w-full gap-3 pb-2">
              {groupBy === 'members' ? (
                memberColumns.filter((col) => showEmptyColumns || isDragging || col.story_count > 0).map((col) => {
                  const colKey = col.member?.id ?? '__unassigned__';
                  return (
                    <MemberColumn
                      key={colKey}
                      column={col}
                      collapsed={collapsedColumns.has(colKey)}
                      isLoadingMore={!!memberColumnLoading[colKey]}
                    />
                  );
                })
              ) : (
                columns.filter((column) => showEmptyColumns || isDragging || column.task_count > 0).map((column) => (
                  <Column
                    key={column.state.id}
                    column={column}
                    collapsed={collapsedColumns.has(column.state.id)}
                    isLoadingMore={!!columnLoading[column.state.id]}
                  />
                ))
              )}
            </div>
          </div>

          <DragOverlay>
            <DragOverlayCard manager={dragManager} resolveTeamName={resolveTeamName} agentById={agentById} groupBy={groupBy} />
          </DragOverlay>
        </DndContext>
        </DragPreviewContext.Provider>
        </BoardCallbacksContext.Provider>
        </BoardDataContext.Provider>
      ) : null}

      {!loading && viewMode === 'list' && workflow ? (
        <TaskListView
          workspaceId={workspaceId}
          workflow={workflow}
          teams={teams}
          assignableMembers={assignableMembers}
          epics={refEpics}
          sprints={refSprints}
          filters={filters}
          teamId={storeTeamId}
          onOpenTask={openStory}
          groupBy={listGroupBy}
          onGroupByChange={setListGroupBy}
          showToolbar={false}
        />
      ) : null}

      {workflow ? (
        <CreateTaskModal
          open={createOpen}
          onOpenChange={setCreateOpen}
          workspaceId={workspaceId}
          workflow={workflow}
          initialStateId={createStateId || workflow.states[0]?.id || ''}
          initialTeamId={storeTeamId ?? undefined}
          initialOwnerMemberId={createOwnerMemberId}
          onCreate={handleCreate}
        />
      ) : null}

    </div>
    </StoryFilterProvider>
  );
}
