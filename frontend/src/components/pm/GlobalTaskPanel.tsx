import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { TaskDetailPanel } from '@/components/pm/TaskDetailPanel';
import {
  closeTaskRoute,
  getActiveTaskRoute,
  type TaskOverlayLocationLike,
} from '@/components/pm/task-detail/taskRouteNavigation';
import {
  getTaskOverlayPresentationState,
  type LoadedTaskState,
} from '@/components/pm/task-detail/taskOverlayState';
import { buildPatchedTaskFromDetail } from '@/components/pm/task-detail/taskDetailEventPayload';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { pmRecurringTemplateService } from '@/lib/services/pmRecurringTemplateService';
import type { StoryDetail, StoryRecurringSummary } from '@/lib/pmTypes';
import { useTaskPanelStore } from '@/stores/taskPanelStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';

interface GlobalTaskPanelProps {
  workspaceId: string;
}

export function GlobalTaskPanel({ workspaceId }: GlobalTaskPanelProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const overlayLocation = location as TaskOverlayLocationLike;
  const workspaceSlug = useWorkspaceStore((s) => s.currentWorkspace?.slug ?? '');
  const contextualTaskId = useTaskPanelStore((s) => s.taskId);
  const requestKey = useTaskPanelStore((s) => s.requestKey);
  const closeContextualTask = useTaskPanelStore((s) => s.close);
  const activeStoryRoute = useMemo(() => getActiveTaskRoute(overlayLocation), [overlayLocation]);
  const activeStoryId = activeStoryRoute?.taskId ?? contextualTaskId;

  const [loadedStory, setLoadedStory] = useState<LoadedTaskState | null>(null);
  const presentation = useMemo(
    () => getTaskOverlayPresentationState(activeStoryId, loadedStory),
    [activeStoryId, loadedStory],
  );

  // Use refs for close handler to avoid re-triggering story load effect
  const locationRef = useRef(overlayLocation);
  locationRef.current = overlayLocation;
  const navigateRef = useRef(navigate);
  navigateRef.current = navigate;

  const handleClose = useCallback(() => {
    if (!workspaceSlug) return;
    closeTaskRoute(navigateRef.current as never, locationRef.current, workspaceSlug);
  }, [workspaceSlug]);

  useEffect(() => {
    if (activeStoryRoute && contextualTaskId) {
      closeContextualTask();
    }
  }, [activeStoryRoute, closeContextualTask, contextualTaskId]);

  useEffect(() => {
    if (!activeStoryId || !workspaceId) return;

    let cancelled = false;

    (async () => {
      try {
        const [storyRes, wfRes] = await Promise.all([
          pmTaskService.get(workspaceId, activeStoryId),
          pmWorkflowService.list(workspaceId),
        ]);
        if (cancelled) return;
        if (!storyRes.data) {
          toast.error('Failed to load story');
          handleClose();
          return;
        }

        let recurring: StoryRecurringSummary | null = null;
        if (storyRes.data.task.recurring_template_id) {
          const { data } = await pmRecurringTemplateService.getByStory(workspaceId, activeStoryId);
          if (!cancelled) recurring = data ?? null;
        }
        if (cancelled) return;

        const workflow = wfRes.data?.find(
          (item) => item.workflow.id === storyRes.data!.task.workflow_id,
        );

        setLoadedStory({
          storyId: activeStoryId,
          storyDetail: storyRes.data,
          states: workflow?.states ?? [],
          recurringSummary: recurring,
        });
      } catch {
        if (!cancelled) {
          toast.error('Failed to load story');
          handleClose();
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [activeStoryId, handleClose, requestKey, workspaceId]);

  const handleStoryUpdated = useCallback((updated: StoryDetail) => {
    const patchedStory = buildPatchedTaskFromDetail(updated);
    setLoadedStory((current) =>
      current
        ? {
            ...current,
            storyId: updated.task.id,
            storyDetail: updated,
          }
        : {
            storyId: updated.task.id,
            storyDetail: updated,
            states: [],
            recurringSummary: null,
          },
    );
    window.dispatchEvent(
      new CustomEvent('task-panel-updated', { detail: { story: patchedStory, storyDetail: updated } }),
    );
    window.dispatchEvent(
      new CustomEvent('task-updated', {
        detail: { entity: 'story', action: 'updated', entity_id: updated.task.id, local: true },
      }),
    );
  }, []);

  const handleStoryArchived = useCallback(
    (archivedStoryId: string) => {
      handleClose();
      window.dispatchEvent(
        new CustomEvent('task-panel-archived', {
          detail: { storyId: archivedStoryId },
        }),
      );
    },
    [handleClose],
  );

  return (
    <TaskDetailPanel
      workspaceId={workspaceId}
      open={presentation.open}
      loading={presentation.loading}
      taskDetail={presentation.storyDetail}
      states={presentation.states}
      initialRecurringSummary={presentation.recurringSummary}
      onOpenChange={(isOpen) => {
        if (!isOpen) handleClose();
      }}
      onTaskUpdated={handleStoryUpdated}
      onTaskArchived={handleStoryArchived}
    />
  );
}
