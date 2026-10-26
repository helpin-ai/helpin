import { describe, expect, it } from 'vitest'
import type { SupportConversationSLA } from '../pmTypes'
import { describeSLAItem, formatSLARemaining, snoozePresets, supportSLAChip } from '../supportSLA'

const now = new Date('2026-10-26T10:00:00Z').getTime()
const at = (minutes: number) => new Date(now + minutes * 60_000).toISOString()

describe('formatSLARemaining', () => {
  it('formats signed durations compactly', () => {
    expect(formatSLARemaining(30_000)).toBe('<1m')
    expect(formatSLARemaining(12 * 60_000)).toBe('12m')
    expect(formatSLARemaining(125 * 60_000)).toBe('2h 5m')
    expect(formatSLARemaining(26 * 60 * 60_000)).toBe('1d 2h')
    expect(formatSLARemaining(-5 * 60_000)).toBe('-5m')
  })
})

describe('supportSLAChip', () => {
  const sla = (metrics: SupportConversationSLA['metrics']): SupportConversationSLA => ({ policy_name: 'Standard', metrics, breached: false })

  it('picks the open target due soonest and grades urgency', () => {
    const chip = supportSLAChip(sla([
      { metric: 'resolution', state: 'pending', due_at: at(600) },
      { metric: 'first_response', state: 'pending', due_at: at(10) },
    ]), now)
    expect(chip).toMatchObject({ metric: 'first_response', tone: 'urgent', remaining: '10m' })
    expect(supportSLAChip(sla([{ metric: 'first_response', state: 'pending', due_at: at(45) }]), now)?.tone).toBe('warning')
    expect(supportSLAChip(sla([{ metric: 'first_response', state: 'pending', due_at: at(240) }]), now)?.tone).toBe('ok')
  })

  it('shows an open breach as overdue and ignores finished targets', () => {
    expect(supportSLAChip(sla([{ metric: 'first_response', state: 'breached', due_at: at(-20) }]), now)).toMatchObject({ tone: 'breached', remaining: '-20m' })
    expect(supportSLAChip(sla([
      { metric: 'first_response', state: 'breached', due_at: at(-20), achieved_at: at(-5) },
      { metric: 'resolution', state: 'achieved', due_at: at(60) },
      { metric: 'next_response', state: 'paused' },
    ]), now)).toBeNull()
    expect(supportSLAChip(null, now)).toBeNull()
  })
})

describe('describeSLAItem', () => {
  it('describes each state', () => {
    expect(describeSLAItem({ metric: 'first_response', state: 'pending', due_at: at(30) }, now)).toBe('First response: due in 30m')
    expect(describeSLAItem({ metric: 'resolution', state: 'paused' }, now)).toBe('Resolution: paused while waiting on the customer')
    expect(describeSLAItem({ metric: 'next_response', state: 'achieved' }, now)).toBe('Next response: met')
    expect(describeSLAItem({ metric: 'first_response', state: 'breached', due_at: at(-90) }, now)).toBe('First response: overdue by 1h 30m')
    expect(describeSLAItem({ metric: 'first_response', state: 'breached', due_at: at(-90), achieved_at: at(-10) }, now)).toBe('First response: missed')
    expect(describeSLAItem({ metric: 'first_response', state: 'cancelled' }, now)).toBe('First response: not needed')
  })

  it('explains AI handling and policies that no longer apply', () => {
    expect(describeSLAItem({ metric: 'first_response', state: 'paused' }, now, { ai_handling: true })).toBe('First response: waits while the AI answers')
    expect(describeSLAItem({ metric: 'resolution', state: 'cancelled' }, now, { ended: true })).toBe('Resolution: stopped, no policy applies')
    expect(describeSLAItem({ metric: 'first_response', state: 'breached', due_at: at(-90) }, now, { ended: true })).toBe('First response: missed')
  })
})

describe('snoozePresets', () => {
  it('offers later today in the morning and next Monday at nine', () => {
    const wednesdayMorning = new Date(2026, 9, 28, 9, 30)
    const presets = snoozePresets(wednesdayMorning)
    expect(presets.map((preset) => preset.key)).toEqual(['later', 'tomorrow', 'monday', 'week'])
    expect(presets[0].until).toEqual(new Date(2026, 9, 28, 12, 30))
    expect(presets[1].until).toEqual(new Date(2026, 9, 29, 9, 0))
    expect(presets[2].until).toEqual(new Date(2026, 10, 2, 9, 0))
    expect(presets[3].until).toEqual(new Date(2026, 10, 4, 9, 0))
  })

  it('drops later today in the evening and skips to the following Monday on Mondays', () => {
    const mondayEvening = new Date(2026, 9, 26, 22, 0)
    const presets = snoozePresets(mondayEvening)
    expect(presets.map((preset) => preset.key)).toEqual(['tomorrow', 'monday', 'week'])
    expect(presets[1].until).toEqual(new Date(2026, 10, 2, 9, 0))
  })
})
