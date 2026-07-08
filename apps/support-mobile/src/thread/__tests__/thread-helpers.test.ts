import type { SupportMessage } from '@helpin-ai/support-core'
import { formatDayLabel, groupMessages, splitQuotedHtml } from '../thread-helpers'

function message(overrides: Partial<SupportMessage> = {}): SupportMessage {
  return {
    id: 'm1',
    workspace_id: 'w1',
    conversation_id: 'c1',
    sender_type: 'customer',
    content: 'hello',
    is_internal: false,
    created_at: '2026-07-06T10:00:00.000Z',
    updated_at: '2026-07-06T10:00:00.000Z',
    ...overrides,
  }
}

describe('formatDayLabel', () => {
  const now = new Date('2026-07-08T12:00:00.000Z')

  test('same calendar day is "Today"', () => {
    expect(formatDayLabel('2026-07-08T01:00:00.000Z', now)).toBe('Today')
  })

  test('previous calendar day is "Yesterday"', () => {
    expect(formatDayLabel('2026-07-07T23:00:00.000Z', now)).toBe('Yesterday')
  })

  test('older same-year date formats as "EEE, MMM d" with no year', () => {
    expect(formatDayLabel('2026-07-06T10:00:00.000Z', now)).toBe('Mon, Jul 6')
  })

  test('a prior calendar year includes the year', () => {
    expect(formatDayLabel('2025-12-25T10:00:00.000Z', now)).toBe('Thu, Dec 25, 2025')
  })
})

describe('groupMessages', () => {
  beforeEach(() => {
    // groupMessages calls formatDayLabel with its default `now = new Date()`
    // (its own signature takes only `messages`), so the 3-day fixture below
    // needs a fixed system clock to get deterministic "Today"/"Yesterday" vs.
    // short-date labels.
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-07-08T12:00:00.000Z'))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  test('empty input returns []', () => {
    expect(groupMessages([])).toEqual([])
  })

  test('inserts day separators across a 3-day fixture', () => {
    const messages = [
      message({ id: 'm1', created_at: '2026-07-05T10:00:00.000Z', updated_at: '2026-07-05T10:00:00.000Z' }),
      message({ id: 'm2', created_at: '2026-07-07T10:00:00.000Z', updated_at: '2026-07-07T10:00:00.000Z' }),
      message({ id: 'm3', created_at: '2026-07-08T10:00:00.000Z', updated_at: '2026-07-08T10:00:00.000Z' }),
    ]
    const items = groupMessages(messages)
    const dayItems = items.filter((item) => item.kind === 'day')
    expect(dayItems.map((item) => (item as { label: string }).label)).toEqual(['Sun, Jul 5', 'Yesterday', 'Today'])
    // one cluster per day, one message per cluster
    const clusterItems = items.filter((item) => item.kind === 'cluster')
    expect(clusterItems).toHaveLength(3)
  })

  test('consecutive same-sender messages within 3 minutes collapse into one cluster', () => {
    const messages = [
      message({ id: 'm1', sender_type: 'user', sender_user_id: 'u1', created_at: '2026-07-06T10:00:00.000Z' }),
      message({ id: 'm2', sender_type: 'user', sender_user_id: 'u1', created_at: '2026-07-06T10:02:00.000Z' }),
      message({ id: 'm3', sender_type: 'user', sender_user_id: 'u1', created_at: '2026-07-06T10:04:00.000Z' }),
    ]
    const items = groupMessages(messages)
    expect(items).toHaveLength(2) // 1 day separator + 1 cluster
    const cluster = items[1]
    if (cluster.kind !== 'cluster') throw new Error('expected cluster')
    expect(cluster.messages.map((m) => m.id)).toEqual(['m1', 'm2', 'm3'])
  })

  test('a gap of exactly 3 minutes stays in the same cluster (break boundary is strictly greater-than)', () => {
    const messages = [
      message({ id: 'm1', sender_type: 'user', sender_user_id: 'u1', created_at: '2026-07-06T10:00:00.000Z' }),
      message({ id: 'm2', sender_type: 'user', sender_user_id: 'u1', created_at: '2026-07-06T10:03:00.000Z' }),
    ]
    const items = groupMessages(messages)
    const clusters = items.filter((item) => item.kind === 'cluster')
    expect(clusters).toHaveLength(1)
  })

  test('a gap of more than 3 minutes breaks the cluster', () => {
    const messages = [
      message({ id: 'm1', sender_type: 'user', sender_user_id: 'u1', created_at: '2026-07-06T10:00:00.000Z' }),
      message({ id: 'm2', sender_type: 'user', sender_user_id: 'u1', created_at: '2026-07-06T10:03:01.000Z' }),
    ]
    const items = groupMessages(messages)
    const clusters = items.filter((item) => item.kind === 'cluster')
    expect(clusters).toHaveLength(2)
  })

  test('a sender change breaks the cluster even within 3 minutes', () => {
    const messages = [
      message({ id: 'm1', sender_type: 'customer', created_at: '2026-07-06T10:00:00.000Z' }),
      message({ id: 'm2', sender_type: 'user', sender_user_id: 'u1', created_at: '2026-07-06T10:01:00.000Z' }),
    ]
    const items = groupMessages(messages)
    const clusters = items.filter((item) => item.kind === 'cluster')
    expect(clusters).toHaveLength(2)
  })

  test('an internal note breaks the cluster from a public reply by the same sender', () => {
    const messages = [
      message({ id: 'm1', sender_type: 'user', sender_user_id: 'u1', is_internal: false, created_at: '2026-07-06T10:00:00.000Z' }),
      message({ id: 'm2', sender_type: 'user', sender_user_id: 'u1', is_internal: true, created_at: '2026-07-06T10:01:00.000Z' }),
    ]
    const items = groupMessages(messages)
    const clusters = items.filter((item) => item.kind === 'cluster')
    expect(clusters).toHaveLength(2)
  })
})

describe('splitQuotedHtml', () => {
  test('no quoted history returns the original html with quoted: null', () => {
    const html = '<p>Hello there</p><p>Thanks!</p>'
    expect(splitQuotedHtml(html)).toEqual({ visible: html, quoted: null })
  })

  test('empty input returns quoted: null', () => {
    expect(splitQuotedHtml('')).toEqual({ visible: '', quoted: null })
  })

  test('collapses a gmail_quote div behind the split', () => {
    const html =
      '<div>Sure, sounds good.</div>' +
      '<div class="gmail_quote">On Mon, Jul 6, 2026, Jane wrote:<blockquote>Original message</blockquote></div>'
    const result = splitQuotedHtml(html)
    expect(result.visible).toBe('<div>Sure, sounds good.</div>')
    expect(result.quoted).toContain('gmail_quote')
    expect(result.quoted).toContain('Original message')
  })

  test('collapses a backend-marked data-helpin-quote element behind the split', () => {
    const html = '<p>My reply</p><div data-helpin-quote="true"><p>Quoted history</p></div>'
    const result = splitQuotedHtml(html)
    expect(result.visible).toBe('<p>My reply</p>')
    expect(result.quoted).toBe('<div data-helpin-quote="true"><p>Quoted history</p></div>')
  })

  test('collapses a trailing blockquote even without a type="cite" attribute', () => {
    const html = '<p>Reply text</p><blockquote>Old thread content</blockquote>'
    const result = splitQuotedHtml(html)
    expect(result.visible).toBe('<p>Reply text</p>')
    expect(result.quoted).toBe('<blockquote>Old thread content</blockquote>')
  })

  test('does not collapse a non-trailing blockquote (real content follows it)', () => {
    const html = '<blockquote>An inline citation</blockquote><p>My actual reply</p>'
    const result = splitQuotedHtml(html)
    expect(result.quoted).toBeNull()
    expect(result.visible).toBe(html)
  })

  test('a quote marker as the very first node yields an empty visible section and preserves the quoted content', () => {
    const html = '<div class="gmail_quote">On Mon, Jane wrote:<blockquote>Forwarded body</blockquote></div>'
    const result = splitQuotedHtml(html)
    expect(result.visible).toBe('')
    expect(result.quoted).toContain('Forwarded body')
  })

  test('collapses a gmail_signature wrapper (mirrors desktop COLLAPSIBLE_SELECTOR)', () => {
    const html = '<p>Reply text</p><div class="gmail_signature">-- <br>Jane Doe</div>'
    const result = splitQuotedHtml(html)
    expect(result.visible).toBe('<p>Reply text</p>')
    expect(result.quoted).toContain('gmail_signature')
  })
})
