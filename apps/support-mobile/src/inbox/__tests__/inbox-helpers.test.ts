import type { SupportConversation } from '@helpin-ai/support-core'
import { filtersForSegment, formatRelativeTime, isUnread, previewText } from '../inbox-helpers'

function conversation(overrides: Partial<SupportConversation> = {}): SupportConversation {
  return {
    id: 'c1',
    workspace_id: 'w1',
    display_id: 1,
    subject: 'Hello',
    status: 'open',
    priority: 'medium',
    source: 'email',
    created_at: '2026-07-01T00:00:00.000Z',
    updated_at: '2026-07-01T00:00:00.000Z',
    ...overrides,
  }
}

describe('formatRelativeTime', () => {
  const now = new Date('2026-07-08T12:00:00.000Z')

  test('under 60 seconds ago is "Now"', () => {
    expect(formatRelativeTime('2026-07-08T11:59:30.000Z', now)).toBe('Now')
  })

  test('minutes ago formats as "Nm"', () => {
    expect(formatRelativeTime('2026-07-08T11:48:00.000Z', now)).toBe('12m')
  })

  test('hours ago formats as "Nh"', () => {
    expect(formatRelativeTime('2026-07-08T09:00:00.000Z', now)).toBe('3h')
  })

  test('yesterday (previous calendar day) formats as "Yesterday"', () => {
    expect(formatRelativeTime('2026-07-07T08:00:00.000Z', now)).toBe('Yesterday')
  })

  test('more than 7 days ago formats as short month/day', () => {
    expect(formatRelativeTime('2026-07-02T00:00:00.000Z', now)).toBe('Jul 2')
  })

  test('a prior calendar year includes the year', () => {
    expect(formatRelativeTime('2025-12-25T00:00:00.000Z', now)).toBe('Dec 25, 2025')
  })
})

describe('previewText', () => {
  test('strips HTML tags', () => {
    expect(previewText(conversation({ last_message: '<p>Hello <b>world</b></p>' }))).toBe('Hello world')
  })

  test('collapses whitespace', () => {
    expect(previewText(conversation({ last_message: 'Hello \n\n   world  ' }))).toBe('Hello world')
  })

  test('falls back to "No messages yet" when there is no last message', () => {
    expect(previewText(conversation({ last_message: undefined }))).toBe('No messages yet')
  })

  test('falls back to "No messages yet" when the last message is only whitespace/tags', () => {
    expect(previewText(conversation({ last_message: '<p>   </p>' }))).toBe('No messages yet')
  })
})

describe('isUnread', () => {
  test('true when unread_count is greater than zero', () => {
    expect(isUnread(conversation({ unread_count: 3 }))).toBe(true)
  })

  test('false when unread_count is zero', () => {
    expect(isUnread(conversation({ unread_count: 0 }))).toBe(false)
  })

  test('false when unread_count is undefined', () => {
    expect(isUnread(conversation({ unread_count: undefined }))).toBe(false)
  })
})

describe('filtersForSegment', () => {
  test('mine maps to filter=mine', () => {
    expect(filtersForSegment('mine', null)).toEqual({ filter: 'mine' })
  })

  test('unassigned maps to assigned_to=unassigned (there is no filter=unassigned server-side)', () => {
    expect(filtersForSegment('unassigned', null)).toEqual({ assigned_to: 'unassigned' })
  })

  test('all sends no filter/assigned_to', () => {
    expect(filtersForSegment('all', null)).toEqual({})
  })

  test('a selected mailbox is merged into every segment', () => {
    expect(filtersForSegment('mine', 'mb-1')).toEqual({ filter: 'mine', mailbox_id: 'mb-1' })
    expect(filtersForSegment('all', 'mb-1')).toEqual({ mailbox_id: 'mb-1' })
  })

  test('a mailboxId of "all" is treated the same as null', () => {
    expect(filtersForSegment('all', 'all')).toEqual({})
  })
})
