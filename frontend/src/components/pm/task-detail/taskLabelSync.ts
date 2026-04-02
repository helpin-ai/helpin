import type { StoryDetail } from '@/lib/pmTypes';

interface LabelSyncResult {
  error?: string | null;
}

interface ReloadTaskResult {
  data?: StoryDetail | null;
  error?: string | null;
}

interface SyncTaskLabelsWithFeedbackOptions {
  workspaceId: string;
  storyId: string;
  currentLabelIds: string[];
  nextLabelIds: string[];
  syncLabels: (workspaceId: string, storyId: string, currentIds: string[], nextIds: string[]) => Promise<LabelSyncResult[]>;
  reloadTask: (workspaceId: string, storyId: string) => Promise<ReloadTaskResult>;
  onTaskUpdated: (story: StoryDetail) => void;
  onSaved?: () => void | Promise<void>;
  setSaving: (saving: boolean) => void;
  setSaveError: (error: string | null) => void;
}

export async function syncTaskLabelsWithFeedback({
  workspaceId,
  storyId,
  currentLabelIds,
  nextLabelIds,
  syncLabels,
  reloadTask,
  onTaskUpdated,
  onSaved,
  setSaving,
  setSaveError,
}: SyncTaskLabelsWithFeedbackOptions) {
  setSaving(true);
  try {
    const results = await syncLabels(workspaceId, storyId, currentLabelIds, nextLabelIds);
    const syncError = results.find((result) => result?.error)?.error;
    if (syncError) {
      setSaveError(syncError);
      return;
    }

    const { data, error } = await reloadTask(workspaceId, storyId);
    if (error || !data) {
      setSaveError(error ?? 'Failed to save changes');
      return;
    }

    setSaveError(null);
    onTaskUpdated(data);
    await onSaved?.();
  } catch (error) {
    setSaveError(error instanceof Error ? error.message : 'Failed to save changes');
  } finally {
    setSaving(false);
  }
}
