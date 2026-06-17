import { afterEach, describe, expect, it, vi } from 'vitest';

import { timeAgo } from '@/lib/utils';

describe('timeAgo', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('keeps recent day-old timestamps relative for up to 30 days', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-06-17T12:00:00Z'));

    expect(timeAgo('2026-06-02T12:00:00Z')).toBe('15 days ago');
    expect(timeAgo('2026-06-16T12:00:00Z')).toBe('1 day ago');
  });

  it('uses an absolute date for timestamps 30 days or older', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-06-17T12:00:00Z'));

    expect(timeAgo('2026-05-18T12:00:00Z')).toBe('May 18, 12:00 PM');
  });
});
