import { describe, expect, it, vi } from 'vitest';

import { syncStoryLabelsWithFeedback } from '../storyLabelSync';

describe('syncStoryLabelsWithFeedback', () => {
  it('toggles save feedback and updates the story on success', async () => {
    const setSaving = vi.fn();
    const setSaveError = vi.fn();
    const syncLabels = vi.fn().mockResolvedValue([{ error: null }]);
    const reloadStory = vi.fn().mockResolvedValue({ data: { story: { id: 'story-1' } } });
    const reloadActivity = vi.fn();
    const onStoryUpdated = vi.fn();

    await syncStoryLabelsWithFeedback({
      currentLabelIds: ['label-a'],
      nextLabelIds: ['label-b'],
      onStoryUpdated,
      onSaved: reloadActivity,
      reloadStory,
      setSaveError,
      setSaving,
      storyId: 'story-1',
      syncLabels,
      workspaceId: 'ws-1',
    });

    expect(setSaving).toHaveBeenNthCalledWith(1, true);
    expect(syncLabels).toHaveBeenCalledWith('ws-1', 'story-1', ['label-a'], ['label-b']);
    expect(reloadStory).toHaveBeenCalledWith('ws-1', 'story-1');
    expect(onStoryUpdated).toHaveBeenCalledWith({ story: { id: 'story-1' } });
    expect(reloadActivity).toHaveBeenCalledTimes(1);
    expect(setSaveError).toHaveBeenCalledWith(null);
    expect(setSaving).toHaveBeenLastCalledWith(false);
  });

  it('surfaces sync failure through the save error state', async () => {
    const setSaving = vi.fn();
    const setSaveError = vi.fn();
    const syncLabels = vi.fn().mockResolvedValue([{ error: 'Label add failed' }]);
    const reloadStory = vi.fn();
    const onStoryUpdated = vi.fn();

    await syncStoryLabelsWithFeedback({
      currentLabelIds: ['label-a'],
      nextLabelIds: ['label-b'],
      onStoryUpdated,
      reloadStory,
      setSaveError,
      setSaving,
      storyId: 'story-1',
      syncLabels,
      workspaceId: 'ws-1',
    });

    expect(setSaveError).toHaveBeenCalledWith('Label add failed');
    expect(reloadStory).not.toHaveBeenCalled();
    expect(onStoryUpdated).not.toHaveBeenCalled();
    expect(setSaving).toHaveBeenLastCalledWith(false);
  });
});
