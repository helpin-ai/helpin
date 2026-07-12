import type { NavFilter } from '@/stores/supportInboxStore'
import type {
  SupportInboxScopeListResponse,
  SupportInboxView,
  SupportInboxViewCount,
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

/**
 * Unread count for a builtin view. Joins nav filter → builtin view record
 * (`view_key === "nav:<filter>"`) → its `id` → the count entry's `view_id`.
 * Matches how the web resolves builtin view counts.
 */
export function resolveViewCount(
  navFilter: NavFilter,
  builtinViews: SupportInboxView[],
  counts: SupportInboxViewCount[],
): number {
  const view = builtinViews.find((v) => v.view_key === `nav:${navFilter}`)
  if (!view) return 0
  return counts.find((c) => c.view_id === view.id)?.unread_count ?? 0
}

/** Unread count for a custom view (its own `id` is the `view_id`). */
export function resolveCustomViewCount(viewId: string, counts: SupportInboxViewCount[]): number {
  return counts.find((c) => c.view_id === viewId)?.unread_count ?? 0
}

export interface DrawerItem {
  key: string
  label: string
  count: number
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
  builtinViews: SupportInboxView[]
  counts: SupportInboxViewCount[]
  customViews: SupportInboxView[]
  scopes?: SupportInboxScopeListResponse
}): DrawerGroup[] {
  const { builtinViews, counts, customViews, scopes } = args

  const itemFor = (navFilter: NavFilter): DrawerItem => ({
    key: `builtin:${navFilter}`,
    label: BUILTIN_VIEW_LABELS[navFilter],
    count: resolveViewCount(navFilter, builtinViews, counts),
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
        count: scope.unread_count,
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
