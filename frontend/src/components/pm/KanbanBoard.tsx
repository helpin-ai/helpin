import { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react';
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
import type { Agent, CreateStoryRequest, Story, StoryMemberColumn, StoryStateColumn, Label, EpicWithStats, SprintWithStats } from '@/lib/pmTypes';
import type { AssignableMember } from '@/lib/types';
import { agentService } from '@/lib/services/agentService';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { pmLabelService } from '@/lib/services/pmLabelService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmSprintService } from '@/lib/services/pmSprintService';
import { StateTypeIcon } from '@/lib/pmConstants';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { useAuthStore } from '@/stores/authStore';
import { useSession, useAutomationRulesByWorkflow } from '@/hooks/queries';
import { UserAvatar } from './UserAvatar';
import { StoryCard } from './StoryCard';
import { CreateStoryModal } from './CreateStoryModal';
import { useStoryPanelStore } from '@/stores/storyPanelStore';
import { StoryFilterProvider, StoryFilterTrigger, StoryFilterBar } from './StoryFilters';
import { StoryListView } from './StoryListView';
import { ViewBar } from './ViewBar';
import { BoardDisplayMenu } from './BoardDisplayMenu';
import { useBoardDisplayStore } from '@/stores/boardDisplayStore';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
import { createPMDnDTraceID, logPMDnD } from '@/lib/pmDnDDebug';
import { commitDropBeforeClearingPreview, getSameStateBoardDropIndex, getStateBoardPreviewInsertIndex } from './KanbanBoard.dnd';

interface KanbanBoardProps {
  workspaceId: string;
  teamId?: string;
}

interface ColumnProps {
  column: StoryStateColumn;
  collapsed: boolean;
  onToggleCollapse: (stateId: string) => void;
  onCreate: (stateId: string) => void;
  onOpen: (story: Story) => void;
  findTeamName: (teamId: string | undefined) => string | undefined;
  workspaceId: string;
  assignableMembers: AssignableMember[];
  ownerNameMap: Map<string, string>;
  agentById: Map<string, Agent>;
  onOwnerChanged: (story: Story) => void;
  onPriorityChanged: (story: Story) => void;
  onSeverityChanged: (story: Story) => void;
  onEstimateChanged: (story: Story) => void;
  automatedStateIds?: Set<string>;
  onLoadMore: (stateId: string) => void;
  isLoadingMore: boolean;
}

const Column = memo(function Column({ column, collapsed, onToggleCollapse, onCreate, onOpen, findTeamName, workspaceId, assignableMembers, ownerNameMap, agentById, onOwnerChanged, onPriorityChanged, onSeverityChanged, onEstimateChanged, onLoadMore, isLoadingMore, automatedStateIds }: ColumnProps) {
  const { setNodeRef, isOver } = useDroppable({ id: column.state.id });
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const loadMoreRef = useRef<HTMLDivElement | null>(null);
  const groupedStories = column.state.state_type === 'done' ? column.story_groups ?? [] : [];

  useEffect(() => {
    if (!column.has_more || isLoadingMore || !scrollRef.current || !loadMoreRef.current) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          onLoadMore(column.state.id);
        }
      },
      {
        root: scrollRef.current,
        rootMargin: '0px 0px 160px 0px',
      },
    );

    observer.observe(loadMoreRef.current);
    return () => observer.disconnect();
  }, [column.has_more, column.state.id, column.stories.length, isLoadingMore, onLoadMore]);

  if (collapsed) {
    return (
      <QuickTooltip label={`Expand ${column.state.name}`}>
      <section
        ref={setNodeRef}
        className={cn(
          'relative flex h-full w-[44px] shrink-0 cursor-pointer flex-col items-center rounded-md border border-border/50 bg-muted/30 transition-colors hover:bg-muted/50',
          isOver && 'bg-primary/10 ring-2 ring-inset ring-primary/30',
        )}
        onClick={() => onToggleCollapse(column.state.id)}
      >
        {column.state.color && (
          <div className="absolute top-0 left-2 right-2 h-[3px] rounded-b-full" style={{ backgroundColor: column.state.color }} />
        )}
        <ChevronsLeftRight className="mt-3 mb-2 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        <StateTypeIcon stateType={column.state.state_type} className="mb-2 h-4 w-4 shrink-0" />
        {automatedStateIds?.has(column.state.id) && (
          <Bot className="mb-1 h-3.5 w-3.5 shrink-0 text-violet-500" />
        )}
        <span className="text-xs font-medium text-muted-foreground">{column.story_count}</span>
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
    <section className="flex h-full w-[340px] shrink-0 flex-col">
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
            <QuickTooltip label={`${column.story_count} ${column.story_count === 1 ? 'story' : 'stories'}`}>
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
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 opacity-0 group-hover/header:opacity-100 transition-opacity"
              onClick={() => onToggleCollapse(column.state.id)}
            >
              <ChevronsRightLeft className="h-3.5 w-3.5" />
            </Button>
          </QuickTooltip>
          <QuickTooltip label="Create story">
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => onCreate(column.state.id)}>
              <Plus className="h-4 w-4" />
            </Button>
          </QuickTooltip>
        </div>
      </header>

      <SortableContext items={column.stories.map((story) => story.id)} strategy={verticalListSortingStrategy}>
        <div
          ref={(node) => {
            setNodeRef(node);
            scrollRef.current = node;
          }}
          className={`min-h-0 flex-1 overflow-y-auto p-2 flex flex-col rounded-md transition-all duration-200 ${
            isOver ? 'bg-primary/10 ring-2 ring-inset ring-primary/30 gap-4' : 'gap-2'
          }`}
        >
          {groupedStories.length > 0 ? (
            groupedStories.map((group) => (
              <div key={group.key} className="space-y-2">
                <div className="rounded-md bg-muted px-3 py-1 text-center text-xs font-semibold text-muted-foreground">
                  {group.label}
                </div>
                {group.stories.map((story) => (
                  <StoryCard
                    key={story.id}
                    story={story}
                    onOpen={onOpen}
                    teamName={findTeamName(story.team_id)}
                    workspaceId={workspaceId}
                    assignableMembers={assignableMembers}
                    ownerNameMap={ownerNameMap}
                    assignedAgent={story.assigned_agent_id ? agentById.get(story.assigned_agent_id) ?? null : null}
                    onOwnerChanged={onOwnerChanged}
                    onPriorityChanged={onPriorityChanged}
                    onSeverityChanged={onSeverityChanged}
                    onEstimateChanged={onEstimateChanged}
                  />
                ))}
              </div>
            ))
          ) : (
            column.stories.map((story) => (
              <StoryCard
                key={story.id}
                story={story}
                onOpen={onOpen}
                teamName={findTeamName(story.team_id)}
                workspaceId={workspaceId}
                assignableMembers={assignableMembers}
                ownerNameMap={ownerNameMap}
                assignedAgent={story.assigned_agent_id ? agentById.get(story.assigned_agent_id) ?? null : null}
                onOwnerChanged={onOwnerChanged}
                onPriorityChanged={onPriorityChanged}
                onSeverityChanged={onSeverityChanged}
                onEstimateChanged={onEstimateChanged}
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
                <span>{column.story_count - column.stories.length} remaining</span>
              )}
            </div>
          ) : null}

          <Button
            variant="ghost"
            className="w-full justify-start text-xs text-muted-foreground"
            onClick={() => onCreate(column.state.id)}
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
  column: StoryMemberColumn;
  collapsed: boolean;
  onToggleCollapse: (key: string) => void;
  onCreate: (memberId: string | null) => void;
  onOpen: (story: Story) => void;
  findTeamName: (teamId: string | undefined) => string | undefined;
  workspaceId: string;
  assignableMembers: AssignableMember[];
  ownerNameMap: Map<string, string>;
  onOwnerChanged: (story: Story) => void;
  onPriorityChanged: (story: Story) => void;
  onSeverityChanged: (story: Story) => void;
  onEstimateChanged: (story: Story) => void;
  onLoadMore: (memberId: string | null) => void;
  isLoadingMore: boolean;
}

const MemberColumn = memo(function MemberColumn({ column, collapsed, onToggleCollapse, onCreate, onOpen, findTeamName, workspaceId, assignableMembers, ownerNameMap, onOwnerChanged, onPriorityChanged, onSeverityChanged, onEstimateChanged, onLoadMore, isLoadingMore }: MemberColumnProps) {
  const colKey = column.member?.id ?? '__unassigned__';
  const { setNodeRef, isOver } = useDroppable({ id: colKey });
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const loadMoreRef = useRef<HTMLDivElement | null>(null);
  const displayName = column.member?.display_name ?? 'Unassigned';

  useEffect(() => {
    if (!column.has_more || isLoadingMore || !scrollRef.current || !loadMoreRef.current) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          onLoadMore(column.member?.id ?? null);
        }
      },
      { root: scrollRef.current, rootMargin: '0px 0px 160px 0px' },
    );
    observer.observe(loadMoreRef.current);
    return () => observer.disconnect();
  }, [column.has_more, column.member?.id, column.stories.length, isLoadingMore, onLoadMore]);

  if (collapsed) {
    return (
      <QuickTooltip label={`Expand ${displayName}`}>
        <section
          ref={setNodeRef}
          className={cn(
            'relative flex h-full w-[44px] shrink-0 cursor-pointer flex-col items-center rounded-md border border-border/50 bg-muted/30 pt-3 transition-colors hover:bg-muted/50',
            isOver && 'bg-primary/10 ring-2 ring-inset ring-primary/30',
          )}
          onClick={() => onToggleCollapse(colKey)}
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
            <QuickTooltip label={`${column.story_count} ${column.story_count === 1 ? 'story' : 'stories'}`}>
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
            <Button variant="ghost" size="icon" className="h-7 w-7 opacity-0 group-hover/header:opacity-100 transition-opacity" onClick={() => onToggleCollapse(colKey)}>
              <ChevronsRightLeft className="h-3.5 w-3.5" />
            </Button>
          </QuickTooltip>
          <QuickTooltip label="Create story">
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => onCreate(column.member?.id ?? null)}>
              <Plus className="h-4 w-4" />
            </Button>
          </QuickTooltip>
        </div>
      </header>

      <SortableContext items={column.stories.map((story) => story.id)} strategy={verticalListSortingStrategy}>
        <div
          ref={(node) => {
            setNodeRef(node);
            scrollRef.current = node;
          }}
          className={`min-h-0 flex-1 overflow-y-auto p-2 flex flex-col rounded-md transition-all duration-200 ${isOver ? 'bg-primary/10 ring-2 ring-inset ring-primary/30 gap-4' : 'gap-2'}`}
        >
          {column.stories.map((story) => (
            <StoryCard
              key={story.id}
              story={story}
              onOpen={onOpen}
              teamName={findTeamName(story.team_id)}
              workspaceId={workspaceId}
              assignableMembers={assignableMembers}
              ownerNameMap={ownerNameMap}
              onOwnerChanged={onOwnerChanged}
              onPriorityChanged={onPriorityChanged}
              onSeverityChanged={onSeverityChanged}
              onEstimateChanged={onEstimateChanged}
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
            onClick={() => onCreate(column.member?.id ?? null)}
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

export function KanbanBoard({ workspaceId, teamId }: KanbanBoardProps) {
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
  const createStory = usePMBoardStore((state) => state.createStory);
  const moveStory = usePMBoardStore((state) => state.moveStory);
  const patchStory = usePMBoardStore((state) => state.patchStory);
  const refreshBoard = usePMBoardStore((state) => state.refreshBoard);
  const loadMoreColumn = usePMBoardStore((state) => state.loadMoreColumn);
  const loadViews = usePMBoardStore((state) => state.loadViews);
  const memberColumns = usePMBoardStore((state) => state.memberColumns);
  const memberColumnLoading = usePMBoardStore((state) => state.memberColumnLoading);
  const loadMemberBoard = usePMBoardStore((state) => state.loadMemberBoard);
  const loadMoreMemberColumn = usePMBoardStore((state) => state.loadMoreMemberColumn);
  const moveMemberStory = usePMBoardStore((state) => state.moveMemberStory);

  const currentUser = useAuthStore((s) => s.user);
  const { data: sessionMembership } = useSession(workspaceId);
  const currentMemberId = sessionMembership?.id;
  const { teams, findTeamName } = useAccessibleTeams(workspaceId);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
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
  const [agents, setAgents] = useState<Agent[]>([]);

  useEffect(() => {
    pmLabelService.list(workspaceId).then((r) => { if (r.data) setRefLabels(r.data); });
    pmEpicService.list(workspaceId).then((r) => { if (r.data) setRefEpics(r.data); });
    pmSprintService.list(workspaceId).then((r) => { if (r.data) setRefSprints(r.data); });
    agentService.list(workspaceId).then((r) => { if (r.data) setAgents(r.data); });
  }, [workspaceId]);
  const agentById = useMemo(
    () => new Map(agents.map((agent) => [agent.id, agent])),
    [agents],
  );

  const [activeStory, setActiveStory] = useState<Story | null>(null);
  const [dragPreviewColumns, setDragPreviewColumns] = useState<StoryStateColumn[] | null>(null);
  const [dragPreviewMemberColumns, setDragPreviewMemberColumns] = useState<StoryMemberColumn[] | null>(null);
  const displayColumns = dragPreviewColumns ?? columns;
  const displayMemberColumns = dragPreviewMemberColumns ?? memberColumns;
  const [createOpen, setCreateOpen] = useState(false);
  const [createStateId, setCreateStateId] = useState<string>('');
  const [createOwnerMemberId, setCreateOwnerMemberId] = useState<string | undefined>(undefined);
  const openStoryPanel = useStoryPanelStore((s) => s.openStory);
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
    window.addEventListener('story-created', handler);
    return () => window.removeEventListener('story-created', handler);
  }, [refreshBoard, groupBy, loadMemberBoard, showEmptyColumns, activeMemberIds]);

  // Open ?story= URL param in global panel on mount
  useEffect(() => {
    if (!workflow) return;
    const maybeStory = new URLSearchParams(window.location.search).get('story');
    if (!maybeStory) return;
    const match = maybeStory.match(/^(\d+)$/);
    if (!match) return;
    (async () => {
      const res = await pmStoryService.getByDisplayId(workspaceId, Number(match[1]));
      if (res.data) openStoryPanel(res.data.story.id);
    })();
  }, [workspaceId, workflow, openStoryPanel]);

  const openStory = useCallback(
    (story: Story) => openStoryPanel(story.id),
    [openStoryPanel]
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
      const story = { ...updated.story };
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
        const patched = cols.map((col: StoryMemberColumn) => {
          const idx = col.stories.findIndex((s) => s.id === story.id);
          if (idx < 0) return col;
          const stories = [...col.stories];
          stories[idx] = { ...stories[idx], ...story };
          return { ...col, stories };
        });
        usePMBoardStore.setState({ memberColumns: patched });
      } else {
        if (!patchStory('updated', story.id, story)) refreshBoard();
      }
    };
    const onArchived = (e: Event) => {
      const storyId = (e as CustomEvent)?.detail?.storyId;
      if (!storyId) return;
      if (groupBy === 'members') {
        const cols = usePMBoardStore.getState().memberColumns;
        const updated = cols.map((col: StoryMemberColumn) => {
          const idx = col.stories.findIndex((s) => s.id === storyId);
          if (idx < 0) return col;
          return { ...col, stories: col.stories.filter((s) => s.id !== storyId), story_count: col.story_count - 1 };
        });
        usePMBoardStore.setState({ memberColumns: updated });
      } else {
        if (!patchStory('deleted', storyId)) refreshBoard();
      }
    };
    window.addEventListener('story-panel-updated', onUpdated);
    window.addEventListener('story-panel-archived', onArchived);
    return () => {
      window.removeEventListener('story-panel-updated', onUpdated);
      window.removeEventListener('story-panel-archived', onArchived);
    };
  }, [groupBy, columns, ownerNameMap, patchStory, refreshBoard]);

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

  const findMemberKeyByItemId = useCallback(
    (id: string): string | null => {
      for (const column of memberColumns) {
        const colKey = column.member?.id ?? '__unassigned__';
        if (colKey === id) return colKey;
        if (column.stories.some((story) => story.id === id)) return colKey;
      }
      return null;
    },
    [memberColumns]
  );

  const onDragStart = useCallback(
    (event: DragStartEvent) => {
      const allStories = groupBy === 'members'
        ? memberColumns.flatMap((col) => col.stories)
        : columns.flatMap((column) => column.stories);
      const story = allStories.find((item) => item.id === String(event.active.id));
      setActiveStory(story ?? null);
      // Initialize preview columns for cross-column displacement
      if (groupBy === 'members') {
        setDragPreviewMemberColumns(memberColumns.map((c) => ({ ...c, stories: [...c.stories] })));
      } else {
        setDragPreviewColumns(columns.map((c) => ({ ...c, stories: [...c.stories], story_groups: c.story_groups?.map((g) => ({ ...g, stories: [...g.stories] })) ?? [] })));
      }
    },
    [columns, memberColumns, groupBy]
  );

  const onDragOver = useCallback(
    (event: DragOverEvent) => {
      const { active, over } = event;
      if (!over) return;
      const activeId = String(active.id);
      const overId = String(over.id);
      if (activeId === overId) return;

      if (groupBy === 'members') {
        const cols = dragPreviewMemberColumns;
        if (!cols) return;
        let fromKey: string | null = null;
        let toKey: string | null = null;
        for (const col of cols) {
          const key = col.member?.id ?? '__unassigned__';
          if (col.stories.some((s) => s.id === activeId)) fromKey = key;
          if (key === overId || col.stories.some((s) => s.id === overId)) toKey = key;
        }
        if (!fromKey || !toKey || fromKey === toKey) return;

        const next = cols.map((c) => ({ ...c, stories: [...c.stories] }));
        const fromCol = next.find((c) => (c.member?.id ?? '__unassigned__') === fromKey);
        const toCol = next.find((c) => (c.member?.id ?? '__unassigned__') === toKey);
        if (!fromCol || !toCol) return;
        const idx = fromCol.stories.findIndex((s) => s.id === activeId);
        if (idx < 0) return;
        const [story] = fromCol.stories.splice(idx, 1);
        let insertIdx: number;
        if (overId === toKey) {
          insertIdx = toCol.stories.length;
        } else {
          const overIdx = toCol.stories.findIndex((s) => s.id === overId);
          insertIdx = overIdx >= 0 ? overIdx : toCol.stories.length;
          if (overIdx >= 0) {
            const r = active.rect.current.translated;
            const belowMid = r ? r.top + r.height / 2 > over.rect.top + over.rect.height / 2 : false;
            if (belowMid) insertIdx = overIdx + 1;
          }
        }
        toCol.stories.splice(insertIdx, 0, story);
        setDragPreviewMemberColumns(next);
      } else {
        const cols = dragPreviewColumns;
        if (!cols) return;
        let fromStateId: string | null = null;
        let toStateId: string | null = null;
        for (const col of cols) {
          if (col.stories.some((s) => s.id === activeId)) fromStateId = col.state.id;
          if (col.state.id === overId || col.stories.some((s) => s.id === overId)) toStateId = col.state.id;
        }
        if (!fromStateId || !toStateId || fromStateId === toStateId) return;

        const next = cols.map((c) => ({ ...c, stories: [...c.stories], story_groups: c.story_groups?.map((g) => ({ ...g, stories: [...g.stories] })) ?? [] }));
        const fromCol = next.find((c) => c.state.id === fromStateId);
        const toCol = next.find((c) => c.state.id === toStateId);
        if (!fromCol || !toCol) return;
        const idx = fromCol.stories.findIndex((s) => s.id === activeId);
        if (idx < 0) return;
        const [story] = fromCol.stories.splice(idx, 1);
        const overIdx = toCol.stories.findIndex((s) => s.id === overId);
        const r = active.rect.current.translated;
        const belowMid = overIdx >= 0 && r ? r.top + r.height / 2 > over.rect.top + over.rect.height / 2 : false;
        const insertIdx = getStateBoardPreviewInsertIndex({
          toStateType: toCol.state.state_type,
          overId,
          toStateId,
          overIdx,
          columnLength: toCol.stories.length,
          pointerBelowMid: belowMid,
        });
        toCol.stories.splice(insertIdx, 0, story);
        setDragPreviewColumns(next);
      }
    },
    [groupBy, dragPreviewColumns, dragPreviewMemberColumns],
  );

  const clearDragPreview = useCallback(() => {
    setActiveStory(null);
    setDragPreviewColumns(null);
    setDragPreviewMemberColumns(null);
  }, []);

  const onDragEnd = useCallback(
    async (event: DragEndEvent) => {
      // Capture preview state before clearing — it tells us where the story ended up
      const savedPreviewCols = dragPreviewColumns;
      const savedPreviewMemCols = dragPreviewMemberColumns;

      const { active, over } = event;
      if (!over) {
        clearDragPreview();
        return;
      }
      const activeId = String(active.id);
      const overId = String(over.id);

      if (groupBy === 'members') {
        const fromKey = findMemberKeyByItemId(activeId);
        if (!fromKey) {
          clearDragPreview();
          return;
        }

        // Check if onDragOver moved story cross-column in preview
        let crossKey: string | null = null;
        let crossIdx = 0;
        if (savedPreviewMemCols) {
          for (const col of savedPreviewMemCols) {
            const key = col.member?.id ?? '__unassigned__';
            const idx = col.stories.findIndex((s) => s.id === activeId);
            if (idx >= 0 && key !== fromKey) { crossKey = key; crossIdx = idx; break; }
          }
        }

        if (crossKey) {
          // Cross-member: use preview target
          const fromColumn = memberColumns.find((c) => (c.member?.id ?? '__unassigned__') === fromKey);
          const toColumn = memberColumns.find((c) => (c.member?.id ?? '__unassigned__') === crossKey);
          if (!fromColumn || !toColumn) {
            clearDragPreview();
            return;
          }
          await commitDropBeforeClearingPreview({
            commit: () => moveMemberStory({
              workspaceId,
              storyId: activeId,
              fromMemberId: fromColumn.member?.id ?? null,
              toMemberId: toColumn.member?.id ?? null,
              toIndex: crossIdx,
            }),
            clearPreview: clearDragPreview,
          });
        } else {
          clearDragPreview();
        }
        return;
      }

      // ── State board ──
      const fromStateId = findStateIdByItemId(activeId);
      if (!fromStateId) {
        clearDragPreview();
        return;
      }
      const debugTraceID = createPMDnDTraceID();

      // Check if onDragOver moved story cross-column in preview
      let crossStateId: string | null = null;
      let crossIdx = 0;
      if (savedPreviewCols) {
        for (const col of savedPreviewCols) {
          const idx = col.stories.findIndex((s) => s.id === activeId);
          if (idx >= 0 && col.state.id !== fromStateId) { crossStateId = col.state.id; crossIdx = idx; break; }
        }
      }

      if (crossStateId) {
        // Cross-column: use preview target
        logPMDnD('drag_end_cross_state', {
          trace_id: debugTraceID,
          story_id: activeId,
          over_id: overId,
          from_state_id: fromStateId,
          to_state_id: crossStateId,
          to_index: crossIdx,
        });
        await commitDropBeforeClearingPreview({
          commit: () => moveStory({
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
        // Same-column reorder: use over.id with arrayMove semantics
        if (activeId === overId) {
          clearDragPreview();
          return;
        }
        const toStateId = findStateIdByItemId(overId);
        if (!toStateId || fromStateId !== toStateId) {
          clearDragPreview();
          return;
        }
        const fromColumn = columns.find((c) => c.state.id === fromStateId);
        if (!fromColumn) {
          clearDragPreview();
          return;
        }
        if (fromColumn.state.state_type === 'done') {
          logPMDnD('drag_end_done_column_noop', {
            trace_id: debugTraceID,
            story_id: activeId,
            state_id: fromStateId,
            over_id: overId,
          });
          clearDragPreview();
          return;
        }
        const fromIndex = fromColumn.stories.findIndex((s) => s.id === activeId);
        const overIndex = overId === toStateId
          ? fromColumn.stories.length - 1
          : fromColumn.stories.findIndex((s) => s.id === overId);
        const toIndex = getSameStateBoardDropIndex({
          overId,
          stateId: toStateId,
          overIndex,
          columnLength: fromColumn.stories.length,
        });
        if (fromIndex < 0 || overIndex < 0 || fromIndex === toIndex) {
          clearDragPreview();
          return;
        }
        logPMDnD('drag_end_same_state', {
          trace_id: debugTraceID,
          story_id: activeId,
          over_id: overId,
          state_id: toStateId,
          state_type: fromColumn.state.state_type,
          from_index: fromIndex,
          over_index: overIndex,
          to_index: toIndex,
        });
        await commitDropBeforeClearingPreview({
          commit: () => moveStory({
            workspaceId,
            storyId: activeId,
            fromStateId,
            toStateId,
            toIndex,
            debugTraceID,
          }),
          clearPreview: clearDragPreview,
        });
      }
    },
    [columns, memberColumns, groupBy, findStateIdByItemId, findMemberKeyByItemId, moveStory, moveMemberStory, workspaceId, clearDragPreview, dragPreviewColumns, dragPreviewMemberColumns]
  );

  const handleCreate = useCallback(
    async (payload: CreateStoryRequest) => {
      const story = await createStory(payload);
      return story ? { id: story.id } : undefined;
    },
    [createStory]
  );

  const handleStoryPatched = useCallback((story: Story) => {
    // Enrich with owner_name for board display (update API doesn't include it)
    const ownerKey = story.owner_member_id;
    if (ownerKey && !story.owner_name) {
      const ownerName = ownerNameMap.get(ownerKey);
      if (ownerName) story = { ...story, owner_name: ownerName };
    }
    if (groupBy === 'members') {
      // Enrich with state info from workflow columns
      const stateCol = columns.find((c) => c.state.id === story.workflow_state_id);
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
      const patched = patchStory('updated', story.id, story);
      if (!patched) {
        refreshBoard();
      }
    }
  }, [patchStory, refreshBoard, ownerNameMap, groupBy, columns]);

  return (
    <StoryFilterProvider
      assignableMembers={assignableMembers}
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
      <header className="flex flex-wrap items-center gap-2 border-b border-border/70 px-3 py-2">
        <StoryFilterTrigger />

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

        <div className="ml-auto flex items-center gap-1">
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
          <BoardDisplayMenu />
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
                displayMemberColumns.filter((col) => showEmptyColumns || !!activeStory || col.story_count > 0).map((col) => {
                  const colKey = col.member?.id ?? '__unassigned__';
                  return (
                    <MemberColumn
                      key={colKey}
                      column={col}
                      collapsed={collapsedColumns.has(colKey)}
                      onToggleCollapse={toggleCollapse}
                      onCreate={handleCreateForMember}
                      onOpen={openStory}
                      findTeamName={resolveTeamName}
                      workspaceId={workspaceId}
                      assignableMembers={assignableMembers}
                      ownerNameMap={ownerNameMap}
                      onOwnerChanged={handleStoryPatched}
                      onPriorityChanged={handleStoryPatched}
                      onSeverityChanged={handleStoryPatched}
                      onEstimateChanged={handleStoryPatched}
                      onLoadMore={loadMoreMemberColumn}
                      isLoadingMore={!!memberColumnLoading[colKey]}
                    />
                  );
                })
              ) : (
                displayColumns.filter((column) => showEmptyColumns || !!activeStory || column.story_count > 0).map((column) => (
                  <Column
                    key={column.state.id}
                    column={column}
                    collapsed={collapsedColumns.has(column.state.id)}
                    onToggleCollapse={toggleCollapse}
                    onCreate={handleCreateForState}
                    onOpen={openStory}
                    findTeamName={resolveTeamName}
                    workspaceId={workspaceId}
                    assignableMembers={assignableMembers}
                    ownerNameMap={ownerNameMap}
                    agentById={agentById}
                    onOwnerChanged={handleStoryPatched}
                    onPriorityChanged={handleStoryPatched}
                    onSeverityChanged={handleStoryPatched}
                    onEstimateChanged={handleStoryPatched}
                    onLoadMore={loadMoreColumn}
                    isLoadingMore={!!columnLoading[column.state.id]}
                    automatedStateIds={automatedStateIds}
                  />
                ))
              )}
            </div>
          </div>

          <DragOverlay>
            {activeStory ? (
              <StoryCard
                story={activeStory}
                onOpen={() => {}}
                isOverlay
                teamName={resolveTeamName(activeStory.team_id)}
                ownerNameMap={ownerNameMap}
                assignedAgent={activeStory.assigned_agent_id ? agentById.get(activeStory.assigned_agent_id) ?? null : null}
                showStateBadge={groupBy === 'members'}
              />
            ) : null}
          </DragOverlay>
        </DndContext>
      ) : null}

      {!loading && viewMode === 'list' && workflow ? (
        <StoryListView
          workspaceId={workspaceId}
          workflow={workflow}
          teams={teams}
          assignableMembers={assignableMembers}
          epics={refEpics}
          sprints={refSprints}
          filters={filters}
          teamId={storeTeamId}
          onOpenStory={openStory}
        />
      ) : null}

      {workflow ? (
        <CreateStoryModal
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
