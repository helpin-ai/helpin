import { describe, expect, it } from 'vitest'
import {
  EMPTY_CONVERSATION_SEARCH_FILTERS,
  activeConversationSearchFilterCount,
  buildConversationSearchParams,
} from '../search-filters'

describe('mobile conversation search filters', () => {
  it('maps every mobile filter to the dedicated web search contract', () => {
    const filters = {
      ...EMPTY_CONVERSATION_SEARCH_FILTERS,
      sort: 'oldest' as const,
      statuses: ['open', 'resolved'],
      priorities: ['urgent'],
      assignedTo: ['me', 'unassigned'],
      mailboxIds: ['shared', 'mailbox-1'],
      tagIds: ['tag-1'],
      aiStates: ['handoff'],
      customerEmail: ' ada@example.com ',
      title: ' Refund ',
      createdFrom: '2026-08-01',
      createdTo: '2026-08-20',
    }

    expect(buildConversationSearchParams(' payment failed ', filters)).toEqual({
      q: 'payment failed',
      sort: 'oldest',
      statuses: 'open,resolved',
      priorities: 'urgent',
      assigned_to: 'me,unassigned',
      mailbox_ids: 'shared,mailbox-1',
      tag_ids: 'tag-1',
      ai: 'handoff',
      customer_email: 'ada@example.com',
      title: 'Refund',
      created_from: '2026-08-01',
      created_to: '2026-08-20',
    })
    expect(activeConversationSearchFilterCount(filters)).toBe(14)
  })

  it('does not count relevance or emit empty values', () => {
    expect(buildConversationSearchParams('', EMPTY_CONVERSATION_SEARCH_FILTERS)).toEqual({
      q: undefined,
      sort: 'relevance',
      statuses: undefined,
      priorities: undefined,
      assigned_to: undefined,
      mailbox_ids: undefined,
      tag_ids: undefined,
      ai: undefined,
      customer_email: undefined,
      title: undefined,
      created_from: undefined,
      created_to: undefined,
    })
    expect(activeConversationSearchFilterCount(EMPTY_CONVERSATION_SEARCH_FILTERS)).toBe(0)
  })
})
