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
    return `Cluster rebuild failed: ${result.embedding_error || 'embedding backfill failed'}`
  }
  const embeddings = result.embeddings_created ?? 0
  const skipped = result.missing_embeddings ?? result.skipped ?? 0
  return `Cluster rebuild complete: scanned ${result.gaps_scanned} gaps, ${embeddings} embeddings backfilled, ${result.auto_merged} merged, ${result.suggestions_created} for review, ${skipped} skipped.`
}
