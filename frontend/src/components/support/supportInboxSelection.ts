export function shouldClearConversationForMailbox(
  selectedMailboxId: string,
  conversationMailboxId?: string | null,
) {
  return selectedMailboxId !== 'all' && conversationMailboxId !== selectedMailboxId;
}
