import { describe, expect, test } from 'vitest'
import type { SupportInboxView, SupportInboxViewCount } from '@helpin-ai/support-core'
import {
  BUILTIN_VIEW_ORDER,
  buildDrawerGroups,
  isItemActive,
  resolveCustomViewCount,
  resolveViewCount,
} from '../view-list-config'

function builtinView(navKey: string, id: string): SupportInboxView {
  return {
    id, workspace_id: 'ws', name: navKey, filters: {}, is_shared: false,
    view_type: 'default', view_key: `nav:${navKey}`, created_by: 'u',
    created_at: '', updated_at: '',
  }
}

describe('builtin order + grouping', () => {
  test('matches the web sidebar order', () => {
    expect(BUILTIN_VIEW_ORDER.map((v) => v.navFilter)).toEqual(
      ['inbox', 'mine', 'waiting', 'resolved', 'spam', 'ai_active', 'resolved_by_ai'])
  })
  test('ai group is exactly the two AI views', () => {
    expect(BUILTIN_VIEW_ORDER.filter((v) => v.group === 'ai').map((v) => v.navFilter))
      .toEqual(['ai_active', 'resolved_by_ai'])
  })
})

describe('count resolution', () => {
  const builtins = [builtinView('inbox', 'v-inbox'), builtinView('waiting', 'v-waiting')]
  const counts: SupportInboxViewCount[] = [
    { view_id: 'v-inbox', total_count: 10, unread_count: 3 },
    { view_id: 'v-cust', total_count: 5, unread_count: 2 },
  ]
  test('joins view_key -> id -> view_id, returning total + unread', () => {
    expect(resolveViewCount('inbox', builtins, counts)).toEqual({ total: 10, unread: 3 })
  })
  test('returns zeros when the view or its count is missing', () => {
    expect(resolveViewCount('mine', builtins, counts)).toEqual({ total: 0, unread: 0 })
    expect(resolveViewCount('waiting', builtins, counts)).toEqual({ total: 0, unread: 0 })
  })
  test('custom view count keyed by its own id', () => {
    expect(resolveCustomViewCount('v-cust', counts)).toEqual({ total: 5, unread: 2 })
    expect(resolveCustomViewCount('nope', counts)).toEqual({ total: 0, unread: 0 })
  })
})

describe('buildDrawerGroups', () => {
  const builtins = [builtinView('inbox', 'v-inbox')]
  const counts: SupportInboxViewCount[] = [{ view_id: 'v-inbox', total_count: 10, unread_count: 3 }]

  test('always includes Views + AI, omits empty Custom/Team sections', () => {
    const groups = buildDrawerGroups({ builtinViews: builtins, counts, customViews: [] })
    expect(groups.map((g) => g.title)).toEqual(['Views', 'AI'])
    expect(groups[0].items[0]).toMatchObject({ label: 'Inbox', count: { total: 10, unread: 3 } })
  })

  test('adds Custom views and Team inboxes when present', () => {
    const groups = buildDrawerGroups({
      builtinViews: builtins,
      counts,
      customViews: [{ ...builtinView('x', 'v-cust'), view_key: null, name: 'VIPs' }],
      scopes: {
        shared_inbox: { id: 's', name: 'Shared', handle: '', icon: '', is_shared: true, is_default: true, unread_count: 0, active: true },
        mailboxes: [{ id: 'm1', name: 'Billing', handle: '', icon: '', is_shared: false, is_default: false, unread_count: 7, active: true }],
      },
    })
    expect(groups.map((g) => g.title)).toEqual(['Views', 'AI', 'Custom views', 'Team inboxes'])
    const team = groups.find((g) => g.title === 'Team inboxes')!
    expect(team.items[0]).toMatchObject({ label: 'Billing', count: { total: 7, unread: 7 } })
    expect(team.items[0].selection).toMatchObject({ kind: 'mailbox', mailboxId: 'm1' })
  })
})

describe('isItemActive', () => {
  test('matches builtin by navFilter and mailbox by id', () => {
    const inboxItem = { key: 'builtin:inbox', label: 'Inbox', count: { total: 0, unread: 0 }, selection: { kind: 'builtin' as const, navFilter: 'inbox' as const, mailboxId: 'all' } }
    expect(isItemActive(inboxItem, { kind: 'builtin', navFilter: 'inbox', mailboxId: 'all' })).toBe(true)
    expect(isItemActive(inboxItem, { kind: 'builtin', navFilter: 'mine', mailboxId: 'all' })).toBe(false)
  })
})
