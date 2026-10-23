import type { NavFilter } from '@/stores/supportInboxStore';

/** The inbox a clear belongs to, as the server records it. */
export interface InboxClearScope {
  scope: 'mailbox' | 'all' | 'mine';
  mailboxId: string | null;
  /** Stable key for matching a celebration to the view on screen. */
  scopeKey: string;
}

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/**
 * Returns the clearable inbox for a built-in view, or null for views that are
 * not an inbox to clear (waiting, resolved, spam, AI views).
 */
export function inboxClearScopeFor(navFilter: NavFilter, selectedMailboxId: string): InboxClearScope | null {
  if (navFilter === 'mine') return { scope: 'mine', mailboxId: null, scopeKey: 'mine' };
  if (navFilter !== 'inbox') return null;
  if (selectedMailboxId === 'all') return { scope: 'all', mailboxId: null, scopeKey: 'all' };
  // Pseudo ids such as "shared" have no mailbox row to count against.
  if (!UUID.test(selectedMailboxId)) return { scope: 'all', mailboxId: null, scopeKey: `mailbox:${selectedMailboxId}` };
  return { scope: 'mailbox', mailboxId: selectedMailboxId, scopeKey: `mailbox:${selectedMailboxId}` };
}

/** Names the inbox for the celebration copy. */
export function inboxClearNames(scope: InboxClearScope, mailboxName: string | null): { inbox: string; inboxInSentence: string } {
  if (scope.scope === 'mine') return { inbox: 'Your queue', inboxInSentence: 'your queue' };
  if (scope.scope === 'mailbox' && mailboxName) return { inbox: mailboxName, inboxInSentence: mailboxName };
  if (scope.scopeKey !== 'all' && mailboxName) return { inbox: mailboxName, inboxInSentence: mailboxName };
  return { inbox: 'Your inbox', inboxInSentence: 'your inbox' };
}

export const OWN_REMOVAL_WINDOW_MS = 30_000;

/**
 * True when the list just went from some conversations to none and every one
 * that left was removed by the viewer's own resolve or spam action. A clear by
 * the AI, a teammate, a filter change or a page load never qualifies.
 */
export function isOwnInboxClear({
  previousIds,
  currentIds,
  ownRemovals,
  now,
  windowMs = OWN_REMOVAL_WINDOW_MS,
}: {
  previousIds: string[];
  currentIds: string[];
  ownRemovals: Record<string, number>;
  now: number;
  windowMs?: number;
}): boolean {
  if (currentIds.length > 0 || previousIds.length === 0) return false;
  return previousIds.every((id) => {
    const removedAt = ownRemovals[id];
    return removedAt !== undefined && now - removedAt <= windowMs;
  });
}
