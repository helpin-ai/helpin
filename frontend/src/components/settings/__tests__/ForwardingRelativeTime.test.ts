import { expect, it } from 'vitest';
import { forwardingRelativeTime } from '../ForwardingRelativeTime';
it('formats recent and older delivery times without throwing for bad data', () => {
  const now = Date.parse('2026-09-09T12:00:00Z');
  expect(forwardingRelativeTime('2026-09-09T11:59:59Z', now)).toBe('just now');
  expect(forwardingRelativeTime('2026-09-09T11:59:00Z', now)).toContain('1 min');
  expect(forwardingRelativeTime('2026-09-08T12:00:00Z', now)).toContain('1 day');
  expect(forwardingRelativeTime('invalid', now)).toBeNull();
});
