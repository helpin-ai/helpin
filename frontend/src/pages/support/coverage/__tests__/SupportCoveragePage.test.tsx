// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import type { CoveragePipelineHealthV2, SupportCoverageGapListItem } from '@/lib/supportCoverageTypes'

let health: CoveragePipelineHealthV2
const listGaps = vi.fn()
const toasts = vi.hoisted(() => ({ error: vi.fn(), warning: vi.fn(), success: vi.fn(), dismiss: vi.fn() }))

vi.mock('sonner', () => ({ toast: toasts }))

vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: (select: (state: unknown) => unknown) => select({ currentWorkspace: { id: 'ws-1', slug: 'acme' } }),
}))
vi.mock('@/hooks/queries/useDocs', () => ({
  useDocsSpaces: () => ({ data: [] }),
  useDocsCollections: () => ({ data: [] }),
}))
vi.mock('@/hooks/queries/useSession', () => ({
  useWorkspaceAccess: () => ({ data: {} }),
  usePermissions: () => ({ has: () => true }),
}))
vi.mock('@edition', () => ({ UpgradeRequiredDialog: () => null }))
vi.mock('@edition/errors', () => ({ getUpgradeRequiredReason: () => null }))
vi.mock('@/lib/services/supportCoverageService', () => ({
  supportCoverageService: {
    getSummary: async () => ({ data: { total_open_gaps: 1, new_gaps_this_week: 1, gaps_fixed_this_week: 0, total_evidence_count: 2 }, error: null }),
    listGaps: (...args: unknown[]) => listGaps(...args),
    getLatestClusterRebuild: async () => ({ data: null, error: null }),
    getPipelineHealthV2: async () => ({ data: health, error: null }),
  },
}))

import { SupportCoveragePage } from '../SupportCoveragePage'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const gap = {
  id: 'gap-1', workspace_id: 'ws-1', title: 'Reconnect Instagram after a token expires',
  gap_kind: 'content', gap_category: 'knowledge', status: 'open', confidence: 0.8,
  evidence_count: 2, first_seen_at: '2026-10-20T10:00:00Z', last_seen_at: '2026-10-21T10:00:00Z',
} as unknown as SupportCoverageGapListItem

let container: HTMLDivElement | null = null
let root: Root | null = null

beforeEach(() => {
  health = {
    healthy: true, failures: [], latest_batch: null,
    rollout: { requested_mode: 'v2_write', capture_enabled: true, assignment_enabled: true, read_v2_enabled: true, write_v2_enabled: true },
  }
  listGaps.mockImplementation(async (_ws: string, filters: Record<string, string>) => ({
    data: filters.has_merge_suggestions ? { items: [], total: 0 } : { items: [gap], total: 1 },
    error: null,
  }))
})

afterEach(() => {
  act(() => root?.unmount())
  container?.remove()
  container = null
  root = null
  listGaps.mockReset()
  Object.values(toasts).forEach((mock) => mock.mockReset())
})

async function renderPage() {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  await act(async () => {
    root?.render(<TooltipProvider><SupportCoveragePage /></TooltipProvider>)
  })
  return container
}

describe('SupportCoveragePage', () => {
  it('shows one list of gaps without separate review queues', async () => {
    const page = await renderPage()

    expect(page.textContent).toContain('Reconnect Instagram after a token expires')
    expect(page.textContent).not.toContain('Needs review')
    expect(page.textContent).not.toContain('Customer topics')
    expect(page.textContent).not.toContain('Review earlier detections')
    expect(page.querySelector('[aria-label="Coverage views"]')).toBeNull()
  })

  it('states why analysis stopped where the user can see it', async () => {
    health = { ...health, healthy: false, reanalysis_status: 'failed', reanalysis_error: 'The analysis worker stopped responding.' }

    const page = await renderPage()

    expect(page.textContent).toContain('Needs attention')
    expect(page.textContent).toContain('Analysis stopped: The analysis worker stopped responding.')
  })

  it('raises an error notification when analysis stops', async () => {
    health = { ...health, healthy: false, reanalysis_status: 'failed', reanalysis_error: 'The analysis worker stopped responding.' }

    await renderPage()

    expect(toasts.error).toHaveBeenCalledTimes(1)
    expect(toasts.error).toHaveBeenCalledWith(
      'Analysis stopped: The analysis worker stopped responding.',
      expect.objectContaining({ classNames: { toast: expect.stringContaining('bg-red-50') } }),
    )
    expect(toasts.warning).not.toHaveBeenCalled()
  })

  it('raises a warning notification while a failed attempt is retried', async () => {
    health = { ...health, reanalysis_status: 'retrying', reanalysis_attempt: 2, reanalysis_error: 'A database rule rejected the run.' }

    await renderPage()

    expect(toasts.warning).toHaveBeenCalledTimes(1)
    expect(toasts.warning.mock.calls[0][1]).toMatchObject({ classNames: { toast: expect.stringContaining('bg-amber-50') } })
    expect(toasts.error).not.toHaveBeenCalled()
  })

  it('keeps routine status out of the page body', async () => {
    const page = await renderPage()

    expect(page.textContent).not.toContain('Analysis stopped')
    expect(page.querySelector('p[role="status"]')).toBeNull()
    expect(toasts.error).not.toHaveBeenCalled()
    expect(toasts.warning).not.toHaveBeenCalled()
  })
})
