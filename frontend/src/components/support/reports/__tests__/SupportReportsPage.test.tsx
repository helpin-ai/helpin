// @vitest-environment jsdom
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { SupportReportOverview, SupportReportSummary, SupportReportTopic } from '@/lib/supportReportTypes'

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
vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, params, search, className, children }: { to: string; params: { slug: string }; search: Record<string, string | undefined>; className?: string; children: ReactNode }) => {
    const query = new URLSearchParams(Object.entries(search).filter((entry): entry is [string, string] => entry[1] !== undefined))
    return <a className={className} href={`${to.replace('$slug', params.slug)}?${query}`}>{children}</a>
  },
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

function topic(overrides: Partial<SupportReportTopic> = {}): SupportReportTopic {
  const duration = { count: 0, median_seconds: null, p90_seconds: null }
  return {
    tag_id: '', name: '', color: null, conversations_created: 0, previous_conversations_created: 0, ai_involved: 0,
    ai_resolution_rate: null, ai_handoff_rate: null, first_human_response: duration, resolution: duration,
    csat_responses: 0, csat_average: null,
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
  daily: [
    { date: '2026-10-01', conversations_created: 3, conversations_resolved: 1, resolved_by_ai: 1, median_first_human_response_seconds: 300 },
    { date: '2026-10-02', conversations_created: 1, conversations_resolved: 2, resolved_by_ai: 0, median_first_human_response_seconds: null },
  ],
  // Billing and Login overlap: their rows add up to more than 12.
  topics: [
    topic({
      tag_id: 'tag-billing', name: 'Billing', color: '#0f766e', conversations_created: 6, previous_conversations_created: 4,
      ai_involved: 5, ai_resolution_rate: 0.4, ai_handoff_rate: 0.6,
      first_human_response: { count: 3, median_seconds: 900, p90_seconds: 1800 },
      resolution: { count: 4, median_seconds: 7200, p90_seconds: 9000 }, csat_responses: 2, csat_average: 4.5,
    }),
    topic({ tag_id: 'tag-login', name: 'Login', conversations_created: 5, previous_conversations_created: 5 }),
    topic({ conversations_created: 3, previous_conversations_created: 1 }),
  ],
  channels: [{ channel: 'widget', conversations_created: 10, conversations_resolved: 7, first_human_response: { count: 5, median_seconds: 480, p90_seconds: 900 } }],
  teammates: [{ user_id: 'u1', name: 'Uma One', resolved: 4, replies_sent: 11, first_human_response: { count: 3, median_seconds: 420, p90_seconds: 600 }, csat_responses: 2, csat_average: 4.5 }],
  csat: {
    distribution: [0, 0, 1, 0, 2], ai_average: 5, ai_responses: 1, human_average: 3.5, human_responses: 2,
    recent_comments: [{ conversation_id: 'c1', display_id: 7, subject: 'Refund', rating: 5, comment: 'Great help', channel: 'widget', resolved_by: 'human', submitted_at: '2026-10-02T10:00:00Z' }],
  },
  sla: {
    conversations: 6,
    first_response: { met: 4, missed: 1, hit_rate: 0.8 },
    next_response: { met: 2, missed: 0, hit_rate: 1 },
    resolution: { met: 1, missed: 3, hit_rate: 0.25 },
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

  it('moves the period end forward when the tab regains focus', () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    try {
      vi.setSystemTime(new Date(2026, 9, 26, 7, 10, 30))
      mockOverview.mockReturnValue({ data: report, isLoading: false, isFetching: false, error: null, refetch: vi.fn() })
      act(() => root.render(<SupportReportsPage />))
      const endOf = () => new Date((mockOverview.mock.calls.at(-1) as [string, { to: string }])[1].to)
      expect(endOf()).toEqual(new Date(2026, 9, 26, 7, 11))

      // A rating given in another tab a few minutes later.
      vi.setSystemTime(new Date(2026, 9, 26, 7, 13, 39))
      act(() => window.dispatchEvent(new Event('focus')))
      expect(endOf()).toEqual(new Date(2026, 9, 26, 7, 14))
    } finally {
      vi.useRealTimers()
    }
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
    expect(text).not.toContain('Open now')
    expect(text).toContain('First response80%4 met · 1 missed')
    expect(text).toContain('Resolution25%1 met · 3 missed')
    expect(container.querySelectorAll('[data-testid="report-day"]')).toHaveLength(2)
    expect(container.querySelector('a[href="/w/acme/support/c1"]')).not.toBeNull()

    const [, params] = mockOverview.mock.calls.at(-1) as [string, { from: string; to: string; mailboxIds: string[] }]
    expect(params.mailboxIds).toEqual([])
    expect(new Date(params.to).getTime() - new Date(params.from).getTime()).toBeGreaterThanOrEqual(29 * 86_400_000)
    expect(new Date(params.to).getTime()).toBeGreaterThanOrEqual(Date.now() - 1000)
    expect(new Date(params.to).getTime()).toBeLessThanOrEqual(Date.now() + 60_000)
  })

  it('shows portal self-service only when customers wrote portal requests', () => {
    mockOverview.mockReturnValue({ data: report, isLoading: false, isFetching: false, error: null, refetch: vi.fn() })
    act(() => root.render(<SupportReportsPage />))
    expect(container.textContent).not.toContain('Portal self-service')

    const portal_intake = { with_suggestions: 9, opened: 5, submitted_after_opening: 2, likely_deflected: 3, submitted_without_suggestions: 4 }
    mockOverview.mockReturnValue({ data: { ...report, portal_intake }, isLoading: false, isFetching: false, error: null, refetch: vi.fn() })
    act(() => root.render(<SupportReportsPage />))
    const text = container.textContent ?? ''
    expect(text).toContain('Portal self-service')
    expect(text).toContain('Likely answered by an article3')
    expect(text).toContain('Opened an article5')
    expect(text).toContain('Of 9 forms that showed suggestions')
    expect(text).toContain('Asked after reading2')
    expect(text).toContain('Nothing to suggest4')
    expect(container.querySelector('a[href="/w/acme/support/coverage"]')).not.toBeNull()
  })

  it('puts the AI agent above the daily chart, followed by topics', () => {
    mockOverview.mockReturnValue({ data: report, isLoading: false, isFetching: false, error: null, refetch: vi.fn() })
    act(() => root.render(<SupportReportsPage />))
    const titles = Array.from(container.querySelectorAll('section')).map((section) => section.firstElementChild?.textContent ?? '')
    expect(titles.slice(0, 4)).toEqual(['Volume and speed', 'AI agent', 'Conversations per day', 'Topicsby conversation tag'])
  })

  it('shows topics by tag with links to their conversations', () => {
    mockOverview.mockReturnValue({ data: report, isLoading: false, isFetching: false, error: null, refetch: vi.fn() })
    act(() => root.render(<SupportReportsPage />))

    const section = Array.from(container.querySelectorAll('section')).find((candidate) => candidate.firstElementChild?.textContent?.startsWith('Topics'))
    const rows = Array.from(section?.querySelectorAll('tbody tr') ?? []).map((row) => Array.from(row.querySelectorAll('td')).map((cell) => cell.textContent))
    expect(rows).toEqual([
      ['Billing', '6+50% vs previous period', '50%', '40%', '60%', '15m', '2h', '4.5 (2)'],
      // Unchanged volume shows no trend; topics without AI or ratings show dashes.
      ['Login', '5', '42%', '—', '—', '—', '—', '—'],
      ['No tag', '3+200% vs previous period', '25%', '—', '—', '—', '—', '—'],
    ])
    expect(container.textContent).toContain('Conversations with several tags count under each tag.')

    const [, params] = mockOverview.mock.calls.at(-1) as [string, { from: string }]
    const from = new Date(params.from)
    const createdFrom = `${from.getFullYear()}-${String(from.getMonth() + 1).padStart(2, '0')}-${String(from.getDate()).padStart(2, '0')}`
    const billing = Array.from(container.querySelectorAll('a')).find((link) => link.textContent === 'Billing')
    expect(billing?.getAttribute('href')).toBe(`/w/acme/support/search?tag_ids=tag-billing&created_from=${createdFrom}`)
  })

  it('collapses long topic lists', () => {
    const many = Array.from({ length: 10 }, (_, index) => topic({ tag_id: `tag-${index}`, name: `Topic ${index}`, conversations_created: 1 }))
    mockOverview.mockReturnValue({ data: { ...report, topics: many }, isLoading: false, isFetching: false, error: null, refetch: vi.fn() })
    act(() => root.render(<SupportReportsPage />))
    const topicLinks = () => Array.from(container.querySelectorAll('a')).filter((link) => link.textContent?.startsWith('Topic '))
    expect(topicLinks()).toHaveLength(8)
    const showAll = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Show all 10 topics')
    act(() => showAll?.click())
    expect(topicLinks()).toHaveLength(10)
  })

  it('shows a day’s numbers in a tooltip on hover', () => {
    Object.assign(globalThis, { ResizeObserver: class { observe() {} unobserve() {} disconnect() {} } })
    mockOverview.mockReturnValue({ data: report, isLoading: false, isFetching: false, error: null, refetch: vi.fn() })
    act(() => root.render(<SupportReportsPage />))
    const [firstDay] = container.querySelectorAll('[data-testid="report-day"]')
    act(() => { firstDay.dispatchEvent(new MouseEvent('mouseover', { bubbles: true })) })
    const stats = document.body.querySelector('[data-testid="report-day-stats"]')
    expect(stats?.textContent).toBe('Created3Resolved1Resolved by AI1First response (team)5m')
    act(() => { firstDay.parentElement?.dispatchEvent(new MouseEvent('mouseout', { bubbles: true, relatedTarget: document.body })) })
    expect(document.body.querySelector('[data-testid="report-day-stats"]')).toBeNull()
  })

  it('shows the help assistant only when visitors asked it questions', () => {
    mockOverview.mockReturnValue({ data: report, isLoading: false, isFetching: false, error: null, refetch: vi.fn() })
    act(() => root.render(<SupportReportsPage />))
    expect(container.textContent).not.toContain('Help assistant')

    const help_assistant = { conversations: 3, questions: 4, answered: 3, handed_off: 1, failed: 0, helpful: 2, not_helpful: 1, unanswered: [{ question: 'Can I get a refund?', asked_at: new Date().toISOString() }] }
    mockOverview.mockReturnValue({ data: { ...report, help_assistant }, isLoading: false, isFetching: false, error: null, refetch: vi.fn() })
    act(() => root.render(<SupportReportsPage />))
    const text = container.textContent ?? ''
    expect(text).toContain('Help assistant3 conversations')
    expect(text).toContain('Answered from the articles75%')
    expect(text).toContain('3 of 4 questions')
    expect(text).toContain('Rated helpful67%')
    expect(text).toContain('Can I get a refund?')
  })

  it('explains an empty CSAT period', () => {
    const emptySLA = { met: 0, missed: 0, hit_rate: null }
    mockOverview.mockReturnValue({ data: { ...report, summary: summary(), teammates: [], channels: [], topics: [topic({ conversations_created: 2 })], sla: { conversations: 0, first_response: emptySLA, next_response: emptySLA, resolution: emptySLA } }, isLoading: false, isFetching: false, error: null, refetch: vi.fn() })
    act(() => root.render(<SupportReportsPage />))
    expect(container.textContent).toContain('None of the conversations in this period have tags.')
    expect(container.textContent).toContain('No ratings in this period')
    expect(container.querySelector('a[href="/w/acme/settings/support-sla"]')).not.toBeNull()
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
