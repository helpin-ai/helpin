export function shouldClearConversationForMailbox(
  selectedMailboxId: string,
  conversationMailboxId?: string | null,
) {
  return selectedMailboxId !== 'all' && conversationMailboxId !== selectedMailboxId;
}

/**
 * The inbox context to switch to so a routed conversation stays open, or null
 * when the current context already contains it. Falls back to the general
 * inbox ('all') when the conversation has no mailbox or its mailbox is not in
 * the inbox picker.
 */
export function mailboxContextForRoutedConversation(
  selectedMailboxId: string,
  conversationMailboxId: string | null | undefined,
  selectableMailboxes: ReadonlyArray<{ id: string; active?: boolean }>,
): string | null {
  if (!shouldClearConversationForMailbox(selectedMailboxId, conversationMailboxId)) return null;
  const selectable = !!conversationMailboxId &&
    selectableMailboxes.some((mailbox) => mailbox.id === conversationMailboxId && mailbox.active !== false);
  return selectable ? conversationMailboxId : 'all';
}
