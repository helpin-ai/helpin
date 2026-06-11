import type { ConversationListResponse, SupportConversation } from './support-types'

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
  )
}

export function updateConversationListUnreadCount(
  current: ConversationListResponse | undefined,
  conversationId: string,
  unreadCount: number,
): ConversationListResponse | undefined {
  if (!current || !Array.isArray(current.data)) {
    return current
  }

  let changed = false
  const nextData = current.data.map((conversation) => {
    if (conversation.id !== conversationId) {
      return conversation
    }
    if ((conversation.unread_count ?? 0) === unreadCount) {
      return conversation
    }
    changed = true
    return { ...conversation, unread_count: unreadCount }
  })

  if (!changed) {
    return current
  }

  return {
    ...current,
    data: nextData,
  }
}

export function updateConversationUnreadCount(
  current: SupportConversation | undefined,
  unreadCount: number,
): SupportConversation | undefined {
  if (!current) {
    return current
  }
  if ((current.unread_count ?? 0) === unreadCount) {
    return current
  }
  return { ...current, unread_count: unreadCount }
}
