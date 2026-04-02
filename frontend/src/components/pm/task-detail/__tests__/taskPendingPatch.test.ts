import { describe, expect, it } from 'vitest';
import { getFlushablePendingTaskPatch, hasPendingTaskSave } from '@/components/pm/task-detail/taskPendingPatch';

describe('getFlushablePendingTaskPatch', () => {
  it('returns metadata-only patches so they can be flushed on close', () => {
    expect(
      getFlushablePendingTaskPatch(
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
    expect(getFlushablePendingTaskPatch({}, 0)).toBeNull();
  });

  it('returns null for description patches while uploads are still pending', () => {
    expect(
      getFlushablePendingTaskPatch(
        {
          description: '<p>draft</p>',
        },
        1,
      ),
    ).toBeNull();
  });

  it('treats a flushable metadata patch as a pending save for the top indicator', () => {
    expect(
      hasPendingTaskSave(
        {
          sprint_id: 'sprint-123',
        },
        0,
      ),
    ).toBe(true);
  });

  it('does not treat blocked description uploads as a pending save', () => {
    expect(
      hasPendingTaskSave(
        {
          description: '<p>draft</p>',
        },
        1,
      ),
    ).toBe(false);
  });
});
