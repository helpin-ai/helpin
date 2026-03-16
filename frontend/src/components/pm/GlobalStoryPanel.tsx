import { useCallback, useEffect, useState } from 'react';
import { toast } from 'sonner';
import { useStoryPanelStore } from '@/stores/storyPanelStore';
import { StoryDetailPanel } from '@/components/pm/StoryDetailPanel';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import type { StoryDetail, WorkflowState } from '@/lib/pmTypes';

interface GlobalStoryPanelProps {
  workspaceId: string;
}

export function GlobalStoryPanel({ workspaceId }: GlobalStoryPanelProps) {
  const open = useStoryPanelStore((s) => s.open);
  const storyId = useStoryPanelStore((s) => s.storyId);
  const storyDetail = useStoryPanelStore((s) => s.storyDetail);
  const requestKey = useStoryPanelStore((s) => s.requestKey);
  const close = useStoryPanelStore((s) => s.close);
  const reveal = useStoryPanelStore((s) => s.reveal);
  const setStoryDetail = useStoryPanelStore((s) => s.setStoryDetail);

  const [states, setStates] = useState<WorkflowState[]>([]);

  // Fetch story detail + workflow states, then reveal panel
  useEffect(() => {
    if (!storyId || !workspaceId) return;

    let cancelled = false;

    (async () => {
      try {
        const [storyRes, wfRes] = await Promise.all([
          pmStoryService.get(workspaceId, storyId),
          pmWorkflowService.list(workspaceId),
        ]);
        if (cancelled) return;
        if (!storyRes.data) {
          toast.error('Failed to load story');
          close();
          return;
        }

        const wf = wfRes.data?.find(
          (w) => w.workflow.id === storyRes.data!.story.workflow_id
        );
        setStates(wf?.states ?? []);
        reveal(storyRes.data);
      } catch {
        if (!cancelled) {
          toast.error('Failed to load story');
          close();
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [requestKey, workspaceId, reveal]);

  const handleStoryUpdated = useCallback(
    (updated: StoryDetail) => {
      setStoryDetail(updated);
      window.dispatchEvent(
        new CustomEvent('story-panel-updated', { detail: { story: updated } })
      );
    },
    [setStoryDetail]
  );

  const handleStoryArchived = useCallback(
    (archivedStoryId: string) => {
      close();
      window.dispatchEvent(
        new CustomEvent('story-panel-archived', {
          detail: { storyId: archivedStoryId },
        })
      );
    },
    [close]
  );

  return (
    <StoryDetailPanel
      workspaceId={workspaceId}
      open={open}
      storyDetail={storyDetail}
      states={states}
      onOpenChange={(isOpen) => {
        if (!isOpen) close();
      }}
      onStoryUpdated={handleStoryUpdated}
      onStoryArchived={handleStoryArchived}
    />
  );
}
