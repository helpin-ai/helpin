import { useCallback, useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { StoryDetailPanel } from '@/components/pm/StoryDetailPanel';
import {
  closeStoryRoute,
  getActiveStoryRoute,
  type StoryOverlayLocationLike,
} from '@/components/pm/story-detail/storyRouteNavigation';
import {
  getStoryOverlayPresentationState,
  type LoadedStoryState,
} from '@/components/pm/story-detail/storyOverlayState';
import { buildPatchedStoryFromDetail } from '@/components/pm/story-detail/storyDetailEventPayload';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { pmRecurringTemplateService } from '@/lib/services/pmRecurringTemplateService';
import type { StoryDetail, StoryRecurringSummary } from '@/lib/pmTypes';
import { useStoryPanelStore } from '@/stores/storyPanelStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';

interface GlobalStoryPanelProps {
  workspaceId: string;
}

export function GlobalStoryPanel({ workspaceId }: GlobalStoryPanelProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const overlayLocation = location as StoryOverlayLocationLike;
  const workspaceSlug = useWorkspaceStore((s) => s.currentWorkspace?.slug ?? '');
  const contextualStoryId = useStoryPanelStore((s) => s.storyId);
  const requestKey = useStoryPanelStore((s) => s.requestKey);
  const closeContextualStory = useStoryPanelStore((s) => s.close);
  const activeStoryRoute = useMemo(() => getActiveStoryRoute(overlayLocation), [overlayLocation]);
  const activeStoryId = activeStoryRoute?.storyId ?? contextualStoryId;

  const [loadedStory, setLoadedStory] = useState<LoadedStoryState | null>(null);
  const presentation = useMemo(
    () => getStoryOverlayPresentationState(activeStoryId, loadedStory),
    [activeStoryId, loadedStory],
  );

  const handleClose = useCallback(() => {
    if (!workspaceSlug) return;
    closeStoryRoute(navigate as never, overlayLocation, workspaceSlug);
  }, [navigate, overlayLocation, workspaceSlug]);

  useEffect(() => {
    if (activeStoryRoute && contextualStoryId) {
      closeContextualStory();
    }
  }, [activeStoryRoute, closeContextualStory, contextualStoryId]);

  useEffect(() => {
    if (!activeStoryId || !workspaceId) return;

    let cancelled = false;

    (async () => {
      try {
        const [storyRes, wfRes] = await Promise.all([
          pmStoryService.get(workspaceId, activeStoryId),
          pmWorkflowService.list(workspaceId),
        ]);
        if (cancelled) return;
        if (!storyRes.data) {
          toast.error('Failed to load story');
          handleClose();
          return;
        }

        let recurring: StoryRecurringSummary | null = null;
        if (storyRes.data.story.recurring_template_id) {
          const { data } = await pmRecurringTemplateService.getByStory(workspaceId, activeStoryId);
          if (!cancelled) recurring = data ?? null;
        }
        if (cancelled) return;

        const workflow = wfRes.data?.find(
          (item) => item.workflow.id === storyRes.data!.story.workflow_id,
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
    const patchedStory = buildPatchedStoryFromDetail(updated);
    setLoadedStory((current) =>
      current
        ? {
            ...current,
            storyId: updated.story.id,
            storyDetail: updated,
          }
        : {
            storyId: updated.story.id,
            storyDetail: updated,
            states: [],
            recurringSummary: null,
          },
    );
    window.dispatchEvent(
      new CustomEvent('story-panel-updated', { detail: { story: patchedStory, storyDetail: updated } }),
    );
    window.dispatchEvent(
      new CustomEvent('story-updated', {
        detail: { entity: 'story', action: 'updated', entity_id: updated.story.id, local: true },
      }),
    );
  }, []);

  const handleStoryArchived = useCallback(
    (archivedStoryId: string) => {
      handleClose();
      window.dispatchEvent(
        new CustomEvent('story-panel-archived', {
          detail: { storyId: archivedStoryId },
        }),
      );
    },
    [handleClose],
  );

  return (
    <StoryDetailPanel
      workspaceId={workspaceId}
      open={presentation.open}
      loading={presentation.loading}
      storyDetail={presentation.storyDetail}
      states={presentation.states}
      initialRecurringSummary={presentation.recurringSummary}
      onOpenChange={(isOpen) => {
        if (!isOpen) handleClose();
      }}
      onStoryUpdated={handleStoryUpdated}
      onStoryArchived={handleStoryArchived}
    />
  );
}
