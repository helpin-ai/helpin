import { describe, expect, it } from 'vitest'
import { formatPortalReplyBy, formatPortalRelativeTime } from '../portalTime'

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

describe('formatPortalReplyBy', () => {
  const now = new Date(2026, 9, 28, 10, 0)
  const at = (days: number, hours: number) => new Date(2026, 9, 28 + days, hours, 30).toISOString()
  const time = (iso: string) => new Date(iso).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })

  it('names the day relative to today', () => {
    expect(formatPortalReplyBy(at(0, 14), now)).toBe(`today at ${time(at(0, 14))}`)
    expect(formatPortalReplyBy(at(1, 9), now)).toBe(`tomorrow at ${time(at(1, 9))}`)
    expect(formatPortalReplyBy(at(3, 9), now)).toMatch(/^[A-Z][a-z]+day at /)
    expect(formatPortalReplyBy(at(10, 9), now)).toMatch(/ at /)
    expect(formatPortalReplyBy(at(10, 9), now)).not.toMatch(/day at/)
  })

  it('shows nothing once the time has passed', () => {
    expect(formatPortalReplyBy(at(0, 9), now)).toBeNull()
    expect(formatPortalReplyBy(null, now)).toBeNull()
  })
})
