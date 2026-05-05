import {
  DndContext,
  DragOverlay,
  PointerSensor,
  pointerWithin,
  rectIntersection,
  type CollisionDetection,
  type DragEndEvent,
  type DragOverEvent,
  type DragStartEvent,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import { useCallback, useMemo, useRef, useState } from 'react';
import type { AssignableMember } from '@/lib/types';
import type { SprintPlanningWorkspace as SprintPlanningWorkspaceData, SprintPlanningTaskPreview } from '@/lib/pmTypes';
import { SprintPlanningBacklogPanel } from './SprintPlanningBacklogPanel';
import { SprintPlanningColumn } from './SprintPlanningColumn';
import { SprintPlanningEmptyState } from './SprintPlanningEmptyState';
import { SprintPlanningTaskCard } from './SprintPlanningTaskCard';

// Prefer the pointer target, but fall back to geometry when release/motion
// briefly leaves the pointer outside a column rect during cross-column drags.
const sprintCollision: CollisionDetection = (args) => {
  const pointerHits = pointerWithin(args);
  if (pointerHits.length > 0) {
    return pointerHits;
  }
  return rectIntersection(args);
};

interface SprintPlanningWorkspaceProps {
  workspace: SprintPlanningWorkspaceData | null;
  workspaceId: string;
  workspaceSlug: string;
  backlogOpen: boolean;
  onBacklogToggle: () => void;
  canEdit: boolean;
  members: AssignableMember[];
  onOpenSprint: (sprintId: string) => void;
  onOpenTask: (taskId: string) => void;
  onCreateSprint: () => void;
  onCreateTask: (sprintId?: string) => void;
  onAssignTask: (task: SprintPlanningTaskPreview, sprintId: string | null) => void;
}

export function SprintPlanningWorkspace({
  workspace,
  workspaceId,
  workspaceSlug,
  backlogOpen,
  onBacklogToggle,
  canEdit,
  members,
  onOpenSprint,
  onOpenTask,
  onCreateSprint,
  onCreateTask,
  onAssignTask,
}: SprintPlanningWorkspaceProps) {
  const ownerByMemberId = useMemo(() => {
    const map = new Map<string, AssignableMember>();
    for (const member of members) {
      map.set(member.id, member);
      if (member.user_id) map.set(member.user_id, member);
    }
    return map;
  }, [members]);

  const preferredSprintId =
    workspace?.buckets.find((bucket) => bucket.key === 'active')?.sprints?.[0]?.sprint.id ??
    workspace?.buckets.find((bucket) => bucket.key === 'upcoming')?.sprints?.[0]?.sprint.id ??
    workspace?.buckets.flatMap((bucket) => bucket.sprints ?? [])?.[0]?.sprint.id ??
    null;
  const hasAnySprint = Boolean(workspace?.buckets.some((bucket) => (bucket.sprints?.length ?? 0) > 0));

  const [activeTask, setActiveTask] = useState<SprintPlanningTaskPreview | null>(null);
  const [activeDropTargetId, setActiveDropTargetId] = useState<string | null>(null);
  // Refs persist across the render gap where activeTask is cleared but
  // workspace data hasn't propagated yet — also avoids stale closures in
  // memoized callbacks so columns don't re-render during drag
  const droppedTaskIdRef = useRef<string | null>(null);
  const activeTaskRef = useRef<SprintPlanningTaskPreview | null>(null);
  // Track the last droppable the pointer was over — fallback for onDragEnd
  // when event.over is null (pointer moved slightly during mouse release)
  const overContainerRef = useRef<string | null>(null);
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 6,
      },
    }),
  );

  const resolveDropTarget = useCallback((overId: string | null) => {
    if (!overId) return null;
    if (overId === 'backlog-dropzone' || overId.startsWith('sprint:')) return overId;
    return null;
  }, []);

  const handleDragStart = useCallback((event: DragStartEvent) => {
    const task = event.active.data.current?.task as SprintPlanningTaskPreview | undefined;
    droppedTaskIdRef.current = null;
    overContainerRef.current = null;
    setActiveDropTargetId(null);
    activeTaskRef.current = task ?? null;
    setActiveTask(task ?? null);
  }, []);

  const handleDragOver = useCallback((event: DragOverEvent) => {
    const nextDropTargetId = resolveDropTarget(event.over?.id ? String(event.over.id) : null);
    // Only update when a valid droppable is found — don't clear on gaps
    // between columns, otherwise onDragEnd has no fallback target.
    if (nextDropTargetId) {
      overContainerRef.current = nextDropTargetId;
      setActiveDropTargetId((current) => current === nextDropTargetId ? current : nextDropTargetId);
    }
  }, [resolveDropTarget]);

  const handleDragEnd = useCallback((event: DragEndEvent) => {
    const task = (event.active.data.current?.task as SprintPlanningTaskPreview | undefined) ?? activeTaskRef.current;
    // Use event.over when available; fall back to last container from onDragOver
    // (preserved even when pointer enters gaps between columns)
    const overId = resolveDropTarget(event.over?.id ? String(event.over.id) : null) ?? overContainerRef.current;
    overContainerRef.current = null;
    setActiveDropTargetId(null);
    if (!task || !overId) {
      activeTaskRef.current = null;
      setActiveTask(null);
      return;
    }
    // Remember which task was dropped — this ref survives the render gap
    // between activeTask clearing and workspace data propagating
    droppedTaskIdRef.current = task.id;
    // Apply optimistic update
    if (overId === 'backlog-dropzone') {
      onAssignTask(task, null);
    } else if (overId.startsWith('sprint:')) {
      onAssignTask(task, overId.replace('sprint:', ''));
    }
    activeTaskRef.current = null;
    setActiveTask(null);
  }, [onAssignTask, resolveDropTarget]);

  const handleDragCancel = useCallback(() => {
    activeTaskRef.current = null;
    overContainerRef.current = null;
    setActiveDropTargetId(null);
    setActiveTask(null);
  }, []);

  // Hide the task being dragged OR just dropped from the backlog list.
  // The ref bridges the gap: when activeTask clears but workspace data
  // hasn't updated yet, droppedTaskIdRef still filters the card out.
  const hideStoryId = activeTask?.id ?? droppedTaskIdRef.current;
  const backlogTasks = useMemo(() => {
    const raw = workspace?.backlog_tasks ?? [];
    if (!hideStoryId) return raw;
    return raw.filter((s) => s.id !== hideStoryId);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- hideStoryId uses ref, recompute when backlog changes
  }, [workspace?.backlog_tasks, activeTask]);

  // Stable ordered list of sprint cards — avoids recreating during drag
  const sprintCards = useMemo(
    () =>
      ['upcoming', 'active', 'completed'].flatMap(
        (key) => workspace?.buckets.find((b) => b.key === key)?.sprints ?? [],
      ),
    [workspace?.buckets],
  );

  if (!workspace || !hasAnySprint) {
    return <SprintPlanningEmptyState canEdit={canEdit} onCreateSprint={onCreateSprint} />;
  }

  return (
    <DndContext sensors={sensors} collisionDetection={sprintCollision} onDragStart={handleDragStart} onDragOver={handleDragOver} onDragEnd={handleDragEnd} onDragCancel={handleDragCancel}>
      <div className="flex gap-4 xl:gap-5">
        <div className="min-w-0 flex-1 overflow-x-auto pb-4">
          <div className="flex min-w-max gap-5">
            {/* Order: upcoming → active → completed (left to right) */}
            {sprintCards.map((card) => (
              <SprintPlanningColumn
                key={card.sprint.id}
                card={card}
                workspaceId={workspaceId}
                workspaceSlug={workspaceSlug}
                ownerByMemberId={ownerByMemberId}
                canEdit={canEdit}
                isDropTargetActive={activeDropTargetId === `sprint:${card.sprint.id}`}
                onOpenSprint={onOpenSprint}
                onOpenTask={onOpenTask}
                onCreateTask={onCreateTask}
              />
            ))}
          </div>
        </div>

        <SprintPlanningBacklogPanel
          open={backlogOpen}
          onToggle={onBacklogToggle}
          tasks={backlogTasks}
          total={workspace.backlog_total}
          ownerByMemberId={ownerByMemberId}
          canEdit={canEdit}
          onOpenTask={onOpenTask}
          onAddToActiveSprint={(task) => onAssignTask(task, preferredSprintId)}
          onCreateTask={() => onCreateTask()}
        />
      </div>
      <DragOverlay>
        {activeTask ? (
          <div className="w-[300px] rotate-[1deg] shadow-xl">
            <SprintPlanningTaskCard
              task={activeTask}
              owner={activeTask.owner_member_ids?.[0] ? ownerByMemberId.get(activeTask.owner_member_ids[0]) : undefined}
              compact
              onOpenTask={onOpenTask}
            />
          </div>
        ) : null}
      </DragOverlay>
    </DndContext>
  );
}
