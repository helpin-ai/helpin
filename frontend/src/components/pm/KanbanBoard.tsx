import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  closestCorners,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
  useDroppable,
} from '@dnd-kit/core';
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { BarChart3, Columns2, LayoutList, Loader2, Maximize2, Minimize2, Plus, StickyNote, User } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { usePMBoardStore } from '@/stores/pmBoardStore';
import type { CreateStoryRequest, Story, StoryMemberColumn, StoryStateColumn, Label, EpicWithStats, SprintWithStats } from '@/lib/pmTypes';
import type { AssignableMember } from '@/lib/types';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { pmLabelService } from '@/lib/services/pmLabelService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmSprintService } from '@/lib/services/pmSprintService';
import { StateTypeIcon } from '@/lib/pmConstants';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { useAuthStore } from '@/stores/authStore';
import { useSession } from '@/hooks/queries';
import { UserAvatar } from './UserAvatar';
import { StoryCard } from './StoryCard';
import { CreateStoryModal } from './CreateStoryModal';
import { StoryDetailPanel } from './StoryDetailPanel';
import { StoryFilterProvider, StoryFilterTrigger, StoryFilterBar } from './StoryFilters';
import { StoryListView } from './StoryListView';
import { ViewBar } from './ViewBar';
import { BoardDisplayMenu } from './BoardDisplayMenu';
import { useBoardDisplayStore } from '@/stores/boardDisplayStore';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';

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
  onOwnerChanged: (story: Story) => void;
  onPriorityChanged: (story: Story) => void;
  onSeverityChanged: (story: Story) => void;
  onEstimateChanged: (story: Story) => void;
  onLoadMore: (stateId: string) => void;
  isLoadingMore: boolean;
}

function Column({ column, collapsed, onToggleCollapse, onCreate, onOpen, findTeamName, workspaceId, assignableMembers, ownerNameMap, onOwnerChanged, onPriorityChanged, onSeverityChanged, onEstimateChanged, onLoadMore, isLoadingMore }: ColumnProps) {
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
        className="relative flex h-full w-[44px] shrink-0 cursor-pointer flex-col items-center rounded-md border border-border/50 bg-muted/30 transition-colors hover:bg-muted/50"
        onClick={() => onToggleCollapse(column.state.id)}
      >
        {column.state.color && (
          <div className="absolute top-0 left-2 right-2 h-[3px] rounded-b-full" style={{ backgroundColor: column.state.color }} />
        )}
        <Maximize2 className="mb-3 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        <StateTypeIcon stateType={column.state.state_type} className="mb-2 h-4 w-4 shrink-0" />
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
    <section className="flex h-full w-[360px] shrink-0 flex-col">
      <header className="flex items-center justify-between px-3 pt-4 pb-3 relative">
        {column.state.color && (
          <div className="absolute top-0 left-3 right-3 h-[3px] rounded-b-full" style={{ backgroundColor: column.state.color }} />
        )}
        <div className="min-w-0">
          {column.state.description ? (
            <QuickTooltip label={column.state.description}>
              <p className="flex items-center gap-1.5 truncate text-sm font-semibold cursor-default">
                <StateTypeIcon stateType={column.state.state_type} className="h-4 w-4 shrink-0" />
                {column.state.name}
              </p>
            </QuickTooltip>
          ) : (
            <p className="flex items-center gap-1.5 truncate text-sm font-semibold">
              <StateTypeIcon stateType={column.state.state_type} className="h-4 w-4 shrink-0" />
              {column.state.name}
            </p>
          )}
          <p className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
            <span className="inline-flex items-center gap-1.5" title={`${column.story_count} items`}>
              <StickyNote className="h-3 w-3" />
              {column.story_count}
            </span>
            <span className="inline-flex items-center gap-1.5" title={`${column.point_total} pts`}>
              <BarChart3 className="h-3 w-3" />
              {column.point_total}
            </span>
          </p>
        </div>
        <div className="flex items-center gap-0.5">
          <QuickTooltip label="Collapse column">
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => onToggleCollapse(column.state.id)}
            >
              <Minimize2 className="h-3.5 w-3.5" />
            </Button>
          </QuickTooltip>
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => onCreate(column.state.id)}>
            <Plus className="h-4 w-4" />
          </Button>
        </div>
      </header>

      <SortableContext items={column.stories.map((story) => story.id)} strategy={verticalListSortingStrategy}>
        <div
          ref={(node) => {
            setNodeRef(node);
            scrollRef.current = node;
          }}
          className={`min-h-0 flex-1 space-y-2 overflow-y-auto p-2 transition-colors ${
            isOver ? 'bg-primary/5' : ''
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
}

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

function MemberColumn({ column, collapsed, onToggleCollapse, onCreate, onOpen, findTeamName, workspaceId, assignableMembers, ownerNameMap, onOwnerChanged, onPriorityChanged, onSeverityChanged, onEstimateChanged, onLoadMore, isLoadingMore }: MemberColumnProps) {
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
          className="relative flex h-full w-[44px] shrink-0 cursor-pointer flex-col items-center rounded-md border border-border/50 bg-muted/30 pt-3 transition-colors hover:bg-muted/50"
          onClick={() => onToggleCollapse(colKey)}
        >
          <Maximize2 className="mb-3 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
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
    <section className="flex h-full w-[360px] shrink-0 flex-col">
      <header className="flex items-center justify-between px-3 pt-4 pb-3">
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
            <span className="inline-flex items-center gap-1.5" title={`${column.story_count} items`}>
              <StickyNote className="h-3 w-3" />
              {column.story_count}
            </span>
            <span className="inline-flex items-center gap-1.5" title={`${column.point_total} pts`}>
              <BarChart3 className="h-3 w-3" />
              {column.point_total}
            </span>
          </p>
        </div>
        <div className="flex items-center gap-0.5">
          <QuickTooltip label="Collapse column">
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => onToggleCollapse(colKey)}>
              <Minimize2 className="h-3.5 w-3.5" />
            </Button>
          </QuickTooltip>
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => onCreate(column.member?.id ?? null)}>
            <Plus className="h-4 w-4" />
          </Button>
        </div>
      </header>

      <SortableContext items={column.stories.map((story) => story.id)} strategy={verticalListSortingStrategy}>
        <div
          ref={(node) => {
            setNodeRef(node);
            scrollRef.current = node;
          }}
          className={`min-h-0 flex-1 space-y-2 overflow-y-auto p-2 transition-colors ${isOver ? 'bg-primary/5' : ''}`}
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
}

export function KanbanBoard({ workspaceId, teamId }: KanbanBoardProps) {
  const {
    workflow,
    columns,
    loading,
    error,
    teamId: storeTeamId,
    filters,
    columnLoading,
    loadBoard,
    setTeamFilter,
    setFilters,
    createStory,
    moveStory,
    patchStory,
    refreshBoard,
    loadMoreColumn,
    loadViews,
    memberColumns,
    memberColumnLoading,
    loadMemberBoard,
    loadMoreMemberColumn,
    moveMemberStory,
  } = usePMBoardStore();

  const currentUser = useAuthStore((s) => s.user);
  const { data: sessionMembership } = useSession(workspaceId);
  const currentMemberId = sessionMembership?.id;
  const { teams, findTeamName } = useWorkspaceTeams(workspaceId);
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

  // Load member board when groupBy switches to 'members'
  const activeMemberIds = useMemo(() => assignableMembers.filter((m) => m.status === 'active').map((m) => m.id), [assignableMembers]);
  useEffect(() => {
    if (groupBy === 'members' && workflow) {
      loadMemberBoard(showEmptyColumns ? activeMemberIds : undefined, showEmptyColumns);
    }
  }, [groupBy, workflow, showEmptyColumns, activeMemberIds, loadMemberBoard]);

  // Sync URL team param → store on mount / prop change; default to first team
  useEffect(() => {
    setTeamFilter(teamId ?? teams[0]?.id ?? null);
  }, [teamId, teams, setTeamFilter]);

  const [refLabels, setRefLabels] = useState<Label[]>([]);
  const [refEpics, setRefEpics] = useState<EpicWithStats[]>([]);
  const [refSprints, setRefSprints] = useState<SprintWithStats[]>([]);

  useEffect(() => {
    pmLabelService.list(workspaceId).then((r) => { if (r.data) setRefLabels(r.data); });
    pmEpicService.list(workspaceId).then((r) => { if (r.data) setRefEpics(r.data); });
    pmSprintService.list(workspaceId).then((r) => { if (r.data) setRefSprints(r.data); });
  }, [workspaceId]);

  const [activeStory, setActiveStory] = useState<Story | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [createStateId, setCreateStateId] = useState<string>('');
  const [createOwnerMemberId, setCreateOwnerMemberId] = useState<string | undefined>(undefined);
  const [selectedStory, setSelectedStory] = useState<Awaited<ReturnType<typeof pmStoryService.get>>['data'] | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
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

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 8 } }));

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

  // Re-fetch open story detail when WS story events arrive
  useEffect(() => {
    const refetchDetail = (e: Event) => {
      const storyId = selectedStory?.story?.id;
      if (!storyId || !detailOpen) return;
      const detail = (e as CustomEvent)?.detail;
      if (detail?.entity_id === storyId) {
        pmStoryService.get(workspaceId, storyId).then((res) => {
          if (res.data) setSelectedStory(res.data);
        });
      }
    };
    window.addEventListener('story-updated', refetchDetail);
    return () => {
      window.removeEventListener('story-updated', refetchDetail);
    };
  }, [workspaceId, selectedStory?.story?.id, detailOpen]);

  useEffect(() => {
    if (!workflow) return;
    const maybeStory = new URLSearchParams(window.location.search).get('story');
    if (!maybeStory) return;
    const match = maybeStory.match(/^(\d+)$/);
    if (!match) return;

    (async () => {
      const displayId = Number(match[1]);
      const res = await pmStoryService.getByDisplayId(workspaceId, displayId);
      if (res.data) {
        setSelectedStory(res.data);
        setDetailOpen(true);
      }
    })();
  }, [workspaceId, workflow]);

  const openStory = useCallback(
    async (story: Story) => {
      setSelectedStory(null);
      setDetailLoading(true);
      setDetailOpen(true);
      const detail = await pmStoryService.get(workspaceId, story.id);
      setDetailLoading(false);
      if (!detail.data) return;
      setSelectedStory(detail.data);
    },
    [workspaceId]
  );

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
    },
    [columns, memberColumns, groupBy]
  );

  const onDragEnd = useCallback(
    async (event: DragEndEvent) => {
      setActiveStory(null);
      const { active, over } = event;
      if (!over) return;

      const activeId = String(active.id);
      const overId = String(over.id);
      if (activeId === overId) return;

      if (groupBy === 'members') {
        const fromKey = findMemberKeyByItemId(activeId);
        const toKey = findMemberKeyByItemId(overId);
        if (!fromKey || !toKey) return;

        const fromColumn = memberColumns.find((c) => (c.member?.id ?? '__unassigned__') === fromKey);
        const toColumn = memberColumns.find((c) => (c.member?.id ?? '__unassigned__') === toKey);
        if (!fromColumn || !toColumn) return;

        const overIndex = overId === toKey
          ? toColumn.stories.length
          : toColumn.stories.findIndex((s) => s.id === overId);
        if (overIndex < 0) return;

        const fromIndex = fromColumn.stories.findIndex((s) => s.id === activeId);
        if (fromIndex < 0) return;
        if (fromKey === toKey && fromIndex === overIndex) return;

        const fromMemberId = fromColumn.member?.id ?? null;
        const toMemberId = toColumn.member?.id ?? null;

        await moveMemberStory({
          workspaceId,
          storyId: activeId,
          fromMemberId,
          toMemberId,
          toIndex: overIndex,
        });
        return;
      }

      const fromStateId = findStateIdByItemId(activeId);
      const toStateId = findStateIdByItemId(overId);
      if (!fromStateId || !toStateId) return;

      const fromColumn = columns.find((column) => column.state.id === fromStateId);
      const toColumn = columns.find((column) => column.state.id === toStateId);
      if (!fromColumn || !toColumn) return;

      const overIndex = overId === toStateId
        ? toColumn.stories.length
        : toColumn.stories.findIndex((story) => story.id === overId);
      if (overIndex < 0) return;

      const fromIndex = fromColumn.stories.findIndex((story) => story.id === activeId);
      if (fromIndex < 0) return;
      if (fromStateId === toStateId && fromIndex === overIndex) return;

      await moveStory({
        workspaceId,
        storyId: activeId,
        fromStateId,
        toStateId,
        toIndex: overIndex,
      });
    },
    [columns, memberColumns, groupBy, findStateIdByItemId, findMemberKeyByItemId, moveStory, moveMemberStory, workspaceId]
  );

  const handleCreate = useCallback(
    async (payload: CreateStoryRequest) => {
      await createStory(payload);
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
            value={storeTeamId ?? teams[0]?.id ?? ''}
            onValueChange={(value) => setTeamFilter(value)}
          >
            <SelectTrigger className="h-7 w-auto gap-1.5 text-xs px-2.5">
              <span className="text-muted-foreground">Team:</span>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
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
          collisionDetection={closestCorners}
          onDragStart={onDragStart}
          onDragEnd={onDragEnd}
        >
          <div className="min-h-0 flex-1 overflow-x-auto">
            <div className="flex h-full min-w-full gap-3 pb-2">
              {groupBy === 'members' ? (
                memberColumns.filter((col) => showEmptyColumns || col.story_count > 0).map((col) => {
                  const colKey = col.member?.id ?? '__unassigned__';
                  return (
                    <MemberColumn
                      key={colKey}
                      column={col}
                      collapsed={collapsedColumns.has(colKey)}
                      onToggleCollapse={toggleCollapse}
                      onCreate={(memberId) => {
                        setCreateOwnerMemberId(memberId ?? undefined);
                        setCreateStateId(workflow?.states[0]?.id || '');
                        setCreateOpen(true);
                      }}
                      onOpen={openStory}
                      findTeamName={storeTeamId ? () => undefined : findTeamName}
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
                columns.filter((column) => showEmptyColumns || column.story_count > 0).map((column) => (
                  <Column
                    key={column.state.id}
                    column={column}
                    collapsed={collapsedColumns.has(column.state.id)}
                    onToggleCollapse={toggleCollapse}
                    onCreate={(stateId) => {
                      setCreateOwnerMemberId(undefined);
                      setCreateStateId(stateId);
                      setCreateOpen(true);
                    }}
                    onOpen={openStory}
                    findTeamName={storeTeamId ? () => undefined : findTeamName}
                    workspaceId={workspaceId}
                    assignableMembers={assignableMembers}
                    ownerNameMap={ownerNameMap}
                    onOwnerChanged={handleStoryPatched}
                    onPriorityChanged={handleStoryPatched}
                    onSeverityChanged={handleStoryPatched}
                    onEstimateChanged={handleStoryPatched}
                    onLoadMore={loadMoreColumn}
                    isLoadingMore={!!columnLoading[column.state.id]}
                  />
                ))
              )}
            </div>
          </div>

          <DragOverlay>
            {activeStory ? <StoryCard story={activeStory} onOpen={() => {}} isOverlay teamName={storeTeamId ? undefined : findTeamName(activeStory.team_id)} ownerNameMap={ownerNameMap} showStateBadge={groupBy === 'members'} /> : null}
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

      <StoryDetailPanel
        workspaceId={workspaceId}
        open={detailOpen}
        loading={detailLoading}
        onOpenChange={setDetailOpen}
        storyDetail={selectedStory}
        states={workflow?.states ?? []}
        onStoryUpdated={(updated) => {
          setSelectedStory(updated);
          const story = { ...updated.story };
          const ownerKey = story.owner_member_id;
          if (ownerKey && !story.owner_name) {
            story.owner_name = updated.owner_member
              ? ownerNameMap.get(updated.owner_member.id) ?? updated.owner_member.display_name ?? updated.owner_member.email
              : ownerNameMap.get(ownerKey);
          }
          if (groupBy === 'members') {
            // Enrich with state info from workflow columns
            const stateCol = columns.find((c) => c.state.id === story.workflow_state_id);
            if (stateCol) {
              story.state_name = stateCol.state.name;
              story.state_type = stateCol.state.state_type;
              story.state_color = stateCol.state.color;
            }
            const cols = usePMBoardStore.getState().memberColumns;
            const patched = cols.map((col) => {
              const idx = col.stories.findIndex((s) => s.id === story.id);
              if (idx < 0) return col;
              const stories = [...col.stories];
              stories[idx] = { ...stories[idx], ...story };
              return { ...col, stories };
            });
            usePMBoardStore.setState({ memberColumns: patched });
          } else {
            const patched = patchStory('updated', story.id, story);
            if (!patched) {
              refreshBoard();
            }
          }
        }}
        onStoryArchived={() => {
          setDetailOpen(false);
          const storyId = selectedStory?.story.id ?? '';
          setSelectedStory(null);
          if (groupBy === 'members') {
            const cols = usePMBoardStore.getState().memberColumns;
            const updated = cols.map((col) => {
              const idx = col.stories.findIndex((s) => s.id === storyId);
              if (idx < 0) return col;
              const stories = col.stories.filter((s) => s.id !== storyId);
              return { ...col, stories, story_count: col.story_count - 1 };
            });
            usePMBoardStore.setState({ memberColumns: updated });
          } else {
            const patched = patchStory('deleted', storyId);
            if (!patched) {
              refreshBoard();
            }
          }
        }}
      />
    </div>
    </StoryFilterProvider>
  );
}
