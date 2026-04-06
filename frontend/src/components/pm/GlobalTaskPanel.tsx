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
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { pmRecurringTemplateService } from '@/lib/services/pmRecurringTemplateService';
import { parseTaskKey } from '@/lib/taskKeyUtils';
import type { TaskDetail, TaskRecurringSummary } from '@/lib/pmTypes';
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
  const activeTaskRoute = useMemo(() => getActiveTaskRoute(overlayLocation), [overlayLocation]);
  const activeTaskId = activeTaskRoute?.taskId ?? contextualTaskId;

  const [loadedTask, setLoadedTask] = useState<LoadedTaskState | null>(null);
  const presentation = useMemo(
    () => getTaskOverlayPresentationState(activeTaskId, loadedTask),
    [activeTaskId, loadedTask],
  );

  // Use refs for close handler to avoid re-triggering task load effect
  const locationRef = useRef(overlayLocation);
  locationRef.current = overlayLocation;
  const navigateRef = useRef(navigate);
  navigateRef.current = navigate;

  const handleClose = useCallback(() => {
    if (!workspaceSlug) return;
    closeTaskRoute(navigateRef.current as never, locationRef.current, workspaceSlug);
  }, [workspaceSlug]);

  useEffect(() => {
    if (activeTaskRoute && contextualTaskId) {
      closeContextualTask();
    }
  }, [activeTaskRoute, closeContextualTask, contextualTaskId]);

  // Open task panel from ?task= URL param on any page (e.g. sprints, epics).
  // Supports both task key format ("HLP-123") and bare numeric display_id ("123").
  useEffect(() => {
    if (activeTaskId || !workspaceId || !workspaceSlug) return;
    const maybeTask = new URLSearchParams(window.location.search).get('task');
    if (!maybeTask) return;

    // Try task key format first (e.g. "HLP-123"), fall back to bare number.
    let displayId: number | null = null;
    const parsed = parseTaskKey(maybeTask);
    if (parsed) {
      displayId = parsed.displayId;
    } else {
      const numMatch = maybeTask.match(/^(\d+)$/);
      if (numMatch) displayId = Number(numMatch[1]);
    }
    if (!displayId) return;

    (async () => {
      const res = await pmTaskService.getByDisplayId(workspaceId, displayId!);
      if (res.data && workspaceSlug) {
        openTaskRoute(navigateRef.current as never, { pathname: window.location.pathname } as never, workspaceSlug, res.data.task.id);
      }
    })();
  }, [activeTaskId, workspaceId, workspaceSlug]);

  useEffect(() => {
    if (!activeTaskId || !workspaceId) return;

    let cancelled = false;

    (async () => {
      try {
        const [taskRes, wfRes] = await Promise.all([
          pmTaskService.get(workspaceId, activeTaskId),
          pmWorkflowService.list(workspaceId),
        ]);
        if (cancelled) return;
        if (!taskRes.data) {
          toast.error('Failed to load task');
          handleClose();
          return;
        }

        let recurring: TaskRecurringSummary | null = null;
        if (taskRes.data.task.recurring_template_id) {
          const { data } = await pmRecurringTemplateService.getByTask(workspaceId, activeTaskId);
          if (!cancelled) recurring = data ?? null;
        }
        if (cancelled) return;

        const workflow = wfRes.data?.find(
          (item) => item.workflow.id === taskRes.data!.task.workflow_id,
        );

        setLoadedTask({
          taskId: activeTaskId,
          taskDetail: taskRes.data,
          states: workflow?.states ?? [],
          recurringSummary: recurring,
        });
      } catch {
        if (!cancelled) {
          toast.error('Failed to load task');
          handleClose();
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [activeTaskId, handleClose, requestKey, workspaceId]);

  const handleStoryUpdated = useCallback((updated: TaskDetail) => {
    const patchedTask = buildPatchedTaskFromDetail(updated);
    setLoadedTask((current) =>
      current
        ? {
            ...current,
            taskId: updated.task.id,
            taskDetail: updated,
          }
        : {
            taskId: updated.task.id,
            taskDetail: updated,
            states: [],
            recurringSummary: null,
          },
    );
    window.dispatchEvent(
      new CustomEvent('task-panel-updated', { detail: { task: patchedTask, taskDetail: updated } }),
    );
    window.dispatchEvent(
      new CustomEvent('task-updated', {
        detail: { entity: 'task', action: 'updated', entity_id: updated.task.id, local: true },
      }),
    );
  }, []);

  const handleStoryArchived = useCallback(
    (archivedTaskId: string) => {
      handleClose();
      window.dispatchEvent(
        new CustomEvent('task-panel-archived', {
          detail: { taskId: archivedTaskId },
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
      taskDetail={presentation.taskDetail}
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
