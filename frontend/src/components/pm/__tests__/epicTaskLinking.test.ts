import { describe, expect, it } from 'vitest'

import { getLinkTasksDisabledReason } from '../epicTaskLinking'

describe('getLinkTasksDisabledReason', () => {
  it('requires edit access', () => {
    expect(getLinkTasksDisabledReason(false, 'team-a')).toBe('You do not have permission to edit this epic.')
  })

  it('requires the epic to belong to a team', () => {
    expect(getLinkTasksDisabledReason(true, '')).toBe('Assign this epic to a team before linking tasks.')
  })

  it('allows linking for editable team-owned epics', () => {
    expect(getLinkTasksDisabledReason(true, 'team-a')).toBeNull()
  })
})
