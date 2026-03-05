import { useCallback, useEffect, useMemo, useState } from 'react';
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
import { BarChart3, Columns2, LayoutList, Loader2, Plus, StickyNote } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { usePMBoardStore } from '@/stores/pmBoardStore';
import type { CreateStoryRequest, Story, StoryStateColumn, Label, EpicWithStats, SprintWithStats } from '@/lib/pmTypes';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { pmLabelService } from '@/lib/services/pmLabelService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmSprintService } from '@/lib/services/pmSprintService';
import { StateTypeIcon } from '@/lib/pmConstants';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { useWorkspaceMembers } from '@/hooks/useWorkspaceMembers';
import { useAuthStore } from '@/stores/authStore';
import { StoryCard } from './StoryCard';
import { CreateStoryModal } from './CreateStoryModal';
import { StoryDetailPanel } from './StoryDetailPanel';
import { StoryFilterProvider, StoryFilterTrigger, StoryFilterBar } from './StoryFilters';
import { StoryListView } from './StoryListView';
import { ViewBar } from './ViewBar';

interface KanbanBoardProps {
  workspaceId: string;
  teamId?: string;
}

interface ColumnProps {
  column: StoryStateColumn;
  onCreate: (stateId: string) => void;
  onOpen: (story: Story) => void;
  findTeamName: (teamId: string | undefined) => string | undefined;
}

function Column({ column, onCreate, onOpen, findTeamName }: ColumnProps) {
  const { setNodeRef, isOver } = useDroppable({ id: column.state.id });

  return (
    <section className="flex h-full w-[360px] shrink-0 flex-col">
      <header className="flex items-center justify-between px-3 pt-4 pb-3">
        <div className="min-w-0">
          <p className="flex items-center gap-1.5 truncate text-sm font-semibold">
            <StateTypeIcon stateType={column.state.state_type} className="h-4 w-4 shrink-0" />
            {column.state.name}
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
        <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => onCreate(column.state.id)}>
          <Plus className="h-4 w-4" />
        </Button>
      </header>

      <SortableContext items={column.stories.map((story) => story.id)} strategy={verticalListSortingStrategy}>
        <div
          ref={setNodeRef}
          className={`min-h-0 flex-1 space-y-2 overflow-y-auto p-2 transition-colors ${
            isOver ? 'bg-primary/5' : ''
          }`}
        >
          {column.stories.map((story) => (
            <StoryCard key={story.id} story={story} onOpen={onOpen} teamName={findTeamName(story.team_id)} />
          ))}

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

export function KanbanBoard({ workspaceId, teamId }: KanbanBoardProps) {
  const {
    workflows,
    workflow,
    columns,
    loading,
    error,
    teamId: storeTeamId,
    filters,
    loadBoard,
    setWorkflow,
    setTeamFilter,
    setFilters,
    createStory,
    moveStory,
    refreshBoard,
    loadViews,
  } = usePMBoardStore();

  const currentUser = useAuthStore((s) => s.user);
  const { teams, findTeamName } = useWorkspaceTeams(workspaceId);
  const { members } = useWorkspaceMembers(workspaceId);

  // Sync URL team param → store on mount / prop change
  useEffect(() => {
    setTeamFilter(teamId ?? null);
  }, [teamId, setTeamFilter]);

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

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 8 } }));

  useEffect(() => {
    loadBoard(workspaceId);
  }, [workspaceId, loadBoard]);

  // Load views once board and user are ready
  useEffect(() => {
    if (currentUser?.id) {
      loadViews(workspaceId, currentUser.id);
    }
  }, [workspaceId, currentUser?.id, loadViews]);

  // Refresh board when a story is created via the global modal
  useEffect(() => {
    const handler = () => { refreshBoard(); };
    window.addEventListener('story-created', handler);
    return () => window.removeEventListener('story-created', handler);
  }, [refreshBoard]);

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
    const match = maybeStory.match(/^TP-(\d+)$/i);
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

  const onDragStart = useCallback(
    (event: DragStartEvent) => {
      const story = columns.flatMap((column) => column.stories).find((item) => item.id === String(event.active.id));
      setActiveStory(story ?? null);
    },
    [columns]
  );

  const onDragEnd = useCallback(
    async (event: DragEndEvent) => {
      setActiveStory(null);
      const { active, over } = event;
      if (!over) return;

      const activeId = String(active.id);
      const overId = String(over.id);
      if (activeId === overId) return;

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
    [columns, findStateIdByItemId, moveStory, workspaceId]
  );

  const handleCreate = useCallback(
    async (payload: CreateStoryRequest) => {
      await createStory(payload);
    },
    [createStory]
  );

  const totalStories = useMemo(
    () => columns.reduce((sum, column) => sum + column.story_count, 0),
    [columns]
  );

  return (
    <StoryFilterProvider
      members={members}
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
        <div className="flex items-center gap-2">
          <h2 className="text-sm font-semibold">Stories</h2>
          <Badge variant="secondary" className="rounded-full px-2 py-0 text-xs">
            {totalStories}
          </Badge>
        </div>

        <Select
          value={workflow?.workflow.id}
          onValueChange={(value) => {
            setWorkflow(value);
          }}
        >
          <SelectTrigger className="h-8 w-[260px]">
            <SelectValue placeholder="Select workflow" />
          </SelectTrigger>
          <SelectContent>
            {workflows.map((entry) => (
              <SelectItem key={entry.workflow.id} value={entry.workflow.id}>
                {entry.workflow.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <StoryFilterTrigger />

        <div className="ml-auto flex items-center gap-1">
          <Button
            variant={viewMode === 'board' ? 'default' : 'ghost'}
            size="icon"
            className="h-7 w-7"
            onClick={() => setViewMode('board')}
            title="Board view"
          >
            <Columns2 className="h-4 w-4" />
          </Button>
          <Button
            variant={viewMode === 'list' ? 'default' : 'ghost'}
            size="icon"
            className="h-7 w-7"
            onClick={() => setViewMode('list')}
            title="List view"
          >
            <LayoutList className="h-4 w-4" />
          </Button>
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
              {columns.map((column) => (
                <Column
                  key={column.state.id}
                  column={column}
                  onCreate={(stateId) => {
                    setCreateStateId(stateId);
                    setCreateOpen(true);
                  }}
                  onOpen={openStory}
                  findTeamName={findTeamName}
                />
              ))}
            </div>
          </div>

          <DragOverlay>
            {activeStory ? <StoryCard story={activeStory} onOpen={() => {}} isOverlay teamName={findTeamName(activeStory.team_id)} /> : null}
          </DragOverlay>
        </DndContext>
      ) : null}

      {!loading && viewMode === 'list' && workflow ? (
        <StoryListView
          workspaceId={workspaceId}
          workflow={workflow}
          teams={teams}
          members={members}
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
          refreshBoard();
        }}
        onStoryArchived={() => {
          setDetailOpen(false);
          setSelectedStory(null);
          refreshBoard();
        }}
      />
    </div>
    </StoryFilterProvider>
  );
}
