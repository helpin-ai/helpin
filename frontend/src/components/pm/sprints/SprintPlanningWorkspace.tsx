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
  backlogOpen: boolean;
  onBacklogToggle: () => void;
  canEdit: boolean;
  members: AssignableMember[];
  onOpenSprint: (sprintId: string) => void;
  onOpenStory: (storyId: string) => void;
  onCreateSprint: () => void;
  onCreateStory: (sprintId?: string) => void;
  onAssignStory: (story: SprintPlanningTaskPreview, sprintId: string | null) => void;
}

export function SprintPlanningWorkspace({
  workspace,
  backlogOpen,
  onBacklogToggle,
  canEdit,
  members,
  onOpenSprint,
  onOpenStory,
  onCreateSprint,
  onCreateStory,
  onAssignStory,
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

  const [activeStory, setActiveStory] = useState<SprintPlanningTaskPreview | null>(null);
  // Ref persists the dropped story ID across the render gap where activeStory
  // is cleared but workspace data hasn't propagated yet
  const droppedStoryIdRef = useRef<string | null>(null);
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 6,
      },
    }),
  );

  const handleDragStart = (event: DragStartEvent) => {
    const story = event.active.data.current?.story as SprintPlanningTaskPreview | undefined;
    droppedStoryIdRef.current = null;
    setActiveStory(story ?? null);
  };

  const handleDragEnd = (event: DragEndEvent) => {
    const story = (event.active.data.current?.story as SprintPlanningTaskPreview | undefined) ?? activeStory;
    const overId = event.over?.id ? String(event.over.id) : null;
    if (!story || !overId) {
      setActiveStory(null);
      return;
    }
    // Remember which story was dropped — this ref survives the render gap
    // between activeStory clearing and workspace data propagating
    droppedStoryIdRef.current = story.id;
    // Apply optimistic update
    if (overId === 'backlog-dropzone') {
      onAssignStory(story, null);
    } else if (overId.startsWith('sprint:')) {
      onAssignStory(story, overId.replace('sprint:', ''));
    }
    setActiveStory(null);
  };

  // Hide the story being dragged OR just dropped from the backlog list.
  // The ref bridges the gap: when activeStory clears but workspace data
  // hasn't updated yet, droppedStoryIdRef still filters the card out.
  const hideStoryId = activeStory?.id ?? droppedStoryIdRef.current;
  const backlogStories = useMemo(() => {
    const raw = workspace?.backlog_stories ?? [];
    if (!hideStoryId) return raw;
    return raw.filter((s) => s.id !== hideStoryId);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- hideStoryId uses ref, recompute when backlog changes
  }, [workspace?.backlog_stories, activeStory]);

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
                ownerByMemberId={ownerByMemberId}
                canEdit={canEdit}
                onOpenSprint={onOpenSprint}
                onOpenStory={onOpenStory}
                onCreateStory={onCreateStory}
              />
            ))}
          </div>
        </div>

        <SprintPlanningBacklogPanel
          open={backlogOpen}
          onToggle={onBacklogToggle}
          stories={backlogStories}
          total={workspace.backlog_total}
          ownerByMemberId={ownerByMemberId}
          canEdit={canEdit}
          onOpenStory={onOpenStory}
          onAddToActiveSprint={(story) => onAssignStory(story, preferredSprintId)}
          onCreateStory={() => onCreateStory()}
        />
      </div>
      <DragOverlay>
        {activeStory ? (
          <div className="w-[300px] rotate-[1deg] shadow-xl">
            <SprintPlanningTaskCard
              story={activeStory}
              owner={activeStory.owner_member_id ? ownerByMemberId.get(activeStory.owner_member_id) : undefined}
              compact
            />
          </div>
        ) : null}
      </DragOverlay>
    </DndContext>
  );
}
