import type { SupportConversation } from '@helpin-ai/support-core'

/** Fixed row height the virtualizer's `estimateSize` must match exactly. */
export const CONVERSATION_CELL_HEIGHT = 80

const MS_PER_MINUTE = 60_000
const MS_PER_HOUR = 60 * MS_PER_MINUTE
const MS_PER_DAY = 24 * MS_PER_HOUR

function startOfDay(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate())
}

/**
 * Short relative timestamp for a conversation's last-activity time, matching
 * the iMessage/Mail-style ladder: "Now" under a minute, minutes, hours,
 * "Yesterday" for the previous calendar day, otherwise a short date (with the
 * year appended when it isn't the current one).
 */
export function formatRelativeTime(iso: string, now: Date = new Date()): string {
  const date = new Date(iso)
  const diffMs = now.getTime() - date.getTime()

  if (diffMs < MS_PER_MINUTE) return 'Now'
  if (diffMs < MS_PER_HOUR) return `${Math.floor(diffMs / MS_PER_MINUTE)}m`
  if (diffMs < MS_PER_DAY) return `${Math.floor(diffMs / MS_PER_HOUR)}h`

  const dayDiff = Math.round((startOfDay(now).getTime() - startOfDay(date).getTime()) / MS_PER_DAY)
  if (dayDiff === 1) return 'Yesterday'

  const sameYear = date.getFullYear() === now.getFullYear()
  return new Intl.DateTimeFormat('en-US', {
    month: 'short',
    day: 'numeric',
    year: sameYear ? undefined : 'numeric',
  }).format(date)
}

const HTML_TAG_RE = /<[^>]*>/g
const WHITESPACE_RE = /\s+/g

/** Plain-text preview of a conversation's last message, or a fallback when there isn't one. */
export function previewText(conversation: SupportConversation): string {
  const raw = conversation.last_message ?? ''
  const stripped = raw.replace(HTML_TAG_RE, ' ').replace(WHITESPACE_RE, ' ').trim()
  return stripped || 'No messages yet'
}

/** Single source of truth for a conversation's unread visual state. */
export function isUnread(conversation: Pick<SupportConversation, 'unread_count'>): boolean {
  return (conversation.unread_count ?? 0) > 0
}
