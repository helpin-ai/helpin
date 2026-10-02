import type { CoveragePipelineHealthV2 } from '@/lib/supportCoverageTypes'

const pauseReasons: Record<string, string> = {
  terminal_failure_rate: 'too many analysis failures',
  duplicate_idempotency_violation: 'duplicate processing was detected',
  cross_workspace_invariant: 'workspace isolation needs attention',
  auto_attach_precision: 'topic matching needs review',
}

export function coverageHealthPresentation(health: CoveragePipelineHealthV2) {
  if (!health.rollout.capture_enabled) {
    return { title: 'Analysis not enabled', description: 'An administrator needs to enable coverage analysis in the server configuration.' }
  }
  if (health.rollout.pause_reasons?.length) {
    const reasons = health.rollout.pause_reasons.map(reason => pauseReasons[reason] ?? 'a processing check needs attention')
    return { title: 'Processing paused', description: `New results are held because ${reasons.join('; ')}. An administrator needs to resolve these checks before processing resumes.` }
  }
  if (health.latest_batch?.status === 'running') {
    return { title: 'Analyzing conversations', description: '' }
  }
  if (health.latest_batch?.status === 'queued') {
    return { title: 'Analysis queued', description: '' }
  }
  if (health.failures.length || health.latest_batch?.status === 'failed' || health.latest_batch?.status === 'partial_failed' || !health.healthy) {
    return { title: 'Analysis needs attention', description: 'Review the failed attempts below. An administrator can retry them after resolving the cause.' }
  }
  if (!health.latest_batch) {
    return { title: 'Waiting for the first analysis', description: 'Eligible conversations are checked every three hours.' }
  }
  return { title: 'Analysis completed', description: '' }
}
