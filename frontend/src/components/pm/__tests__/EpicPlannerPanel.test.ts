import { describe, expect, it } from 'vitest';

import { nextCompletedRunNotificationId } from '../EpicPlannerPanel';

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
