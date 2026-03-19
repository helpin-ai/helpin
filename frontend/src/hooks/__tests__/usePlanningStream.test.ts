import { describe, expect, it } from 'vitest';

import {
  planningPendingLabel,
  shouldShowPlanningEmptyState,
} from '../usePlanningStream';

describe('usePlanningStream helpers', () => {
  it('returns explicit copy for startup and retry states', () => {
    expect(planningPendingLabel('starting')).toBe('Starting planner...');
    expect(planningPendingLabel('retrying')).toBe('Planner stalled, retrying once...');
    expect(planningPendingLabel('message')).toBe('Thinking...');
  });

  it('suppresses the empty placeholder while a turn is pending', () => {
    expect(
      shouldShowPlanningEmptyState({
        messageCount: 0,
        isStreaming: false,
        turnPending: true,
        streamError: null,
      }),
    ).toBe(false);
  });
});
