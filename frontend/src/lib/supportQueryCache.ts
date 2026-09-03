import type { InfiniteData } from '@tanstack/react-query';
import type { ConversationListResponse, ConversationStatus, MessageSenderType, SupportConversation } from './pmTypes';

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
  // Select from the current cached list before invalidation so row actions can advance without a visible jump.
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

export type SupportPersonalReadPatch = {
  conversationId: string;
  unreadCount: number;
  version: number;
};

function patchConversationPersonalRead(
  conversation: SupportConversation,
  patch: SupportPersonalReadPatch,
): SupportConversation {
  if (conversation.id !== patch.conversationId) return conversation;
  if ((conversation.personal_state_version ?? 0) >= patch.version) return conversation;
  return {
    ...conversation,
    unread_count: Math.max(0, patch.unreadCount),
    personal_state_version: patch.version,
  };
}

export function patchConversationPersonalReadInCache(
  current: SupportConversationListCache | undefined,
  patch: SupportPersonalReadPatch,
): SupportConversationListCache | undefined {
  if (!current) return current;
  if (isInfiniteConversationListResponse(current)) {
    return {
      ...current,
      pages: current.pages.map((page) => ({
        ...page,
        data: page.data.map((conversation) => patchConversationPersonalRead(conversation, patch)),
      })),
    };
  }
  if (!isConversationListResponse(current)) return current;
  return {
    ...current,
    data: current.data.map((conversation) => patchConversationPersonalRead(conversation, patch)),
  };
}

export function patchConversationDetailPersonalRead(
  current: SupportConversation | undefined,
  patch: SupportPersonalReadPatch,
): SupportConversation | undefined {
  if (!current) return current;
  return patchConversationPersonalRead(current, patch);
}

type SupportMessageActivityPatch = {
  conversationId: string;
  messageId?: string;
  timestamp: string;
  message?: {
    content?: unknown;
    sender_type?: unknown;
    sender_display_name?: unknown;
    message_type?: unknown;
    system_event_type?: unknown;
  };
};

export type SupportConversationStatusPatch = {
  conversationId: string;
  oldStatus?: ConversationStatus | string | null;
  status: ConversationStatus;
  flowState?: SupportConversation['flow_state'];
  updatedAt?: string;
  mailboxId?: string | null;
};

function isPublicReply(message: SupportMessageActivityPatch['message']): boolean {
  return typeof message?.sender_type === 'string' &&
    (message.message_type === undefined || message.message_type === 'reply') &&
    !message.system_event_type;
}

function conversationAIControlsResponse(conversation: SupportConversation): boolean {
  return !conversation.human_takeover && (
    conversation.flow_state === 'ai_handling' ||
    (!conversation.flow_state && conversation.ai_state === 'pending')
  );
}

function patchConversationForMessageActivity(
  conversation: SupportConversation,
  patch: SupportMessageActivityPatch,
): SupportConversation {
  const next: SupportConversation = {
    ...conversation,
    updated_at: patch.timestamp,
    list_last_message_id: patch.messageId ?? conversation.list_last_message_id,
    list_last_message_at: patch.timestamp,
  };

  if (!isPublicReply(patch.message)) {
    return next;
  }

  const senderType = patch.message?.sender_type as MessageSenderType;
  const content = typeof patch.message?.content === 'string' ? patch.message.content : '';
  const senderDisplayName = typeof patch.message?.sender_display_name === 'string'
    ? patch.message.sender_display_name
    : conversation.last_message_sender_display_name;

  return {
    ...next,
    last_message: content || conversation.last_message,
    last_message_sender_type: senderType,
    last_message_sender_display_name: senderDisplayName,
    awaiting_reply: senderType === 'customer',
    customer_awaiting_response: senderType === 'customer',
    needs_human_reply: senderType === 'customer'
      ? conversation.status !== 'resolved' && conversation.status !== 'spam' && !conversationAIControlsResponse(conversation)
      : false,
    last_customer_message_id: senderType === 'customer' && patch.messageId
      ? patch.messageId
      : conversation.last_customer_message_id,
  };
}

function isMessagePatchNewer(conversation: SupportConversation, patch: SupportMessageActivityPatch): boolean {
  if (!conversation.list_last_message_at) return true;
  const currentTime = Date.parse(conversation.list_last_message_at);
  const patchTime = Date.parse(patch.timestamp);
  if (Number.isFinite(currentTime) && Number.isFinite(patchTime)) {
    if (patchTime !== currentTime) return patchTime > currentTime;
  } else if (patch.timestamp !== conversation.list_last_message_at) {
    return patch.timestamp > conversation.list_last_message_at;
  }
  if (!patch.messageId || !conversation.list_last_message_id) return false;
  return patch.messageId > conversation.list_last_message_id;
}

function moveConversationInList(
  conversations: SupportConversation[] | undefined,
  patch: SupportMessageActivityPatch,
): { conversations: SupportConversation[] | undefined; changed: boolean } {
  if (!Array.isArray(conversations)) {
    return { conversations, changed: false };
  }
  const index = conversations.findIndex((conversation) => conversation.id === patch.conversationId);
  if (index === -1) {
    return { conversations, changed: false };
  }
  if (!isMessagePatchNewer(conversations[index], patch)) {
    return { conversations, changed: false };
  }
  const patched = patchConversationForMessageActivity(conversations[index], patch);
  const next = conversations.slice();
  next.splice(index, 1);
  next.unshift(patched);
  return { conversations: next, changed: true };
}

export function moveConversationToTopForMessageActivity(
  current: SupportConversationListCache | undefined,
  patch: SupportMessageActivityPatch,
): SupportConversationListCache | undefined {
  if (!current) {
    return current;
  }

  if (isInfiniteConversationListResponse(current)) {
    let found: SupportConversation | null = null;
    const pagesWithoutConversation = current.pages.map((page) => {
      if (!Array.isArray(page.data)) {
        return page;
      }
      const index = page.data.findIndex((conversation) => conversation.id === patch.conversationId);
      if (index === -1) {
        return page;
      }
      found = patchConversationForMessageActivity(page.data[index], patch);
      return {
        ...page,
        data: page.data.filter((conversation) => conversation.id !== patch.conversationId),
      };
    });

    if (!found || !pagesWithoutConversation[0] || !Array.isArray(pagesWithoutConversation[0].data)) {
      return current;
    }

    const firstPage = pagesWithoutConversation[0];
    return {
      ...current,
      pages: [
        {
          ...firstPage,
          data: [found, ...firstPage.data],
        },
        ...pagesWithoutConversation.slice(1),
      ],
    };
  }

  if (!isConversationListResponse(current)) {
    return current;
  }

  const result = moveConversationInList(current.data, patch);
  return result.changed ? { ...current, data: result.conversations ?? current.data } : current;
}

function patchConversationForStatus(
  conversation: SupportConversation,
  patch: SupportConversationStatusPatch,
): SupportConversation {
  return {
    ...conversation,
    status: patch.status,
    flow_state: patch.flowState ?? conversation.flow_state,
    mailbox_id: patch.mailboxId === undefined ? conversation.mailbox_id : patch.mailboxId,
    updated_at: patch.updatedAt ?? conversation.updated_at,
  };
}

export function patchConversationStatusInCache(
  current: SupportConversationListCache | undefined,
  patch: SupportConversationStatusPatch,
): SupportConversationListCache | undefined {
  if (!current) {
    return current;
  }

  if (isInfiniteConversationListResponse(current)) {
    let changed = false;
    const pages = current.pages.map((page) => {
      if (!Array.isArray(page.data)) {
        return page;
      }
      let pageChanged = false;
      const data = page.data.map((conversation) => {
        if (conversation.id !== patch.conversationId) {
          return conversation;
        }
        pageChanged = true;
        changed = true;
        return patchConversationForStatus(conversation, patch);
      });
      return pageChanged ? { ...page, data } : page;
    });
    return changed ? { ...current, pages } : current;
  }

  if (!isConversationListResponse(current)) {
    return current;
  }

  let changed = false;
  const data = current.data.map((conversation) => {
    if (conversation.id !== patch.conversationId) {
      return conversation;
    }
    changed = true;
    return patchConversationForStatus(conversation, patch);
  });

  return changed ? { ...current, data } : current;
}

export function patchConversationDetailStatus(
  current: SupportConversation | undefined,
  patch: SupportConversationStatusPatch,
): SupportConversation | undefined {
  if (!current || current.id !== patch.conversationId) {
    return current;
  }
  return patchConversationForStatus(current, patch);
}
