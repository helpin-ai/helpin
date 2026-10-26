import { describe, expect, it } from 'vitest'
import { compactMacroActions, describeMacroActions, hasMacroActions, macroTextMarker, macroTextPresent } from '../supportMacros'

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

describe('macroTextPresent', () => {
  it('finds the inserted text despite formatting and spacing', () => {
    const marker = macroTextMarker('**Thanks!** I have asked billing to\n reissue the invoice.')
    expect(macroTextPresent('Hi Ada, Thanks! I have asked billing to reissue the invoice. Best', marker)).toBe(true)
  })

  it('matches text that starts with a link or a list', () => {
    expect(macroTextPresent('see our guide for steps.', macroTextMarker('see [our guide](https://help.example.com/a) for steps.'))).toBe(true)
    expect(macroTextPresent('Open settings\nChoose Billing', macroTextMarker('1. Open settings\n2. Choose Billing'))).toBe(true)
    expect(macroTextPresent('Heads up: it works', macroTextMarker('## Heads up: it works'))).toBe(true)
  })

  it('notices when the inserted text was deleted', () => {
    const marker = macroTextMarker('I have asked billing to reissue the invoice.')
    expect(macroTextPresent('Something else entirely', marker)).toBe(false)
    expect(macroTextPresent('anything', '')).toBe(false)
  })
})
