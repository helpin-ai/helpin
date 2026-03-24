import { describe, expect, it } from 'vitest'

import { buildChecklistMentionOptions } from '../ChecklistItems'

describe('buildChecklistMentionOptions', () => {
  it('returns active members and teams with their respective handles', () => {
    const results = buildChecklistMentionOptions(
      '',
      [
        {
          id: 'member-1',
          user_id: 'user-1',
          role: 'member',
          email: 'alice@example.com',
          display_name: 'Alice Smith',
          status: 'active',
        },
        {
          id: 'member-2',
          user_id: 'user-2',
          role: 'member',
          email: 'bob@example.com',
          display_name: 'Bob Jones',
          status: 'inactive',
        },
      ],
      [
        {
          id: 'team-1',
          name: 'Engineering',
          handle: 'engineering',
        },
        {
          id: 'team-2',
          name: 'Platform',
        },
      ],
    )

    expect(results).toEqual([
      {
        id: 'team-1',
        name: 'Engineering',
        handle: 'engineering',
        type: 'team',
      },
      {
        id: 'user-1',
        name: 'Alice Smith',
        handle: 'alice.smith',
        type: 'member',
        avatarUrl: undefined,
      },
    ])
  })

  it('filters results by either member or team text matches', () => {
    const results = buildChecklistMentionOptions(
      'eng',
      [
        {
          id: 'member-1',
          role: 'member',
          email: 'engineer@example.com',
          display_name: 'Jamie',
          status: 'active',
        },
      ],
      [
        {
          id: 'team-1',
          name: 'Engineering',
          handle: 'engineering',
        },
      ],
    )

    expect(results).toEqual([
      {
        id: 'team-1',
        name: 'Engineering',
        handle: 'engineering',
        type: 'team',
      },
      {
        id: 'member-1',
        name: 'Jamie',
        handle: 'jamie',
        type: 'member',
        avatarUrl: undefined,
      },
    ])
  })
})
