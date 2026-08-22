import React from 'react'
import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flattenConversationSearchPages } from '../support-pages'
import { configureSupportApi } from '../support-service'
import { useInfiniteConversationSearch } from '../use-support-core'
import type {
  SupportConversation,
  SupportConversationSearchResponse,
} from '../support-types'

function conversation(id: string): SupportConversation {
  return {
    id,
    workspace_id: 'ws-1',
    display_id: Number(id.slice(-1)),
    subject: id,
    status: 'open',
    priority: 'medium',
    source: 'widget',
    created_at: '2026-08-20T10:00:00Z',
    updated_at: '2026-08-20T10:00:00Z',
  }
}

function searchPage(ids: string[], page: number, totalPages: number): SupportConversationSearchResponse {
  return {
    data: ids.map((id) => ({
      conversation: conversation(id),
      display_id: Number(id.slice(-1)),
      matched_fields: ['message'],
      snippet: `Matched ${id}`,
      highlights: [{ field: 'message', text: `Matched ${id}`, ranges: [{ start: 0, end: 7 }] }],
      score: 1,
    })),
    total: 3,
    page,
    per_page: 50,
    total_pages: totalPages,
    meta: { sort: 'relevance', query: 'refund', total_capped: false, total_cap: 10_000 },
  }
}

describe('support full-text search', () => {
  let api: { get: ReturnType<typeof vi.fn>; post: ReturnType<typeof vi.fn>; put: ReturnType<typeof vi.fn>; del: ReturnType<typeof vi.fn> }
  let queryClient: QueryClient

  beforeEach(() => {
    api = { get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() }
    configureSupportApi(api)
    queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  })

  it('uses the dedicated search endpoint, retains match metadata, and paginates', async () => {
    api.get.mockImplementation(async (path: string) => ({
      data: path.includes('page=2')
        ? searchPage(['conv-3'], 2, 2)
        : searchPage(['conv-1', 'conv-2'], 1, 2),
      error: null,
    }))

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )
    const { result } = renderHook(
      () => useInfiniteConversationSearch('ws-1', {
        q: 'refund',
        sort: 'relevance',
        statuses: 'open,resolved',
        tag_ids: 'tag-1',
      }),
      { wrapper },
    )

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api.get.mock.calls[0][0]).toContain('/support/inbox/search?')
    expect(api.get.mock.calls[0][0]).toContain('q=refund')
    expect(api.get.mock.calls[0][0]).toContain('statuses=open%2Cresolved')
    expect(api.get.mock.calls[0][0]).toContain('tag_ids=tag-1')
    expect(flattenConversationSearchPages(result.current.data)[0].matched_fields).toEqual(['message'])

    await act(async () => result.current.fetchNextPage())
    expect(api.get.mock.calls[1][0]).toContain('page=2')
    await waitFor(() => {
      expect(flattenConversationSearchPages(result.current.data).map((item) => item.conversation.id)).toEqual([
        'conv-1', 'conv-2', 'conv-3',
      ])
    })
  })
})
