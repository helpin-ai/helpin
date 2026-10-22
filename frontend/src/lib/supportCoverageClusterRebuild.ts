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
  if (result.status === 'failed' || result.embedding_status === 'failed') {
    return `Duplicate check failed: ${result.embedding_error || 'gap comparison failed'}`
  }
  const unclear = result.suggestions_created
  return `Duplicate check complete: ${result.auto_merged} merged, ${result.kept_separate ?? 0} kept separate, ${unclear} unclear ${unclear === 1 ? 'pair' : 'pairs'} left as merge suggestions.`
}
