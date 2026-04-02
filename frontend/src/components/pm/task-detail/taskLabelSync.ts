import type { TaskDetail } from '@/lib/pmTypes';

interface LabelSyncResult {
  error?: string | null;
}

interface ReloadTaskResult {
  data?: TaskDetail | null;
  error?: string | null;
}

interface SyncTaskLabelsWithFeedbackOptions {
  workspaceId: string;
  taskId: string;
  currentLabelIds: string[];
  nextLabelIds: string[];
  syncLabels: (workspaceId: string, taskId: string, currentIds: string[], nextIds: string[]) => Promise<LabelSyncResult[]>;
  reloadTask: (workspaceId: string, taskId: string) => Promise<ReloadTaskResult>;
  onTaskUpdated: (detail: TaskDetail) => void;
  onSaved?: () => void | Promise<void>;
  setSaving: (saving: boolean) => void;
  setSaveError: (error: string | null) => void;
}

export async function syncTaskLabelsWithFeedback({
  workspaceId,
  taskId,
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
    const results = await syncLabels(workspaceId, taskId, currentLabelIds, nextLabelIds);
    const syncError = results.find((result) => result?.error)?.error;
    if (syncError) {
      setSaveError(syncError);
      return;
    }

    const { data, error } = await reloadTask(workspaceId, taskId);
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
