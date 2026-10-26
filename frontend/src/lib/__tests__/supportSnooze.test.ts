import { describe, expect, it } from 'vitest'
import { formatSnoozeUntil } from '../supportSnooze'

describe('formatSnoozeUntil', () => {
  const now = new Date(2026, 9, 26, 10, 0) // Monday

  it('shows only the time for later today', () => {
    expect(formatSnoozeUntil(new Date(2026, 9, 26, 15, 30), now)).toBe('15:30')
  })

  it('shows the weekday within the next week', () => {
    expect(formatSnoozeUntil(new Date(2026, 9, 30, 9, 0), now)).toBe('Fri 09:00')
  })

  it('shows the date a week or more ahead, so the same weekday is not ambiguous', () => {
    expect(formatSnoozeUntil(new Date(2026, 10, 2, 9, 0), now)).toBe('Nov 2, 09:00')
    expect(formatSnoozeUntil(new Date(2026, 11, 24, 9, 0), now)).toBe('Dec 24, 09:00')
  })
})
