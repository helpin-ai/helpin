/** Inbox ownership is selected with team_inbox; mailbox_ids only filters a list. */
export function emailForwardingInboxHref(
  workspaceSlug: string | undefined,
  mailboxId?: string | null,
  conversationId?: string | null,
): string | null {
  if (!workspaceSlug) return null;
  const search = new URLSearchParams({ view: mailboxId ? 'team' : 'inbox' });
  if (mailboxId) search.set('team_inbox', mailboxId);
  // A previously resolved confirmation must remain visible, and saved view
  // filters must not hide a conversation opened directly from setup.
  if (conversationId) search.set('states', 'open,waiting_on_customer,resolved,spam');
  const conversationPath = conversationId ? `/${encodeURIComponent(conversationId)}` : '';
  return `/w/${encodeURIComponent(workspaceSlug)}/support${conversationPath}?${search}`;
}
