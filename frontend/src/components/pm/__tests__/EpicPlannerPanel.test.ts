import { describe, expect, it } from 'vitest';

import { nextCompletedRunNotificationId, shouldReloadEpicPlannerRuns } from '../EpicPlannerPanel';

describe('nextCompletedRunNotificationId', () => {
  it('notifies once for a newly completed run', () => {
    expect(
      nextCompletedRunNotificationId({ id: 'run-1', status: 'completed' }, null),
    ).toBe('run-1');
  });

  it('does not notify again for the same completed run id', () => {
    expect(
      nextCompletedRunNotificationId({ id: 'run-1', status: 'completed' }, 'run-1'),
    ).toBeNull();
  });

  it('does not notify for non-completed runs', () => {
    expect(
      nextCompletedRunNotificationId({ id: 'run-1', status: 'running' }, null),
    ).toBeNull();
  });
});

describe('shouldReloadEpicPlannerRuns', () => {
  it('reloads for queued and running events on the active epic', () => {
    expect(
      shouldReloadEpicPlannerRuns(
        { parent_type: 'epic', parent_id: 'epic-1', data: { status: 'queued' } },
        'epic-1',
      ),
    ).toBe(true);
    expect(
      shouldReloadEpicPlannerRuns(
        { parent_type: 'epic', parent_id: 'epic-1', data: { status: 'running' } },
        'epic-1',
      ),
    ).toBe(true);
  });

  it('ignores events for other targets', () => {
    expect(
      shouldReloadEpicPlannerRuns(
        { parent_type: 'task', parent_id: 'epic-1' },
        'epic-1',
      ),
    ).toBe(false);
    expect(
      shouldReloadEpicPlannerRuns(
        { parent_type: 'epic', parent_id: 'epic-2' },
        'epic-1',
      ),
    ).toBe(false);
  });
});
