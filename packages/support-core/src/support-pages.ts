import type { InfiniteData } from '@tanstack/react-query'

import type {
  ConversationListResponse,
  SupportConversation,
  SupportMessage,
  SupportMessagePage,
} from './support-types'

export type ConversationListPages = InfiniteData<ConversationListResponse, number>
export type SupportMessagePages = InfiniteData<SupportMessagePage, string | undefined>

export function flattenConversationPages(data?: ConversationListPages): SupportConversation[] {
  if (!data) return []

  const seen = new Set<string>()
  const conversations: SupportConversation[] = []
  for (const page of data.pages) {
    for (const conversation of page.data) {
      if (seen.has(conversation.id)) continue
      seen.add(conversation.id)
      conversations.push(conversation)
    }
  }
  return conversations
}

export function flattenSupportMessagePages(data?: SupportMessagePages): SupportMessage[] {
  if (!data) return []

  const seen = new Set<string>()
  const messages: SupportMessage[] = []
  // Page zero is the newest page. Older pages are appended to InfiniteData,
  // so walk backward to restore chronological thread order.
  for (let pageIndex = data.pages.length - 1; pageIndex >= 0; pageIndex -= 1) {
    for (const message of data.pages[pageIndex].data) {
      if (seen.has(message.id)) continue
      seen.add(message.id)
      messages.push(message)
    }
  }
  return messages
}

export function seedSupportMessagePages(messages: SupportMessage[]): SupportMessagePages {
  return {
    pages: [{ data: messages, has_more: false }],
    pageParams: [undefined],
  }
}

export function appendMessageToNewestPage(
  data: SupportMessagePages | undefined,
  message: SupportMessage,
): SupportMessagePages {
  if (!data) return seedSupportMessagePages([message])
  if (data.pages.some((page) => page.data.some((item) => item.id === message.id))) return data

  return {
    ...data,
    pages: data.pages.map((page, index) =>
      index === 0 ? { ...page, data: [...page.data, message] } : page,
    ),
  }
}

export function replaceMessageInPages(
  data: SupportMessagePages | undefined,
  messageId: string,
  replacement: SupportMessage,
): SupportMessagePages | undefined {
  if (!data) return data
  return {
    ...data,
    pages: data.pages.map((page) => ({
      ...page,
      data: page.data.map((message) => (message.id === messageId ? replacement : message)),
    })),
  }
}
