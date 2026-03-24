import { describe, expect, it } from 'vitest'

import { filterMentionTeams, getMentionSuggestions } from '../mentionSuggestions'

describe('getMentionSuggestions', () => {
  it('keeps an exact team handle visible when crowded member matches fill the limit', () => {
    const members = Array.from({ length: 10 }, (_, index) => ({
      id: `member-${index}`,
      user_id: `user-${index}`,
      role: 'member' as const,
      email: `eng${index}@example.com`,
      display_name: `Engineer ${index}`,
      status: 'active' as const,
    }))

    const results = getMentionSuggestions(
      'eng',
      members,
      [
        {
          id: 'team-1',
          name: 'Engineering',
          handle: 'eng',
        },
      ],
      6,
    )

    expect(results).toHaveLength(6)
    expect(results.some((item) => item.type === 'team' && item.handle === 'eng')).toBe(true)
  })

  it('keeps inactive members out of the suggestion set', () => {
    const results = getMentionSuggestions(
      'alice',
      [
        {
          id: 'member-1',
          role: 'member',
          email: 'alice@example.com',
          display_name: 'Alice Example',
          status: 'inactive',
        },
      ],
      [],
      6,
    )

    expect(results).toEqual([])
  })

  it('filters teams down to the allowed team ids', () => {
    const results = filterMentionTeams(
      [
        { id: 'team-1', name: 'Engineering', handle: 'eng' },
        { id: 'team-2', name: 'Design', handle: 'design' },
      ],
      ['team-2'],
    )

    expect(results).toEqual([{ id: 'team-2', name: 'Design', handle: 'design' }])
  })
})
