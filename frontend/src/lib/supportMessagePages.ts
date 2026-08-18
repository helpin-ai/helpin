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
