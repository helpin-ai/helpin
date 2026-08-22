import type { InfiniteData } from '@tanstack/react-query'

import type { SupportMessage, SupportMessagePage } from '@/lib/pmTypes'

export type SupportMessagePages = InfiniteData<SupportMessagePage, string | undefined>

export function seedSupportMessagePages(messages: SupportMessage[]): SupportMessagePages {
  return {
    pages: [{ data: messages, has_more: false }],
    pageParams: [undefined],
  }
}

export function flattenSupportMessagePages(data?: SupportMessagePages): SupportMessage[] {
  if (!data) return []

  const seen = new Set<string>()
  const messages: SupportMessage[] = []
  for (let pageIndex = data.pages.length - 1; pageIndex >= 0; pageIndex -= 1) {
    for (const message of data.pages[pageIndex].data) {
      if (seen.has(message.id)) continue
      seen.add(message.id)
      messages.push(message)
    }
  }
  return messages
}

export function appendMessageToNewestPage(
  data: SupportMessagePages | undefined,
  message: SupportMessage,
): SupportMessagePages {
  if (!data) return seedSupportMessagePages([message])
  if (data.pages.some((page) => page.data.some((item) => item.id === message.id))) return data

  const clientMessageID = message.client_message_id?.trim()
  if (clientMessageID && data.pages.some((page) => page.data.some((item) =>
    item.id === clientMessageID || item.client_message_id === clientMessageID,
  ))) {
    return {
      ...data,
      pages: data.pages.map((page) => ({
        ...page,
        data: page.data.map((item) =>
          item.id === clientMessageID || item.client_message_id === clientMessageID ? message : item,
        ),
      })),
    }
  }

  // The backend creates this event immediately before a teammate's first
  // reply. The reply itself is optimistic in the UI, though, so its WebSocket
  // event can arrive after that reply is already on screen. Insert the event
  // in front of the pending reply instead of briefly rendering it below the
  // reply and then correcting the order on a later refetch.
  const shouldPrecedeOptimisticReply = message.message_type === 'system'
    && message.system_event_type === 'teammate_joined'
    && !!message.sender_user_id;

  return {
    ...data,
    pages: data.pages.map((page, index) => {
      if (index !== 0) return page;

      const pendingReplyIndex = shouldPrecedeOptimisticReply
        ? page.data.findIndex((item) =>
          (item.id.startsWith('optimistic-') || item.client_message_id?.startsWith('optimistic-'))
          && item.sender_type === 'user'
          && item.sender_user_id === message.sender_user_id
          && !item.is_internal
          && (item.message_type === undefined || item.message_type === 'reply'),
        )
        : -1;

      if (pendingReplyIndex < 0) {
        return { ...page, data: [...page.data, message] };
      }

      return {
        ...page,
        data: [
          ...page.data.slice(0, pendingReplyIndex),
          message,
          ...page.data.slice(pendingReplyIndex),
        ],
      };
    }),
  }
}

export function replaceMessageInPages(
  data: SupportMessagePages | undefined,
  messageID: string,
  replacement: SupportMessage,
): SupportMessagePages | undefined {
  if (!data) return data
  return {
    ...data,
    pages: data.pages.map((page) => ({
      ...page,
      data: page.data.map((message) => message.id === messageID ? replacement : message),
    })),
  }
}

export function removeMessageFromPages(
  data: SupportMessagePages | undefined,
  messageID: string,
): SupportMessagePages | undefined {
  if (!data) return data
  return {
    ...data,
    pages: data.pages.map((page) => ({
      ...page,
      data: page.data.filter((message) => message.id !== messageID),
    })),
  }
}
