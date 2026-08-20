import React from 'react'
import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  flattenConversationPages,
  flattenSupportMessagePages,
} from '../support-pages'
import { configureSupportApi } from '../support-service'
import {
  useConversationMessages,
  useInfiniteConversations,
} from '../use-support-core'
import type {
  ConversationListResponse,
  SupportConversation,
  SupportMessage,
} from '../support-types'

type FakeApi = {
  get: ReturnType<typeof vi.fn>
  post: ReturnType<typeof vi.fn>
  put: ReturnType<typeof vi.fn>
  del: ReturnType<typeof vi.fn>
}

function wrapperFor(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return React.createElement(QueryClientProvider, { client: queryClient }, children)
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
    per_page: 50,
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

describe('support infinite queries', () => {
  let api: FakeApi
  let queryClient: QueryClient

  beforeEach(() => {
    api = {
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn(),
      del: vi.fn(),
    }
    configureSupportApi(api)
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
  })

  it('requests numbered conversation pages and keeps prior results', async () => {
    api.get.mockImplementation(async (path: string) => {
      const data = path.includes('page=2')
        ? conversationPage(['conv-3'], 2, 2)
        : conversationPage(['conv-1', 'conv-2'], 1, 2)
      return { data, error: null }
    })

    const { result } = renderHook(
      () => useInfiniteConversations('ws-1', { search: 'Ada Lovelace' }),
      { wrapper: wrapperFor(queryClient) },
    )

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(flattenConversationPages(result.current.data).map((item) => item.id)).toEqual([
      'conv-1',
      'conv-2',
    ])

    await act(async () => {
      await result.current.fetchNextPage()
    })

    await waitFor(() => expect(result.current.data?.pages).toHaveLength(2))

    expect(flattenConversationPages(result.current.data).map((item) => item.id)).toEqual([
      'conv-1',
      'conv-2',
      'conv-3',
    ])
    expect(api.get.mock.calls.map(([path]) => path)).toEqual([
      '/support/inbox/conversations?workspace_id=ws-1&search=Ada%20Lovelace&page=1&per_page=50',
      '/support/inbox/conversations?workspace_id=ws-1&search=Ada%20Lovelace&page=2&per_page=50',
    ])
  })

  it('follows the message cursor and exposes the thread chronologically', async () => {
    api.get
      .mockResolvedValueOnce({
        data: {
          data: [message('msg-03'), message('msg-04')],
          has_more: true,
          next_cursor: 'older cursor',
        },
        error: null,
      })
      .mockResolvedValueOnce({
        data: {
          data: [message('msg-01'), message('msg-02')],
          has_more: false,
        },
        error: null,
      })

    const { result } = renderHook(
      () => useConversationMessages('ws-1', 'conv-1'),
      { wrapper: wrapperFor(queryClient) },
    )

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(flattenSupportMessagePages(result.current.data).map((item) => item.id)).toEqual([
      'msg-03',
      'msg-04',
    ])

    await act(async () => {
      await result.current.fetchNextPage()
    })

    await waitFor(() => expect(result.current.data?.pages).toHaveLength(2))

    expect(flattenSupportMessagePages(result.current.data).map((item) => item.id)).toEqual([
      'msg-01',
      'msg-02',
      'msg-03',
      'msg-04',
    ])
    expect(api.get.mock.calls.map(([path]) => path)).toEqual([
      '/support/inbox/conversations/conv-1/message-pages?workspace_id=ws-1&limit=20',
      '/support/inbox/conversations/conv-1/message-pages?workspace_id=ws-1&limit=20&cursor=older%20cursor',
    ])
    expect(result.current.hasNextPage).toBe(false)
  })
})
