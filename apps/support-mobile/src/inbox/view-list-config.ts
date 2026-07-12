import type { NavFilter } from '@/stores/supportInboxStore'
import type {
  SupportInboxScopeListResponse,
  SupportInboxView,
  SupportInboxViewCount,
  UnreadStats,
} from '@helpin-ai/support-core'
import { BUILTIN_VIEW_LABELS, type ViewSelection } from './use-inbox-filters'

/**
 * Builtin views in the exact order and grouping of the web sidebar:
 * Inbox · Mine · Waiting · Resolved · Spam (the "views" group), then
 * AI Handling · AI Resolved (the "ai" group).
 */
export const BUILTIN_VIEW_ORDER: { navFilter: NavFilter; group: 'views' | 'ai' }[] = [
  { navFilter: 'inbox', group: 'views' },
  { navFilter: 'mine', group: 'views' },
  { navFilter: 'waiting', group: 'views' },
  { navFilter: 'resolved', group: 'views' },
  { navFilter: 'spam', group: 'views' },
  { navFilter: 'ai_active', group: 'ai' },
  { navFilter: 'resolved_by_ai', group: 'ai' },
]

/** A view's badge data: `total` is the number shown; `unread > 0` drives the red dot (mirrors web). */
export interface ViewCount {
  total: number
  unread: number
}

const ZERO_COUNT: ViewCount = { total: 0, unread: 0 }

function countFor(viewId: string | undefined, counts: SupportInboxViewCount[]): ViewCount {
  const entry = viewId ? counts.find((c) => c.view_id === viewId) : undefined
  if (!entry) return ZERO_COUNT
  return { total: entry.total_count ?? 0, unread: entry.unread_count ?? 0 }
}

/**
 * Count for a builtin view, sourced exactly like the web sidebar: Inbox / Mine /
 * Waiting / AI Handling come from the unread-stats endpoint (number = `*_total`
 * workload, dot = unread). Resolved / Spam / AI Resolved intentionally show NO
 * badge — web omits them too (their `total` is undefined there).
 */
export function builtinViewCount(navFilter: NavFilter, stats?: UnreadStats): ViewCount {
  if (!stats) return ZERO_COUNT
  switch (navFilter) {
    case 'inbox':
      return { total: stats.inbox_total, unread: stats.inbox }
    case 'mine':
      return { total: stats.mine_total, unread: stats.mine }
    case 'waiting':
      return { total: stats.waiting_total, unread: stats.waiting }
    case 'ai_active':
      return { total: stats.ai_active_total, unread: stats.ai_active }
    default:
      return ZERO_COUNT
  }
}

/** Count for a custom view (its own `id` is the `view_id`). */
export function resolveCustomViewCount(viewId: string, counts: SupportInboxViewCount[]): ViewCount {
  return countFor(viewId, counts)
}

export interface DrawerItem {
  key: string
  label: string
  count: ViewCount
  selection: ViewSelection
}

export interface DrawerGroup {
  title: string
  items: DrawerItem[]
}

/**
 * Assemble the four drawer sections (Views / AI / Custom views / Team inboxes)
 * from the fetched data. Empty sections (no custom views / no team inboxes) are
 * omitted. Builtin views are scoped to all inboxes (`mailboxId: 'all'`); team
 * inboxes are the "inbox" view scoped to that mailbox, with their own unread
 * counts sourced from inbox scopes.
 */
export function buildDrawerGroups(args: {
  unreadStats?: UnreadStats
  counts: SupportInboxViewCount[]
  customViews: SupportInboxView[]
  scopes?: SupportInboxScopeListResponse
}): DrawerGroup[] {
  const { unreadStats, counts, customViews, scopes } = args

  const itemFor = (navFilter: NavFilter): DrawerItem => ({
    key: `builtin:${navFilter}`,
    label: BUILTIN_VIEW_LABELS[navFilter],
    count: builtinViewCount(navFilter, unreadStats),
    selection: { kind: 'builtin', navFilter, mailboxId: 'all' },
  })

  const groups: DrawerGroup[] = [
    {
      title: 'Views',
      items: BUILTIN_VIEW_ORDER.filter((v) => v.group === 'views').map((v) => itemFor(v.navFilter)),
    },
    {
      title: 'AI',
      items: BUILTIN_VIEW_ORDER.filter((v) => v.group === 'ai').map((v) => itemFor(v.navFilter)),
    },
  ]

  if (customViews.length > 0) {
    groups.push({
      title: 'Custom views',
      items: customViews.map((view) => ({
        key: `custom:${view.id}`,
        label: view.name,
        count: resolveCustomViewCount(view.id, counts),
        selection: {
          kind: 'custom',
          viewId: view.id,
          name: view.name,
          filters: (view.filters ?? undefined) as Record<string, string> | undefined,
        },
      })),
    })
  }

  const teamInboxes = scopes?.mailboxes ?? []
  if (teamInboxes.length > 0) {
    groups.push({
      title: 'Team inboxes',
      items: teamInboxes.map((scope) => ({
        key: `mailbox:${scope.id}`,
        label: scope.name,
        count: { total: scope.total_count ?? scope.unread_count, unread: scope.unread_count },
        selection: { kind: 'mailbox', mailboxId: scope.id, mailboxName: scope.name },
      })),
    })
  }

  return groups
}

/** Whether a drawer item is the currently active selection (for highlighting). */
export function isItemActive(item: DrawerItem, active: ViewSelection): boolean {
  switch (item.selection.kind) {
    case 'builtin':
      return active.kind === 'builtin' && active.navFilter === item.selection.navFilter
    case 'mailbox':
      return active.kind === 'mailbox' && active.mailboxId === item.selection.mailboxId
    case 'custom':
      return active.kind === 'custom' && active.viewId === item.selection.viewId
  }
}
