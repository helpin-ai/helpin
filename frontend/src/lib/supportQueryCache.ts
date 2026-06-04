import type { InfiniteData } from '@tanstack/react-query';
import type { ConversationListResponse, SupportConversation } from './pmTypes';

export type SupportConversationListCache =
  | ConversationListResponse
  | InfiniteData<ConversationListResponse>;

export function isSupportConversationListQueryKey(
  queryKey: readonly unknown[],
  workspaceId: string,
): boolean {
  const qualifier = queryKey[3];
  return (
    Array.isArray(queryKey) &&
    queryKey[0] === 'support' &&
    queryKey[1] === workspaceId &&
    queryKey[2] === 'conversations' &&
    (queryKey.length === 3 || qualifier === 'infinite' || typeof qualifier !== 'string')
  );
}

function isConversationListResponse(value: unknown): value is ConversationListResponse {
  return !!value && typeof value === 'object' && Array.isArray((value as ConversationListResponse).data);
}

function isInfiniteConversationListResponse(
  value: SupportConversationListCache | undefined,
): value is InfiniteData<ConversationListResponse> {
  return !!value && typeof value === 'object' && Array.isArray((value as InfiniteData<ConversationListResponse>).pages);
}

export function extractConversationListConversations(
  current: SupportConversationListCache | undefined,
): SupportConversation[] {
  if (!current) {
    return [];
  }
  if (isInfiniteConversationListResponse(current)) {
    return current.pages.flatMap((page) => page.data ?? []);
  }
  if (isConversationListResponse(current)) {
    return current.data;
  }
  return [];
}

export function getConversationListUnreadCount(
  current: SupportConversationListCache | undefined,
  conversationId: string,
): number {
  return extractConversationListConversations(current).find((conversation) => conversation.id === conversationId)?.unread_count ?? 0;
}

export function getNextConversationIdAfterRemoval(
  conversations: Pick<SupportConversation, 'id'>[],
  conversationId: string,
): string | null {
  const currentIdx = conversations.findIndex((conversation) => conversation.id === conversationId);
  if (currentIdx === -1) {
    return conversations[0]?.id ?? null;
  }
  return conversations[currentIdx + 1]?.id ?? conversations[currentIdx - 1]?.id ?? null;
}

export function updateConversationListUnreadCount(
  current: SupportConversationListCache | undefined,
  conversationId: string,
  unreadCount: number,
): SupportConversationListCache | undefined {
  if (!current) {
    return current;
  }

  if (isInfiniteConversationListResponse(current)) {
    let changed = false;
    const nextPages = current.pages.map((page) => {
      if (!Array.isArray(page.data)) {
        return page;
      }
      const nextData = page.data.map((conversation) => {
        if (conversation.id !== conversationId) {
          return conversation;
        }
        if ((conversation.unread_count ?? 0) === unreadCount) {
          return conversation;
        }
        changed = true;
        return { ...conversation, unread_count: unreadCount };
      });
      return changed ? { ...page, data: nextData } : page;
    });
    return changed ? { ...current, pages: nextPages } : current;
  }

  if (!isConversationListResponse(current)) {
    return current;
  }

  let changed = false;
  const nextData = current.data.map((conversation) => {
    if (conversation.id !== conversationId) {
      return conversation;
    }
    if ((conversation.unread_count ?? 0) === unreadCount) {
      return conversation;
    }
    changed = true;
    return { ...conversation, unread_count: unreadCount };
  });

  return changed ? { ...current, data: nextData } : current;
}

export function updateConversationUnreadCount(
  current: SupportConversation | undefined,
  unreadCount: number,
): SupportConversation | undefined {
  if (!current) {
    return current;
  }
  if ((current.unread_count ?? 0) === unreadCount) {
    return current;
  }
  return { ...current, unread_count: unreadCount };
}
