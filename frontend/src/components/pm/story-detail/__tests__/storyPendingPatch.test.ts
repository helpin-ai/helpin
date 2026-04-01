import { describe, expect, it } from 'vitest';
import { getFlushablePendingStoryPatch, hasPendingStorySave } from '@/components/pm/story-detail/storyPendingPatch';

describe('getFlushablePendingStoryPatch', () => {
  it('returns metadata-only patches so they can be flushed on close', () => {
    expect(
      getFlushablePendingStoryPatch(
        {
          epic_id: 'epic-123',
        },
        0,
      ),
    ).toEqual({
      epic_id: 'epic-123',
    });
  });

  it('returns null for empty pending patches', () => {
    expect(getFlushablePendingStoryPatch({}, 0)).toBeNull();
  });

  it('returns null for description patches while uploads are still pending', () => {
    expect(
      getFlushablePendingStoryPatch(
        {
          description: '<p>draft</p>',
        },
        1,
      ),
    ).toBeNull();
  });

  it('treats a flushable metadata patch as a pending save for the top indicator', () => {
    expect(
      hasPendingStorySave(
        {
          sprint_id: 'sprint-123',
        },
        0,
      ),
    ).toBe(true);
  });

  it('does not treat blocked description uploads as a pending save', () => {
    expect(
      hasPendingStorySave(
        {
          description: '<p>draft</p>',
        },
        1,
      ),
    ).toBe(false);
  });
});
