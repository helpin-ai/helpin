import {
  DndContext,
  DragOverlay,
  PointerSensor,
  closestCenter,
  type DragEndEvent,
  type DragStartEvent,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import { useMemo, useRef, useState } from 'react';
import type { AssignableMember } from '@/lib/types';
import type { SprintPlanningWorkspace as SprintPlanningWorkspaceData, SprintPlanningTaskPreview } from '@/lib/pmTypes';
import { SprintPlanningBacklogPanel } from './SprintPlanningBacklogPanel';
import { SprintPlanningColumn } from './SprintPlanningColumn';
import { SprintPlanningEmptyState } from './SprintPlanningEmptyState';
import { SprintPlanningTaskCard } from './SprintPlanningTaskCard';

interface SprintPlanningWorkspaceProps {
  workspace: SprintPlanningWorkspaceData | null;
  workspaceId: string;
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
  // Ref persists the dropped task ID across the render gap where activeTask
  // is cleared but workspace data hasn't propagated yet
  const droppedTaskIdRef = useRef<string | null>(null);
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 6,
      },
    }),
  );

  const handleDragStart = (event: DragStartEvent) => {
    const task = event.active.data.current?.task as SprintPlanningTaskPreview | undefined;
    droppedTaskIdRef.current = null;
    setActiveTask(task ?? null);
  };

  const handleDragEnd = (event: DragEndEvent) => {
    const task = (event.active.data.current?.task as SprintPlanningTaskPreview | undefined) ?? activeTask;
    const overId = event.over?.id ? String(event.over.id) : null;
    if (!task || !overId) {
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
    setActiveTask(null);
  };

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

  if (!workspace || !hasAnySprint) {
    return <SprintPlanningEmptyState canEdit={canEdit} onCreateSprint={onCreateSprint} />;
  }

  return (
    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragStart={handleDragStart} onDragEnd={handleDragEnd}>
      <div className="flex gap-4 xl:gap-5">
        <div className="min-w-0 flex-1 overflow-x-auto pb-4">
          <div className="flex min-w-max gap-5">
            {/* Order: upcoming → active → completed (left to right) */}
            {['upcoming', 'active', 'completed'].flatMap(
              (key) => workspace.buckets.find((b) => b.key === key)?.sprints ?? [],
            ).map((card) => (
              <SprintPlanningColumn
                key={card.sprint.id}
                card={card}
                workspaceId={workspaceId}
                ownerByMemberId={ownerByMemberId}
                canEdit={canEdit}
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
              owner={activeTask.owner_member_id ? ownerByMemberId.get(activeTask.owner_member_id) : undefined}
              compact
            />
          </div>
        ) : null}
      </DragOverlay>
    </DndContext>
  );
}
