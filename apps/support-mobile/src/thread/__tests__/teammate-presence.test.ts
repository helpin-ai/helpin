import { describe, expect, it } from 'vitest'
import { sortTeammatesByPresence, teammatePresenceDotClass, teammatePresenceLabel } from '../teammate-presence'

const members = [
  { id: 'm-away', user_id: 'u-away', role: 'member', email: 'away@example.com', display_name: 'Away' },
  { id: 'm-offline', user_id: 'u-offline', role: 'member', email: 'offline@example.com', display_name: 'Offline' },
  { id: 'm-online', user_id: 'u-online', role: 'member', email: 'online@example.com', display_name: 'Online' },
]
const statuses = [
  { user_id: 'u-offline', status: 'offline' as const, source: 'auto' as const },
  { user_id: 'u-online', status: 'online' as const, source: 'auto' as const },
  { user_id: 'u-away', status: 'away' as const, source: 'manual' as const, manual_status: 'away' as const },
]

describe('teammate presence presentation', () => {
  it('orders online, away, then offline teammates', () => {
    expect(sortTeammatesByPresence(members, statuses).map((member) => member.user_id)).toEqual(['u-online', 'u-away', 'u-offline'])
  })

  it('provides accessible labels and stable color classes', () => {
    expect(teammatePresenceLabel('online')).toBe('Online')
    expect(teammatePresenceLabel()).toBe('Availability unknown')
    expect(teammatePresenceDotClass('away')).toContain('amber')
  })
})
