import type { StoryDetail } from '@/lib/pmTypes';

interface LabelSyncResult {
  error?: string | null;
}

interface ReloadStoryResult {
  data?: StoryDetail | null;
  error?: string | null;
}

interface SyncStoryLabelsWithFeedbackOptions {
  workspaceId: string;
  storyId: string;
  currentLabelIds: string[];
  nextLabelIds: string[];
  syncLabels: (workspaceId: string, storyId: string, currentIds: string[], nextIds: string[]) => Promise<LabelSyncResult[]>;
  reloadStory: (workspaceId: string, storyId: string) => Promise<ReloadStoryResult>;
  onStoryUpdated: (story: StoryDetail) => void;
  onSaved?: () => void | Promise<void>;
  setSaving: (saving: boolean) => void;
  setSaveError: (error: string | null) => void;
}

export async function syncStoryLabelsWithFeedback({
  workspaceId,
  storyId,
  currentLabelIds,
  nextLabelIds,
  syncLabels,
  reloadStory,
  onStoryUpdated,
  onSaved,
  setSaving,
  setSaveError,
}: SyncStoryLabelsWithFeedbackOptions) {
  setSaving(true);
  try {
    const results = await syncLabels(workspaceId, storyId, currentLabelIds, nextLabelIds);
    const syncError = results.find((result) => result?.error)?.error;
    if (syncError) {
      setSaveError(syncError);
      return;
    }

    const { data, error } = await reloadStory(workspaceId, storyId);
    if (error || !data) {
      setSaveError(error ?? 'Failed to save changes');
      return;
    }

    setSaveError(null);
    onStoryUpdated(data);
    await onSaved?.();
  } catch (error) {
    setSaveError(error instanceof Error ? error.message : 'Failed to save changes');
  } finally {
    setSaving(false);
  }
}
