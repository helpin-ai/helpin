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
        avatarUrl: undefined,
        handle: 'alice.smith',
        id: 'user-1',
        label: 'Alice Smith',
        secondaryText: 'alice@example.com',
        type: 'member',
      },
      {
        avatarUrl: undefined,
        handle: 'bob.jones',
        id: 'user-2',
        label: 'Bob Jones',
        secondaryText: 'bob@example.com',
        type: 'member',
      },
      {
        handle: 'engineering',
        id: 'team-1',
        label: 'Engineering',
        secondaryText: '@engineering',
        type: 'team',
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
        handle: 'engineering',
        id: 'team-1',
        label: 'Engineering',
        secondaryText: '@engineering',
        type: 'team',
      },
      {
        avatarUrl: undefined,
        handle: 'jamie',
        id: 'member-1',
        label: 'Jamie',
        secondaryText: 'engineer@example.com',
        type: 'member',
      },
    ])
  })
})
