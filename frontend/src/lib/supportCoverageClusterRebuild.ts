import type { SupportCoverageClusterRebuildResult } from '@/lib/supportCoverageTypes'

export function isClusterRebuildResult(value: unknown): value is SupportCoverageClusterRebuildResult {
  if (!value || typeof value !== 'object') return false
  const result = value as Partial<SupportCoverageClusterRebuildResult>
  return (
    typeof result.run_id === 'string' &&
    result.run_id.length > 0 &&
    typeof result.gaps_scanned === 'number' &&
    typeof result.auto_merged === 'number' &&
    typeof result.suggestions_created === 'number'
  )
}

export function formatClusterRebuildSuccess(result: SupportCoverageClusterRebuildResult): string {
  return `Cluster rebuild complete: scanned ${result.gaps_scanned} gaps, ${result.auto_merged} merged, ${result.suggestions_created} for review.`
}
