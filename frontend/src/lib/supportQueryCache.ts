import type { ConversationListResponse, SupportConversation } from './pmTypes';

export function isSupportConversationListQueryKey(
  queryKey: readonly unknown[],
  workspaceId: string,
): boolean {
  return (
    Array.isArray(queryKey) &&
    queryKey[0] === 'support' &&
    queryKey[1] === workspaceId &&
    queryKey[2] === 'conversations' &&
    (queryKey.length === 3 || typeof queryKey[3] !== 'string')
  );
}

export function updateConversationListUnreadCount(
  current: ConversationListResponse | undefined,
  conversationId: string,
  unreadCount: number,
): ConversationListResponse | undefined {
  if (!current || !Array.isArray(current.data)) {
    return current;
  }

  return {
    ...current,
    data: current.data.map((conversation) =>
      conversation.id === conversationId
        ? { ...conversation, unread_count: unreadCount }
        : conversation,
    ),
  };
}

export function updateConversationUnreadCount(
  current: SupportConversation | undefined,
  unreadCount: number,
): SupportConversation | undefined {
  return current ? { ...current, unread_count: unreadCount } : current;
}
