import type { ConversationListPages } from './support-pages'
import type { ConversationListResponse, SupportConversation } from './support-types'

export type ConversationListCache = ConversationListResponse | ConversationListPages

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

function isConversationListPages(value: ConversationListCache): value is ConversationListPages {
  return 'pages' in value && Array.isArray(value.pages)
}

export function findConversationInListCache(
  current: ConversationListCache | undefined,
  conversationId: string,
): SupportConversation | undefined {
  if (!current) return undefined
  if (isConversationListPages(current)) {
    for (const page of current.pages) {
      const conversation = page.data.find((item) => item.id === conversationId)
      if (conversation) return conversation
    }
    return undefined
  }
  return current.data.find((conversation) => conversation.id === conversationId)
}

function updateConversationPageUnreadCount(
  current: ConversationListResponse,
  conversationId: string,
  unreadCount: number,
): ConversationListResponse {
  let changed = false
  const nextData = current.data.map((conversation) => {
    if (conversation.id !== conversationId) return conversation
    if ((conversation.unread_count ?? 0) === unreadCount) return conversation
    changed = true
    return { ...conversation, unread_count: unreadCount }
  })
  return changed ? { ...current, data: nextData } : current
}

export function updateConversationListUnreadCount(
  current: ConversationListCache | undefined,
  conversationId: string,
  unreadCount: number,
): ConversationListCache | undefined {
  if (!current) return current
  if (isConversationListPages(current)) {
    let changed = false
    const pages = current.pages.map((page) => {
      const nextPage = updateConversationPageUnreadCount(page, conversationId, unreadCount)
      if (nextPage !== page) changed = true
      return nextPage
    })
    return changed ? { ...current, pages } : current
  }
  return updateConversationPageUnreadCount(current, conversationId, unreadCount)
}

export function updateConversationUnreadCount(
  current: SupportConversation | undefined,
  unreadCount: number,
): SupportConversation | undefined {
  if (!current) return current
  if ((current.unread_count ?? 0) === unreadCount) return current
  return { ...current, unread_count: unreadCount }
}
