import { describe, expect, it } from 'vitest'

import {
  appendMessageToNewestPage,
  flattenConversationPages,
  flattenSupportMessagePages,
  replaceMessageInPages,
  seedSupportMessagePages,
  type ConversationListPages,
} from '../support-pages'
import {
  findConversationInListCache,
  updateConversationListUnreadCount,
} from '../support-query-cache'
import type {
  ConversationListResponse,
  SupportConversation,
  SupportMessage,
  SupportMessagePage,
} from '../support-types'

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

function messagePage(ids: string[], nextCursor?: string): SupportMessagePage {
  return {
    data: ids.map(message),
    has_more: Boolean(nextCursor),
    next_cursor: nextCursor,
  }
}

function conversation(id: string): SupportConversation {
  return {
    id,
    workspace_id: 'ws-1',
    display_id: Number(id.slice(-1)),
    subject: id,
    status: 'open',
    priority: 'medium',
    source: 'widget',
    created_at: '2026-08-18T10:00:00Z',
    updated_at: '2026-08-18T10:00:00Z',
  }
}

function conversationPage(ids: string[], page: number, totalPages: number): ConversationListResponse {
  return {
    data: ids.map(conversation),
    total: 3,
    page,
    per_page: 2,
    total_pages: totalPages,
    meta: {
      unread: {
        total: 0,
        my_inbox: 0,
        unassigned: 0,
        ai_active: 0,
        inbox: 0,
        mine: 0,
        waiting: 0,
        inbox_total: 0,
        mine_total: 0,
        waiting_total: 0,
        ai_active_total: 0,
      },
    },
  }
}

describe('paginated support caches', () => {
  it('flattens message pages into chronological order and deduplicates cursor overlap', () => {
    const data = {
      pages: [
        messagePage(['msg-03', 'msg-04'], 'older'),
        messagePage(['msg-01', 'msg-02', 'msg-03']),
      ],
      pageParams: [undefined, 'older'],
    }

    expect(flattenSupportMessagePages(data).map((item) => item.id)).toEqual([
      'msg-01',
      'msg-02',
      'msg-03',
      'msg-04',
    ])
  })

  it('appends and replaces optimistic messages without discarding loaded history', () => {
    const initial = {
      pages: [messagePage(['optimistic-03'], 'older'), messagePage(['msg-01', 'msg-02'])],
      pageParams: [undefined, 'older'],
    }
    const appended = appendMessageToNewestPage(initial, message('msg-04'))
    const duplicated = appendMessageToNewestPage(appended, message('msg-04'))
    const replaced = replaceMessageInPages(duplicated, 'optimistic-03', message('msg-03'))

    expect(flattenSupportMessagePages(replaced).map((item) => item.id)).toEqual([
      'msg-01',
      'msg-02',
      'msg-03',
      'msg-04',
    ])
    expect(replaced?.pages).toHaveLength(2)
    expect(seedSupportMessagePages([]).pages).toHaveLength(1)
  })

  it('flattens conversation pages and updates unread state beyond the first page', () => {
    const pages: ConversationListPages = {
      pages: [
        conversationPage(['conv-1', 'conv-2'], 1, 2),
        conversationPage(['conv-2', 'conv-3'], 2, 2),
      ],
      pageParams: [1, 2],
    }

    expect(flattenConversationPages(pages).map((item) => item.id)).toEqual([
      'conv-1',
      'conv-2',
      'conv-3',
    ])

    const updated = updateConversationListUnreadCount(pages, 'conv-3', 4)
    expect(findConversationInListCache(updated, 'conv-3')?.unread_count).toBe(4)
    expect(findConversationInListCache(updated, 'conv-1')?.unread_count).toBeUndefined()
  })
})
