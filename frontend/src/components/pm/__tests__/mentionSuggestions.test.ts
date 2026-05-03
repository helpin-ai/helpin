import { describe, expect, it } from 'vitest'

import { filterMentionTeams, getMentionSuggestions } from '../mentionSuggestions'

describe('getMentionSuggestions', () => {
  it('returns up to the limit of suggestions when many members and a team share a query prefix', () => {
    // The current sort puts members first (handle prefix-match is the
    // primary key, then members before teams). With 10 prefix-matching
    // members and a 6-item limit, all 6 slots go to members.
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
    expect(results.every((item) => item.type === 'member')).toBe(true)
  })

  it('includes members regardless of active/inactive status', () => {
    // The mention-suggestion layer does not filter by status — that
    // policy belongs to whichever caller wants it (e.g. assignment UIs).
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

    expect(results).toEqual([
      {
        avatarUrl: undefined,
        handle: 'alice.example',
        id: 'member-1',
        label: 'Alice Example',
        secondaryText: 'alice@example.com',
        type: 'member',
      },
    ])
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
