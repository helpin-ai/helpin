import { describe, expect, it } from 'vitest'
import type { SupportSLAPolicy } from '../pmTypes'
import { draftToPolicyInput, emptySLAPolicyDraft, formatSLATarget, policyToDraft, targetDraft } from '../supportSLAPolicyForm'

describe('SLA policy form', () => {
  it('requires a name and at least one target', () => {
    const draft = { ...emptySLAPolicyDraft(), firstResponse: { enabled: false, amount: '1', unit: 'hours' as const } }
    const result = draftToPolicyInput(draft)
    expect(result.input).toBeNull()
    expect(result.errors).toEqual(['Give the policy a name.', 'Turn on at least one target.'])
  })

  it('rejects fractional, zero and overlong targets', () => {
    const base = { ...emptySLAPolicyDraft(), name: 'X' }
    expect(draftToPolicyInput({ ...base, firstResponse: { enabled: true, amount: '1.5', unit: 'hours' } }).errors).toContain('First response needs a whole number greater than zero.')
    expect(draftToPolicyInput({ ...base, firstResponse: { enabled: true, amount: '0', unit: 'hours' } }).errors).toHaveLength(1)
    expect(draftToPolicyInput({ ...base, firstResponse: { enabled: true, amount: '91', unit: 'days' } }).errors).toContain('First response can be at most 90 days.')
  })

  it('converts units to minutes and normalizes the name', () => {
    const { input } = draftToPolicyInput({
      ...emptySLAPolicyDraft(),
      name: '  Urgent   customers ',
      priorities: ['urgent'],
      firstResponse: { enabled: true, amount: '30', unit: 'minutes' },
      nextResponse: { enabled: true, amount: '2', unit: 'hours' },
      resolution: { enabled: true, amount: '3', unit: 'days' },
    })
    expect(input).toMatchObject({
      name: 'Urgent customers', priorities: ['urgent'],
      first_response_minutes: 30, next_response_minutes: 120, resolution_minutes: 4320,
      pause_while_waiting: true, business_hours: false, enabled: true,
    })
  })

  it('round-trips a stored policy into the largest whole unit', () => {
    expect(targetDraft(1440, { enabled: false, amount: '1', unit: 'hours' })).toEqual({ enabled: true, amount: '1', unit: 'days' })
    expect(targetDraft(90, { enabled: false, amount: '1', unit: 'hours' })).toEqual({ enabled: true, amount: '90', unit: 'minutes' })
    expect(targetDraft(null, { enabled: true, amount: '4', unit: 'hours' })).toEqual({ enabled: false, amount: '4', unit: 'hours' })
    const policy = { name: 'P', enabled: false, priorities: ['high'], mailbox_ids: ['shared'], channels: ['email'], first_response_minutes: 60, next_response_minutes: null, resolution_minutes: 2880, business_hours: true, pause_while_waiting: false } as unknown as SupportSLAPolicy
    const draft = policyToDraft(policy)
    expect(draft.firstResponse).toEqual({ enabled: true, amount: '1', unit: 'hours' })
    expect(draft.nextResponse.enabled).toBe(false)
    expect(draftToPolicyInput(draft).input).toMatchObject({ first_response_minutes: 60, next_response_minutes: null, resolution_minutes: 2880, business_hours: true, pause_while_waiting: false, enabled: false })
  })

  it('formats targets for the policy list', () => {
    expect(formatSLATarget(30)).toBe('30m')
    expect(formatSLATarget(90)).toBe('1h 30m')
    expect(formatSLATarget(120)).toBe('2h')
    expect(formatSLATarget(2880)).toBe('2d')
    expect(formatSLATarget(null)).toBeNull()
  })
})
