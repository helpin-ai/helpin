import { describe, expect, it, vi } from 'vitest';

import { syncTaskLabelsWithFeedback } from '../taskLabelSync';

describe('syncTaskLabelsWithFeedback', () => {
  it('toggles save feedback and updates the task on success', async () => {
    const setSaving = vi.fn();
    const setSaveError = vi.fn();
    const syncLabels = vi.fn().mockResolvedValue([{ error: null }]);
    const reloadTask = vi.fn().mockResolvedValue({ data: { task: { id: 'task-1' } } });
    const reloadActivity = vi.fn();
    const onTaskUpdated = vi.fn();

    await syncTaskLabelsWithFeedback({
      currentLabelIds: ['label-a'],
      nextLabelIds: ['label-b'],
      onTaskUpdated,
      onSaved: reloadActivity,
      reloadTask,
      setSaveError,
      setSaving,
      taskId: 'task-1',
      syncLabels,
      workspaceId: 'ws-1',
    });

    expect(setSaving).toHaveBeenNthCalledWith(1, true);
    expect(syncLabels).toHaveBeenCalledWith('ws-1', 'task-1', ['label-a'], ['label-b']);
    expect(reloadTask).toHaveBeenCalledWith('ws-1', 'task-1');
    expect(onTaskUpdated).toHaveBeenCalledWith({ task: { id: 'task-1' } });
    expect(reloadActivity).toHaveBeenCalledTimes(1);
    expect(setSaveError).toHaveBeenCalledWith(null);
    expect(setSaving).toHaveBeenLastCalledWith(false);
  });

  it('surfaces sync failure through the save error state', async () => {
    const setSaving = vi.fn();
    const setSaveError = vi.fn();
    const syncLabels = vi.fn().mockResolvedValue([{ error: 'Label add failed' }]);
    const reloadTask = vi.fn();
    const onTaskUpdated = vi.fn();

    await syncTaskLabelsWithFeedback({
      currentLabelIds: ['label-a'],
      nextLabelIds: ['label-b'],
      onTaskUpdated,
      reloadTask,
      setSaveError,
      setSaving,
      taskId: 'task-1',
      syncLabels,
      workspaceId: 'ws-1',
    });

    expect(setSaveError).toHaveBeenCalledWith('Label add failed');
    expect(reloadTask).not.toHaveBeenCalled();
    expect(onTaskUpdated).not.toHaveBeenCalled();
    expect(setSaving).toHaveBeenLastCalledWith(false);
  });
});
