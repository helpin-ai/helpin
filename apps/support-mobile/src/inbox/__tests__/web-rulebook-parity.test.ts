import { describe, expect, test } from 'vitest'
import {
  buildConversationListRequestFilters,
  defaultConversationListFiltersForNav,
} from '@/lib/supportInboxFilters'
import type { NavFilter } from '@/stores/supportInboxStore'

// These aliases resolve to the web app's real filter modules
// (frontend/src/lib/*). The value of this test is twofold: it fails to even
// import if the read-in-place alias breaks, and it pins the exact per-view
// server query so a future web change surfaces here as a diff.
const BUILTIN: NavFilter[] = [
  'inbox',
  'mine',
  'waiting',
  'resolved',
  'spam',
  'ai_active',
  'resolved_by_ai',
]

describe('web support rulebook is importable and deterministic per view', () => {
  test.each(BUILTIN)('view %s produces stable request filters', (navFilter) => {
    const params = buildConversationListRequestFilters({
      navFilter,
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: defaultConversationListFiltersForNav(navFilter),
    })
    expect(params).toMatchSnapshot()
  })

  test('inbox default targets the shared mailbox', () => {
    const params = buildConversationListRequestFilters({
      navFilter: 'inbox',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: defaultConversationListFiltersForNav('inbox'),
    })
    expect(params).toMatchObject({ mailbox_id: 'shared', filter: 'inbox' })
  })

  test('waiting uses the server waiting filter', () => {
    const params = buildConversationListRequestFilters({
      navFilter: 'waiting',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: defaultConversationListFiltersForNav('waiting'),
    })
    expect(params).toEqual({ filter: 'waiting' })
  })

  test('mine maps to the mine filter', () => {
    const params = buildConversationListRequestFilters({
      navFilter: 'mine',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: defaultConversationListFiltersForNav('mine'),
    })
    expect(params).toMatchObject({ filter: 'mine' })
  })
})
