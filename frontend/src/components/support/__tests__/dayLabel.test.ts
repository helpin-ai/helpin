import { afterEach, describe, expect, it, vi } from 'vitest';
import { getDayLabel } from '../helpers';

describe('timeline date labels', () => {
  afterEach(() => vi.useRealTimers());

  it.each([
    ['2026-09-13T09:00:00', 'Today'],
    ['2026-09-12T09:00:00', 'Yesterday'],
    ['2026-09-11T09:00:00', 'Fri, Sep 11'],
    ['2026-03-27T09:00:00', 'Fri, Mar 27'],
    ['2025-09-11T09:00:00', 'Thu, Sep 11, 2025'],
    ['2026-09-14T09:00:00', 'Mon, Sep 14'],
  ])('formats %s as %s', (date, expected) => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-09-13T12:00:00'));
    expect(getDayLabel(date)).toBe(expected);
  });

  it('keeps Yesterday across a year boundary', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2027-01-01T12:00:00'));
    expect(getDayLabel('2026-12-31T23:00:00')).toBe('Yesterday');
  });
});
