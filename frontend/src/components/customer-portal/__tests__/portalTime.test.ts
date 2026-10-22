import { describe, expect, it } from 'vitest'
import { formatPortalRelativeTime } from '../portalTime'

describe('formatPortalRelativeTime', () => {
  const now = new Date('2026-09-28T12:00:00Z').getTime()
  it.each([
    ['2026-09-28T11:59:40Z', 'just now'],
    ['2026-09-28T11:55:00Z', '5 min ago'],
    ['2026-09-28T11:00:00Z', '1 hour ago'],
    ['2026-09-28T09:00:00Z', '3 hours ago'],
    ['2026-09-27T09:00:00Z', 'yesterday'],
    ['2026-09-25T09:00:00Z', '3 days ago'],
  ])('%s is %s', (value, expected) => {
    expect(formatPortalRelativeTime(value, now)).toBe(expected)
  })
  it('falls back to a date for older messages', () => {
    expect(formatPortalRelativeTime('2026-09-01T09:00:00Z', now)).toMatch(/Sep|9/)
    expect(formatPortalRelativeTime(null, now)).toBe('')
  })
})
