import { describe, expect, it } from 'vitest'
import {
  formatClusterRebuildSuccess,
  isClusterRebuildResult,
} from '@/lib/supportCoverageClusterRebuild'

describe('support coverage cluster rebuild response handling', () => {
  it('rejects missing or malformed rebuild results', () => {
    expect(isClusterRebuildResult(null)).toBe(false)
    expect(isClusterRebuildResult({})).toBe(false)
    expect(isClusterRebuildResult({ auto_merged: 0, suggestions_created: 0 })).toBe(false)
  })

  it('formats a real rebuild result with scanned count', () => {
    expect(
      formatClusterRebuildSuccess({
        run_id: 'run-1',
        status: 'completed',
        gaps_scanned: 12,
        clusters_found: 3,
        auto_merged: 1,
        kept_separate: 3,
        suggestions_created: 2,
        skipped: 0,
        embedding_status: 'complete',
        embeddings_created: 4,
        started_at: '2026-06-17T14:00:00Z',
        completed_at: '2026-06-17T14:00:02Z',
      }),
    ).toBe('Duplicate check complete: 1 merged, 3 kept separate, 2 unclear pairs left as merge suggestions.')
  })

  it('formats embedding failure as a failed rebuild', () => {
    expect(
      formatClusterRebuildSuccess({
        run_id: 'run-1',
        status: 'failed',
        gaps_scanned: 12,
        clusters_found: 0,
        auto_merged: 0,
        suggestions_created: 0,
        skipped: 0,
        embedding_status: 'failed',
        embedding_error: 'provider down',
        started_at: '2026-06-17T14:00:00Z',
      }),
    ).toBe('Duplicate check failed: provider down')
  })
})
