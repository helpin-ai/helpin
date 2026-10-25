// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { SupportReportOverview, SupportReportSummary } from '@/lib/supportReportTypes'

const mockOverview = vi.fn()
const mockExport = vi.fn()

vi.mock('@/hooks/queries/useSupport', () => ({
  useInboxScopes: () => ({
    data: { shared_inbox: { id: 'shared', name: 'Shared inbox' }, mailboxes: [{ id: 'mailbox-1', name: 'Billing' }] },
  }),
}))
vi.mock('@/hooks/queries/useSupportReports', () => ({
  useSupportReportOverview: (...args: unknown[]) => mockOverview(...args),
}))
vi.mock('@/lib/services/supportReportsService', () => ({
  supportReportsService: { exportConversations: (...args: unknown[]) => mockExport(...args) },
}))
vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: (selector: (state: unknown) => unknown) => selector({ currentWorkspace: { id: 'ws-1', slug: 'acme' } }),
}))

import { SupportReportsPage } from '../SupportReportsPage'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function summary(overrides: Partial<SupportReportSummary> = {}): SupportReportSummary {
  const duration = { count: 0, median_seconds: null, p90_seconds: null }
  return {
    conversations_created: 0, conversations_resolved: 0, resolved_by_ai: 0, resolved_by_human: 0,
    first_response: duration, first_human_response: duration, resolution: duration,
    ai_involved: 0, ai_resolved: 0, ai_resolved_confirmed: 0, ai_resolved_assumed: 0, ai_handoffs: 0,
    ai_involvement_rate: null, ai_resolution_rate: null, ai_handoff_rate: null,
    csat_responses: 0, csat_average: null, csat_positive_rate: null,
    ...overrides,
  }
}

const report: SupportReportOverview = {
  range: { from: '2026-10-01T00:00:00Z', to: '2026-10-08T00:00:00Z', previous_from: '2026-09-24T00:00:00Z', previous_to: '2026-10-01T00:00:00Z', timezone: 'UTC' },
  summary: summary({
    conversations_created: 12, conversations_resolved: 9,
    first_human_response: { count: 6, median_seconds: 600, p90_seconds: 7200 },
    resolution: { count: 9, median_seconds: 5400, p90_seconds: 86400 },
    ai_involved: 8, ai_resolved: 4, ai_resolved_confirmed: 3, ai_resolved_assumed: 1, ai_handoffs: 2,
    ai_resolution_rate: 0.5, ai_involvement_rate: 0.6667, ai_handoff_rate: 0.25,
    csat_responses: 3, csat_average: 4.3, csat_positive_rate: 0.6667,
  }),
  previous: summary({ conversations_created: 10, conversations_resolved: 9, first_human_response: { count: 4, median_seconds: 900, p90_seconds: 900 } }),
  backlog: { open: 5, waiting_on_customer: 2, unassigned: 1 },
  daily: [
    { date: '2026-10-01', conversations_created: 3, conversations_resolved: 1, resolved_by_ai: 1, median_first_human_response_seconds: 300 },
    { date: '2026-10-02', conversations_created: 1, conversations_resolved: 2, resolved_by_ai: 0, median_first_human_response_seconds: null },
  ],
  channels: [{ channel: 'widget', conversations_created: 10, conversations_resolved: 7, first_human_response: { count: 5, median_seconds: 480, p90_seconds: 900 } }],
  teammates: [{ user_id: 'u1', name: 'Uma One', resolved: 4, replies_sent: 11, first_human_response: { count: 3, median_seconds: 420, p90_seconds: 600 }, csat_responses: 2, csat_average: 4.5 }],
  csat: {
    distribution: [0, 0, 1, 0, 2], ai_average: 5, ai_responses: 1, human_average: 3.5, human_responses: 2,
    recent_comments: [{ conversation_id: 'c1', display_id: 7, subject: 'Refund', rating: 5, comment: 'Great help', channel: 'widget', resolved_by: 'human', submitted_at: '2026-10-02T10:00:00Z' }],
  },
}

describe('SupportReportsPage', () => {
  let container: HTMLDivElement
  let root: Root

  beforeEach(() => {
    vi.clearAllMocks()
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(() => {
    act(() => root.unmount())
    container.remove()
  })

  it('shows headline metrics, trends, CSAT and teammates', () => {
    mockOverview.mockReturnValue({ data: report, isLoading: false, isFetching: false, error: null, refetch: vi.fn() })
    act(() => root.render(<SupportReportsPage />))

    const text = container.textContent ?? ''
    expect(text).toContain('New conversations12')
    expect(text).toContain('+20% vs previous period')
    expect(text).toContain('First response (team)10m')
    expect(text).toContain('−5m vs previous period')
    expect(text).toContain('AI resolution rate50%')
    expect(text).toContain('3 confirmed, 1 assumed')
    expect(text).toContain('4.3 / 5')
    expect(text).toContain('Great help')
    expect(text).toContain('Uma One')
    expect(text).toContain('Open now 5')
    expect(container.querySelectorAll('[data-testid="report-day"]')).toHaveLength(2)
    expect(container.querySelector('a[href="/w/acme/support/c1"]')).not.toBeNull()

    const [, params] = mockOverview.mock.calls.at(-1) as [string, { from: string; to: string; mailboxIds: string[] }]
    expect(params.mailboxIds).toEqual([])
    expect(new Date(params.to).getTime() - new Date(params.from).getTime()).toBeGreaterThanOrEqual(29 * 86_400_000)
  })

  it('explains an empty CSAT period', () => {
    mockOverview.mockReturnValue({ data: { ...report, summary: summary(), teammates: [], channels: [] }, isLoading: false, isFetching: false, error: null, refetch: vi.fn() })
    act(() => root.render(<SupportReportsPage />))
    expect(container.textContent).toContain('No ratings in this period')
    expect(container.textContent).toContain('No teammate replies or resolutions in this period.')
  })

  it('offers a retry when the report fails', () => {
    const refetch = vi.fn()
    mockOverview.mockReturnValue({ data: undefined, isLoading: false, isFetching: false, error: new Error('choose a report period of up to one year'), refetch })
    act(() => root.render(<SupportReportsPage />))
    expect(container.textContent).toContain('The report could not load')
    const retry = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Try again')
    act(() => retry?.click())
    expect(refetch).toHaveBeenCalled()
  })

  it('exports the current period as CSV', async () => {
    mockOverview.mockReturnValue({ data: report, isLoading: false, isFetching: false, error: null, refetch: vi.fn() })
    mockExport.mockResolvedValue({ blob: new Blob(['a,b']), filename: 'support.csv' })
    const createObjectURL = vi.fn(() => 'blob:report')
    const revokeObjectURL = vi.fn()
    Object.assign(URL, { createObjectURL, revokeObjectURL })
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    act(() => root.render(<SupportReportsPage />))
    const exportButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Export CSV'))
    await act(async () => { exportButton?.click() })
    expect(mockExport).toHaveBeenCalledWith('ws-1', expect.objectContaining({ mailboxIds: [] }))
    expect(click).toHaveBeenCalled()
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:report')
    click.mockRestore()
  })
})
