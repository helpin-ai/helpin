import { describe, expect, it } from 'vitest'

import type { SupportMessage, SupportMessagePage } from '@/lib/pmTypes'
import {
  appendMessageToNewestPage,
  flattenSupportMessagePages,
  removeMessageFromPages,
  replaceMessageInPages,
  seedSupportMessagePages,
} from '@/lib/supportMessagePages'

function message(id: string): SupportMessage {
  return {
    id,
    workspace_id: 'ws-1',
    conversation_id: 'conv-1',
    sender_type: 'customer',
    content: id,
    is_internal: false,
    created_at: `2026-08-18T10:${id.slice(-2)}:00Z`,
    updated_at: `2026-08-18T10:${id.slice(-2)}:00Z`,
  }
}

function page(ids: string[], nextCursor?: string): SupportMessagePage {
  return {
    data: ids.map(message),
    has_more: Boolean(nextCursor),
    next_cursor: nextCursor,
  }
}

describe('support message page cache', () => {
  it('flattens fetched pages into chronological order', () => {
    const data = {
      pages: [page(['msg-03', 'msg-04'], 'older'), page(['msg-01', 'msg-02'])],
      pageParams: [undefined, 'older'],
    }

    expect(flattenSupportMessagePages(data).map((item) => item.id)).toEqual([
      'msg-01',
      'msg-02',
      'msg-03',
      'msg-04',
    ])
  })

  it('deduplicates realtime messages while appending to the newest page', () => {
    const initial = seedSupportMessagePages([message('msg-01')])
    const appended = appendMessageToNewestPage(initial, message('msg-02'))
    const duplicated = appendMessageToNewestPage(appended, message('msg-02'))

    expect(flattenSupportMessagePages(duplicated).map((item) => item.id)).toEqual(['msg-01', 'msg-02'])
  })

  it('replaces the correlated optimistic message when realtime arrives first', () => {
    const optimistic = {
      ...message('client-msg-01'),
      client_message_id: 'client-msg-01',
    }
    const realtime = {
      ...message('msg-02'),
      client_message_id: 'client-msg-01',
    }

    const initial = seedSupportMessagePages([message('msg-01'), optimistic])
    const reconciled = appendMessageToNewestPage(initial, realtime)

    expect(flattenSupportMessagePages(reconciled).map((item) => item.id)).toEqual(['msg-01', 'msg-02'])
  })

  it('does not replace an unrelated optimistic message', () => {
    const optimistic = {
      ...message('client-msg-01'),
      client_message_id: 'client-msg-01',
    }
    const realtime = {
      ...message('msg-02'),
      client_message_id: 'different-client-message',
    }

    const initial = seedSupportMessagePages([optimistic])
    const appended = appendMessageToNewestPage(initial, realtime)

    expect(flattenSupportMessagePages(appended).map((item) => item.id)).toEqual(['client-msg-01', 'msg-02'])
  })

  it('replaces and removes messages without discarding loaded older pages', () => {
    const initial = {
      pages: [page(['optimistic-02'], 'older'), page(['msg-01'])],
      pageParams: [undefined, 'older'],
    }
    const replaced = replaceMessageInPages(initial, 'optimistic-02', message('msg-02'))
    const removed = removeMessageFromPages(replaced, 'msg-01')

    expect(flattenSupportMessagePages(removed).map((item) => item.id)).toEqual(['msg-02'])
    expect(removed?.pages).toHaveLength(2)
  })
})


describe('joined status delivery', () => {
  it('keeps the joined event before its exact reply before and after confirmation', () => {
    const pending = { ...message('client-01'), sender_type: 'user' as const, client_message_id: 'client-01' }
    const other = { ...message('other-02'), sender_type: 'user' as const, content: pending.content, client_message_id: 'client-other' }
    const joined = { ...message('join-03'), message_type: 'system', system_event_type: 'teammate_joined',
      metadata: JSON.stringify({ reply_client_message_id: pending.client_message_id }) }
    const confirmed = { ...pending, id: 'saved-04', client_message_id: undefined, metadata: JSON.stringify({ client_message_id: pending.client_message_id }) }
    const current = appendMessageToNewestPage(seedSupportMessagePages([other, pending]), joined)
    expect(flattenSupportMessagePages(current).map((m) => m.id)).toEqual([other.id, joined.id, pending.id])
    const accepted = appendMessageToNewestPage(current, confirmed)
    expect(flattenSupportMessagePages(accepted).map((m) => m.id)).toEqual([other.id, joined.id, confirmed.id])
  })

  it('leaves unrelated, legacy and unloaded-reply system events in server order', () => {
    const rows = [message('msg-01'),
      { ...message('join-02'), message_type: 'system', system_event_type: 'teammate_joined', metadata: '{invalid' },
      { ...message('join-03'), message_type: 'system', system_event_type: 'teammate_joined', metadata: JSON.stringify({ reply_client_message_id: 'unloaded' }) },
    ]
    expect(flattenSupportMessagePages(seedSupportMessagePages(rows))).toEqual(rows)
  })
})


it('retains optimistic author details when the widget-shaped realtime acknowledgement omits them', () => {
  const pending = { ...message('client-01'), client_message_id: 'client-01', sender_type: 'user' as const,
    sender_user_id: 'user-1', sender_display_name: 'Waqar' }
  const realtime = { ...message('saved-02'), client_message_id: 'client-01', sender_type: 'user' as const }
  const rows = flattenSupportMessagePages(appendMessageToNewestPage(seedSupportMessagePages([pending]), realtime))
  expect(rows).toHaveLength(1)
  expect(rows[0]).toMatchObject({ id: 'saved-02', sender_user_id: 'user-1', sender_display_name: 'Waqar' })
})
