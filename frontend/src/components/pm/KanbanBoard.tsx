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
import type { CreateStoryRequest, Story, StoryStateColumn } from '@/lib/pmTypes';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { StateTypeIcon } from '@/lib/pmConstants';
import { StoryCard } from './StoryCard';
import { CreateStoryModal } from './CreateStoryModal';
import { StoryDetailPanel } from './StoryDetailPanel';

interface KanbanBoardProps {
  workspaceId: string;
}

interface ColumnProps {
  column: StoryStateColumn;
  onCreate: (stateId: string) => void;
  onOpen: (story: Story) => void;
}

function Column({ column, onCreate, onOpen }: ColumnProps) {
  const { setNodeRef, isOver } = useDroppable({ id: column.state.id });

  return (
    <section className="flex h-full w-[360px] shrink-0 flex-col">
      <header className="flex items-center justify-between px-2 py-2">
        <div className="min-w-0">
          <p className="flex items-center gap-1.5 truncate text-sm font-semibold">
            <StateTypeIcon stateType={column.state.state_type} className="h-4 w-4 shrink-0" />
            {column.state.name}
          </p>
          <p className="flex items-center gap-2 text-xs text-muted-foreground">
            <span className="inline-flex items-center gap-1" title={`${column.story_count} items`}>
              <StickyNote className="h-3 w-3" />
              {column.story_count}
            </span>
            <span className="inline-flex items-center gap-1" title={`${column.point_total} pts`}>
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
            <StoryCard key={story.id} story={story} onOpen={onOpen} />
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

export function KanbanBoard({ workspaceId }: KanbanBoardProps) {
  const {
    workflows,
    workflow,
    columns,
    loading,
    error,
    loadBoard,
    setWorkflow,
    createStory,
    moveStory,
    refreshBoard,
  } = usePMBoardStore();

  const [activeStory, setActiveStory] = useState<Story | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [createStateId, setCreateStateId] = useState<string>('');
  const [selectedStory, setSelectedStory] = useState<Awaited<ReturnType<typeof pmStoryService.get>>['data'] | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const [viewMode, setViewMode] = useState<'board' | 'list'>('board');

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 8 } }));

  useEffect(() => {
    loadBoard(workspaceId);
  }, [workspaceId, loadBoard]);

  // Listen for "Create story" button click from the header
  useEffect(() => {
    const handler = () => {
      const firstState = workflow?.states[0]?.id;
      if (!firstState) return;
      setCreateStateId(firstState);
      setCreateOpen(true);
    };
    window.addEventListener('add-work-item', handler);
    return () => window.removeEventListener('add-work-item', handler);
  }, [workflow]);

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
    <div className="flex h-full min-h-0 flex-col gap-3">
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
          <SelectTrigger className="ml-auto h-8 w-[260px]">
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

        <div className="flex items-center gap-1">
          <Button
            variant={viewMode === 'board' ? 'default' : 'outline'}
            size="sm"
            onClick={() => setViewMode('board')}
          >
            <Columns2 className="h-4 w-4" />
            Board
          </Button>
          <Button
            variant={viewMode === 'list' ? 'default' : 'outline'}
            size="sm"
            onClick={() => setViewMode('list')}
            disabled
          >
            <LayoutList className="h-4 w-4" />
            List
          </Button>
        </div>

      </header>

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
                />
              ))}
            </div>
          </div>

          <DragOverlay>
            {activeStory ? <StoryCard story={activeStory} onOpen={() => {}} isOverlay /> : null}
          </DragOverlay>
        </DndContext>
      ) : null}

      {viewMode === 'list' ? (
        <div className="rounded-md border border-border/70 bg-background/70 p-4 text-sm text-muted-foreground">
          List view is planned for Phase 2.
        </div>
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
  );
}
