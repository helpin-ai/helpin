import { addDays, isSameDay } from 'date-fns'
import type { CoveragePipelineHealthV2 } from '@/lib/supportCoverageTypes'

const MAX_ANALYSIS_ATTEMPTS = 3

export interface CoverageNextRun {
  at: string
  label: string
  /** Full date and time zone, for the tooltip. */
  detail: string
}

/** When the next automatic analysis starts, in the viewer's own time zone. */
export function coverageNextRun(health: CoveragePipelineHealthV2 | null | undefined, now = new Date()): CoverageNextRun | null {
  if (!health?.next_run_at) return null
  const at = new Date(health.next_run_at)
  if (!Number.isFinite(at.getTime()) || at <= now) return null
  const time = at.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
  const day = isSameDay(at, now)
    ? 'today'
    : isSameDay(at, addDays(now, 1))
      ? 'tomorrow'
      : at.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' })
  return {
    at: at.toISOString(),
    label: `Next analysis ${day} at ${time}`,
    detail: at.toLocaleString(undefined, {
      weekday: 'long', month: 'long', day: 'numeric', hour: 'numeric', minute: '2-digit', timeZoneName: 'short',
    }),
  }
}

export function coverageAnalysisActive(health: CoveragePipelineHealthV2 | null | undefined) {
  return health?.reanalysis_status === 'queued' || health?.reanalysis_status === 'running' ||
    health?.reanalysis_status === 'retrying' || health?.latest_batch?.status === 'running'
}

function needsAttention(health: CoveragePipelineHealthV2) {
  return health.reanalysis_status === 'failed' || health.latest_batch?.status === 'failed' || !health.healthy
}

function retryingDescription(health: CoveragePipelineHealthV2) {
  const attempt = health.reanalysis_attempt ?? 2
  const reason = health.reanalysis_error ? `: ${health.reanalysis_error}` : ''
  return `Attempt ${attempt - 1} of ${MAX_ANALYSIS_ATTEMPTS} failed${reason}. Retrying automatically.`
}

function attentionDescription(health: CoveragePipelineHealthV2) {
  if (health.reanalysis_error) return `Analysis stopped: ${health.reanalysis_error}`
  const stuck = health.failures.filter(failure => failure.status !== 'retryable').length
  if (stuck > 0) {
    return `${stuck} ${stuck === 1 ? 'conversation' : 'conversations'} could not be analyzed after ${MAX_ANALYSIS_ATTEMPTS} tries. Re-analyze to try again; contact an administrator if this continues.`
  }
  return 'The last analysis did not finish. Re-analyze to try again; contact an administrator if this continues.'
}

function retryLaterDescription(health: CoveragePipelineHealthV2) {
  const count = health.failures.length
  if (count === 0) return ''
  return `${count} ${count === 1 ? 'conversation' : 'conversations'} will be retried in the next run.`
}

export function coverageHealthPresentation(health: CoveragePipelineHealthV2) {
  if (!health.rollout.capture_enabled) {
    return { title: 'Analysis not enabled', description: 'An administrator needs to enable coverage analysis in the server configuration.' }
  }
  if (health.reanalysis_status === 'retrying') {
    return { title: 'Retrying analysis', description: retryingDescription(health) }
  }
  if (health.latest_batch?.status === 'running' || health.reanalysis_status === 'running') {
    return { title: 'Analyzing conversations', description: '' }
  }
  if (health.latest_batch?.status === 'queued' || health.reanalysis_status === 'queued') {
    return { title: 'Analysis queued', description: '' }
  }
  if (needsAttention(health)) {
    return { title: 'Analysis needs attention', description: attentionDescription(health) }
  }
  if (!health.latest_batch) {
    return { title: 'Waiting for the first analysis', description: 'Eligible conversations are checked every 12 hours.' }
  }
  return { title: 'Analysis completed', description: retryLaterDescription(health) }
}

export interface CoverageAnalysisControlState {
  pill: string | null
  disabled: boolean
  description: string
  /** The description is shown on the page because the user may need to act. */
  inline?: boolean
  /** How urgent an inline description is: retries and missing schedules warn, failures are errors. */
  tone?: 'warning' | 'error'
}

export function coverageAnalysisControl(health: CoveragePipelineHealthV2 | null, submitting = false): CoverageAnalysisControlState {
  if (!health || health.reanalysis_status === 'unavailable') {
    return { pill: 'Status unavailable', disabled: true, description: 'Reload the page to check whether analysis is available.' }
  }
  if (!health.rollout.capture_enabled) {
    return { pill: 'Not enabled', disabled: true, description: 'An administrator needs to enable coverage analysis.' }
  }
  if (health.reanalysis_status === 'retrying') {
    return { pill: 'Retrying', disabled: true, description: retryingDescription(health), inline: true, tone: 'warning' }
  }
  if (health.reanalysis_status === 'running' || health.latest_batch?.status === 'running') {
    return { pill: 'Analyzing', disabled: true, description: 'Conversations are being checked. Results appear when analysis finishes.' }
  }
  if (submitting || health.reanalysis_status === 'queued' || health.latest_batch?.status === 'queued') {
    return { pill: 'Queued', disabled: true, description: 'Analysis is queued and will start when a worker is available.' }
  }
  // The server refuses a manual run while a rollout check is paused, but the
  // automatic analysis keeps running, so the page shows the next run instead.
  if (health.rollout.pause_reasons?.length) {
    return { pill: null, disabled: true, description: 'Re-analyze is unavailable until recent analysis failures clear. Automatic analysis keeps running.' }
  }
  if (needsAttention(health)) {
    return { pill: 'Needs attention', disabled: false, description: attentionDescription(health), inline: true, tone: 'error' }
  }
  if (health.schedule_status === 'missing') {
    return { pill: 'Not scheduled', disabled: false, description: 'Automatic analysis is not scheduled, so new conversations are not being checked. An administrator needs to restart the API to restore it. Re-analyze still works.', inline: true, tone: 'warning' }
  }
  return { pill: null, disabled: false, description: '' }
}
