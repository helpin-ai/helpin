import { describe, expect, it } from 'vitest'
import type { CoveragePipelineHealthV2 } from '@/lib/supportCoverageTypes'
import { coverageHealthPresentation, coverageAnalysisControl } from '../coverageHealth'

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
    ['partial_failed', 'Analysis needs attention'],
  ])('presents %s accurately', (status, title) => {
    expect(coverageHealthPresentation(health(status)).title).toBe(title)
  })
  it('distinguishes disabled analysis from a paused pipeline', () => {
    const value = health()
    value.rollout.capture_enabled = false
    value.rollout.requested_mode = 'disabled'
    expect(coverageHealthPresentation(value).title).toBe('Analysis not enabled')
  })
  it('explains why assignments are paused even while capture continues', () => {
    const value = health('succeeded')
    value.rollout.assignment_enabled = false
    value.rollout.pause_reasons = ['terminal_failure_rate', 'duplicate_idempotency_violation']
    const state = coverageHealthPresentation(value)
    expect(state.title).toBe('Processing paused')
    expect(state.description).toContain('too many analysis failures')
    expect(state.description).toContain('duplicate processing')
    expect(state.description).not.toContain('terminal_failure_rate')
  })
  it('does not report success while unresolved attempts exist', () => {
    const value = health('succeeded')
    value.failures = [{ id: 'attempt' } as CoveragePipelineHealthV2['failures'][number]]
    expect(coverageHealthPresentation(value).title).toBe('Analysis needs attention')
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
 it('explains pauses', () => {
  const value=health('succeeded'); value.rollout.pause_reasons=['terminal_failure_rate']
  expect(coverageAnalysisControl(value)).toMatchObject({pill:'Paused',disabled:true})
  expect(coverageAnalysisControl(value).description).toContain('too many analysis failures')
 })
 it('allows retry after failure but blocks unknown/disabled states', () => {
  expect(coverageAnalysisControl(health('failed'))).toMatchObject({pill:'Needs attention',disabled:false})
  expect(coverageAnalysisControl(null)).toMatchObject({pill:'Status unavailable',disabled:true})
  const value=health(); value.rollout.capture_enabled=false
  expect(coverageAnalysisControl(value)).toMatchObject({pill:'Not enabled',disabled:true})
 })
})
