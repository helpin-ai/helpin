import {
  buildConversationListRequestFilters,
  defaultConversationListFiltersForNav,
  parseSupportInboxViewFilters,
} from '@/lib/supportInboxFilters'
import type { NavFilter } from '@/stores/supportInboxStore'
import type { ConversationFilters } from '@helpin-ai/support-core'

/**
 * What the user has selected in the views drawer. Display names are carried so
 * the inbox top bar can title itself without a second lookup.
 */
export type ViewSelection =
  | { kind: 'builtin'; navFilter: NavFilter; mailboxId: string }
  | { kind: 'mailbox'; mailboxId: string; mailboxName: string }
  | { kind: 'custom'; viewId: string; name: string; filters: Record<string, string> | undefined }

/** Display labels for builtin views, matching the web sidebar wording. */
export const BUILTIN_VIEW_LABELS: Record<NavFilter, string> = {
  inbox: 'Inbox',
  mine: 'Mine',
  waiting: 'Waiting',
  resolved: 'Resolved',
  spam: 'Spam',
  ai_active: 'AI Handling',
  resolved_by_ai: 'AI Resolved',
}

/** The current selection's display title for the inbox top bar. */
export function selectionTitle(selection: ViewSelection): string {
  switch (selection.kind) {
    case 'builtin':
      return BUILTIN_VIEW_LABELS[selection.navFilter]
    case 'mailbox':
      return selection.mailboxName
    case 'custom':
      return selection.name
  }
}

/**
 * Convert a drawer selection into the exact server query the web app sends for
 * the same view, by reusing the web's own `buildConversationListRequestFilters`.
 * This is the parity guarantee: mobile never hand-derives per-view params.
 */
export function selectionToConversationFilters(selection: ViewSelection): ConversationFilters | undefined {
  switch (selection.kind) {
    case 'builtin':
      return buildConversationListRequestFilters({
        navFilter: selection.navFilter,
        selectedMailboxId: selection.mailboxId,
        searchQuery: '',
        listFilters: defaultConversationListFiltersForNav(selection.navFilter),
      })
    case 'mailbox':
      // A team inbox is the "inbox" view scoped to that mailbox — same as web.
      return buildConversationListRequestFilters({
        navFilter: 'inbox',
        selectedMailboxId: selection.mailboxId,
        searchQuery: '',
        listFilters: defaultConversationListFiltersForNav('inbox'),
      })
    case 'custom': {
      const parsed = parseSupportInboxViewFilters(selection.filters, 'inbox')
      return buildConversationListRequestFilters({
        navFilter: parsed.navFilter,
        selectedMailboxId: parsed.selectedMailboxId,
        searchQuery: parsed.searchQuery,
        listFilters: parsed.listFilters,
      })
    }
  }
}
