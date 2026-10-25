import { describe, expect, it } from 'vitest'
import { compactMacroActions, describeMacroActions, hasMacroActions } from '../supportMacros'

describe('macro helpers', () => {
  it('detects whether a shortcut has actions', () => {
    expect(hasMacroActions(undefined)).toBe(false)
    expect(hasMacroActions({})).toBe(false)
    expect(hasMacroActions({ add_tag_ids: [] })).toBe(false)
    expect(hasMacroActions({ assign_user_id: '' })).toBe(true)
    expect(hasMacroActions({ snooze_minutes: 60 })).toBe(true)
  })

  it('describes actions in the order they run', () => {
    expect(describeMacroActions({
      status: 'resolved', priority: 'low', assign_user_id: 'u1', mailbox_id: 'shared',
      add_tag_ids: ['t1'], remove_tag_ids: ['t2'],
    }, {
      tags: new Map([['t1', 'Billing'], ['t2', 'Bug']]),
      members: new Map([['u1', 'Ada']]),
    })).toEqual(['move to Main inbox', 'assign to Ada', 'set priority low', 'tag Billing', 'remove tag Bug', 'resolve'])
    expect(describeMacroActions({ assign_user_id: '', snooze_minutes: 1440, mailbox_id: 'gone' })).toEqual(['move to another inbox', 'unassign', 'snooze for 1 day'])
  })

  it('drops unset fields', () => {
    expect(compactMacroActions({ status: undefined, add_tag_ids: [], priority: 'high' })).toEqual({ priority: 'high' })
  })
})
