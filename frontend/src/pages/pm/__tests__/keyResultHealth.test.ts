import { describe, expect, it } from 'vitest';
import { keyResultHealth } from '../keyResultHealth';

describe('key-result timeline status', () => {
  const now = new Date(2026, 8, 16, 12);
  const start = '2026-09-01T00:00:00Z', end = '2026-10-01T00:00:00Z';
  it('compares achievement with elapsed calendar days, allowing ten percentage points', () => {
    expect(keyResultHealth(50, start, end, now).label).toBe('On track');
    expect(keyResultHealth(40, start, end, now).label).toBe('Slightly behind');
    expect(keyResultHealth(39.9, start, end, now).label).toBe('Behind');
  });
  it('keeps incomplete results overdue after the entire target day, even without a start date', () => {
    expect(keyResultHealth(99, start, end, new Date(2026, 9, 1, 23, 59)).label).toBe('Slightly behind');
    expect(keyResultHealth(99, undefined, end, new Date(2026, 9, 2)).label).toBe('Overdue');
    expect(keyResultHealth(100, undefined, end, new Date(2026, 9, 2)).label).toBe('Complete');
  });
  it('does not invent a pace for missing, invalid, inverted, or future dates', () => {
    for (const dates of [[undefined, undefined], [undefined, end], [start, undefined], ['bad', end], [end, start], [end, '2026-11-01']]) {
      expect(keyResultHealth(20, dates[0], dates[1], now).label).toBeNull();
    }
  });
});
