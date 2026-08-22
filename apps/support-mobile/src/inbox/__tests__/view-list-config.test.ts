import { describe, expect, test } from 'vitest'
import type { SupportInboxView, SupportInboxViewCount, UnreadStats } from '@helpin-ai/support-core'
import {
  BUILTIN_VIEW_ORDER,
  buildDrawerGroups,
  builtinViewCount,
  isItemActive,
  resolveCustomViewCount,
} from '../view-list-config'

function builtinView(navKey: string, id: string): SupportInboxView {
  return {
    id, workspace_id: 'ws', name: navKey, filters: {}, is_shared: false,
    view_type: 'default', view_key: `nav:${navKey}`, created_by: 'u',
    created_at: '', updated_at: '',
  }
}

const STATS: UnreadStats = {
  total: 0, my_inbox: 0, unassigned: 0,
  inbox: 3, mine: 1, waiting: 0, ai_active: 2,
  inbox_total: 10, mine_total: 4, waiting_total: 5, ai_active_total: 6,
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
  const counts: SupportInboxViewCount[] = [{ view_id: 'v-cust', total_count: 5, unread_count: 2 }]

  test('builtin views source from unread-stats: number = *_total, dot = unread', () => {
    expect(builtinViewCount('inbox', STATS)).toEqual({ total: 10, unread: 3 })
    expect(builtinViewCount('mine', STATS)).toEqual({ total: 4, unread: 1 })
    expect(builtinViewCount('waiting', STATS)).toEqual({ total: 5, unread: 0 })
    expect(builtinViewCount('ai_active', STATS)).toEqual({ total: 6, unread: 2 })
  })
  test('resolved / spam / ai-resolved show no badge (zeros), matching web', () => {
    expect(builtinViewCount('resolved', STATS)).toEqual({ total: 0, unread: 0 })
    expect(builtinViewCount('spam', STATS)).toEqual({ total: 0, unread: 0 })
    expect(builtinViewCount('resolved_by_ai', STATS)).toEqual({ total: 0, unread: 0 })
  })
  test('returns zeros when stats are missing', () => {
    expect(builtinViewCount('inbox', undefined)).toEqual({ total: 0, unread: 0 })
  })
  test('custom view count keyed by its own id (views/counts endpoint)', () => {
    expect(resolveCustomViewCount('v-cust', counts)).toEqual({ total: 5, unread: 2 })
    expect(resolveCustomViewCount('nope', counts)).toEqual({ total: 0, unread: 0 })
  })
})

describe('buildDrawerGroups', () => {
  test('always includes Views + AI, omits empty Custom/Team sections', () => {
    const groups = buildDrawerGroups({ unreadStats: STATS, counts: [], customViews: [] })
    expect(groups.map((g) => g.title)).toEqual(['Views', 'AI'])
    expect(groups[0].items[0]).toMatchObject({ label: 'Inbox', count: { total: 10, unread: 3 } })
  })

  test('adds Custom views and Team inboxes when present', () => {
    const groups = buildDrawerGroups({
      unreadStats: STATS,
      counts: [{ view_id: 'v-cust', total_count: 5, unread_count: 2 }],
      customViews: [{ ...builtinView('x', 'v-cust'), view_key: null, name: 'VIPs' }],
      scopes: {
        shared_inbox: { id: 's', name: 'Shared', handle: '', icon: '', is_shared: true, is_default: true, unread_count: 0, active: true },
        mailboxes: [{ id: 'm1', name: 'Billing', handle: '', icon: '', is_shared: false, is_default: false, unread_count: 7, total_count: 12, active: true }],
      },
    })
    expect(groups.map((g) => g.title)).toEqual(['Views', 'AI', 'Custom views', 'Team inboxes'])
    const custom = groups.find((g) => g.title === 'Custom views')!
    expect(custom.items[0]).toMatchObject({ label: 'VIPs', count: { total: 5, unread: 2 } })
    const team = groups.find((g) => g.title === 'Team inboxes')!
    expect(team.items[0]).toMatchObject({ label: 'Billing', count: { total: 12, unread: 7 } })
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
