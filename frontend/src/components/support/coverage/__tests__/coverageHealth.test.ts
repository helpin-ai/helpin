import { describe, expect, it } from 'vitest'
import type { CoveragePipelineHealthV2 } from '@/lib/supportCoverageTypes'
import { coverageHealthPresentation, coverageAnalysisControl, coverageNextRun } from '../coverageHealth'

const health = (status?: string): CoveragePipelineHealthV2 => ({
  healthy: true,
  failures: [],
  latest_batch: status ? {
    id: 'batch', status, candidate_count: 5, succeeded_count: 0,
    retryable_count: 0, dead_letter_count: 0, failure_class: '', failure_message: '',
  } : null,
  rollout: { requested_mode: 'v2_write', capture_enabled: true, assignment_enabled: true, read_v2_enabled: true, write_v2_enabled: true },
})

describe('coverage analysis status', () => {
  it.each([
    [undefined, 'Waiting for the first analysis'],
    ['queued', 'Analysis queued'],
    ['running', 'Analyzing conversations'],
    ['succeeded', 'Analysis completed'],
    ['failed', 'Analysis needs attention'],
  ])('presents %s accurately', (status, title) => {
    expect(coverageHealthPresentation(health(status)).title).toBe(title)
  })
  it('distinguishes disabled analysis from a paused pipeline', () => {
    const value = health()
    value.rollout.capture_enabled = false
    value.rollout.requested_mode = 'disabled'
    expect(coverageHealthPresentation(value).title).toBe('Analysis not enabled')
  })
  it('does not report a topic rollout pause as paused analysis', () => {
    const value = health('succeeded')
    value.rollout.assignment_enabled = false
    value.rollout.pause_reasons = ['terminal_failure_rate']
    expect(coverageHealthPresentation(value).title).toBe('Analysis completed')
  })
  it('does not report success while attempts need a person', () => {
    const value = health('partial_failed')
    value.healthy = false
    value.failures = [{ id: 'attempt', status: 'dead_letter' } as CoveragePipelineHealthV2['failures'][number]]
    const state = coverageHealthPresentation(value)
    expect(state.title).toBe('Analysis needs attention')
    expect(state.description).toContain('1 conversation could not be analyzed after 3 tries')
  })
  it('treats conversations that retry automatically as a completed run', () => {
    const value = health('partial_failed')
    value.failures = [
      { id: 'a', status: 'retryable' },
      { id: 'b', status: 'retryable' },
    ] as CoveragePipelineHealthV2['failures']
    const state = coverageHealthPresentation(value)
    expect(state.title).toBe('Analysis completed')
    expect(state.description).toBe('2 conversations will be retried in the next run.')
    expect(coverageAnalysisControl(value).pill).toBeNull()
  })
})

describe('compact analysis control', () => {
 it('hides routine completed and waiting states', () => {
  expect(coverageAnalysisControl(health('succeeded')).pill).toBeNull()
  expect(coverageAnalysisControl(health()).pill).toBeNull()
 })
 it('shows persisted work and prevents duplicates', () => {
  const value={...health('succeeded'),reanalysis_status:'queued' as const}
  expect(coverageAnalysisControl(value)).toMatchObject({pill:'Queued',disabled:true})
  expect(coverageAnalysisControl({...value,reanalysis_status:'running'})).toMatchObject({pill:'Analyzing',disabled:true})
 })
 it('holds manual runs during a rollout pause without a paused pill', () => {
  const value=health('succeeded'); value.rollout.pause_reasons=['terminal_failure_rate']
  expect(coverageAnalysisControl(value)).toMatchObject({pill:null,disabled:true})
  expect(coverageAnalysisControl(value).description).toContain('Automatic analysis keeps running')
  expect(coverageAnalysisControl({...value,reanalysis_status:'running'}).pill).toBe('Analyzing')
 })
 it('allows retry after failure but blocks unknown/disabled states', () => {
  expect(coverageAnalysisControl(health('failed'))).toMatchObject({pill:'Needs attention',disabled:false})
  expect(coverageAnalysisControl(null)).toMatchObject({pill:'Status unavailable',disabled:true})
  const value=health(); value.rollout.capture_enabled=false
  expect(coverageAnalysisControl(value)).toMatchObject({pill:'Not enabled',disabled:true})
 })
})

describe('next automatic analysis', () => {
  const now = new Date(2026, 9, 23, 9, 0)
  const time = (date: Date) => date.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
  const at = (date: Date) => ({ ...health('succeeded'), next_run_at: date.toISOString() })

  it('names today and tomorrow in the viewer time zone', () => {
    const tonight = new Date(2026, 9, 23, 18, 0)
    const morning = new Date(2026, 9, 24, 6, 0)
    expect(coverageNextRun(at(tonight), now)?.label).toBe(`Next analysis today at ${time(tonight)}`)
    expect(coverageNextRun(at(morning), now)?.label).toBe(`Next analysis tomorrow at ${time(morning)}`)
  })
  it('uses the date for later runs', () => {
    const later = new Date(2026, 9, 26, 6, 0)
    expect(coverageNextRun(at(later), now)?.label).toBe(
      `Next analysis ${later.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' })} at ${time(later)}`,
    )
  })
  it('shows nothing without a future run', () => {
    expect(coverageNextRun(health('succeeded'), now)).toBeNull()
    expect(coverageNextRun(at(new Date(2026, 9, 23, 8, 0)), now)).toBeNull()
    expect(coverageNextRun(null, now)).toBeNull()
  })
})

describe('analysis problems a person can act on', () => {
  it('shows why a reassessment stopped', () => {
    const value = { ...health('succeeded'), healthy: false, reanalysis_status: 'failed' as const, reanalysis_error: 'The analysis worker stopped responding.' }
    expect(coverageAnalysisControl(value)).toMatchObject({
      pill: 'Needs attention', disabled: false, inline: true,
      description: 'Analysis stopped: The analysis worker stopped responding.',
    })
  })
  it('shows an automatic retry with its reason and blocks duplicates', () => {
    const value = { ...health('succeeded'), reanalysis_status: 'retrying' as const, reanalysis_attempt: 2, reanalysis_error: 'provider unavailable' }
    expect(coverageAnalysisControl(value)).toMatchObject({
      pill: 'Retrying', disabled: true, inline: true,
      description: 'Attempt 1 of 3 failed: provider unavailable. Retrying automatically.',
    })
    expect(coverageHealthPresentation(value).title).toBe('Retrying analysis')
  })
  it('warns when automatic analysis is not scheduled', () => {
    const value = { ...health('succeeded'), schedule_status: 'missing' as const }
    expect(coverageAnalysisControl(value)).toMatchObject({ pill: 'Not scheduled', disabled: false, inline: true })
  })
  it('keeps routine states in the tooltip only', () => {
    const value = { ...health('succeeded'), reanalysis_status: 'queued' as const }
    expect(coverageAnalysisControl(value).inline).toBeUndefined()
  })
})
