import { describe, expect, it } from 'vitest';

import { dockDateGroup } from '../dockDateGroups';

describe('dockDateGroup', () => {
  const now = new Date('2026-08-18T15:00:00');

  it.each([
    ['2026-08-18T00:01:00', 'today'],
    ['2026-08-17T12:00:00', 'yesterday'],
    ['2026-08-13T12:00:00', 'previous_7_days'],
    ['2026-08-01T12:00:00', 'older'],
  ] as const)('groups %s under %s', (timestamp, expected) => {
    expect(dockDateGroup(timestamp, now)).toBe(expected);
  });
});
