import type { InfiniteData } from '@tanstack/react-query'

import type { SupportMessage, SupportMessagePage } from '@/lib/pmTypes'

export type SupportMessagePages = InfiniteData<SupportMessagePage, string | undefined>

function messageMetadataString(message: SupportMessage, key: string): string | null {
  if (!message.metadata) return null
  try {
    const value = JSON.parse(message.metadata)?.[key]
    return typeof value === 'string' && value.trim() ? value.trim() : null
  } catch {
    return null
  }
}

export function supportMessageClientID(message: SupportMessage): string | null {
  return message.client_message_id?.trim() || messageMetadataString(message, 'client_message_id')
}

export function supportMessageRenderKey(message: SupportMessage): string {
  const clientID = supportMessageClientID(message)
  return clientID ? `client:${clientID}` : message.id
}

export function joinedReplyClientMessageID(message: SupportMessage): string | null {
  return message.message_type === 'system' && message.system_event_type === 'teammate_joined'
    ? messageMetadataString(message, 'reply_client_message_id') : null
}

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
  // Realtime rows arrive independently while the reply may already be visible
  // optimistically. Apply the same join/reply order as the saved history, using
  // explicit correlation rather than names, text, or the client's clock.
  const replyIDs = new Set(messages.filter((message) => message.message_type !== 'system')
    .map(supportMessageClientID).filter(Boolean))
  const joins = new Map<string, SupportMessage[]>()
  for (const message of messages) {
    const replyID = joinedReplyClientMessageID(message)
    if (replyID && replyIDs.has(replyID)) joins.set(replyID, [...(joins.get(replyID) ?? []), message])
  }
  if (joins.size === 0) return messages
  return messages.flatMap((message) => {
    const replyID = joinedReplyClientMessageID(message)
    if (replyID && joins.has(replyID)) return []
    return [...(joins.get(supportMessageClientID(message) ?? '') ?? []), message]
  })
}

export function appendMessageToNewestPage(
  data: SupportMessagePages | undefined,
  message: SupportMessage,
): SupportMessagePages {
  if (!data) return seedSupportMessagePages([message])
  if (data.pages.some((page) => page.data.some((item) => item.id === message.id))) return data

  const clientMessageID = supportMessageClientID(message)
  if (clientMessageID && data.pages.some((page) => page.data.some((item) =>
    item.id === clientMessageID || supportMessageClientID(item) === clientMessageID,
  ))) {
    return {
      ...data,
      pages: data.pages.map((page) => ({
        ...page,
        data: page.data.map((item) =>
          item.id === clientMessageID || supportMessageClientID(item) === clientMessageID ? { ...item, ...message } : item,
        ),
      })),
    }
  }

  return {
    ...data,
    pages: data.pages.map((page, index) => index === 0
      ? { ...page, data: [...page.data, message] }
      : page),
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
