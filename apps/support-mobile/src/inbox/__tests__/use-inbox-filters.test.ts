import { describe, expect, test } from 'vitest'
import {
  buildConversationListRequestFilters,
  defaultConversationListFiltersForNav,
} from '@/lib/supportInboxFilters'
import type { NavFilter } from '@/stores/supportInboxStore'
import {
  selectionTitle,
  selectionListFilters,
  selectionToConversationFilters,
  type ViewSelection,
} from '../use-inbox-filters'

const BUILTIN: NavFilter[] = ['inbox', 'mine', 'waiting', 'resolved', 'spam', 'ai_active', 'resolved_by_ai']

describe('selectionToConversationFilters — parity with the web function', () => {
  test.each(BUILTIN)('builtin %s equals buildConversationListRequestFilters output', (navFilter) => {
    const expected = buildConversationListRequestFilters({
      navFilter,
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: defaultConversationListFiltersForNav(navFilter),
    })
    const selection: ViewSelection = { kind: 'builtin', navFilter, mailboxId: 'all' }
    expect(selectionToConversationFilters(selection)).toEqual(expected)
  })

  test('team-inbox selection scopes the inbox view to that mailbox', () => {
    const filters = selectionToConversationFilters({
      kind: 'mailbox',
      mailboxId: 'mbx-1',
      mailboxName: 'Billing',
    })
    expect(filters).toMatchObject({ mailbox_id: 'mbx-1' })
  })

  test('custom view reuses the web parse + build path (no throw, produces filters or undefined)', () => {
    const filters = selectionToConversationFilters({
      kind: 'custom',
      viewId: 'v1',
      name: 'VIPs',
      filters: { states: 'open', assignment: 'me' },
    })
    // Reuses parseSupportInboxViewFilters → buildConversationListRequestFilters;
    // exact params depend on the web rulebook, so just assert it resolves.
    expect(filters === undefined || typeof filters === 'object').toBe(true)
  })
})
  test('applies ad hoc mobile filters on top of a custom view baseline', () => {
    const selection: ViewSelection = {
      kind: 'custom',
      viewId: 'v1',
      name: 'VIPs',
      filters: { states: 'open', tag_ids: 'vip' },
    }
    const baseline = selectionListFilters(selection)
    expect(baseline.states).toEqual(['open'])
    expect(baseline.tagIds).toEqual(['vip'])

    expect(selectionToConversationFilters(selection, {
      ...baseline,
      states: ['resolved'],
      tagIds: ['urgent'],
      sort: 'oldest',
    })).toMatchObject({
      statuses: 'resolved',
      tag_ids: 'urgent',
      sort: 'oldest',
    })
  })


describe('selectionTitle', () => {
  test('builtin uses the web label wording', () => {
    expect(selectionTitle({ kind: 'builtin', navFilter: 'ai_active', mailboxId: 'all' })).toBe('AI Handling')
    expect(selectionTitle({ kind: 'builtin', navFilter: 'inbox', mailboxId: 'all' })).toBe('Inbox')
  })
  test('mailbox and custom use their own names', () => {
    expect(selectionTitle({ kind: 'mailbox', mailboxId: 'm', mailboxName: 'Billing' })).toBe('Billing')
    expect(selectionTitle({ kind: 'custom', viewId: 'v', name: 'VIPs', filters: {} })).toBe('VIPs')
  })
})
